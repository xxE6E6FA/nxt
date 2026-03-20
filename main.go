package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

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

	if flags.Update {
		cmd.RunUpdate()
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	setup.EnsureSetup(cfg, flags.Setup)

	interactive := !flags.JSON && !flags.Debug && term.IsTerminal(int(os.Stdout.Fd())) //nolint:gosec // Fd() fits in int on all supported platforms

	// Benchmark mode: fetch everything with no cache, print timings, exit.
	if flags.Benchmark {
		runBenchmark(cfg, flags.Verbose)
		return
	}

	// Build the fetch function used by both interactive and non-interactive paths.
	fetchData := func(updateSource func(string, render.SourceStatus), noCache bool) render.FetchResult {
		skipCache := flags.NoCache || noCache

		var (
			issues      []model.LinearIssue
			prs         []model.PullRequest
			authoredPRs []model.PullRequest
			scanRes     source.ScanResult

			warnMu   sync.Mutex
			warnings []string
		)

		addWarning := func(msg string) {
			warnMu.Lock()
			warnings = append(warnings, msg)
			warnMu.Unlock()
		}

		// Phase 1: Linear + worktrees + authored PRs in parallel
		g := new(errgroup.Group)

		g.Go(func() error {
			notifySource(updateSource, "Linear", render.StatusLoading)
			if !skipCache {
				if served := serveCached("linear", &issues, cache.LinearTTL, updateSource, "Linear", func() {
					fetched, fetchErr := source.FetchLinearIssues(cfg.Linear.APIKey)
					if fetchErr == nil {
						_ = cache.Set("linear", fetched)
					}
				}); served {
					return nil
				}
			}
			var fetchErr error
			issues, fetchErr = source.FetchLinearIssues(cfg.Linear.APIKey)
			if fetchErr != nil {
				addWarning(fmt.Sprintf("linear: %v", fetchErr))
				notifySource(updateSource, "Linear", render.StatusError)
				return nil
			}
			_ = cache.Set("linear", issues)
			notifySource(updateSource, "Linear", render.StatusDone)
			return nil
		})

		g.Go(func() error {
			notifySource(updateSource, "Worktrees", render.StatusLoading)
			if len(cfg.Local.BaseDirs) == 0 {
				notifySource(updateSource, "Worktrees", render.StatusDone)
				return nil
			}
			if !skipCache {
				if served := serveCached("worktrees", &scanRes, cache.WorktreesTTL, updateSource, "Worktrees", func() {
					fetched, fetchErr := source.ScanWorktrees(cfg.Local.BaseDirs)
					if fetchErr == nil {
						_ = cache.Set("worktrees", fetched)
					}
				}); served {
					return nil
				}
			}
			var fetchErr error
			scanRes, fetchErr = source.ScanWorktrees(cfg.Local.BaseDirs)
			if fetchErr != nil {
				addWarning(fmt.Sprintf("git: %v", fetchErr))
				notifySource(updateSource, "Worktrees", render.StatusError)
				return nil
			}
			_ = cache.Set("worktrees", scanRes)
			notifySource(updateSource, "Worktrees", render.StatusDone)
			return nil
		})

		// Check GitHub cache — stale-while-revalidate aware
		var githubCacheState int // 0=miss, 1=fresh, 2=stale
		if !skipCache {
			hit, stale := cache.GetStale("github", &prs, cache.GitHubTTL, cache.StaleTTL)
			if hit && !stale {
				githubCacheState = 1
			} else if hit && stale {
				githubCacheState = 2
			}
		}

		// Start authored PR search in parallel — doesn't need Linear data
		var ghErr error
		if githubCacheState == 0 {
			notifySource(updateSource, "GitHub", render.StatusLoading)
			g.Go(func() error {
				authoredPRs, ghErr = source.FetchAuthoredPRs()
				if ghErr != nil {
					addWarning(fmt.Sprintf("github authored: %v", ghErr))
				}
				return nil
			})
		}

		_ = g.Wait()

		// Phase 2: Fetch Linear-linked PRs (needs Linear data) and merge
		switch githubCacheState {
		case 1: // fresh cache hit
			notifySource(updateSource, "GitHub", render.StatusCached)
		case 2: // stale — serve cached data, revalidate in background
			notifySource(updateSource, "GitHub", render.StatusCached)
			go func() {
				authored, fetchErr := source.FetchAuthoredPRs()
				if fetchErr == nil {
					_ = cache.Set("github", authored)
				}
			}()
		default: // cache miss — fetch synchronously
			// Authored PR search already covers all open PRs by the user.
			// Linked PR URLs from Linear are used only for matching (by the linker),
			// not for fetching — teammate PRs on shared issues are skipped entirely.
			prs = authoredPRs
			if ghErr != nil {
				notifySource(updateSource, "GitHub", render.StatusError)
			} else {
				notifySource(updateSource, "GitHub", render.StatusDone)
			}
			if ghErr == nil {
				_ = cache.Set("github", prs)
			}
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
	result := fetchData(nil, false)

	// Debug
	if flags.Debug {
		printDebug(result.Items)
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

func runBenchmark(cfg *config.Config, verbose bool) {
	type timing struct {
		Name     string
		Duration time.Duration
		Count    int
		Error    string
	}

	var mu sync.Mutex
	var timings []timing
	record := func(name string, d time.Duration, count int, err error) {
		t := timing{Name: name, Duration: d, Count: count}
		if err != nil {
			t.Error = err.Error()
		}
		mu.Lock()
		timings = append(timings, t)
		mu.Unlock()
	}

	totalStart := time.Now()

	fmt.Fprintf(os.Stderr, "Fetching all sources (no cache)...\n\n")

	// Phase 1: parallel fetches
	g := new(errgroup.Group)
	var issues []model.LinearIssue
	var scanRes source.ScanResult
	var authoredPRs []model.PullRequest

	g.Go(func() error {
		start := time.Now()
		var err error
		issues, err = source.FetchLinearIssues(cfg.Linear.APIKey)
		record("Linear issues", time.Since(start), len(issues), err)
		return nil
	})

	g.Go(func() error {
		if len(cfg.Local.BaseDirs) == 0 {
			record("Worktrees", 0, 0, nil)
			return nil
		}
		start := time.Now()
		var err error
		scanRes, err = source.ScanWorktrees(cfg.Local.BaseDirs)
		record("Worktrees", time.Since(start), len(scanRes.Worktrees), err)
		return nil
	})

	g.Go(func() error {
		start := time.Now()
		var err error
		authoredPRs, err = source.FetchAuthoredPRs()
		record("GitHub authored PRs", time.Since(start), len(authoredPRs), err)
		return nil
	})

	_ = g.Wait()

	if verbose {
		authoredURLs := make(map[string]bool, len(authoredPRs))
		for _, pr := range authoredPRs {
			authoredURLs[pr.URL] = true
		}
		fmt.Fprintf(os.Stderr, "\n--- Linear Issues & Attachments ---\n")
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "  %s  %s  (state: %s)\n", issue.Identifier, issue.Title, issue.Status)
			for _, u := range issue.PRURLs {
				tag := " [teammate]"
				if !source.IsPRURL(u) {
					tag = " [commit, skip]"
				} else if authoredURLs[u] {
					tag = " [yours]"
				}
				fmt.Fprintf(os.Stderr, "    → %s%s\n", u, tag)
			}
		}
		fmt.Fprintf(os.Stderr, "\n--- Authored PRs ---\n")
		for _, pr := range authoredPRs {
			fmt.Fprintf(os.Stderr, "  #%d %s  %s\n", pr.Number, pr.Repo, pr.URL)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	prs := authoredPRs

	linkStart := time.Now()
	items := linker.Link(issues, prs, scanRes.Worktrees, scanRes.RepoMap)
	scorer.Score(items)
	record("Link + Score", time.Since(linkStart), len(items), nil)

	totalDuration := time.Since(totalStart)

	// Print results
	fmt.Fprintf(os.Stderr, "%-25s %10s %8s %s\n", "SOURCE", "TIME", "COUNT", "STATUS")
	fmt.Fprintf(os.Stderr, "%-25s %10s %8s %s\n", "─────────────────────────", "──────────", "────────", "──────")
	for _, t := range timings {
		status := "ok"
		if t.Error != "" {
			status = "ERR: " + t.Error
		}
		fmt.Fprintf(os.Stderr, "%-25s %10s %8d %s\n", t.Name, t.Duration.Round(time.Millisecond), t.Count, status)
	}
	fmt.Fprintf(os.Stderr, "%-25s %10s %8s\n", "─────────────────────────", "──────────", "────────")
	fmt.Fprintf(os.Stderr, "%-25s %10s %8d\n", "TOTAL", totalDuration.Round(time.Millisecond), len(items))
}

// notifySource sends a status update if the callback is non-nil.
func notifySource(updateSource func(string, render.SourceStatus), name string, status render.SourceStatus) {
	if updateSource != nil {
		updateSource(name, status)
	}
}

// serveCached checks the stale-while-revalidate cache for the given key.
// If a cache hit is found, it notifies the source (using sourceName) as cached,
// optionally kicks off a background revalidation, and returns true.
// Returns false on cache miss.
func serveCached[T any](key string, dest *T, ttl time.Duration, updateSource func(string, render.SourceStatus), sourceName string, revalidate func()) bool {
	hit, stale := cache.GetStale(key, dest, ttl, cache.StaleTTL)
	if !hit {
		return false
	}
	notifySource(updateSource, sourceName, render.StatusCached)
	if stale {
		go revalidate()
	}
	return true
}

// printDebug writes a debug dump of work items to stderr.
func printDebug(items []model.WorkItem) {
	fmt.Fprintf(os.Stderr, "\n--- Debug ---\n")
	for _, item := range items {
		if item.Issue == nil {
			continue
		}
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
	fmt.Fprintf(os.Stderr, "---\n\n")
}
