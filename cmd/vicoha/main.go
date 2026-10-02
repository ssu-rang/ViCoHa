package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"vicoha/internal/runner"
)

func main() {
	fs := flag.NewFlagSet("vicoha", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	implement := fs.String("implement-command", os.Getenv("VICOHA_IMPLEMENT_COMMAND"), "agent executable for implementation (or VICOHA_IMPLEMENT_COMMAND)")
	review := fs.String("review-command", os.Getenv("VICOHA_REVIEW_COMMAND"), "independent review executable (or VICOHA_REVIEW_COMMAND)")
	root := fs.String("repo", ".", "repository directory")
	skills := fs.String("skills-dir", "skills", "ViCoHa skills directory (independent of --repo)")
	limit := fs.Int("max-iterations", 2, "maximum review passes (must be at least 1)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		os.Exit(2)
	}
	task := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(task) == "" || *implement == "" || *review == "" || *limit < 1 {
		fmt.Fprintln(os.Stderr, "usage: vicoha --implement-command PATH --review-command PATH [--repo DIR] [--skills-dir DIR] [--max-iterations N>=1] <task>")
		os.Exit(2)
	}
	result := runner.Run(*root, *skills, task, *implement, *review, *limit)
	fmt.Printf("status: %s\n", result.Status)
	if result.Message != "" {
		fmt.Println(result.Message)
	}
	for _, check := range result.Verification {
		fmt.Printf("verification: %s: %s\n", check.Command, check.Status)
	}
	if result.Status != runner.Succeeded {
		os.Exit(1)
	}
}
