package source

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLinearIssuesEmptyKey(t *testing.T) {
	_, err := FetchLinearIssues("")
	if err == nil {
		t.Fatal("expected error for empty API key, got nil")
	}
}

func TestFetchLinearIssuesWithHTTPTest(t *testing.T) {
	response := `{
		"data": {
			"viewer": {
				"assignedIssues": {
					"nodes": [{
						"id": "id-1",
						"identifier": "ENG-42",
						"title": "Test issue",
						"branchName": "eng-42-test",
						"url": "https://linear.app/team/issue/ENG-42",
						"priority": 1,
						"dueDate": "2025-06-15",
						"createdAt": "2025-01-01T00:00:00Z",
						"updatedAt": "2025-03-01T12:00:00Z",
						"startedAt": "2025-02-01T09:00:00Z",
						"estimate": 5,
						"slaBreachesAt": null,
						"snoozedUntilAt": null,
						"sortOrder": 2.5,
						"labels": {"nodes": [{"name": "backend"}, {"name": "urgent"}]},
						"state": {"name": "In Progress"},
						"cycle": {
							"id": "cycle-1",
							"startsAt": "2025-03-01",
							"endsAt": "2025-03-14"
						},
						"attachments": {
							"nodes": [
								{"url": "https://github.com/org/repo/pull/100"},
								{"url": "https://github.com/org/repo/pull/101"}
							]
						}
					},{
						"id": "id-2",
						"identifier": "ENG-43",
						"title": "Minimal issue",
						"branchName": "",
						"url": "https://linear.app/team/issue/ENG-43",
						"priority": 4,
						"dueDate": null,
						"createdAt": "2025-03-15T00:00:00Z",
						"updatedAt": "2025-03-15T00:00:00Z",
						"startedAt": null,
						"estimate": null,
						"slaBreachesAt": null,
						"snoozedUntilAt": null,
						"sortOrder": 1.0,
						"labels": {"nodes": []},
						"state": {"name": "Todo"},
						"cycle": null,
						"attachments": {"nodes": []}
					}]
				}
			}
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-key")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(200)
		w.Write([]byte(response)) //nolint:errcheck
	}))
	defer srv.Close()

	issues, err := FetchLinearIssues("test-key", srv.URL)
	if err != nil {
		t.Fatalf("FetchLinearIssues() error: %v", err)
	}

	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2", len(issues))
	}

	// First issue: fully populated
	i := issues[0]
	if i.Identifier != "ENG-42" {
		t.Errorf("Identifier = %q, want ENG-42", i.Identifier)
	}
	if i.Title != "Test issue" {
		t.Errorf("Title = %q", i.Title)
	}
	if i.BranchName != "eng-42-test" {
		t.Errorf("BranchName = %q", i.BranchName)
	}
	if i.Priority != 1 {
		t.Errorf("Priority = %d, want 1", i.Priority)
	}
	if i.DueDate == nil {
		t.Fatal("DueDate is nil")
	}
	if i.DueDate.Format("2006-01-02") != "2025-06-15" {
		t.Errorf("DueDate = %v", i.DueDate)
	}
	if i.Estimate == nil || *i.Estimate != 5 {
		t.Errorf("Estimate = %v", i.Estimate)
	}
	if len(i.Labels) != 2 {
		t.Fatalf("Labels len = %d, want 2", len(i.Labels))
	}
	if i.Labels[0] != "backend" {
		t.Errorf("Labels[0] = %q", i.Labels[0])
	}
	if !i.InCycle {
		t.Error("InCycle = false, want true")
	}
	if i.CycleID != "cycle-1" {
		t.Errorf("CycleID = %q", i.CycleID)
	}
	if len(i.PRURLs) != 2 {
		t.Fatalf("PRURLs len = %d, want 2", len(i.PRURLs))
	}
	if i.PRURLs[0] != "https://github.com/org/repo/pull/100" {
		t.Errorf("PRURLs[0] = %q", i.PRURLs[0])
	}
	if i.StartedAt == nil {
		t.Error("StartedAt is nil")
	}

	// Second issue: minimal
	i2 := issues[1]
	if i2.Identifier != "ENG-43" {
		t.Errorf("Identifier = %q, want ENG-43", i2.Identifier)
	}
	if i2.DueDate != nil {
		t.Errorf("DueDate = %v, want nil", i2.DueDate)
	}
	if i2.Estimate != nil {
		t.Errorf("Estimate = %v, want nil", i2.Estimate)
	}
	if i2.InCycle {
		t.Error("InCycle = true, want false")
	}
	if len(i2.PRURLs) != 0 {
		t.Errorf("PRURLs len = %d, want 0", len(i2.PRURLs))
	}
}

func TestFetchLinearIssuesAPIError(t *testing.T) {
	response := `{
		"data": {"viewer": {"assignedIssues": {"nodes": []}}},
		"errors": [{"message": "Not authenticated"}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(response)) //nolint:errcheck
	}))
	defer srv.Close()

	_, err := FetchLinearIssues("test-key", srv.URL)
	if err == nil {
		t.Fatal("expected error for API error response, got nil")
	}
	if err.Error() != "linear API error: Not authenticated" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestFetchLinearIssuesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("Unauthorized")) //nolint:errcheck
	}))
	defer srv.Close()

	_, err := FetchLinearIssues("test-key", srv.URL)
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}

func TestFetchLinearIssuesBearerPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer already-prefixed" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer already-prefixed")
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"data":{"viewer":{"assignedIssues":{"nodes":[]}}}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	_, err := FetchLinearIssues("Bearer already-prefixed", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
