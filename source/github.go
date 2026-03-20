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

// graphqlSearchNode wraps a PR response from a GraphQL search query,
// adding the repository field that search returns but individual PR queries don't.
type graphqlSearchNode struct {
	graphqlPRResponse
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
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
	full.Labels = append(full.Labels, g.Labels.Nodes...)

	return full
}

var (
	cachedAccounts []string
	accountsOnce   sync.Once
)

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

// searchAuthoredPRs finds open PRs authored by a specific account using a
// single GraphQL search query that returns all fields in one round-trip.
func searchAuthoredPRs(account string) ([]model.PullRequest, error) {
	tok := ghTokenForUser(account)
	if tok == "" {
		return nil, fmt.Errorf("no token for %s", account)
	}

	// Single GraphQL search query — fetches everything we need in one call,
	// replacing the old two-step search + enrich pipeline.
	query := fmt.Sprintf(`{
  search(query: "author:%s is:pr is:open", type: ISSUE, first: 50) {
    nodes {
      ... on PullRequest {
        %s
        repository { nameWithOwner }
      }
    }
  }
}`, account, prGraphQLFragment)

	cmd := exec.Command("gh", "api", "graphql", "-f", "query="+query) //nolint:gosec // gh CLI path is trusted
	cmd.Env = append(os.Environ(), "GH_TOKEN="+tok)

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
		return nil, fmt.Errorf("graphql search: %s", errMsg)
	}

	var resp struct {
		Data struct {
			Search struct {
				Nodes []graphqlSearchNode `json:"nodes"`
			} `json:"search"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("graphql search parse: %w", err)
	}

	if len(resp.Errors) > 0 {
		for _, e := range resp.Errors {
			fmt.Fprintf(os.Stderr, "  ⚠ graphql search: %s\n", e.Message)
		}
	}

	nodes := resp.Data.Search.Nodes
	if len(nodes) == 0 {
		return nil, nil
	}

	prs := make([]model.PullRequest, 0, len(nodes))
	for _, n := range nodes {
		if n.URL == "" {
			continue // skip empty/null nodes
		}
		full := graphqlPRToFull(&n.graphqlPRResponse)
		pr := model.PullRequest{
			Number:           full.Number,
			Title:            full.Title,
			HeadBranch:       full.HeadRefName,
			Repo:             n.Repository.NameWithOwner,
			URL:              full.URL,
			State:            full.State,
			IsDraft:          full.IsDraft,
			Body:             full.Body,
			CIStatus:         deriveCIStatus(full.StatusCheckRollup),
			ReviewState:      deriveReviewState(full.ReviewDecision),
			Additions:        full.Additions,
			Deletions:        full.Deletions,
			ChangedFiles:     full.ChangedFiles,
			Mergeable:        full.Mergeable,
			MergeStateStatus: full.MergeStateStatus,
			Comments:         len(full.Comments),
			ReviewRequests:   len(full.ReviewRequests),
		}
		for _, l := range full.Labels {
			pr.Labels = append(pr.Labels, l.Name)
		}
		if t, err := time.Parse(time.RFC3339, full.CreatedAt); err == nil {
			pr.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, full.UpdatedAt); err == nil {
			pr.UpdatedAt = t
		}
		prs = append(prs, pr)
	}

	return prs, nil
}

func deriveCIStatus(checks []ciCheck) string {
	if len(checks) == 0 {
		return ""
	}
	hasFailure := false
	allSuccess := true
	for _, check := range checks {
		s := strings.ToUpper(check.State)
		if s == model.CheckStateFailure || s == model.CheckStateError {
			hasFailure = true
			allSuccess = false
		} else if s != model.CheckStateSuccess {
			allSuccess = false
		}
	}
	if hasFailure {
		return model.CIFailing
	}
	if allSuccess {
		return model.CIPassing
	}
	return model.CIPending
}

func deriveReviewState(decision string) string {
	switch decision {
	case "APPROVED":
		return model.ReviewApproved
	case "CHANGES_REQUESTED":
		return model.ReviewChangesRequested
	case "REVIEW_REQUIRED":
		return model.ReviewRequired
	}
	return ""
}
