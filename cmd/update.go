package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const repo = "xxE6E6FA/nxt"

// RunUpdate downloads the latest release binary and replaces the current one.
func RunUpdate() {
	current := GetVersion()
	fmt.Fprintf(os.Stderr, "Current version: %s\n", current)

	// Get latest release tag
	latest, err := latestTag()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}

	if latest == current {
		fmt.Fprintf(os.Stderr, "Already up to date.\n")
		return
	}

	fmt.Fprintf(os.Stderr, "Updating to %s...\n", latest)

	if err := downloadAndInstall(latest); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Updated to %s\n", latest)
}

func downloadAndInstall(tag string) error {
	asset := assetName()

	tmpDir, err := os.MkdirTemp("", "nxt-update-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dlCmd := exec.Command("gh", "release", "download", tag,
		"--repo", repo,
		"--pattern", asset,
		"--dir", tmpDir)
	dlCmd.Stderr = os.Stderr
	if err := dlCmd.Run(); err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}

	tarPath := filepath.Join(tmpDir, asset)
	extractCmd := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir, "nxt")
	extractCmd.Stderr = os.Stderr
	if err := extractCmd.Run(); err != nil {
		return fmt.Errorf("extracting: %w", err)
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	newBin := filepath.Join(tmpDir, "nxt")
	if err := os.Rename(newBin, self); err != nil {
		// Cross-device rename; fall back to copy
		if copyErr := copyFile(newBin, self); copyErr != nil {
			return fmt.Errorf("replacing binary: %w", copyErr)
		}
	}

	return nil
}

func latestTag() (string, error) {
	out, err := exec.Command("gh", "release", "view", "--repo", repo, "--json", "tagName", "--jq", ".tagName").Output()
	if err != nil {
		return "", fmt.Errorf("gh release view: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func assetName() string {
	goos := runtime.GOOS
	arch := runtime.GOARCH
	return fmt.Sprintf("nxt_%s_%s.tar.gz", goos, arch)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755) //nolint:gosec // executable binary needs 755
}
