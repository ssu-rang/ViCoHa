// Package verify separates command selection, validation and process execution.
package verify

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"vicoha/internal/result"
)

type Command struct {
	Argv     []string
	Source   string
	Declared bool
	Evidence []string
}

// Run is the deterministic-only entrypoint. Uncertain discovery fails closed.
func Run(root string) ([]result.Verification, error) {
	d, err := Discover(root)
	if err != nil {
		return nil, err
	}
	if !d.Confident {
		return nil, fmt.Errorf("verification discovery needs repository-specific guidance")
	}
	return Execute(context.Background(), root, d.Commands)
}

// Execute validates the entire selection before starting anything, stops at the
// first failure, and derives status exclusively from process execution.
func Execute(ctx context.Context, root string, commands []Command) ([]result.Verification, error) {
	if len(commands) == 0 {
		return nil, fmt.Errorf("no verification commands discovered")
	}
	for _, c := range commands {
		if err := Validate(root, c.Argv); err != nil {
			return nil, err
		}
	}
	checks := make([]result.Verification, 0, len(commands))
	for _, c := range commands {
		args := c.Argv
		executable := args[0]
		if strings.ContainsAny(executable, `/\`) {
			executable = filepath.Join(root, executable)
		}
		cmd := exec.CommandContext(ctx, executable, args[1:]...)
		// Only validated simple arguments cross the Windows shim shell boundary.
		if windowsShim(args[0]) {
			shimArgs := append([]string{"/d", "/c", filepath.FromSlash(args[0])}, args[1:]...)
			cmd = exec.CommandContext(ctx, "cmd.exe", shimArgs...)
		}
		cmd.Dir = root
		start := time.Now()
		out, err := cmd.CombinedOutput()
		check := result.Verification{Command: strings.Join(args, " "), Argv: args, Source: c.Source, RepositoryDeclared: c.Declared, Evidence: c.Evidence, Status: "passed", Output: string(out), DurationMS: time.Since(start).Milliseconds()}
		if cmd.ProcessState != nil {
			code := cmd.ProcessState.ExitCode()
			check.ExitCode = &code
		}
		if err != nil {
			check.Status = "failed"
		}
		checks = append(checks, check)
		if err != nil {
			return checks, fmt.Errorf("verification %q failed: %w\n%s", check.Command, err, strings.TrimSpace(string(out)))
		}
	}
	return checks, nil
}
