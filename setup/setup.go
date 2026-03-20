package setup

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/xxE6E6FA/nxt/config"
	"golang.org/x/term"
)

const (
	keychainAccount = "nxt"
	keychainService = "linear-api-key"
)

// isInteractive returns true if stdin is a terminal.
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// EnsureSetup checks that required credentials exist and runs interactive
// setup if they don't. Returns true if setup was performed.
func EnsureSetup(cfg *config.Config, forceSetup bool) bool {
	// Always try to load key from keychain if not in env/config
	if cfg.Linear.APIKey == "" {
		if key := readKeychain(); key != "" {
			cfg.Linear.APIKey = key
		}
	}

	if forceSetup {
		return runSetup(cfg)
	}

	// Only prompt if we're missing the key AND in an interactive terminal
	if cfg.Linear.APIKey == "" && isInteractive() {
		return runSetup(cfg)
	}

	// Non-interactive with no key — just warn
	if cfg.Linear.APIKey == "" {
		fmt.Fprintln(os.Stderr, "  ⚠ Linear API key not configured — run nxt --setup or set LINEAR_API_KEY")
	}

	return false
}

func runSetup(cfg *config.Config) bool {
	fmt.Println("Welcome to nxt — let's get you set up.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Linear API key
	fmt.Println("Linear API key is needed to fetch your assigned issues.")
	fmt.Println("Create one at: https://linear.app/settings/api")
	fmt.Println("(Personal API keys → New key)")
	fmt.Println()
	fmt.Print("Paste your Linear API key (enter to skip): ")

	key, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	key = strings.TrimSpace(key)

	if key == "" {
		fmt.Println()
		fmt.Println("  Skipped — Linear data won't be available.")
		fmt.Println("  Run nxt --setup to configure later.")
		fmt.Println()
	} else {
		if err := writeKeychain(key); err != nil {
			fmt.Printf("\n  ⚠ Could not store in Keychain: %v\n", err)
			fmt.Println("  Set LINEAR_API_KEY env var instead.")
			fmt.Println()
		} else {
			cfg.Linear.APIKey = key
			fmt.Println()
			fmt.Println("  ✓ Stored in macOS Keychain (not written to disk)")
			fmt.Println()
		}
	}

	// Check gh CLI
	if _, err := exec.LookPath("gh"); err != nil {
		fmt.Println("  ⚠ gh CLI not found — GitHub data will be unavailable")
		fmt.Println("    Install: https://cli.github.com")
		fmt.Println()
	}

	// Ensure config file exists with base_dirs
	if len(cfg.Local.BaseDirs) == 0 {
		home, _ := os.UserHomeDir()
		defaultDir := home + "/code"
		fmt.Printf("Base directory for repos [%s]: ", defaultDir)
		dir, _ := reader.ReadString('\n')
		dir = strings.TrimSpace(dir)
		if dir == "" {
			dir = defaultDir
		}
		cfg.Local.BaseDirs = []string{dir}
	}

	// Write config file (secrets stay in keychain, not on disk)
	if err := config.Write(cfg); err != nil {
		fmt.Printf("  ⚠ Could not write config: %v\n", err)
	} else {
		path, _ := config.Path()
		fmt.Printf("  ✓ Config written to %s\n", path)
	}

	fmt.Println()
	fmt.Println("Setup complete. Fetching your work items...")
	fmt.Println()
	return true
}

func readKeychain() string {
	cmd := exec.Command("security", "find-generic-password",
		"-a", keychainAccount, "-s", keychainService, "-w")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func writeKeychain(key string) error {
	// Delete existing entry first (ignore error if not found)
	_ = exec.Command("security", "delete-generic-password",
		"-a", keychainAccount, "-s", keychainService).Run()

	cmd := exec.Command("security", "add-generic-password",
		"-a", keychainAccount, "-s", keychainService,
		"-w", key,
		"-U", // update if exists
	)
	return cmd.Run()
}
