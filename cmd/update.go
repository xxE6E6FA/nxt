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

	// Determine the asset name for this platform
	asset := assetName()

	// Download to a temp directory
	tmpDir, err := os.MkdirTemp("", "nxt-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	//nolint:gosec // repo and latest are not user-controlled
	dlCmd := exec.Command("gh", "release", "download", latest,
		"--repo", repo,
		"--pattern", asset,
		"--dir", tmpDir)
	dlCmd.Stderr = os.Stderr
	if err := dlCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading release: %v\n", err)
		os.Exit(1)
	}

	// Extract the binary from the tarball
	tarPath := filepath.Join(tmpDir, asset)
	//nolint:gosec // tarPath is constructed from constants
	extractCmd := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir, "nxt")
	extractCmd.Stderr = os.Stderr
	if err := extractCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting: %v\n", err)
		os.Exit(1)
	}

	// Replace the running binary
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding current binary: %v\n", err)
		os.Exit(1)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving symlinks: %v\n", err)
		os.Exit(1)
	}

	newBin := filepath.Join(tmpDir, "nxt")
	if err := os.Rename(newBin, self); err != nil {
		// Cross-device rename; fall back to copy
		if copyErr := copyFile(newBin, self); copyErr != nil {
			fmt.Fprintf(os.Stderr, "Error replacing binary: %v\n", copyErr)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr, "Updated to %s\n", latest)
}

func latestTag() (string, error) {
	out, err := exec.Command("gh", "release", "view", "--repo", repo, "--json", "tagName", "--jq", ".tagName").Output()
	if err != nil {
		return "", fmt.Errorf("gh release view: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func assetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	return fmt.Sprintf("nxt_%s_%s.tar.gz", os, arch)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755) //nolint:gosec // executable binary
}
