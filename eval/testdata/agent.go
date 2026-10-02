// Deterministic executable fixture for testing the eval transport, not a benchmark agent.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	phase := "implement"
	if strings.HasPrefix(string(b), "You are an independent Review Agent") {
		phase = "review"
	}
	if log := os.Getenv("EVAL_PROMPTS"); log != "" {
		root, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(log, filepath.Base(root)+"-"+phase+".txt"), b, 0600); err != nil {
			return err
		}
	}
	if phase == "review" {
		fmt.Print(`{"findings":[]}`)
		return nil
	}
	// Each baseline/initial implement call must start from a fresh clean fixture.
	status, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil || len(status) != 0 {
		return fmt.Errorf("initial implementation repository is not clean: %s (%v)", status, err)
	}
	var edits map[string]string
	if err := json.Unmarshal([]byte(os.Getenv("EVAL_EDITS")), &edits); err != nil {
		return err
	}
	for name, content := range edits {
		if err := os.WriteFile(name, []byte(content), 0600); err != nil {
			return err
		}
	}
	fmt.Print("private implementation output")
	return nil
}
