package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"vicoha/internal/agent"
	"vicoha/internal/harness"
	"vicoha/internal/project"
	"vicoha/internal/result"
	"vicoha/internal/verify"
)

func Run(ctx context.Context, root, skillDir, task, implementCommand, reviewCommand string, maxIterations int) (res result.Result) {
	start := time.Now()
	defer func() { res.DurationMS = time.Since(start).Milliseconds() }()
	res.Reviews = []result.ReviewPass{}
	res.Verification = []result.Verification{}
	fail := func(status result.Status, err error) result.Result {
		res.Status, res.Message = status, err.Error()
		return res
	}
	if maxIterations < 1 {
		return fail(result.ImplementationFailed, fmt.Errorf("max-iterations must be at least 1"))
	}
	if strings.TrimSpace(task) == "" {
		return fail(result.ImplementationFailed, fmt.Errorf("task must not be empty"))
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return fail(result.ImplementationFailed, err)
	}
	implementSkill, err := harness.ReadSkill(skillDir, "implement")
	if err != nil {
		return fail(result.ImplementationFailed, err)
	}
	reviewSkill, err := harness.ReadSkill(skillDir, "review")
	if err != nil {
		return fail(result.ReviewFailed, err)
	}
	base, err := project.Revision(root)
	if err != nil {
		return fail(result.ImplementationFailed, err)
	}
	rules, err := project.Context(root)
	if err != nil {
		return fail(result.ImplementationFailed, err)
	}
	diff, err := project.Diff(root, base)
	if err != nil {
		return fail(result.ImplementationFailed, err)
	}
	implement, reviewer := agent.Exec{Command: implementCommand}, agent.Exec{Command: reviewCommand}
	res.AgentInvocations++
	if _, err := implement.Run(ctx, agent.Request{Root: root, Prompt: harness.ImplementPrompt(implementSkill, task, rules, diff)}); err != nil {
		return fail(result.ImplementationFailed, fmt.Errorf("implement agent: %w", err))
	}
	for pass := 1; pass <= maxIterations; pass++ {
		before, err := project.Snapshot(root, base)
		if err != nil {
			return fail(result.ReviewFailed, err)
		}
		rules, err = project.Context(root)
		if err != nil {
			return fail(result.ReviewFailed, err)
		}
		res.ReviewPasses++
		res.AgentInvocations++
		out, runErr := reviewer.Run(ctx, agent.Request{Root: root, Prompt: harness.ReviewPrompt(reviewSkill, task, rules, before.Diff)})
		after, err := project.Snapshot(root, base)
		if err != nil {
			return fail(result.ReviewFailed, err)
		}
		if before != after {
			return fail(result.ReviewFailed, fmt.Errorf("review agent modified repository state"))
		}
		if runErr != nil {
			return fail(result.ReviewFailed, fmt.Errorf("review agent: %w", runErr))
		}
		report, err := harness.ParseReview(out)
		if err != nil {
			return fail(result.ReviewFailed, fmt.Errorf("invalid review response: %w", err))
		}
		res.Reviews = append(res.Reviews, result.ReviewPass{Pass: pass, Findings: report.Findings})
		actionable := report.Actionable()
		if len(actionable) == 0 {
			checks, err := verify.Run(root)
			res.Verification = append(res.Verification, checks...)
			if err != nil {
				return fail(result.VerificationFailed, err)
			}
			res.Status, res.Message = result.Succeeded, "review and deterministic verification passed"
			return res
		}
		if pass == maxIterations {
			return fail(result.IterationLimit, fmt.Errorf("iteration limit reached with %d actionable finding(s): %s", len(actionable), harness.FormatFindings(actionable)))
		}
		res.RepairPasses++
		res.AgentInvocations++
		if _, err := implement.Run(ctx, agent.Request{Root: root, Prompt: harness.RepairPrompt(implementSkill, task, rules, before.Diff, report)}); err != nil {
			return fail(result.ImplementationFailed, fmt.Errorf("implement agent repair: %w", err))
		}
	}
	panic("unreachable: positive review limit always returns from the loop")
}
