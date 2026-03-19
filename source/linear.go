package source

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xxE6E6FA/nxt/model"
)

const linearAPI = "https://api.linear.app/graphql"

const issuesQuery = `{
  viewer {
    assignedIssues(
      filter: {
        state: { type: { nin: ["completed", "canceled"] } }
      }
      first: 50
      orderBy: updatedAt
    ) {
      nodes {
        id
        identifier
        title
        branchName
        url
        priority
        dueDate
        state { name }
        cycle { id }
      }
    }
  }
}`

type linearResponse struct {
	Data struct {
		Viewer struct {
			AssignedIssues struct {
				Nodes []struct {
					ID         string  `json:"id"`
					Identifier string  `json:"identifier"`
					Title      string  `json:"title"`
					BranchName string  `json:"branchName"`
					URL        string  `json:"url"`
					Priority   int     `json:"priority"`
					DueDate    *string `json:"dueDate"`
					State      struct {
						Name string `json:"name"`
					} `json:"state"`
					Cycle *struct {
						ID string `json:"id"`
					} `json:"cycle"`
				} `json:"nodes"`
			} `json:"assignedIssues"`
		} `json:"viewer"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// FetchLinearIssues retrieves issues assigned to the authenticated user.
func FetchLinearIssues(apiKey string) ([]model.LinearIssue, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("linear API key not configured")
	}

	body, _ := json.Marshal(map[string]string{"query": issuesQuery})
	req, err := http.NewRequest("POST", linearAPI, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("linear API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("linear API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var lr linearResponse
	if err := json.Unmarshal(respBody, &lr); err != nil {
		return nil, fmt.Errorf("failed to parse linear response: %w", err)
	}

	if len(lr.Errors) > 0 {
		return nil, fmt.Errorf("linear API error: %s", lr.Errors[0].Message)
	}

	var issues []model.LinearIssue
	for _, n := range lr.Data.Viewer.AssignedIssues.Nodes {
		issue := model.LinearIssue{
			ID:         n.ID,
			Identifier: n.Identifier,
			Title:      n.Title,
			Status:     n.State.Name,
			Priority:   n.Priority,
			BranchName: n.BranchName,
			URL:        n.URL,
		}
		if n.DueDate != nil {
			t, err := time.Parse("2006-01-02", *n.DueDate)
			if err == nil {
				issue.DueDate = &t
			}
		}
		if n.Cycle != nil {
			issue.CycleID = n.Cycle.ID
			issue.InCycle = true
		}
		issues = append(issues, issue)
	}

	return issues, nil
}
