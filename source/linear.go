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
        createdAt
        updatedAt
        startedAt
        estimate
        slaBreachesAt
        snoozedUntilAt
        sortOrder
        labels { nodes { name } }
        state { name }
        cycle {
          id
          startsAt
          endsAt
        }
        attachments(filter: { sourceType: { eq: "github" } }) {
          nodes {
            url
          }
        }
      }
    }
  }
}`

type linearResponse struct {
	Data struct {
		Viewer struct {
			AssignedIssues struct {
				Nodes []struct {
					ID             string   `json:"id"`
					Identifier     string   `json:"identifier"`
					Title          string   `json:"title"`
					BranchName     string   `json:"branchName"`
					URL            string   `json:"url"`
					Priority       int      `json:"priority"`
					DueDate        *string  `json:"dueDate"`
					CreatedAt      string   `json:"createdAt"`
					UpdatedAt      string   `json:"updatedAt"`
					StartedAt      *string  `json:"startedAt"`
					Estimate       *float64 `json:"estimate"`
					SLABreachesAt  *string  `json:"slaBreachesAt"`
					SnoozedUntilAt *string  `json:"snoozedUntilAt"`
					SortOrder      float64  `json:"sortOrder"`
					Labels         struct {
						Nodes []struct {
							Name string `json:"name"`
						} `json:"nodes"`
					} `json:"labels"`
					State struct {
						Name string `json:"name"`
					} `json:"state"`
					Cycle *struct {
						ID       string  `json:"id"`
						StartsAt *string `json:"startsAt"`
						EndsAt   *string `json:"endsAt"`
					} `json:"cycle"`
					Attachments struct {
						Nodes []struct {
							URL string `json:"url"`
						} `json:"nodes"`
					} `json:"attachments"`
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
			Estimate:   n.Estimate,
			SortOrder:  n.SortOrder,
		}
		if n.DueDate != nil {
			t, err := time.Parse("2006-01-02", *n.DueDate)
			if err == nil {
				issue.DueDate = &t
			}
		}
		if t, err := time.Parse(time.RFC3339, n.CreatedAt); err == nil {
			issue.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, n.UpdatedAt); err == nil {
			issue.UpdatedAt = t
		}
		if n.StartedAt != nil {
			if t, err := time.Parse(time.RFC3339, *n.StartedAt); err == nil {
				issue.StartedAt = &t
			}
		}
		if n.SLABreachesAt != nil {
			if t, err := time.Parse(time.RFC3339, *n.SLABreachesAt); err == nil {
				issue.SLABreachesAt = &t
			}
		}
		if n.SnoozedUntilAt != nil {
			if t, err := time.Parse(time.RFC3339, *n.SnoozedUntilAt); err == nil {
				issue.SnoozedUntilAt = &t
			}
		}
		for _, l := range n.Labels.Nodes {
			issue.Labels = append(issue.Labels, l.Name)
		}
		if n.Cycle != nil {
			issue.CycleID = n.Cycle.ID
			issue.InCycle = true
			issue.CycleStartDate = parseDatePtr("2006-01-02", n.Cycle.StartsAt)
			issue.CycleEndDate = parseDatePtr("2006-01-02", n.Cycle.EndsAt)
		}
		for _, att := range n.Attachments.Nodes {
			if att.URL != "" {
				issue.PRURLs = append(issue.PRURLs, att.URL)
			}
		}
		issues = append(issues, issue)
	}

	return issues, nil
}

// parseDatePtr parses a date string pointer, returning nil on nil input or parse error.
func parseDatePtr(layout string, s *string) *time.Time {
	if s == nil {
		return nil
	}
	t, err := time.Parse(layout, *s)
	if err != nil {
		return nil
	}
	return &t
}
