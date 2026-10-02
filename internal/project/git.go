package project

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Revision captures the commit before implementation, so agent commits remain
// part of the final review diff. The MVP requires a Git repository with a commit.
func Revision(root string) (string, error) {
	s, err := git(root, "rev-parse", "--verify", "HEAD")
	return strings.TrimSpace(s), err
}

// Diff includes committed, staged, unstaged and untracked changes from base.
func Diff(root, base string) (string, error) {
	diff, err := git(root, "diff", "--no-ext-diff", "--no-textconv", "--binary", base, "--", ".")
	if err != nil {
		return "", err
	}
	files, err := git(root, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString(diff)
	for _, name := range strings.Split(files, "\x00") {
		if name == "" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return "", fmt.Errorf("read untracked file %q: %w", name, err)
		}
		fmt.Fprintf(&out, "\n--- untracked file: %s ---\n%s", name, content)
	}
	return out.String(), nil
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(b), nil
}

// State detects changes to HEAD, index, tracked files and non-ignored untracked
// files. It is a read-only policy check, not an operating-system sandbox.
type State struct{ Diff, Revision, HeadRef, Index, Status string }

func Snapshot(root, base string) (State, error) {
	var s State
	var err error
	if s.Diff, err = Diff(root, base); err != nil {
		return s, err
	}
	if s.Revision, err = Revision(root); err != nil {
		return s, err
	}
	// A branch switch at the same commit also changes repository state.
	// In detached HEAD state this returns HEAD instead of a branch ref.
	if s.HeadRef, err = git(root, "rev-parse", "--symbolic-full-name", "HEAD"); err != nil {
		return s, err
	}
	if s.Index, err = git(root, "ls-files", "--stage", "-z"); err != nil {
		return s, err
	}
	s.Status, err = git(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	return s, err
}
