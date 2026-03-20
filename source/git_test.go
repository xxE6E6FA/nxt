package source

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestIsMainBranch(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   bool
	}{
		{"main", "main", true},
		{"master", "master", true},
		{"develop", "develop", false},
		{"main-feature", "main-feature", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMainBranch(tt.branch)
			if got != tt.want {
				t.Errorf("isMainBranch(%q) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}

func TestFindGitRepos(t *testing.T) {
	t.Run("dir with .git returns the dir", func(t *testing.T) {
		tmp := t.TempDir()
		if err := os.Mkdir(filepath.Join(tmp, ".git"), 0o750); err != nil {
			t.Fatal(err)
		}

		repos := findGitRepos(tmp, 1)
		if len(repos) != 1 {
			t.Fatalf("got %d repos, want 1", len(repos))
		}
		if repos[0] != tmp {
			t.Errorf("got %q, want %q", repos[0], tmp)
		}
	})

	t.Run("nested .git found at correct depth", func(t *testing.T) {
		tmp := t.TempDir()
		nested := filepath.Join(tmp, "project")
		if err := os.MkdirAll(filepath.Join(nested, ".git"), 0o750); err != nil {
			t.Fatal(err)
		}

		repos := findGitRepos(tmp, 2)
		if len(repos) != 1 {
			t.Fatalf("got %d repos, want 1", len(repos))
		}
		if repos[0] != nested {
			t.Errorf("got %q, want %q", repos[0], nested)
		}
	})

	t.Run("maxDepth 0 with .git present", func(t *testing.T) {
		tmp := t.TempDir()
		if err := os.Mkdir(filepath.Join(tmp, ".git"), 0o750); err != nil {
			t.Fatal(err)
		}

		repos := findGitRepos(tmp, 0)
		if len(repos) != 1 {
			t.Fatalf("got %d repos, want 1", len(repos))
		}
	})

	t.Run("maxDepth -1 returns nil", func(t *testing.T) {
		tmp := t.TempDir()
		repos := findGitRepos(tmp, -1)
		if repos != nil {
			t.Errorf("got %v, want nil", repos)
		}
	})

	t.Run("skipDirs are skipped", func(t *testing.T) {
		tmp := t.TempDir()
		// Create a node_modules dir with .git inside — should be skipped
		nmDir := filepath.Join(tmp, "node_modules", "pkg")
		if err := os.MkdirAll(filepath.Join(nmDir, ".git"), 0o750); err != nil {
			t.Fatal(err)
		}
		// Create a vendor dir with .git inside — should be skipped
		vendorDir := filepath.Join(tmp, "vendor", "lib")
		if err := os.MkdirAll(filepath.Join(vendorDir, ".git"), 0o750); err != nil {
			t.Fatal(err)
		}

		repos := findGitRepos(tmp, 3)
		if len(repos) != 0 {
			t.Errorf("got %d repos, want 0 (skipDirs should be skipped)", len(repos))
		}
	})
}

func TestFindGitReposUnder(t *testing.T) {
	t.Run("skips dir itself, returns children", func(t *testing.T) {
		tmp := t.TempDir()

		// Even if the base dir has .git, findGitReposUnder should skip it
		if err := os.Mkdir(filepath.Join(tmp, ".git"), 0o750); err != nil {
			t.Fatal(err)
		}

		// Create two child repos
		repoA := filepath.Join(tmp, "repo-a")
		repoB := filepath.Join(tmp, "repo-b")
		if err := os.MkdirAll(filepath.Join(repoA, ".git"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(repoB, ".git"), 0o750); err != nil {
			t.Fatal(err)
		}

		repos := findGitReposUnder(tmp, 4)
		sort.Strings(repos)

		if len(repos) != 2 {
			t.Fatalf("got %d repos, want 2: %v", len(repos), repos)
		}
		if repos[0] != repoA {
			t.Errorf("repos[0] = %q, want %q", repos[0], repoA)
		}
		if repos[1] != repoB {
			t.Errorf("repos[1] = %q, want %q", repos[1], repoB)
		}
	})

	t.Run("dot-prefixed children are skipped", func(t *testing.T) {
		tmp := t.TempDir()
		hidden := filepath.Join(tmp, ".hidden-repo")
		if err := os.MkdirAll(filepath.Join(hidden, ".git"), 0o750); err != nil {
			t.Fatal(err)
		}

		repos := findGitReposUnder(tmp, 4)
		if len(repos) != 0 {
			t.Errorf("got %d repos, want 0 (hidden dirs should be skipped)", len(repos))
		}
	})
}
