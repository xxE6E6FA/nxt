package render

import (
	"testing"
	"time"
)

func TestHyperlink(t *testing.T) {
	tests := []struct {
		name string
		url  string
		text string
		want string
	}{
		{
			name: "empty URL returns plain text",
			url:  "",
			text: "click me",
			want: "click me",
		},
		{
			name: "non-empty URL wraps in OSC 8",
			url:  "https://example.com",
			text: "link",
			want: "\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\",
		},
		{
			name: "empty text with URL",
			url:  "https://example.com",
			text: "",
			want: "\x1b]8;;https://example.com\x1b\\\x1b]8;;\x1b\\",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hyperlink(tt.url, tt.text)
			if got != tt.want {
				t.Errorf("hyperlink(%q, %q) = %q, want %q", tt.url, tt.text, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"truncated with ellipsis", "hello world", 5, "hell…"},
		{"maxLen 1 returns ellipsis", "hello", 1, "…"},
		{"maxLen 0 returns full string", "hello", 0, "hello"},
		{"negative maxLen returns full string", "hello", -1, "hello"},
		{"empty string unchanged", "", 5, ""},
		{"unicode truncation", "café latte", 5, "café…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestHumanDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"just now (seconds)", 30 * time.Second, "just now"},
		{"just now (zero)", 0, "just now"},
		{"minutes", 5 * time.Minute, "5m ago"},
		{"59 minutes", 59 * time.Minute, "59m ago"},
		{"1 hour", 1 * time.Hour, "1h ago"},
		{"23 hours", 23 * time.Hour, "23h ago"},
		{"1 day", 24 * time.Hour, "1d ago"},
		{"3 days", 72 * time.Hour, "3d ago"},
		{"7 days", 168 * time.Hour, "7d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := humanDuration(tt.d)
			if got != tt.want {
				t.Errorf("humanDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestShortenPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "worktree path shortened",
			path: "/some/repo/.git/worktrees/feature-branch",
			want: "…/feature-branch",
		},
		{
			name: "absolute path without home stays absolute",
			path: "/var/repos/project",
			want: "/var/repos/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortenPath(tt.path)
			if got != tt.want {
				t.Errorf("shortenPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
