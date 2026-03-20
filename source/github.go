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

type ghLabel struct {
	Name string `json:"name"`
}

type ghReviewRequest struct {
	Login string `json:"login"`
	Slug  string `json:"slug"`
}

type ghPRFull struct {
	Number            int               `json:"number"`
	Title             string            `json:"title"`
	HeadRefName       string            `json:"headRefName"`
	URL               string            `json:"url"`
	State             string            `json:"state"`
	IsDraft           bool              `json:"isDraft"`
	Body              string            `json:"body"`
	CreatedAt         string            `json:"createdAt"`
	UpdatedAt         string            `json:"updatedAt"`
	Additions         int               `json:"additions"`
	Deletions         int               `json:"deletions"`
	ChangedFiles      int               `json:"changedFiles"`
	Mergeable         string            `json:"mergeable"`
	MergeStateStatus  string            `json:"mergeStateStatus"`
	ReviewDecision    string            `json:"reviewDecision"`
	StatusCheckRollup []ciCheck         `json:"statusCheckRollup"`
	Comments          []json.RawMessage `json:"comments"`
	ReviewRequests    []ghReviewRequest `json:"reviewRequests"`
	Labels            []ghLabel         `json:"labels"`
}

const prViewFields = "number,title,headRefName,url,state,isDraft,body,createdAt,updatedAt,additions,deletions,changedFiles,mergeable,mergeStateStatus,reviewDecision,statusCheckRollup,comments,reviewRequests,labels"

// prGraphQLFragment contains the GraphQL fields to fetch for each PR.
const prGraphQLFragment = `
  number title headRefName url state isDraft body
  createdAt updatedAt additions deletions changedFiles
  mergeable mergeStateStatus reviewDecision
  commits(last: 1) {
    nodes {
      commit {
        statusCheckRollup {
          contexts(first: 100) {
            nodes {
              ... on StatusContext { state }
              ... on CheckRun { conclusion }
            }
          }
        }
      }
    }
  }
  comments { totalCount }
  reviewRequests(first: 10) {
    nodes {
      requestedReviewer {
        ... on User { login }
        ... on Team { slug }
      }
    }
  }
  labels(first: 20) { nodes { name } }
`

// prRepoKey groups PRs by owner/name for batched fetching.
type prRepoKey struct {
	Owner string
	Name  string
}

// graphqlPRResponse mirrors the GraphQL response shape for a single PR node.
type graphqlPRResponse struct {
	Number           int    `json:"number"`
	Title            string `json:"title"`
	HeadRefName      string `json:"headRefName"`
	URL              string `json:"url"`
	State            string `json:"state"`
	IsDraft          bool   `json:"isDraft"`
	Body             string `json:"body"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	ChangedFiles     int    `json:"changedFiles"`
	Mergeable        string `json:"mergeable"`
	MergeStateStatus string `json:"mergeStateStatus"`
	ReviewDecision   string `json:"reviewDecision"`
	Commits          struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					Contexts struct {
						Nodes []struct {
							State      string `json:"state"`
							Conclusion string `json:"conclusion"`
						} `json:"nodes"`
					} `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	Comments struct {
		TotalCount int `json:"totalCount"`
	} `json:"comments"`
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer struct {
				Login string `json:"login"`
				Slug  string `json:"slug"`
			} `json:"requestedReviewer"`
		} `json:"nodes"`
	} `json:"reviewRequests"`
	Labels struct {
		Nodes []ghLabel `json:"nodes"`
	} `json:"labels"`
}

// graphqlPRToFull converts a GraphQL PR response to the existing ghPRFull type.
func graphqlPRToFull(g *graphqlPRResponse) *ghPRFull {
	full := &ghPRFull{
		Number:           g.Number,
		Title:            g.Title,
		HeadRefName:      g.HeadRefName,
		URL:              g.URL,
		State:            g.State,
		IsDraft:          g.IsDraft,
		Body:             g.Body,
		CreatedAt:        g.CreatedAt,
		UpdatedAt:        g.UpdatedAt,
		Additions:        g.Additions,
		Deletions:        g.Deletions,
		ChangedFiles:     g.ChangedFiles,
		Mergeable:        g.Mergeable,
		MergeStateStatus: g.MergeStateStatus,
		ReviewDecision:   g.ReviewDecision,
	}

	// Extract CI checks from commits
	if len(g.Commits.Nodes) > 0 {
		commit := g.Commits.Nodes[0].Commit
		if commit.StatusCheckRollup != nil {
			for _, ctx := range commit.StatusCheckRollup.Contexts.Nodes {
				state := ctx.State
				if state == "" {
					state = ctx.Conclusion
				}
				full.StatusCheckRollup = append(full.StatusCheckRollup, ciCheck{State: state})
			}
		}
	}

	// Convert comments totalCount to a slice of the right length
	full.Comments = make([]json.RawMessage, g.Comments.TotalCount)

	// Convert review requests
	for _, rr := range g.ReviewRequests.Nodes {
		full.ReviewRequests = append(full.ReviewRequests, ghReviewRequest{
			Login: rr.RequestedReviewer.Login,
			Slug:  rr.RequestedReviewer.Slug,
		})
	}

	// Convert labels
	for _, l := range g.Labels.Nodes {
		full.Labels = append(full.Labels, l)
	}

	return full
}

