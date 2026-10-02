package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vicoha/internal/project"
	"vicoha/internal/result"
)

type Status = result.Status

const (
	Succeeded            = result.Succeeded
	ImplementationFailed = result.ImplementationFailed
	ReviewFailed         = result.ReviewFailed
	VerificationFailed   = result.VerificationFailed
	FindingsRemain       = result.FindingsRemain
	IterationLimit       = result.IterationLimit
)

type RunResult = result.Result
type Check = result.Verification

type finding struct {
	Title      string `json:"title"`
	Severity   string `json:"severity"`
	Actionable *bool  `json:"actionable"`
	Details    string `json:"details"`
}
type review struct {
	Findings []finding `json:"findings"`
}

func Run(root, skillDir, task, implementCommand, reviewCommand string, maxIterations int) RunResult {
	if maxIterations < 1 {
		return RunResult{Status: ImplementationFailed, Message: "max-iterations must be at least 1"}
	}
	if strings.TrimSpace(task) == "" {
		return RunResult{Status: ImplementationFailed, Message: "task must not be empty"}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return RunResult{Status: ImplementationFailed, Message: err.Error()}
	}
	implementSkill, err := readSkill(skillDir, "implement")
	if err != nil {
		return RunResult{Status: ImplementationFailed, Message: err.Error()}
	}
	reviewSkill, err := readSkill(skillDir, "review")
	if err != nil {
		return RunResult{Status: ReviewFailed, Message: err.Error()}
	}
	base, err := project.Revision(root)
	if err != nil {
		return RunResult{Status: ImplementationFailed, Message: err.Error()}
	}
	context, err := project.Context(root)
	if err != nil {
		return RunResult{Status: ImplementationFailed, Message: err.Error()}
	}
	diff, err := project.Diff(root, base)
	if err != nil {
		return RunResult{Status: ImplementationFailed, Message: err.Error()}
	}
	implementPrompt := fmt.Sprintf("You are the Implement Agent. Follow this skill exactly:\n%s\n\nOriginal user task:\n%s\n\nRepository context and project rules:\n%s\n\nCurrent git diff:\n%s\n", implementSkill, task, context, diff)
	if _, err := invoke(root, implementCommand, implementPrompt); err != nil {
		return RunResult{Status: ImplementationFailed, Message: "implement agent: " + err.Error()}
	}
	for iteration := 1; iteration <= maxIterations; iteration++ {
		// Review receives the task, final diff, repository rules and review skill only.
		// No implement prompt, output, or private reasoning is included.
		diff, err = project.Diff(root, base)
		if err != nil {
			return RunResult{Status: ReviewFailed, Message: err.Error()}
		}
		context, err = project.Context(root)
		if err != nil {
			return RunResult{Status: ReviewFailed, Message: err.Error()}
		}
		reviewPrompt := fmt.Sprintf("You are an independent Review Agent. Follow this skill exactly:\n%s\n\nOriginal user task:\n%s\n\nRepository context and project rules:\n%s\n\nFinal git diff to review:\n%s\n\nReview is read-only: do not modify repository files. Return only JSON matching {\"findings\":[{\"title\":string,\"severity\":string,\"actionable\":boolean,\"details\":string}]}. Every field is required. Include only concrete defects within task scope.", reviewSkill, task, context, diff)
		out, err := invoke(root, reviewCommand, reviewPrompt)
		if err != nil {
			return RunResult{Status: ReviewFailed, Message: "review agent: " + err.Error()}
		}
		afterReview, err := project.Diff(root, base)
		if err != nil {
			return RunResult{Status: ReviewFailed, Message: err.Error()}
		}
		if afterReview != diff {
			return RunResult{Status: ReviewFailed, Message: "review agent modified repository files"}
		}
		report, err := parseReview(out)
		if err != nil {
			return RunResult{Status: ReviewFailed, Message: "invalid review response: " + err.Error()}
		}
		var actionable []finding
		for _, f := range report.Findings {
			if *f.Actionable {
				actionable = append(actionable, f)
			}
		}
		if len(actionable) == 0 {
			checks, err := Verify(root)
			if err != nil {
				return RunResult{Status: VerificationFailed, Message: err.Error(), Verification: checks}
			}
			return RunResult{Status: Succeeded, Message: "review and deterministic verification passed", Verification: checks}
		}
		if iteration == maxIterations {
			return RunResult{Status: IterationLimit, Message: fmt.Sprintf("iteration limit reached with %d actionable finding(s): %s", len(actionable), formatFindings(actionable))}
		}
		repairPrompt := fmt.Sprintf("You are the Implement Agent continuing the original task. Follow this implement skill:\n%s\n\nOriginal user task:\n%s\n\nCurrent repository context and project rules:\n%s\n\nCurrent git diff:\n%s\n\nValidated actionable review findings:\n%s\n\nFix only findings that are concrete and within the original task scope. Ignore speculative or out-of-scope suggestions. Make the changes in the repository.", implementSkill, task, context, diff, formatFindings(actionable))
		if _, err := invoke(root, implementCommand, repairPrompt); err != nil {
			return RunResult{Status: ImplementationFailed, Message: "implement agent repair: " + err.Error()}
		}
	}
	return RunResult{Status: IterationLimit, Message: "review iteration limit reached"}
}

func readSkill(skillDir, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(skillDir, name, "SKILL.md"))
	if err != nil {
		return "", fmt.Errorf("read %s skill: %w", name, err)
	}
	return string(b), nil
}

func invoke(root, executable, prompt string) (string, error) {
	if strings.ContainsAny(executable, `/\\`) && !filepath.IsAbs(executable) {
		path, err := filepath.Abs(executable)
		if err != nil {
			return "", err
		}
		executable = path
	}
	cmd := exec.Command(executable)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func parseReview(out string) (review, error) {
	var report review
	decoder := json.NewDecoder(strings.NewReader(out))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return report, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return report, fmt.Errorf("expected exactly one JSON object")
	}
	if report.Findings == nil {
		return report, fmt.Errorf("findings must be a non-null array")
	}
	for i, f := range report.Findings {
		if strings.TrimSpace(f.Title) == "" || strings.TrimSpace(f.Severity) == "" || strings.TrimSpace(f.Details) == "" || f.Actionable == nil {
			return report, fmt.Errorf("finding %d requires non-empty title, severity, details and boolean actionable", i+1)
		}
	}
	return report, nil
}

func formatFindings(findings []finding) string {
	b, _ := json.MarshalIndent(findings, "", "  ")
	return string(b)
}
