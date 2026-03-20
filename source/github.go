package source

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/xxE6E6FA/nxt/model"
)

type ghSearchResult struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	State      string `json:"state"`
	IsDraft    bool   `json:"isDraft"`
	Body       string `json:"body"`
	UpdatedAt  string `json:"updatedAt"`
	Repository struct {
		Name          string `json:"name"`
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

type ciCheck struct {
	State string `json:"state"`
}

type ghPRFull struct {
	Number            int       `json:"number"`
	Title             string    `json:"title"`
	HeadRefName       string    `json:"headRefName"`
	URL               string    `json:"url"`
	State             string    `json:"state"`
	IsDraft           bool      `json:"isDraft"`
	Body              string    `json:"body"`
	UpdatedAt         string    `json:"updatedAt"`
	ReviewDecision    string    `json:"reviewDecision"`
	StatusCheckRollup []ciCheck `json:"statusCheckRollup"`
}

const prViewFields = "number,title,headRefName,url,state,isDraft,body,updatedAt,reviewDecision,statusCheckRollup"

// ghAccountTokens caches resolved tokens per GitHub account.
var ghAccountTokens = map[string]string{}
var ghAccountTokensMu sync.Mutex

// tokenForRepo returns a GH_TOKEN that can access the given repo owner.
func tokenForRepo(repoOwner string) string {
	ghAccountTokensMu.Lock()
	if tok, ok := ghAccountTokens[repoOwner]; ok {
		ghAccountTokensMu.Unlock()
		return tok
	}
	ghAccountTokensMu.Unlock()

	accounts := ghAccounts()
	for _, acct := range accounts {
		tok := ghTokenForUser(acct)
		if tok == "" {
			continue
		}
		cmd := exec.Command("gh", "api", fmt.Sprintf("orgs/%s", repoOwner), "--silent")
		cmd.Env = append(os.Environ(), "GH_TOKEN="+tok)
		if err := cmd.Run(); err == nil {
			ghAccountTokensMu.Lock()
			ghAccountTokens[repoOwner] = tok
			ghAccountTokensMu.Unlock()
			return tok
		}
		cmd = exec.Command("gh", "api", fmt.Sprintf("users/%s", repoOwner), "--silent")
		cmd.Env = append(os.Environ(), "GH_TOKEN="+tok)
		if err := cmd.Run(); err == nil {
			ghAccountTokensMu.Lock()
			ghAccountTokens[repoOwner] = tok
			ghAccountTokensMu.Unlock()
			return tok
		}
	}

	return ""
}

var cachedAccounts []string
var accountsOnce sync.Once

func ghAccounts() []string {
	accountsOnce.Do(func() {
		cmd := exec.Command("gh", "auth", "status")
		out, _ := cmd.CombinedOutput()
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "✓ Logged in to") && strings.Contains(line, "account") {
				parts := strings.Fields(line)
				for i, p := range parts {
					if p == "account" && i+1 < len(parts) {
						acct := strings.TrimSuffix(parts[i+1], "(keyring)")
						acct = strings.TrimSpace(acct)
						cachedAccounts = append(cachedAccounts, acct)
					}
				}
			}
		}
	})
	return cachedAccounts
}

func ghTokenForUser(username string) string {
	cmd := exec.Command("gh", "auth", "token", "--user", username)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ghCmd(repoOwner string, args ...string) *exec.Cmd {
	cmd := exec.Command("gh", args...)
	if tok := tokenForRepo(repoOwner); tok != "" {
		cmd.Env = append(os.Environ(), "GH_TOKEN="+tok)
	}
	return cmd
}

func isPRURL(url string) bool {
	return strings.Contains(url, "/pull/")
}

// FetchPullRequests discovers open PRs from two sources in parallel:
// 1. PRs linked from Linear issues (fetched by URL, concurrently)
// 2. PRs authored by the current user (gh search, per account, concurrently)
// Returns deduplicated results.
func FetchPullRequests(prURLs []string) ([]model.PullRequest, error) {
	var mu sync.Mutex
	seen := make(map[string]bool)
	var allPRs []model.PullRequest

	// Deduplicate and filter URLs upfront
	var uniqueURLs []string
	for _, url := range prURLs {
		if isPRURL(url) && !seen[url] {
			seen[url] = true
			uniqueURLs = append(uniqueURLs, url)
		}
	}
	// Reset seen — we used it just for dedup above
	seen = make(map[string]bool)

	var wg sync.WaitGroup

	// Source 1: Fetch all Linear-linked PRs concurrently (bounded to 8)
	sem := make(chan struct{}, 8)
	for _, url := range uniqueURLs {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			pr, err := fetchPRByURL(u)
			if err != nil {
				return
			}
			mu.Lock()
			if !seen[pr.URL] {
				seen[pr.URL] = true
				allPRs = append(allPRs, *pr)
			}
			mu.Unlock()
		}(url)
	}

	// Source 2: Search authored PRs per account concurrently
	for _, acct := range ghAccounts() {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			authored, err := searchAuthoredPRs(a)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ github search (%s): %v\n", a, err)
				return
			}
			mu.Lock()
			for i := range authored {
				if !seen[authored[i].URL] {
					seen[authored[i].URL] = true
					allPRs = append(allPRs, authored[i])
				}
			}
			mu.Unlock()
		}(acct)
	}

	wg.Wait()
	return allPRs, nil
}