// batchFetchPRsByGraphQL fetches multiple PRs across repos in a single GraphQL call.
// prsByRepo maps prRepoKey to a slice of PR numbers.
// Returns a map from "owner/name#number" to the fetched ghPRFull.
func batchFetchPRsByGraphQL(token string, prsByRepo map[prRepoKey][]int) (map[string]*ghPRFull, error) {
	if len(prsByRepo) == 0 {
		return nil, nil
	}

	// Build a single GraphQL query with aliases for each repo and PR
	var queryParts []string
	// Track alias -> repo key + number for mapping results back
	type aliasInfo struct {
		repoAlias string
		prAlias   string
		owner     string
		name      string
		number    int
	}
	var aliases []aliasInfo

	repoIdx := 0
	for key, numbers := range prsByRepo {
		repoAlias := fmt.Sprintf("repo%d", repoIdx)
		var prParts []string
		for prIdx, num := range numbers {
			prAlias := fmt.Sprintf("pr%d_%d", repoIdx, prIdx)
			prParts = append(prParts, fmt.Sprintf("    %s: pullRequest(number: %d) { %s }", prAlias, num, prGraphQLFragment))
			aliases = append(aliases, aliasInfo{
				repoAlias: repoAlias,
				prAlias:   prAlias,
				owner:     key.Owner,
				name:      key.Name,
				number:    num,
			})
		}
		queryParts = append(queryParts, fmt.Sprintf("  %s: repository(owner: %q, name: %q) {\n%s\n  }",
			repoAlias, key.Owner, key.Name, strings.Join(prParts, "\n")))
		repoIdx++
	}

	query := fmt.Sprintf("query {\n%s\n}", strings.Join(queryParts, "\n"))

	cmd := exec.Command("gh", "api", "graphql", "-f", "query="+query)
	cmd.Env = append(os.Environ(), "GH_TOKEN="+token)

	// Capture stdout even on non-zero exit — GraphQL returns partial data with errors
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	out := []byte(stdout.String())
	if len(out) == 0 {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = "empty response"
		}
		return nil, fmt.Errorf("graphql batch fetch: %s", errMsg)
	}

	// Parse the response: { "data": { "repo0": { "pr0_0": { ... }, "pr0_1": { ... } }, "repo1": { ... } } }
	var resp struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("graphql parse response: %w", err)
	}

	// Log partial errors but continue — we'll extract whatever data came back
	if len(resp.Errors) > 0 {
		for _, e := range resp.Errors {
			fmt.Fprintf(os.Stderr, "  ⚠ graphql partial: %s\n", e.Message)
		}
	}

	results := make(map[string]*ghPRFull)

	for _, ai := range aliases {
		repoData, ok := resp.Data[ai.repoAlias]
		if !ok || string(repoData) == "null" {
			continue
		}

		var prMap map[string]json.RawMessage
		if err := json.Unmarshal(repoData, &prMap); err != nil {
			continue
		}

		prData, ok := prMap[ai.prAlias]
		if !ok {
			continue
		}

		var gqlPR graphqlPRResponse
		if err := json.Unmarshal(prData, &gqlPR); err != nil {
			continue
		}

		full := graphqlPRToFull(&gqlPR)
		key := fmt.Sprintf("%s/%s#%d", ai.owner, ai.name, ai.number)
		results[key] = full
	}

	return results, nil
}

// ghTokenCache caches resolved tokens per cache key (owner or owner/repo).
var ghTokenCache = map[string]string{}
var ghTokenCacheMu sync.Mutex

// tokenForRepo returns a GH_TOKEN that can access the given repo owner.
// This is a fast check (org/user level) used when we don't know the specific repo.
func tokenForRepo(repoOwner string) string {
	return tokenLookup(repoOwner, func(tok string) bool {
		cmd := exec.Command("gh", "api", fmt.Sprintf("orgs/%s", repoOwner), "--silent")
		cmd.Env = append(os.Environ(), "GH_TOKEN="+tok)
		if err := cmd.Run(); err == nil {
			return true
		}
		cmd = exec.Command("gh", "api", fmt.Sprintf("users/%s", repoOwner), "--silent")
		cmd.Env = append(os.Environ(), "GH_TOKEN="+tok)
		return cmd.Run() == nil
	})
}

