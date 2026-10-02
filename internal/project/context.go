package project

import (
	"fmt"
	"os"
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
