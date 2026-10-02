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
	phase := os.Getenv("VICOHA_AGENT_ROLE")
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
		billing, _ := os.ReadFile("billing/rate.go")
		if os.Getenv("EVAL_BAD_CHANGE") == "1" && strings.Contains(string(billing), "99") {
			fmt.Print(`{"findings":[{"title":"Unrelated billing change","category":"scope_creep","severity":"high","actionable":true,"file":"billing/rate.go","details":"Restore the original billing rate of 25; the task only changes greetings."}]}`)
			return nil
		}
		fmt.Print(`{"findings":[]}`)
		return nil
	}
	if os.Getenv("EVAL_BAD_CHANGE") == "1" && strings.Contains(string(b), "Actionable review findings:") {
		return os.WriteFile("billing/rate.go", []byte("package billing\n\nfunc Rate() int { return 25 }\n"), 0600)
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
	if os.Getenv("EVAL_BAD_CHANGE") == "1" {
		if err := os.WriteFile("billing/rate.go", []byte("package billing\nfunc Rate() int { return 99 }\n"), 0600); err != nil {
			return err
		}
	}
	fmt.Print("private implementation output")
	return nil
}
