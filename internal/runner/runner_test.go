package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"vicoha/internal/result"
)

// The test binary acts as an executable backend in child processes, exercising
// the real stdin protocol and isolated implement/review invocations.
func TestMain(m *testing.M) {
	if mode := os.Getenv("VICOHA_TEST_AGENT"); mode != "" {
		fakeAgent(mode)
		return
	}
	os.Exit(m.Run())
}

func fakeAgent(mode string) {
	b, _ := io.ReadAll(os.Stdin)
	prompt := string(b)
	phase := "implement"
	if strings.HasPrefix(prompt, "You are an independent Review Agent") {
		phase = "review"
	}
	if strings.HasPrefix(prompt, "You are the Implement Agent continuing") {
		phase = "repair"
	}
	logPath := os.Getenv("VICOHA_TEST_LOG")
	log, _ := os.ReadFile(logPath)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(10)
	}
	fmt.Fprintln(f, phase)
	f.Close()
	if !strings.Contains(prompt, "original task marker") {
		os.Exit(11)
	}
	if phase != "review" {
		if mode == "implement_failure" {
			fmt.Fprintln(os.Stderr, "implement failed")
			os.Exit(12)
		}
		if phase == "repair" && (strings.Contains(prompt, "speculative marker") || !strings.Contains(prompt, "Concrete defect")) {
			os.Exit(20)
		}
		if !strings.Contains(prompt, "implement skill marker") {
			os.Exit(13)
		}
		if err := os.WriteFile("change.txt", []byte("implementation change marker"), 0600); err != nil {
			os.Exit(14)
		}
		if mode == "committed" {
			for _, args := range [][]string{{"add", "change.txt"}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "agent change"}} {
				if err := exec.Command("git", args...).Run(); err != nil {
					os.Exit(15)
				}
			}
		}
		fmt.Print("private implement output marker")
		return
	}
	if strings.Contains(prompt, "private implement output marker") || strings.Contains(prompt, "implement skill marker") || !strings.Contains(prompt, "review skill marker") || !strings.Contains(prompt, "implementation change marker") || !strings.Contains(prompt, "project guidance marker") {
		os.Exit(16)
	}
	if mode == "review_failure" {
		fmt.Fprintln(os.Stderr, "review failed")
		os.Exit(17)
	}
	if mode == "malformed" {
		fmt.Print("{}")
		return
	}
	if mode == "staging" {
		if err := exec.Command("git", "add", "change.txt").Run(); err != nil {
			os.Exit(18)
		}
	}
	if mode == "review_branch" {
		if err := exec.Command("git", "checkout", "-b", "review-branch").Run(); err != nil {
			os.Exit(21)
		}
	}
	if mode == "review_commit" {
		if err := exec.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "review mutation").Run(); err != nil {
			os.Exit(19)
		}
	}
	if mode == "mutating" {
		_ = os.WriteFile("change.txt", []byte("review mutation"), 0600)
	}
	if mode == "review_untracked" {
		_ = os.WriteFile("review-created.txt", []byte("review mutation"), 0600)
	}
	if mode == "review_delete" {
		_ = os.Remove("README.md")
	}
	if mode == "limit" || mode == "one_pass" || (mode == "repair" && !strings.Contains(string(log), "review")) {
		fmt.Print(`{"findings":[{"title":"Concrete defect","severity":"high","actionable":true,"details":"Fix the implementation"},{"title":"speculative marker","severity":"low","actionable":false,"details":"Maybe change unrelated code"}]}`)
		return
	}
	fmt.Print(`{"findings":[]}`)
}

func TestRunWorkflow(t *testing.T) {
	for _, tc := range []struct {
		mode      string
		status    result.Status
		phases    string
		goProject bool
	}{
		{"committed", result.Succeeded, "implement\nreview\n", true},
		{"repair", result.Succeeded, "implement\nreview\nrepair\nreview\n", true},
		{"limit", result.IterationLimit, "implement\nreview\nrepair\nreview\n", false},
		{"implement_failure", result.ImplementationFailed, "implement\n", false},
		{"review_failure", result.ReviewFailed, "implement\nreview\n", false},
		{"malformed", result.ReviewFailed, "implement\nreview\n", false},
		{"mutating", result.ReviewFailed, "implement\nreview\n", false},
		{"review_untracked", result.ReviewFailed, "implement\nreview\n", false},
		{"review_delete", result.ReviewFailed, "implement\nreview\n", false},
		{"staging", result.ReviewFailed, "implement\nreview\n", false},
		{"review_commit", result.ReviewFailed, "implement\nreview\n", false},
		{"review_branch", result.ReviewFailed, "implement\nreview\n", false},
		{"one_pass", result.IterationLimit, "implement\nreview\n", false},
		{"no_checks", result.VerificationFailed, "implement\nreview\n", false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "repo")
			skills := filepath.Join(parent, "skills")
			writeTestFile(t, filepath.Join(root, "README.md"), "fixture")
			writeTestFile(t, filepath.Join(root, "AGENTS.md"), "project guidance marker")
			writeTestFile(t, filepath.Join(skills, "implement", "SKILL.md"), "implement skill marker")
			writeTestFile(t, filepath.Join(skills, "review", "SKILL.md"), "review skill marker")
			if tc.goProject {
				writeTestFile(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.22\n")
				writeTestFile(t, filepath.Join(root, "main.go"), "package main\nfunc main() {}\n")
			}
			for _, args := range [][]string{{"init"}, {"add", "."}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "initial"}} {
				cmd := exec.Command("git", args...)
				cmd.Dir = root
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git: %v: %s", err, out)
				}
			}
			log := filepath.Join(parent, "calls.txt")
			t.Setenv("VICOHA_TEST_AGENT", tc.mode)
			t.Setenv("VICOHA_TEST_LOG", log)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if tc.mode == "committed" {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				executable, err = filepath.Rel(cwd, executable)
				if err != nil {
					t.Fatal(err)
				}
			}
			limit := 2
			if tc.mode == "one_pass" {
				limit = 1
			}
			got := Run(context.Background(), root, skills, "original task marker", executable, executable, limit)
			if got.AgentInvocations != strings.Count(tc.phases, "\n") || got.ReviewPasses != strings.Count(tc.phases, "review") || got.RepairPasses != strings.Count(tc.phases, "repair") || got.DurationMS <= 0 {
				t.Fatalf("incorrect telemetry: %+v", got)
			}
			if got.Status != tc.status {
				t.Fatalf("status %s, want %s: %s", got.Status, tc.status, got.Message)
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if string(calls) != tc.phases {
				t.Fatalf("calls %q, want %q", calls, tc.phases)
			}
			if tc.status == result.Succeeded && len(got.Verification) != 2 {
				t.Fatalf("expected Go test and build: %+v", got.Verification)
			}
		})
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
