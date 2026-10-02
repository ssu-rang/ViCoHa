package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"vicoha/internal/result"
	"vicoha/internal/runner"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vicoha", flag.ContinueOnError)
	fs.SetOutput(stderr)
	implement := fs.String("implement-command", os.Getenv("VICOHA_IMPLEMENT_COMMAND"), "agent executable for implementation (or VICOHA_IMPLEMENT_COMMAND)")
	review := fs.String("review-command", os.Getenv("VICOHA_REVIEW_COMMAND"), "independent review executable (or VICOHA_REVIEW_COMMAND)")
	root := fs.String("repo", ".", "repository directory")
	skills := fs.String("skills-dir", "skills", "ViCoHa skills directory (independent of --repo)")
	limit := fs.Int("max-iterations", 2, "maximum review passes (must be at least 1)")
	jsonOutput := fs.Bool("json", false, "write the structured workflow result as JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	task := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(task) == "" || *implement == "" || *review == "" || *limit < 1 {
		fmt.Fprintln(stderr, "usage: vicoha [--json] --implement-command PATH --review-command PATH [--repo DIR] [--skills-dir DIR] [--max-iterations N>=1] <task>")
		return 2
	}
	res := runner.Run(context.Background(), *root, *skills, task, *implement, *review, *limit)
	if *jsonOutput {
		if err := json.NewEncoder(stdout).Encode(res); err != nil {
			fmt.Fprintln(stderr, "write result:", err)
			return 2
		}
	} else {
		fmt.Fprintf(stdout, "status: %s\n%s\n", res.Status, res.Message)
		for _, check := range res.Verification {
			fmt.Fprintf(stdout, "verification: %s: %s\n", check.Command, check.Status)
		}
	}
	if res.Status != result.Succeeded {
		return 1
	}
	return 0
}
