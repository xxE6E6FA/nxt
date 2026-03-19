package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/xxE6E6FA/nxt/model"
)

// ScanWorktrees finds all git repos in baseDirs and lists their worktrees.
func ScanWorktrees(baseDirs []string) ([]model.Worktree, error) {
	var all []model.Worktree

	for _, baseDir := range baseDirs {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			repoPath := filepath.Join(baseDir, entry.Name())
			gitDir := filepath.Join(repoPath, ".git")
			if _, err := os.Stat(gitDir); err != nil {
				continue
			}
			wts, err := listWorktrees(repoPath)
			if err != nil {
				continue
			}
			all = append(all, wts...)
		}
	}

	return all, nil
}

func listWorktrees(repoRoot string) ([]model.Worktree, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var worktrees []model.Worktree
	var current model.Worktree

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				current.RepoRoot = repoRoot
				current.IsMain = isMainBranch(current.Branch)
				current.LastCommit = getLastCommitTime(current.Path)
				worktrees = append(worktrees, current)
			}
			current = model.Worktree{}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimPrefix(line, "branch ")
			// Strip refs/heads/
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		}
	}
	// Flush last entry
	if current.Path != "" {
		current.RepoRoot = repoRoot
		current.IsMain = isMainBranch(current.Branch)
		current.LastCommit = getLastCommitTime(current.Path)
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

func isMainBranch(branch string) bool {
	return branch == "main" || branch == "master"
}

func getLastCommitTime(worktreePath string) time.Time {
	cmd := exec.Command("git", "-C", worktreePath, "log", "-1", "--format=%cI")
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(out)))
	if err != nil {
		return time.Time{}
	}
	return t
}
