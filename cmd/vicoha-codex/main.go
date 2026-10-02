// vicoha-codex adapts Codex CLI to ViCoHa's stdin/stdout executable contract.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"vicoha/internal/harness"
	"vicoha/schemas"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "vicoha-codex takes a ViCoHa prompt on stdin; configure it with VICOHA_CODEX_* environment variables")
		os.Exit(2)
	}
	if err := run(context.Background(), os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "vicoha-codex:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	prompt, err := io.ReadAll(stdin)
	if err != nil {
		return err
	}
	role := ""
	switch os.Getenv("VICOHA_AGENT_ROLE") {
	case "implement":
		role = "IMPLEMENT"
	case "review":
		role = "REVIEW"
	case "verify-discovery":
		role = "VERIFY_DISCOVERY"
	default:
		return fmt.Errorf("VICOHA_AGENT_ROLE must be implement, review or verify-discovery")
	}
	modelKey := "VICOHA_CODEX_" + role + "_MODEL"
	model := strings.TrimSpace(os.Getenv(modelKey))
	if model == "" {
		return fmt.Errorf("%s must specify a model ID", modelKey)
	}
	command := os.Getenv("VICOHA_CODEX_COMMAND")
	if command == "" {
		command = "codex"
	}
	if strings.ContainsAny(command, `/\`) && !filepath.IsAbs(command) {
		return fmt.Errorf("VICOHA_CODEX_COMMAND must be an absolute path or a PATH command")
	}
	command, err = exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("locate Codex executable: %w", err)
	}
	// Use a native executable on Windows, avoiding shell parsing of model IDs.
	if strings.EqualFold(filepath.Ext(command), ".cmd") || strings.EqualFold(filepath.Ext(command), ".ps1") {
		return fmt.Errorf("VICOHA_CODEX_COMMAND must be a native executable; use the installed codex.exe on Windows")
	}
	// Never share sessions, memories, logs, or user instructions between calls.
	// Only file-based login credentials are copied; CODEX_API_KEY is inherited.
	home, err := os.MkdirTemp("", "vicoha-codex-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(home)
	if os.Getenv("CODEX_API_KEY") == "" {
		source := os.Getenv("CODEX_HOME")
		if source == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("locate Codex credentials: %w", err)
			}
			source = filepath.Join(userHome, ".codex")
		}
		auth, err := os.ReadFile(filepath.Join(source, "auth.json"))
		if err != nil {
			return fmt.Errorf("set CODEX_API_KEY or use a Codex file-based login: %w", err)
		}
		if err := os.WriteFile(filepath.Join(home, "auth.json"), auth, 0600); err != nil {
			return err
		}
	}
	sandbox := "workspace-write"
	if role != "IMPLEMENT" {
		sandbox = "read-only"
	}
	output := filepath.Join(home, "last-message.txt")
	args := []string{"exec", "--ephemeral", "--model", model, "--sandbox", sandbox,
		"-c", `approval_policy="never"`, "--color", "never", "--output-last-message", output}
	if runtime.GOOS == "windows" {
		// A fresh home has no persistent elevated-sandbox user credentials.
		// Keep native sandboxing via the restricted-token implementation.
		args = append(args, "-c", `windows.sandbox="unelevated"`)
	}
	if role != "IMPLEMENT" {
		name, data := "review.schema.json", schemas.Review
		if role == "VERIFY_DISCOVERY" {
			name, data = "verify-discovery.schema.json", schemas.Discovery
		}
		schema := filepath.Join(home, name)
		if err := os.WriteFile(schema, data, 0600); err != nil {
			return err
		}
		args = append(args, "--output-schema", schema)
	}
	if effort := os.Getenv("VICOHA_CODEX_" + role + "_EFFORT"); effort != "" {
		args = append(args, "-c", "model_reasoning_effort="+strconv.Quote(effort))
	}
	args = append(args, "-")
	cmd := exec.CommandContext(ctx, command, args...)
	// Inherit the target cwd and tool environment, replacing any prior home.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "CODEX_HOME") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "CODEX_HOME="+home)
	cmd.Stdin = bytes.NewReader(prompt)
	cmd.Stdout, cmd.Stderr = stderr, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Codex %s execution: %w", strings.ToLower(role), err)
	}
	final, err := os.ReadFile(output)
	if err != nil {
		return fmt.Errorf("read Codex final response: %w", err)
	}
	if len(bytes.TrimSpace(final)) == 0 {
		return fmt.Errorf("Codex returned an empty final response")
	}
	if role == "REVIEW" {
		if _, err := harness.ParseReview(string(final)); err != nil {
			return fmt.Errorf("invalid review response: %w", err)
		}
	}
	if role == "VERIFY_DISCOVERY" {
		if _, err := harness.ParseDiscovery(string(final)); err != nil {
			return fmt.Errorf("invalid verification-discovery response: %w", err)
		}
	}
	_, err = stdout.Write(final)
	return err
}
