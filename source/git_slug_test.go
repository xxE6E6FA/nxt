package source

import "testing"

// TestExtractSlugFromRemoteURL tests the URL parsing logic inside repoSlugFromRemote
// by calling the function with the URL already extracted. Since repoSlugFromRemote
// shells out to git, we test the parsing logic via parseWorktreeOutput and
// extractSlugFromURL which we extract here as table-driven tests against the
// same logic inlined in repoSlugFromRemote.

func TestParseWorktreeOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		wantN int // expected number of entries
		wantP []string
		wantB []string
	}{
		{
			name:  "single worktree",
			input: "worktree /code/repo\nbranch refs/heads/main\n\n",
			wantN: 1,
			wantP: []string{"/code/repo"},
			wantB: []string{"main"},
		},
		{
			name: "multiple worktrees",
			input: "worktree /code/repo\nbranch refs/heads/main\n\n" +
				"worktree /code/repo/.git/worktrees/feature\nbranch refs/heads/feature-x\n\n",
			wantN: 2,
			wantP: []string{"/code/repo", "/code/repo/.git/worktrees/feature"},
			wantB: []string{"main", "feature-x"},
		},
		{
			name:  "detached HEAD (no branch line)",
			input: "worktree /code/repo\nHEAD abc123\ndetached\n\n",
			wantN: 1,
			wantP: []string{"/code/repo"},
			wantB: []string{""},
		},
		{
			name:  "empty input",
			input: "",
			wantN: 0,
		},
		{
			name:  "trailing newline only",
			input: "\n",
			wantN: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := parseWorktreeLines(tt.input)
			if len(entries) != tt.wantN {
				t.Fatalf("got %d entries, want %d", len(entries), tt.wantN)
			}
			for i := range entries {
				if entries[i].path != tt.wantP[i] {
					t.Errorf("entry[%d].path = %q, want %q", i, entries[i].path, tt.wantP[i])
				}
				if entries[i].branch != tt.wantB[i] {
					t.Errorf("entry[%d].branch = %q, want %q", i, entries[i].branch, tt.wantB[i])
				}
			}
		})
	}
}

func TestExtractSlugFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"SSH URL", "git@github.com:owner/repo.git", "owner/repo"},
		{"SSH URL without .git", "git@github.com:owner/repo", "owner/repo"},
		{"HTTPS URL", "https://github.com/owner/repo.git", "owner/repo"},
		{"HTTPS URL without .git", "https://github.com/owner/repo", "owner/repo"},
		{"HTTP URL", "http://github.com/owner/repo.git", "owner/repo"},
		{"nested path returns empty", "https://github.com/a/b/c", ""},
		{"empty URL", "", ""},
		{"just owner", "https://github.com/owner", ""},
		// ssh://git@host:port/owner/repo.git is handled by git, which normalizes
		// to git@host:owner/repo.git before we see it. Not testing that path.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSlugFromURL(tt.url)
			if got != tt.want {
				t.Errorf("extractSlugFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
