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
	phase := os.Getenv("VICOHA_AGENT_ROLE")
	if strings.HasPrefix(prompt, "You are the Implement Agent continuing") {
		if phase != "implement" {
			os.Exit(22)
		}
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
	if phase == "verify-discovery" {
		if strings.Contains(prompt, "private implement output") || strings.Contains(prompt, "implementation change marker") || strings.Contains(prompt, "original task marker") || strings.Contains(prompt, "review skill marker") {
			os.Exit(23)
		}
		switch mode {
		case "discovery_mutation":
			_ = os.WriteFile("discovery.txt", []byte("mutation"), 0600)
		case "discovery_malformed":
			fmt.Print("```json\n{}\n```")
			return
		case "discovery_unsafe":
			fmt.Print(`{"commands":[{"argv":["rm","-rf","."],"reason":"cleanup","evidence":["README.md"]}]}`)
			return
		case "no_checks":
			fmt.Print(`{"commands":[]}`)
			return
		}
		fmt.Print(`{"commands":[{"argv":["go","test","./..."],"reason":"documented tests","evidence":["README.md"]},{"argv":["go","build","./..."],"reason":"documented build","evidence":["go.mod"]}]}`)
		return
	}
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
		fmt.Print(`{"findings":[{"title":"Concrete defect","category":"functional_defect","severity":"high","actionable":true,"details":"Fix the implementation"},{"title":"speculative marker","category":"functional_defect","severity":"low","actionable":false,"details":"Maybe change unrelated code"}]}`)
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
		{"no_checks", result.VerificationFailed, "implement\nreview\nverify-discovery\n", false},
		{"discovery", result.Succeeded, "implement\nreview\nverify-discovery\n", true},
		{"discovery_failure", result.VerificationFailed, "implement\nreview\nverify-discovery\n", true},
		{"discovery_mutation", result.VerificationFailed, "implement\nreview\nverify-discovery\n", true},
		{"discovery_malformed", result.VerificationFailed, "implement\nreview\nverify-discovery\n", true},
		{"discovery_unsafe", result.VerificationFailed, "implement\nreview\nverify-discovery\n", true},
		{"baseline", result.Succeeded, "implement\n", false},
		{"baseline-verify", result.Succeeded, "implement\n", true},
		{"baseline_unknown", result.VerificationFailed, "implement\n", false},
		{"independent-review", result.Succeeded, "implement\nreview\nrepair\nreview\n", false},
		{"no_ai_discovery", result.VerificationFailed, "implement\nreview\n", false},
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
			if strings.HasPrefix(tc.mode, "discovery") {
				writeTestFile(t, filepath.Join(root, "README.md"), "Verify with `go test ./...` and `go build ./...`.")
			}
			if tc.mode == "discovery_failure" {
				writeTestFile(t, filepath.Join(root, "main.go"), "invalid Go source")
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
			options := Options{}
			switch tc.mode {
			case "baseline", "baseline-verify":
				options.Mode = tc.mode
			case "baseline_unknown":
				options.Mode = "baseline-verify"
			case "independent-review":
				options.Mode = tc.mode
				t.Setenv("VICOHA_TEST_AGENT", "repair")
			case "no_ai_discovery":
				options.DisableAIDiscovery = true
			}
			t.Setenv("VICOHA_TEST_LOG", log)
			t.Setenv("VICOHA_AGENT_ROLE", "inherited-invalid-role")
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
			got := RunWithOptions(context.Background(), root, skills, "original task marker", executable, executable, limit, options)
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
			if tc.status == result.Succeeded && tc.goProject && len(got.Verification) != 2 {
				t.Fatalf("expected Go test and build: %+v", got.Verification)
			}
			if (options.Mode == "baseline" || options.Mode == "independent-review") && len(got.Verification) != 0 {
				t.Fatal("verification ran in excluded mode")
			}
			if tc.mode == "discovery" && (got.DiscoveryInvocations != 1 || got.Verification[0].Source != "ai" || got.VerificationCount != 2) {
				t.Fatalf("missing discovery provenance: %+v", got)
			}
			if tc.mode == "discovery_failure" && (len(got.Verification) != 1 || got.Verification[0].Status != "failed") {
				t.Fatalf("discovery decided execution status: %+v", got)
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
