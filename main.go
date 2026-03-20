package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"golang.org/x/sync/errgroup"
	"golang.org/x/term"

	"github.com/xxE6E6FA/nxt/cache"
	"github.com/xxE6E6FA/nxt/cmd"
	"github.com/xxE6E6FA/nxt/config"
	"github.com/xxE6E6FA/nxt/linker"
	"github.com/xxE6E6FA/nxt/model"
	"github.com/xxE6E6FA/nxt/render"
	"github.com/xxE6E6FA/nxt/scorer"
	"github.com/xxE6E6FA/nxt/setup"
	"github.com/xxE6E6FA/nxt/source"
)

func main() {
	flags := cmd.ParseFlags()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	setup.EnsureSetup(cfg, flags.Setup)

	interactive := !flags.JSON && !flags.Debug && term.IsTerminal(int(os.Stdout.Fd()))

	// Build the fetch function used by both interactive and non-interactive paths.
	fetchData := func(updateSource func(string, render.SourceStatus)) render.FetchResult {
		var (
			issues      []model.LinearIssue
			prs         []model.PullRequest
			authoredPRs []model.PullRequest
			scanRes     source.ScanResult
			warnings    []string
		)

		// Phase 1: Linear + worktrees + authored PRs in parallel
		g := new(errgroup.Group)

		g.Go(func() error {
			if updateSource != nil {
				updateSource("Linear", render.StatusLoading)
			}
			if !flags.NoCache {
				if cache.GetWithTTL("linear", &issues, cache.LinearTTL) {
					if updateSource != nil {
						updateSource("Linear", render.StatusCached)
					}
					return nil
				}
			}
			var err error
			issues, err = source.FetchLinearIssues(cfg.Linear.APIKey)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("linear: %v", err))
				if updateSource != nil {
					updateSource("Linear", render.StatusError)
				}
				return nil
			}
			_ = cache.Set("linear", issues)
			if updateSource != nil {
				updateSource("Linear", render.StatusDone)
			}
			return nil
		})

		g.Go(func() error {
			if updateSource != nil {
				updateSource("Worktrees", render.StatusLoading)
			}
			if len(cfg.Local.BaseDirs) == 0 {
				if updateSource != nil {
					updateSource("Worktrees", render.StatusDone)
				}
				return nil
			}
			if !flags.NoCache {
				if cache.GetWithTTL("worktrees", &scanRes, cache.WorktreesTTL) {
					if updateSource != nil {
						updateSource("Worktrees", render.StatusCached)
					}
					return nil
				}
			}
			var err error
			scanRes, err = source.ScanWorktrees(cfg.Local.BaseDirs)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("git: %v", err))
				if updateSource != nil {
					updateSource("Worktrees", render.StatusError)
				}
				return nil
			}
			_ = cache.Set("worktrees", scanRes)
			if updateSource != nil {
				updateSource("Worktrees", render.StatusDone)
			}
			return nil
		})

		// Start authored PR search in parallel — doesn't need Linear data
		githubCached := !flags.NoCache && cache.GetWithTTL("github", &prs, cache.GitHubTTL)
		if !githubCached {
			if updateSource != nil {
				updateSource("GitHub", render.StatusLoading)
			}
			g.Go(func() error {
				var err error
				authoredPRs, err = source.FetchAuthoredPRs()
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("github authored: %v", err))
				}
				return nil
			})
		}

		_ = g.Wait()

		// Phase 2: Fetch Linear-linked PRs (needs Linear data) and merge
		if githubCached {
			if updateSource != nil {
				updateSource("GitHub", render.StatusCached)
			}
		} else {
			var prURLs []string
			for _, issue := range issues {
				prURLs = append(prURLs, issue.PRURLs...)
			}
			linkedPRs, err := source.FetchLinkedPRs(prURLs)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("github linked: %v", err))
			}
			prs = source.MergePRs(authoredPRs, linkedPRs)
			if err != nil {
				if updateSource != nil {
					updateSource("GitHub", render.StatusError)
				}
			} else if updateSource != nil {
				updateSource("GitHub", render.StatusDone)
			}
			_ = cache.Set("github", prs)
		}

		items := linker.Link(issues, prs, scanRes.Worktrees, scanRes.RepoMap)
		scorer.Score(items)

		return render.FetchResult{Items: items, Warnings: warnings}
	}

	// Interactive TUI — alt-screen from the start, actions run inline
	if interactive {
		render.RunInteractive(cfg, cfg.EditorCommand(), fetchData)
		return
	}

	// Non-interactive: fetch synchronously, then output
	result := fetchData(nil)

	// Debug
	if flags.Debug {
		fmt.Fprintf(os.Stderr, "\n--- Debug ---\n")
		for _, item := range result.Items {
			if item.Issue != nil {
				wt := "(no worktree)"
				if item.Worktree != nil {
					wt = item.Worktree.Path
				}
				pr := "(no PR)"
				if item.PR != nil {
					pr = fmt.Sprintf("PR #%d", item.PR.Number)
				}
				fmt.Fprintf(os.Stderr, "  %s  score=%d  branch=%q  %s  %s\n",
					item.Issue.Identifier, item.Score, item.Issue.BranchName, pr, wt)
			}
		}
		fmt.Fprintf(os.Stderr, "---\n\n")
	}

	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "  ⚠ %s\n", w)
	}

	if flags.JSON {
		sort.Slice(result.Items, func(i, j int) bool {
			return result.Items[i].Score > result.Items[j].Score
		})
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result.Items)
		return
	}

	render.Render(result.Items, cfg.Display.MaxItems)
}