func tokenLookup(cacheKey string, check func(tok string) bool) string {
	ghTokenCacheMu.Lock()
	if tok, ok := ghTokenCache[cacheKey]; ok {
		ghTokenCacheMu.Unlock()
		return tok
	}
	ghTokenCacheMu.Unlock()

	accounts := ghAccounts()
	for _, acct := range accounts {
		tok := ghTokenForUser(acct)
		if tok == "" {
			continue
		}
		if check(tok) {
			ghTokenCacheMu.Lock()
			ghTokenCache[cacheKey] = tok
			ghTokenCacheMu.Unlock()
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

// IsPRURL returns true if the URL points to a GitHub pull request (not a commit, issue, etc).
func IsPRURL(url string) bool {
	return strings.Contains(url, "/pull/")
}

// FetchAuthoredPRs searches for open PRs authored by the current user
// across all authenticated GitHub accounts. This does not depend on
// Linear data and can run in parallel with the Linear fetch.
func FetchAuthoredPRs() ([]model.PullRequest, error) {
	var mu sync.Mutex
	var allPRs []model.PullRequest
	var wg sync.WaitGroup

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
			allPRs = append(allPRs, authored...)
			mu.Unlock()
		}(acct)
	}

	wg.Wait()
	return allPRs, nil
}

// searchAuthoredPRs finds open PRs authored by a specific account.
// The search itself returns basic info; we enrich PRs using a batched GraphQL query.
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

	if len(results) == 0 {
		return nil, nil
	}

	// Build basic PRs from search results
	prs := make([]model.PullRequest, len(results))
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
	}

	// Group PRs by repo for batched GraphQL fetch
	prsByRepo := make(map[prRepoKey][]int)
	for _, r := range results {
		parts := strings.SplitN(r.Repository.NameWithOwner, "/", 2)
		if len(parts) != 2 {
			continue
		}
		key := prRepoKey{Owner: parts[0], Name: parts[1]}
		prsByRepo[key] = append(prsByRepo[key], r.Number)
	}

	// Batch fetch all PR details via GraphQL
	details, err := batchFetchPRsByGraphQL(tok, prsByRepo)
	if err != nil {
		// Fallback: enrich individually
		fmt.Fprintf(os.Stderr, "  ⚠ graphql batch failed, falling back to individual fetches: %v\n", err)
		enrichPRsIndividually(prs, results)
		return prs, nil
	}

	// Apply enriched details to PRs
	for i, r := range results {
		lookupKey := fmt.Sprintf("%s#%d", r.Repository.NameWithOwner, r.Number)
		detail, ok := details[lookupKey]
		if !ok {
			continue
		}
		prs[i].HeadBranch = detail.HeadRefName
		prs[i].CIStatus = deriveCIStatus(detail.StatusCheckRollup)
		prs[i].ReviewState = deriveReviewState(detail.ReviewDecision)
		prs[i].Additions = detail.Additions
		prs[i].Deletions = detail.Deletions
		prs[i].ChangedFiles = detail.ChangedFiles
		prs[i].Mergeable = detail.Mergeable
		prs[i].MergeStateStatus = detail.MergeStateStatus
		prs[i].Comments = len(detail.Comments)
		prs[i].ReviewRequests = len(detail.ReviewRequests)
		for _, l := range detail.Labels {
			prs[i].Labels = append(prs[i].Labels, l.Name)
		}
		if t, err := time.Parse(time.RFC3339, detail.CreatedAt); err == nil {
			prs[i].CreatedAt = t
		}
	}

	return prs, nil
}

// enrichPRsIndividually falls back to per-PR subprocess calls when GraphQL batching fails.
func enrichPRsIndividually(prs []model.PullRequest, results []ghSearchResult) {
	var wg sync.WaitGroup
	for i, r := range results {
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
			prs[idx].Additions = detail.Additions
			prs[idx].Deletions = detail.Deletions
			prs[idx].ChangedFiles = detail.ChangedFiles
			prs[idx].Mergeable = detail.Mergeable
			prs[idx].MergeStateStatus = detail.MergeStateStatus
			prs[idx].Comments = len(detail.Comments)
			prs[idx].ReviewRequests = len(detail.ReviewRequests)
			for _, l := range detail.Labels {
				prs[idx].Labels = append(prs[idx].Labels, l.Name)
			}
			if t, err := time.Parse(time.RFC3339, detail.CreatedAt); err == nil {
				prs[idx].CreatedAt = t
			}
		}(i, r.Repository.NameWithOwner, r.URL, r.Number)
	}
	wg.Wait()
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
		Number:           g.Number,
		Title:            g.Title,
		HeadBranch:       g.HeadRefName,
		URL:              g.URL,
		State:            g.State,
		IsDraft:          g.IsDraft,
		Body:             g.Body,
		CIStatus:         deriveCIStatus(g.StatusCheckRollup),
		ReviewState:      deriveReviewState(g.ReviewDecision),
		Additions:        g.Additions,
		Deletions:        g.Deletions,
		ChangedFiles:     g.ChangedFiles,
		Mergeable:        g.Mergeable,
		MergeStateStatus: g.MergeStateStatus,
		Comments:         len(g.Comments),
		ReviewRequests:   len(g.ReviewRequests),
	}

	for _, l := range g.Labels {
		pr.Labels = append(pr.Labels, l.Name)
	}

	if pr.Repo == "" {
		pr.Repo = repoFromURL(originalURL)
	}
	if pr.URL == "" {
		pr.URL = originalURL
	}
	if t, err := time.Parse(time.RFC3339, g.CreatedAt); err == nil {
		pr.CreatedAt = t
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
