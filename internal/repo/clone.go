// Package repo provides Git repository cloning utilities.
package repo

import (
	"fmt"
	"os"
	"os/exec"
)

// Clone clones url into dest using `git clone`.
// If dest already exists and is non-empty, Clone returns an error.
func Clone(url, dest string) error {
	if url == "" {
		return fmt.Errorf("repo: empty repository URL")
	}
	if dest == "" {
		return fmt.Errorf("repo: empty destination path")
	}

	cmd := exec.Command("git", "clone", "--depth=1", url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("repo: git clone %s: %w", url, err)
	}

	return nil
}

// TempClone clones url into a temporary directory and returns:
//   - the path to the cloned directory
//   - a cleanup function that removes the temp directory
//   - any error encountered
//
// The caller must invoke the cleanup function when done, even if an error is returned.
func TempClone(url string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "apiaudit-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("repo: create temp dir: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	if err := Clone(url, dir); err != nil {
		cleanup()
		return "", func() {}, err
	}

	return dir, cleanup, nil
}
