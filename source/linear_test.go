package source

import (
	"encoding/json"
	"testing"
	"time"
)

const labelBug = "bug"

func TestParseDatePtr(t *testing.T) {
	tests := []struct {
		name   string
		layout string
		input  *string
		want   *time.Time
	}{
		{"nil input", "2006-01-02", nil, nil},
		{"valid date", "2006-01-02", strPtr("2025-03-15"), timePtr(time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC))},
		{"invalid date", "2006-01-02", strPtr("not-a-date"), nil},
		{"empty string", "2006-01-02", strPtr(""), nil},
		{"valid RFC3339", time.RFC3339, strPtr("2025-06-01T12:00:00Z"), timePtr(time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDatePtr(tt.layout, tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("parseDatePtr() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parseDatePtr() = nil, want %v", tt.want)
			}
			if !got.Equal(*tt.want) {
				t.Errorf("parseDatePtr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinearResponseParsing(t *testing.T) {
	jsonData := `{
		"data": {
			"viewer": {
				"assignedIssues": {
					"nodes": [{
						"id": "issue-id-1",
						"identifier": "ENG-123",
						"title": "Fix login bug",
						"branchName": "fix/login-bug",
						"url": "https://linear.app/team/issue/ENG-123",
						"priority": 2,
						"dueDate": "2025-04-01",
						"createdAt": "2025-01-15T10:00:00Z",
						"updatedAt": "2025-03-10T15:30:00Z",
						"startedAt": "2025-02-01T09:00:00Z",
						"estimate": 3,
						"slaBreachesAt": "2025-04-15T00:00:00Z",
						"snoozedUntilAt": null,
						"sortOrder": 1.5,
						"labels": {
							"nodes": [
								{"name": "bug"},
								{"name": "high-priority"}
							]
						},
						"state": {"name": "In Progress"},
						"cycle": {
							"id": "cycle-1",
							"startsAt": "2025-03-01",
							"endsAt": "2025-03-14"
						},
						"attachments": {
							"nodes": [
								{"url": "https://github.com/org/repo/pull/42"}
							]
						}
					}]
				}
			}
		}
	}`

	var lr linearResponse
	if err := json.Unmarshal([]byte(jsonData), &lr); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	nodes := lr.Data.Viewer.AssignedIssues.Nodes
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	n := nodes[0]
	if n.ID != "issue-id-1" {
		t.Errorf("ID = %q, want issue-id-1", n.ID)
	}
	if n.Identifier != "ENG-123" {
		t.Errorf("Identifier = %q, want ENG-123", n.Identifier)
	}
	if n.Title != "Fix login bug" {
		t.Errorf("Title = %q", n.Title)
	}
	if n.BranchName != "fix/login-bug" {
		t.Errorf("BranchName = %q", n.BranchName)
	}
	if n.URL != "https://linear.app/team/issue/ENG-123" {
		t.Errorf("URL = %q", n.URL)
	}
	if n.Priority != 2 {
		t.Errorf("Priority = %d, want 2", n.Priority)
	}
	if n.DueDate == nil || *n.DueDate != "2025-04-01" {
		t.Errorf("DueDate = %v", n.DueDate)
	}
	if n.CreatedAt != "2025-01-15T10:00:00Z" {
		t.Errorf("CreatedAt = %q", n.CreatedAt)
	}
	if n.UpdatedAt != "2025-03-10T15:30:00Z" {
		t.Errorf("UpdatedAt = %q", n.UpdatedAt)
	}
	if n.StartedAt == nil || *n.StartedAt != "2025-02-01T09:00:00Z" {
		t.Errorf("StartedAt = %v", n.StartedAt)
	}
	if n.Estimate == nil || *n.Estimate != 3 {
		t.Errorf("Estimate = %v", n.Estimate)
	}
	if n.SLABreachesAt == nil || *n.SLABreachesAt != "2025-04-15T00:00:00Z" {
		t.Errorf("SLABreachesAt = %v", n.SLABreachesAt)
	}
	if n.SnoozedUntilAt != nil {
		t.Errorf("SnoozedUntilAt = %v, want nil", n.SnoozedUntilAt)
	}
	if n.SortOrder != 1.5 {
		t.Errorf("SortOrder = %f, want 1.5", n.SortOrder)
	}
	if n.State.Name != "In Progress" {
		t.Errorf("State.Name = %q", n.State.Name)
	}
	if len(n.Labels.Nodes) != 2 {
		t.Fatalf("Labels len = %d, want 2", len(n.Labels.Nodes))
	}
	if n.Labels.Nodes[0].Name != labelBug {
		t.Errorf("Labels[0] = %q", n.Labels.Nodes[0].Name)
	}
	if n.Cycle == nil {
		t.Fatal("Cycle is nil")
	}
	if n.Cycle.ID != "cycle-1" {
		t.Errorf("Cycle.ID = %q", n.Cycle.ID)
	}
	if n.Cycle.StartsAt == nil || *n.Cycle.StartsAt != "2025-03-01" {
		t.Errorf("Cycle.StartsAt = %v", n.Cycle.StartsAt)
	}
	if n.Cycle.EndsAt == nil || *n.Cycle.EndsAt != "2025-03-14" {
		t.Errorf("Cycle.EndsAt = %v", n.Cycle.EndsAt)
	}
	if len(n.Attachments.Nodes) != 1 {
		t.Fatalf("Attachments len = %d, want 1", len(n.Attachments.Nodes))
	}
	if n.Attachments.Nodes[0].URL != "https://github.com/org/repo/pull/42" {
		t.Errorf("Attachment URL = %q", n.Attachments.Nodes[0].URL)
	}
}

func TestLinearResponseWithErrors(t *testing.T) {
	jsonData := `{
		"data": {"viewer": {"assignedIssues": {"nodes": []}}},
		"errors": [{"message": "Authentication required"}]
	}`

	var lr linearResponse
	if err := json.Unmarshal([]byte(jsonData), &lr); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(lr.Errors) != 1 {
		t.Fatalf("Errors len = %d, want 1", len(lr.Errors))
	}
	if lr.Errors[0].Message != "Authentication required" {
		t.Errorf("Error message = %q", lr.Errors[0].Message)
	}
}

func strPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