func fetchPRByURL(url string) (*model.PullRequest, error) {
	owner := ownerFromURL(url)
	cmd := ghCmd(owner, "pr", "view", url, "--json", prViewFields)

	out, err := cmd.Output()
	if err != nil {
		return nil, wrapExecErr(err)
	}

	var g ghPRFull
	if err := json.Unmarshal(out, &g); err != nil {
		return nil, err
	}

	return fullToPR(&g, url), nil
}

// searchAuthoredPRs finds open PRs authored by a specific account.
// The search itself returns basic info; we enrich each PR concurrently.
func searchAuthoredPRs(account string) ([]model.PullRequest, error) {
	tok := ghTokenForUser(account)
	if tok == "" {
		return nil, fmt.Errorf("no token for %s", account)
	}

	searchCmd := exec.Command("gh", "search", "prs",
		"--author=@me",
		"--state=open",
		"--limit=50",
		"--json", "number,title,url,state,isDraft,body,updatedAt,repository",
	)
	searchCmd.Env = append(os.Environ(), "GH_TOKEN="+tok)

	searchOut, err := searchCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh search prs: %w", wrapExecErr(err))
	}

	var results []ghSearchResult
	if err := json.Unmarshal(searchOut, &results); err != nil {
		return nil, fmt.Errorf("failed to parse gh search output: %w", err)
	}

	// Enrich all PRs concurrently
	prs := make([]model.PullRequest, len(results))
	var wg sync.WaitGroup

	for i, r := range results {
		prs[i] = model.PullRequest{
			Number:  r.Number,
			Title:   r.Title,
			Repo:    r.Repository.NameWithOwner,
			URL:     r.URL,
			State:   r.State,
			IsDraft: r.IsDraft,
			Body:    r.Body,
		}
		if t, err := time.Parse(time.RFC3339, r.UpdatedAt); err == nil {
			prs[i].UpdatedAt = t
		}

		wg.Add(1)
		go func(idx int, repo, url string, num int) {
			defer wg.Done()
			owner := ownerFromURL(url)
			detail, err := fetchPRDetailByRepo(owner, repo, num)
			if err != nil {
				return
			}
			prs[idx].HeadBranch = detail.HeadRefName
			prs[idx].CIStatus = deriveCIStatus(detail.StatusCheckRollup)
			prs[idx].ReviewState = deriveReviewState(detail.ReviewDecision)
		}(i, r.Repository.NameWithOwner, r.URL, r.Number)
	}

	wg.Wait()
	return prs, nil
}

func fetchPRDetailByRepo(repoOwner, repo string, number int) (*ghPRFull, error) {
	cmd := ghCmd(repoOwner, "pr", "view",
		fmt.Sprintf("%d", number),
		"--repo", repo,
		"--json", prViewFields,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, wrapExecErr(err)
	}

	var detail ghPRFull
	if err := json.Unmarshal(out, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func fullToPR(g *ghPRFull, originalURL string) *model.PullRequest {
	pr := &model.PullRequest{
		Number:      g.Number,
		Title:       g.Title,
		HeadBranch:  g.HeadRefName,
		URL:         g.URL,
		State:       g.State,
		IsDraft:     g.IsDraft,
		Body:        g.Body,
		CIStatus:    deriveCIStatus(g.StatusCheckRollup),
		ReviewState: deriveReviewState(g.ReviewDecision),
	}

	if pr.Repo == "" {
		pr.Repo = repoFromURL(originalURL)
	}
	if pr.URL == "" {
		pr.URL = originalURL
	}
	if t, err := time.Parse(time.RFC3339, g.UpdatedAt); err == nil {
		pr.UpdatedAt = t
	}

	return pr
}

func ownerFromURL(url string) string {
	url = strings.TrimPrefix(url, "https://github.com/")
	parts := strings.Split(url, "/")
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

func repoFromURL(url string) string {
	url = strings.TrimPrefix(url, "https://github.com/")
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return ""
}

func deriveCIStatus(checks []ciCheck) string {
	if len(checks) == 0 {
		return ""
	}
	hasFailure := false
	allSuccess := true
	for _, check := range checks {
		s := strings.ToUpper(check.State)
		if s == "FAILURE" || s == "ERROR" {
			hasFailure = true
			allSuccess = false
		} else if s != "SUCCESS" {
			allSuccess = false
		}
	}
	if hasFailure {
		return "failing"
	}
	if allSuccess {
		return "passing"
	}
	return "pending"
}

func deriveReviewState(decision string) string {
	switch decision {
	case "APPROVED":
		return "approved"
	case "CHANGES_REQUESTED":
		return "changes_requested"
	case "REVIEW_REQUIRED":
		return "review_required"
	}
	return ""
}

func wrapExecErr(err error) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
	}
	return err
}
