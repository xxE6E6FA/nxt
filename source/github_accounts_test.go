package source

import (
	"encoding/json"
	"testing"
)

func TestGhAccountsParsing(t *testing.T) {
	// Test the JSON parsing logic that ghAccounts() uses internally.
	// We can't call ghAccounts() directly since it uses sync.Once and shells out,
	// but we can test the parsing of the response format.
	jsonData := `{
		"hosts": {
			"github.com": [
				{"login": "user1", "state": "success"},
				{"login": "user2", "state": "failed"},
				{"login": "", "state": "success"},
				{"login": "user3", "state": "success"}
			],
			"github.example.com": [
				{"login": "corp-user", "state": "success"}
			]
		}
	}`

	var resp struct {
		Hosts map[string][]struct {
			Login string `json:"login"`
			State string `json:"state"`
		} `json:"hosts"`
	}

	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	var accounts []string
	for _, hostAccounts := range resp.Hosts {
		for _, a := range hostAccounts {
			if a.State == "success" && a.Login != "" {
				accounts = append(accounts, a.Login)
			}
		}
	}

	if len(accounts) != 3 {
		t.Fatalf("got %d accounts, want 3", len(accounts))
	}

	// user2 (failed) and empty-login should be excluded
	found := make(map[string]bool)
	for _, a := range accounts {
		found[a] = true
	}
	if !found["user1"] {
		t.Error("missing user1")
	}
	if found["user2"] {
		t.Error("user2 should be excluded (failed state)")
	}
	if !found["user3"] {
		t.Error("missing user3")
	}
	if !found["corp-user"] {
		t.Error("missing corp-user")
	}
}

func TestSearchResponseParsing(t *testing.T) {
	// Test the JSON structure that searchAuthoredPRs parses
	jsonData := `{
		"data": {
			"search": {
				"nodes": [
					{
						"number": 42,
						"title": "Fix thing",
						"headRefName": "fix-thing",
						"url": "https://github.com/org/repo/pull/42",
						"state": "OPEN",
						"isDraft": false,
						"body": "body",
						"createdAt": "2025-01-01T00:00:00Z",
						"updatedAt": "2025-01-02T00:00:00Z",
						"additions": 10,
						"deletions": 5,
						"changedFiles": 3,
						"mergeable": "MERGEABLE",
						"mergeStateStatus": "CLEAN",
						"reviewDecision": "APPROVED",
						"commits": {"nodes": []},
						"comments": {"totalCount": 2},
						"reviewRequests": {"nodes": []},
						"labels": {"nodes": [{"name": "bug"}]},
						"repository": {"nameWithOwner": "org/repo"}
					},
					{
						"number": 0,
						"url": "",
						"commits": {"nodes": []},
						"comments": {"totalCount": 0},
						"reviewRequests": {"nodes": []},
						"labels": {"nodes": []},
						"repository": {"nameWithOwner": ""}
					}
				]
			}
		}
	}`

	var resp struct {
		Data struct {
			Search struct {
				Nodes []graphqlSearchNode `json:"nodes"`
			} `json:"search"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	nodes := resp.Data.Search.Nodes
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}

	// First node should be valid
	n := nodes[0]
	if n.Number != 42 {
		t.Errorf("Number = %d, want 42", n.Number)
	}
	if n.Repository.NameWithOwner != "org/repo" {
		t.Errorf("Repository = %q", n.Repository.NameWithOwner)
	}

	// Convert to full and verify
	full := graphqlPRToFull(&n.graphqlPRResponse)
	if full.ReviewDecision != "APPROVED" {
		t.Errorf("ReviewDecision = %q", full.ReviewDecision)
	}
	if len(full.Labels) != 1 || full.Labels[0].Name != "bug" {
		t.Errorf("Labels = %v", full.Labels)
	}

	// Second node has empty URL — would be skipped in real code
	if nodes[1].URL != "" {
		t.Error("second node should have empty URL")
	}
}
