package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xxE6E6FA/nxt/model"
)

// skipDirs are directory names to skip when scanning for repos.
var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true,
	".cache": true, ".claude": true, "dist": true, "build": true,
}

// ScanResult holds worktrees and a repo-slug-to-path map for linking PRs by repo.
type ScanResult struct {
	Worktrees []model.Worktree
	RepoMap   map[string]string // "owner/name" → local repo root path
}

// ScanWorktrees recursively finds git repos in baseDirs and lists their worktrees.
func ScanWorktrees(baseDirs []string) (ScanResult, error) {
	repoPaths := make([]string, 0, len(baseDirs))

	for _, baseDir := range baseDirs {
		repoPaths = append(repoPaths, findGitReposUnder(baseDir, 4)...)
	}

	// Scan repos in parallel
	type result struct {
		worktrees []model.Worktree
		repoSlug  string // "owner/name" from origin remote
	}
	results := make([]result, len(repoPaths))
	var wg sync.WaitGroup

	for i, repoPath := range repoPaths {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			wts, err := listWorktrees(path)
			if err != nil {
				return
			}
			results[idx] = result{
				worktrees: wts,
				repoSlug:  repoSlugFromRemote(path),
			}
		}(i, repoPath)
	}

	wg.Wait()

	var all []model.Worktree
	repoMap := make(map[string]string)
	for i, r := range results {
		all = append(all, r.worktrees...)
		if r.repoSlug != "" {
			repoMap[r.repoSlug] = repoPaths[i]
		}
	}
	return ScanResult{Worktrees: all, RepoMap: repoMap}, nil
}

// repoSlugFromRemote extracts "owner/name" from a repo's origin remote URL.
func repoSlugFromRemote(repoRoot string) string {
	cmd := exec.Command("git", "-C", repoRoot, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))

	// Handle SSH: git@host:owner/name.git (contains "@" before ":")
	if strings.Contains(url, "@") {
		if idx := strings.Index(url, ":"); idx >= 0 {
			url = url[idx+1:]
		}
	} else {
		// Handle HTTPS: https://github.com/owner/name.git
		url = strings.TrimPrefix(url, "https://github.com/")
		url = strings.TrimPrefix(url, "http://github.com/")
	}
	url = strings.TrimSuffix(url, ".git")

	// Should be "owner/name" now
	if strings.Count(url, "/") == 1 {
		return url
	}
	return ""
}

// findGitReposUnder scans children of dir for git repos, recursing up to maxDepth.
// The dir itself is never returned as a repo (it's a base_dir, not a project).
func findGitReposUnder(dir string, maxDepth int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if skipDirs[name] {
			continue
		}
		repos = append(repos, findGitRepos(filepath.Join(dir, name), maxDepth-1)...)
	}
	return repos
}

// findGitRepos walks a directory up to maxDepth levels looking for .git dirs.
// When a .git is found, the parent is added and recursion stops for that subtree.
func findGitRepos(dir string, maxDepth int) []string {
	if maxDepth < 0 {
		return nil
	}

	gitDir := filepath.Join(dir, ".git")
	info, err := os.Stat(gitDir)
	if err == nil && info.IsDir() {
		return []string{dir}
	}

	// Not a repo — recurse into subdirs
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if skipDirs[name] {
			continue
		}
		repos = append(repos, findGitRepos(filepath.Join(dir, name), maxDepth-1)...)
	}
	return repos
}

func listWorktrees(repoRoot string) ([]model.Worktree, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	type rawEntry struct {
		path   string
		branch string
	}
	var entries []rawEntry
	var current rawEntry

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.path != "" {
				entries = append(entries, current)
			}
			current = rawEntry{}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			current.path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimPrefix(line, "branch ")
			current.branch = strings.TrimPrefix(ref, "refs/heads/")
		}
	}
	if current.path != "" {
		entries = append(entries, current)
	}

	// Track which branches are already covered by a worktree checkout
	wtBranches := make(map[string]bool)
	for _, e := range entries {
		wtBranches[e.branch] = true
	}

	// Fetch last commit times in parallel
	worktrees := make([]model.Worktree, len(entries))
	var wg sync.WaitGroup

	for i, e := range entries {
		worktrees[i] = model.Worktree{
			Path:     e.path,
			Branch:   e.branch,
			RepoRoot: repoRoot,
			IsMain:   isMainBranch(e.branch),
		}
		wg.Add(1)
		go func(idx int, wtPath string) {
			defer wg.Done()
			worktrees[idx].LastCommit = getLastCommitTime(wtPath)
		}(i, e.path)
	}

	wg.Wait()

	// Also include local branches not checked out as worktrees.
	// These link to the repo root so nxt can connect issues to a local folder
	// even when the branch isn't in its own directory.
	localBranches := listLocalBranches(repoRoot)
	for _, branch := range localBranches {
		if wtBranches[branch] || isMainBranch(branch) {
			continue
		}
		worktrees = append(worktrees, model.Worktree{
			Path:     repoRoot,
			Branch:   branch,
			RepoRoot: repoRoot,
			IsMain:   false,
		})
	}

	return worktrees, nil
}

// listLocalBranches returns all local branch names for a repo.
func listLocalBranches(repoRoot string) []string {
	cmd := exec.Command("git", "-C", repoRoot, "branch", "--format=%(refname:short)")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		b := strings.TrimSpace(line)
		if b != "" {
			branches = append(branches, b)
		}
	}
	return branches
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
