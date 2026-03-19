package main

import (
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/sync/errgroup"

	"github.com/xxE6E6FA/nxt/cache"
	"github.com/xxE6E6FA/nxt/cmd"
	"github.com/xxE6E6FA/nxt/config"
	"github.com/xxE6E6FA/nxt/linker"
	"github.com/xxE6E6FA/nxt/model"
	"github.com/xxE6E6FA/nxt/render"
	"github.com/xxE6E6FA/nxt/scorer"
	"github.com/xxE6E6FA/nxt/source"
)

func main() {
	flags := cmd.ParseFlags()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	var (
		issues    []model.LinearIssue
		prs       []model.PullRequest
		worktrees []model.Worktree
		errors    []string
	)

	g := new(errgroup.Group)

	// Linear
	g.Go(func() error {
		if !flags.NoCache {
			if cache.Get("linear", &issues) {
				return nil
			}
		}
		var err error
		issues, err = source.FetchLinearIssues(cfg.Linear.APIKey)
		if err != nil {
			errors = append(errors, fmt.Sprintf("linear: %v", err))
			return nil // don't fail the group
		}
		_ = cache.Set("linear", issues)
		return nil
	})

	// GitHub PRs
	g.Go(func() error {
		if len(cfg.GitHub.Repos) == 0 {
			return nil
		}
		if !flags.NoCache {
			if cache.Get("github", &prs) {
				return nil
			}
		}
		var err error
		prs, err = source.FetchPullRequests(cfg.GitHub.Repos)
		if err != nil {
			errors = append(errors, fmt.Sprintf("github: %v", err))
			return nil
		}
		_ = cache.Set("github", prs)
		return nil
	})

	// Worktrees
	g.Go(func() error {
		if len(cfg.Local.BaseDirs) == 0 {
			return nil
		}
		if !flags.NoCache {
			if cache.Get("worktrees", &worktrees) {
				return nil
			}
		}
		var err error
		worktrees, err = source.ScanWorktrees(cfg.Local.BaseDirs)
		if err != nil {
			errors = append(errors, fmt.Sprintf("git: %v", err))
			return nil
		}
		_ = cache.Set("worktrees", worktrees)
		return nil
	})

	_ = g.Wait()

	// Print warnings for partial failures
	for _, e := range errors {
		fmt.Fprintf(os.Stderr, "  ⚠ %s\n", e)
	}

	// Link
	items := linker.Link(issues, prs, worktrees)

	// Score
	scorer.Score(items)

	// Output
	if flags.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(items)
		return
	}

	render.Render(items, cfg.Display.MaxItems)
}
