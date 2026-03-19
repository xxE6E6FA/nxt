package source

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/xxE6E6FA/nxt/model"
)

type ghPR struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	HeadRefName string `json:"headRefName"`
	URL         string `json:"url"`
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	Body        string `json:"body"`
	UpdatedAt   string `json:"updatedAt"`
	StatusCheckRollup []struct {
		State string `json:"state"`
	} `json:"statusCheckRollup"`
	ReviewDecision string `json:"reviewDecision"`
}

// FetchPullRequests retrieves open PRs authored by the current user for the given repos.
func FetchPullRequests(repos []string) ([]model.PullRequest, error) {
	var allPRs []model.PullRequest

	for _, repo := range repos {
		prs, err := fetchRepoPRs(repo)
		if err != nil {
			// Log but continue — partial data is fine
			fmt.Printf("  ⚠ github: %s: %v\n", repo, err)
			continue
		}
		allPRs = append(allPRs, prs...)
	}

	return allPRs, nil
}

func fetchRepoPRs(repo string) ([]model.PullRequest, error) {
	cmd := exec.Command("gh", "pr", "list",
		"--repo", repo,
		"--author", "@me",
		"--state", "open",
		"--json", "number,title,headRefName,url,state,isDraft,body,updatedAt,statusCheckRollup,reviewDecision",
	)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}

	var ghPRs []ghPR
	if err := json.Unmarshal(out, &ghPRs); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	var prs []model.PullRequest
	for _, g := range ghPRs {
		pr := model.PullRequest{
			Number:     g.Number,
			Title:      g.Title,
			HeadBranch: g.HeadRefName,
			Repo:       repo,
			URL:        g.URL,
			State:      g.State,
			IsDraft:    g.IsDraft,
			Body:       g.Body,
		}

		if t, err := time.Parse(time.RFC3339, g.UpdatedAt); err == nil {
			pr.UpdatedAt = t
		}

		// Derive CI status from statusCheckRollup
		if len(g.StatusCheckRollup) > 0 {
			hasFailure := false
			allSuccess := true
			for _, check := range g.StatusCheckRollup {
				s := strings.ToUpper(check.State)
				if s == "FAILURE" || s == "ERROR" {
					hasFailure = true
					allSuccess = false
				} else if s != "SUCCESS" {
					allSuccess = false
				}
			}
			if hasFailure {
				pr.CIStatus = "failing"
			} else if allSuccess {
				pr.CIStatus = "passing"
			} else {
				pr.CIStatus = "pending"
			}
		}

		// Review state
		switch g.ReviewDecision {
		case "APPROVED":
			pr.ReviewState = "approved"
		case "CHANGES_REQUESTED":
			pr.ReviewState = "changes_requested"
		case "REVIEW_REQUIRED":
			pr.ReviewState = "review_required"
		}

		prs = append(prs, pr)
	}

	return prs, nil
}
