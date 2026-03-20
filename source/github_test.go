package source

import (
	"encoding/json"
	"testing"

	"github.com/xxE6E6FA/nxt/model"
)

func TestDeriveCIStatus(t *testing.T) {
	tests := []struct {
		name   string
		checks []ciCheck
		want   string
	}{
		{"empty checks", nil, ""},
		{"all SUCCESS", []ciCheck{{State: model.CheckStateSuccess}, {State: model.CheckStateSuccess}}, model.CIPassing},
		{"single SUCCESS", []ciCheck{{State: model.CheckStateSuccess}}, model.CIPassing},
		{"one FAILURE among successes", []ciCheck{{State: model.CheckStateSuccess}, {State: model.CheckStateFailure}}, model.CIFailing},
		{"one ERROR among successes", []ciCheck{{State: model.CheckStateSuccess}, {State: model.CheckStateError}}, model.CIFailing},
		{"mix SUCCESS and PENDING", []ciCheck{{State: model.CheckStateSuccess}, {State: "PENDING"}}, model.CIPending},
		{"single PENDING", []ciCheck{{State: "PENDING"}}, model.CIPending},
		{"lowercase failure", []ciCheck{{State: "failure"}}, model.CIFailing},
		{"mixed case Failure", []ciCheck{{State: "Failure"}}, model.CIFailing},
		{"uppercase FAILURE", []ciCheck{{State: model.CheckStateFailure}}, model.CIFailing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveCIStatus(tt.checks)
			if got != tt.want {
				t.Errorf("deriveCIStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveReviewState(t *testing.T) {
	tests := []struct {
		name     string
		decision string
		want     string
	}{
		{"APPROVED", "APPROVED", model.ReviewApproved},
		{"CHANGES_REQUESTED", "CHANGES_REQUESTED", model.ReviewChangesRequested},
		{"REVIEW_REQUIRED", "REVIEW_REQUIRED", model.ReviewRequired},
		{"empty string", "", ""},
		{"unknown value", "SOMETHING_ELSE", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveReviewState(tt.decision)
			if got != tt.want {
				t.Errorf("deriveReviewState(%q) = %q, want %q", tt.decision, got, tt.want)
			}
		})
	}
}

func TestIsPRURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"valid PR URL", "https://github.com/org/repo/pull/123", true},
		{"issue URL", "https://github.com/org/repo/issues/123", false},
		{"commit URL", "https://github.com/org/repo/commit/abc", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPRURL(tt.url)
			if got != tt.want {
				t.Errorf("IsPRURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestGraphqlPRToFull(t *testing.T) {
	t.Run("all fields populated", func(t *testing.T) {
		g := &graphqlPRResponse{
			Number:           42,
			Title:            "Fix the thing",
			HeadRefName:      "fix-thing",
			URL:              "https://github.com/org/repo/pull/42",
			State:            "OPEN",
			IsDraft:          true,
			Body:             "This fixes the thing.",
			CreatedAt:        "2025-01-01T00:00:00Z",
			UpdatedAt:        "2025-01-02T00:00:00Z",
			Additions:        10,
			Deletions:        5,
			ChangedFiles:     3,
			Mergeable:        "MERGEABLE",
			MergeStateStatus: "CLEAN",
			ReviewDecision:   "APPROVED",
		}

		// Set up commits with status checks
		g.Commits.Nodes = append(g.Commits.Nodes, struct {
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
		}{})
		g.Commits.Nodes[0].Commit.StatusCheckRollup = &struct {
			Contexts struct {
				Nodes []struct {
					State      string `json:"state"`
					Conclusion string `json:"conclusion"`
				} `json:"nodes"`
			} `json:"contexts"`
		}{}
		g.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes = []struct {
			State      string `json:"state"`
			Conclusion string `json:"conclusion"`
		}{
			{State: model.CheckStateSuccess},
			{Conclusion: model.CheckStateSuccess},
		}

		// Set up comments
		g.Comments.TotalCount = 7

		// Set up review requests
		g.ReviewRequests.Nodes = []struct {
			RequestedReviewer struct {
				Login string `json:"login"`
				Slug  string `json:"slug"`
			} `json:"requestedReviewer"`
		}{
			{RequestedReviewer: struct {
				Login string `json:"login"`
				Slug  string `json:"slug"`
			}{Login: "alice"}},
			{RequestedReviewer: struct {
				Login string `json:"login"`
				Slug  string `json:"slug"`
			}{Slug: "team-a"}},
		}

		// Set up labels
		g.Labels.Nodes = []ghLabel{{Name: "bug"}, {Name: "urgent"}}

		full := graphqlPRToFull(g)

		if full.Number != 42 {
			t.Errorf("Number = %d, want 42", full.Number)
		}
		if full.Title != "Fix the thing" {
			t.Errorf("Title = %q, want %q", full.Title, "Fix the thing")
		}
		if full.HeadRefName != "fix-thing" {
			t.Errorf("HeadRefName = %q, want %q", full.HeadRefName, "fix-thing")
		}
		if full.URL != "https://github.com/org/repo/pull/42" {
			t.Errorf("URL = %q", full.URL)
		}
		if full.State != "OPEN" {
			t.Errorf("State = %q, want OPEN", full.State)
		}
		if !full.IsDraft {
			t.Error("IsDraft = false, want true")
		}
		if full.Additions != 10 {
			t.Errorf("Additions = %d, want 10", full.Additions)
		}
		if full.Deletions != 5 {
			t.Errorf("Deletions = %d, want 5", full.Deletions)
		}
		if full.ChangedFiles != 3 {
			t.Errorf("ChangedFiles = %d, want 3", full.ChangedFiles)
		}
		if full.Mergeable != "MERGEABLE" {
			t.Errorf("Mergeable = %q", full.Mergeable)
		}
		if full.MergeStateStatus != "CLEAN" {
			t.Errorf("MergeStateStatus = %q", full.MergeStateStatus)
		}
		if full.ReviewDecision != "APPROVED" {
			t.Errorf("ReviewDecision = %q", full.ReviewDecision)
		}

		// CI checks: 2 checks extracted
		if len(full.StatusCheckRollup) != 2 {
			t.Fatalf("StatusCheckRollup len = %d, want 2", len(full.StatusCheckRollup))
		}
		if full.StatusCheckRollup[0].State != model.CheckStateSuccess {
			t.Errorf("check[0].State = %q, want SUCCESS", full.StatusCheckRollup[0].State)
		}
		// Second check uses Conclusion since State is empty
		if full.StatusCheckRollup[1].State != model.CheckStateSuccess {
			t.Errorf("check[1].State = %q, want SUCCESS", full.StatusCheckRollup[1].State)
		}

		// Comments
		if len(full.Comments) != 7 {
			t.Errorf("Comments len = %d, want 7", len(full.Comments))
		}

		// Review requests
		if len(full.ReviewRequests) != 2 {
			t.Fatalf("ReviewRequests len = %d, want 2", len(full.ReviewRequests))
		}
		if full.ReviewRequests[0].Login != "alice" {
			t.Errorf("ReviewRequests[0].Login = %q, want alice", full.ReviewRequests[0].Login)
		}
		if full.ReviewRequests[1].Slug != "team-a" {
			t.Errorf("ReviewRequests[1].Slug = %q, want team-a", full.ReviewRequests[1].Slug)
		}

		// Labels
		if len(full.Labels) != 2 {
			t.Fatalf("Labels len = %d, want 2", len(full.Labels))
		}
		if full.Labels[0].Name != "bug" {
			t.Errorf("Labels[0].Name = %q, want bug", full.Labels[0].Name)
		}
		if full.Labels[1].Name != "urgent" {
			t.Errorf("Labels[1].Name = %q, want urgent", full.Labels[1].Name)
		}
	})

	t.Run("no status checks", func(t *testing.T) {
		g := &graphqlPRResponse{
			Number: 1,
			Title:  "No CI",
		}
		full := graphqlPRToFull(g)
		if len(full.StatusCheckRollup) != 0 {
			t.Errorf("StatusCheckRollup len = %d, want 0", len(full.StatusCheckRollup))
		}
	})

	t.Run("nil StatusCheckRollup", func(t *testing.T) {
		g := &graphqlPRResponse{
			Number: 2,
			Title:  "Nil rollup",
		}
		// Add a commit node but leave StatusCheckRollup nil
		g.Commits.Nodes = append(g.Commits.Nodes, struct {
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
		}{})

		full := graphqlPRToFull(g)
		if len(full.StatusCheckRollup) != 0 {
			t.Errorf("StatusCheckRollup len = %d, want 0", len(full.StatusCheckRollup))
		}
	})

	t.Run("multiple CI checks with mixed state and conclusion", func(t *testing.T) {
		g := &graphqlPRResponse{Number: 3}
		g.Commits.Nodes = append(g.Commits.Nodes, struct {
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
		}{})
		g.Commits.Nodes[0].Commit.StatusCheckRollup = &struct {
			Contexts struct {
				Nodes []struct {
					State      string `json:"state"`
					Conclusion string `json:"conclusion"`
				} `json:"nodes"`
			} `json:"contexts"`
		}{}
		g.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes = []struct {
			State      string `json:"state"`
			Conclusion string `json:"conclusion"`
		}{
			{State: model.CheckStateSuccess},
			{State: "", Conclusion: model.CheckStateFailure},
			{State: "PENDING"},
		}

		full := graphqlPRToFull(g)
		if len(full.StatusCheckRollup) != 3 {
			t.Fatalf("StatusCheckRollup len = %d, want 3", len(full.StatusCheckRollup))
		}
		if full.StatusCheckRollup[0].State != model.CheckStateSuccess {
			t.Errorf("check[0] = %q, want SUCCESS", full.StatusCheckRollup[0].State)
		}
		if full.StatusCheckRollup[1].State != model.CheckStateFailure {
			t.Errorf("check[1] = %q, want FAILURE (from conclusion)", full.StatusCheckRollup[1].State)
		}
		if full.StatusCheckRollup[2].State != "PENDING" {
			t.Errorf("check[2] = %q, want PENDING", full.StatusCheckRollup[2].State)
		}
	})
}

func TestGraphqlPRToFullViaJSON(t *testing.T) {
	// Test via JSON unmarshalling to verify struct tags work end-to-end.
	jsonData := `{
		"number": 99,
		"title": "JSON test",
		"headRefName": "json-branch",
		"url": "https://github.com/org/repo/pull/99",
		"state": "OPEN",
		"isDraft": false,
		"body": "body text",
		"createdAt": "2025-06-01T00:00:00Z",
		"updatedAt": "2025-06-02T00:00:00Z",
		"additions": 1,
		"deletions": 2,
		"changedFiles": 1,
		"mergeable": "MERGEABLE",
		"mergeStateStatus": "CLEAN",
		"reviewDecision": "APPROVED",
		"commits": {
			"nodes": [{
				"commit": {
					"statusCheckRollup": {
						"contexts": {
							"nodes": [
								{"state": "SUCCESS", "conclusion": ""},
								{"state": "", "conclusion": "SUCCESS"}
							]
						}
					}
				}
			}]
		},
		"comments": {"totalCount": 3},
		"reviewRequests": {
			"nodes": [{"requestedReviewer": {"login": "bob", "slug": ""}}]
		},
		"labels": {"nodes": [{"name": "enhancement"}]}
	}`

	var g graphqlPRResponse
	if err := json.Unmarshal([]byte(jsonData), &g); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	full := graphqlPRToFull(&g)
	if full.Number != 99 {
		t.Errorf("Number = %d, want 99", full.Number)
	}
	if len(full.StatusCheckRollup) != 2 {
		t.Errorf("StatusCheckRollup len = %d, want 2", len(full.StatusCheckRollup))
	}
	if len(full.Comments) != 3 {
		t.Errorf("Comments len = %d, want 3", len(full.Comments))
	}
	if len(full.ReviewRequests) != 1 {
		t.Errorf("ReviewRequests len = %d, want 1", len(full.ReviewRequests))
	}
	if full.ReviewRequests[0].Login != "bob" {
		t.Errorf("ReviewRequests[0].Login = %q, want bob", full.ReviewRequests[0].Login)
	}
	if len(full.Labels) != 1 {
		t.Errorf("Labels len = %d, want 1", len(full.Labels))
	}
}
