package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vicoha/internal/harness"
)

type invocation struct {
	Args              []string
	Prompt, Home, Cwd string
	Files             []string
}

func TestMain(m *testing.M) {
	if mode := os.Getenv("VICOHA_TEST_CODEX"); mode != "" {
		prompt, _ := io.ReadAll(os.Stdin)
		cwd, _ := os.Getwd()
		home := os.Getenv("CODEX_HOME")
		call := invocation{Args: os.Args[1:], Prompt: string(prompt), Home: home, Cwd: cwd}
		entries, _ := os.ReadDir(home)
		for _, entry := range entries {
			call.Files = append(call.Files, entry.Name())
		}
		log, err := os.OpenFile(os.Getenv("VICOHA_TEST_CODEX_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(10)
		}
		_ = json.NewEncoder(log).Encode(call)
		log.Close()
		fmt.Println("private Codex progress output")
		fmt.Fprintln(os.Stderr, "Codex diagnostic")
		if mode == "failure" {
			os.Exit(12)
		}
		final := "implementation response"
		if os.Getenv("VICOHA_AGENT_ROLE") == "review" {
			final = `{"findings":[]}`
		}
		if os.Getenv("VICOHA_AGENT_ROLE") == "verify-discovery" {
			final = `{"commands":[]}`
		}
		if mode == "malformed" {
			final = "```json\n{}\n```"
		}
		if mode == "empty" {
			final = " "
		}
		for i, arg := range os.Args {
			if arg == "--output-last-message" && mode != "missing" {
				if err := os.WriteFile(os.Args[i+1], []byte(final), 0600); err != nil {
					os.Exit(13)
				}
			}
		}
		_ = os.WriteFile(filepath.Join(home, "private-session"), []byte("private reasoning"), 0600)
		return
	}
	os.Exit(m.Run())
}

func setup(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("VICOHA_CODEX_COMMAND", executable)
	t.Setenv("VICOHA_CODEX_IMPLEMENT_MODEL", "implement-model")
	t.Setenv("VICOHA_CODEX_REVIEW_MODEL", "review-model")
	t.Setenv("VICOHA_AGENT_ROLE", "review")
	t.Setenv("VICOHA_CODEX_VERIFY_DISCOVERY_MODEL", "discovery-model")
	t.Setenv("VICOHA_CODEX_VERIFY_DISCOVERY_EFFORT", "low")
	t.Setenv("VICOHA_CODEX_IMPLEMENT_EFFORT", "medium")
	t.Setenv("VICOHA_CODEX_REVIEW_EFFORT", "low")
	t.Setenv("VICOHA_TEST_CODEX", "success")
	t.Setenv("VICOHA_TEST_CODEX_LOG", log)
	t.Setenv("CODEX_API_KEY", "test-key")
	return log
}

func TestFreshRoleExecutions(t *testing.T) {
	log := setup(t)
	// Only credentials may cross from the caller's Codex home.
	source := t.TempDir()
	for _, name := range []string{"auth.json", "config.toml", "AGENTS.md", "private-session"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte("test fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CODEX_HOME", source)
	t.Setenv("CODEX_API_KEY", "")
	prompts := []string{
		"Implement wording can change completely",
		"Review wording can change completely",
		harness.RepairPrompt("implement skill", "task", "rules", "diff", harness.Review{}),
		"Review wording can change completely",
	}
	prompts = append(prompts, "Entirely different discovery wording")
	roles := []string{"implement", "review", "implement", "review", "verify-discovery"}
	for i, prompt := range prompts {
		t.Setenv("VICOHA_AGENT_ROLE", roles[i])
		var out, diagnostics bytes.Buffer
		if err := run(context.Background(), strings.NewReader(prompt), &out, &diagnostics); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "private") || !strings.Contains(diagnostics.String(), "private Codex progress") {
			t.Fatalf("stdout=%s stderr=%s", &out, &diagnostics)
		}
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	homes := map[string]bool{}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != len(prompts) {
		t.Fatalf("calls: %s", b)
	}
	for i, line := range lines {
		var call invocation
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			t.Fatal(err)
		}
		if call.Prompt != prompts[i] || call.Cwd != cwd || homes[call.Home] || call.Home == source {
			t.Fatalf("incorrect execution isolation: %+v", call)
		}
		homes[call.Home] = true
		if _, err := os.Stat(call.Home); !os.IsNotExist(err) {
			t.Fatalf("temporary state was not cleaned: %s", call.Home)
		}
		files := "auth.json"
		if roles[i] == "review" {
			files += ",review.schema.json"
		}
		if roles[i] == "verify-discovery" {
			files += ",verify-discovery.schema.json"
		}
		if strings.Join(call.Files, ",") != files {
			t.Fatalf("leaked state: %v", call.Files)
		}
		model, sandbox, effort := "implement-model", "workspace-write", "medium"
		if i%2 == 1 {
			model, sandbox, effort = "review-model", "read-only", "low"
		}
		if roles[i] == "verify-discovery" {
			model, sandbox, effort = "discovery-model", "read-only", "low"
		}
		want := []string{"exec", "--ephemeral", "--model", model, "--sandbox", sandbox,
			"-c", `approval_policy="never"`, "--color", "never", "--output-last-message",
			filepath.Join(call.Home, "last-message.txt")}
		if runtime.GOOS == "windows" {
			want = append(want, "-c", `windows.sandbox="unelevated"`)
		}
		if i%2 == 1 {
			want = append(want, "--output-schema", filepath.Join(call.Home, "review.schema.json"))
		}
		if roles[i] == "verify-discovery" {
			want = append(want, "--output-schema", filepath.Join(call.Home, "verify-discovery.schema.json"))
		}
		want = append(want, "-c", `model_reasoning_effort="`+effort+`"`, "-")
		if strings.Join(call.Args, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("args: %q, want %q", call.Args, want)
		}
	}
}

func TestFailures(t *testing.T) {
	for _, mode := range []string{"failure", "malformed", "empty", "missing", "no-model", "unknown-role", "shell", "relative", "no-auth"} {
		t.Run(mode, func(t *testing.T) {
			setup(t)
			t.Setenv("VICOHA_TEST_CODEX", mode)
			prompt := harness.ReviewPrompt("skill", "task", "rules", "diff")
			switch mode {
			case "no-model":
				t.Setenv("VICOHA_CODEX_REVIEW_MODEL", "")
			case "unknown-role":
				t.Setenv("VICOHA_AGENT_ROLE", "unknown")
			case "shell":
				t.Setenv("VICOHA_CODEX_COMMAND", "codex.cmd")
			case "relative":
				t.Setenv("VICOHA_CODEX_COMMAND", "./codex")
			case "no-auth":
				t.Setenv("CODEX_API_KEY", "")
				t.Setenv("CODEX_HOME", t.TempDir())
			}
			var out, diagnostics bytes.Buffer
			if err := run(context.Background(), strings.NewReader(prompt), &out, &diagnostics); err == nil || out.Len() != 0 {
				t.Fatalf("error=%v stdout=%s", err, &out)
			}
		})
	}
}
