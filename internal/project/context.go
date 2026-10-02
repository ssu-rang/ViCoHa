package project

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Context returns repository guidance, independently of agent output.
func Context(root string) (string, error) {
	var parts []string
	for _, name := range []string{"AGENTS.md", "README.md", "go.mod", "package.json", "pyproject.toml", "Makefile"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err == nil {
			parts = append(parts, "--- "+name+" ---\n"+string(b))
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("read project rules %s: %w", name, err)
		}
	}
	files, err := git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return "", err
	}
	for _, name := range strings.Split(files, "\x00") {
		if filepath.Base(name) != "AGENTS.md" || name == "AGENTS.md" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("read project rules %s: %w", name, err)
		}
		parts = append(parts, "--- "+name+" (applies to that directory) ---\n"+string(b))
	}
	return strings.Join(parts, "\n\n"), nil
}

// Revision captures the commit before implementation, so agent commits remain
// part of the final review diff. The MVP requires a Git repository with a commit.
func Revision(root string) (string, error) {
	s, err := git(root, "rev-parse", "--verify", "HEAD")
	return strings.TrimSpace(s), err
}

// Diff includes committed, staged, unstaged and untracked changes from base.
func Diff(root, base string) (string, error) {
	diff, err := git(root, "diff", base, "--", ".")
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
