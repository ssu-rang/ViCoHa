package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffAndRules(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init"}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "initial"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, out)
		}
	}
	base, err := Revision(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "한글 space.txt"), []byte("untracked marker"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "AGENTS.md"), []byte("nested rule marker"), 0600); err != nil {
		t.Fatal(err)
	}
	diff, err := Diff(root, base)
	if err != nil || !strings.Contains(diff, "untracked marker") {
		t.Fatalf("incomplete diff %q: %v", diff, err)
	}
	context, err := Context(root)
	if err != nil || !strings.Contains(context, "nested rule marker") {
		t.Fatalf("missing rules %q: %v", context, err)
	}
	if _, err := Diff(root, "invalid-revision"); err == nil {
		t.Fatal("invalid revision was silently accepted")
	}
}

func TestMissingGitContextFails(t *testing.T) {
	root := t.TempDir()
	if _, err := Revision(root); err == nil {
		t.Fatal("non-Git repository was accepted")
	}
	if _, err := Diff(root, "HEAD"); err == nil {
		t.Fatal("missing diff was silently accepted")
	}
}
