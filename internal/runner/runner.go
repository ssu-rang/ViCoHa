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

type Options struct {
	Mode               string
	DiscoveryCommand   string
	DisableAIDiscovery bool
}

func Run(ctx context.Context, root, skillDir, task, implementCommand, reviewCommand string, maxIterations int) result.Result {
	return RunWithOptions(ctx, root, skillDir, task, implementCommand, reviewCommand, maxIterations, Options{})
}

func RunWithOptions(ctx context.Context, root, skillDir, task, implementCommand, reviewCommand string, maxIterations int, options Options) (res result.Result) {
	start := time.Now()
	defer func() { res.DurationMS = time.Since(start).Milliseconds() }()
	res.Reviews = []result.ReviewPass{}
	res.Verification = []result.Verification{}
	if options.Mode == "" {
		options.Mode = "vicoha"
	}
	res.Mode = options.Mode
	fail := func(status result.Status, err error) result.Result {
		res.Status, res.Message = status, err.Error()
		return res
	}
	switch options.Mode {
	case "baseline", "baseline-verify", "independent-review", "vicoha":
	default:
		return fail(result.ImplementationFailed, fmt.Errorf("unsupported workflow mode %q", options.Mode))
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
	reviewSkill := ""
	if options.Mode == "vicoha" || options.Mode == "independent-review" {
		reviewSkill, err = harness.ReadSkill(skillDir, "review")
		if err != nil {
			return fail(result.ReviewFailed, err)
		}
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
	finish := func() result.Result {
		if options.Mode == "baseline" || options.Mode == "independent-review" {
			res.Status, res.Message = result.Succeeded, "selected workflow stages passed (verification not run)"
			return res
		}
		discovered, err := verify.Discover(root)
		if err != nil {
			return fail(result.VerificationFailed, err)
		}
		commands := discovered.Commands
		if !discovered.Confident {
			if options.Mode == "baseline-verify" || options.DisableAIDiscovery {
				return fail(result.VerificationFailed, fmt.Errorf("deterministic discovery insufficient and AI discovery disabled"))
			}
			command := options.DiscoveryCommand
			if command == "" {
				command = reviewCommand
			}
			before, err := project.Snapshot(root, base)
			if err != nil {
				return fail(result.VerificationFailed, err)
			}
			res.AgentInvocations++
			res.DiscoveryInvocations++
			out, runErr := (agent.Exec{Command: command}).Run(ctx, agent.Request{Root: root, Role: "verify-discovery", Prompt: harness.DiscoveryPrompt(discovered.Evidence)})
			after, err := project.Snapshot(root, base)
			if err != nil {
				return fail(result.VerificationFailed, err)
			}
			if before != after {
				return fail(result.VerificationFailed, fmt.Errorf("verification-discovery agent modified repository state"))
			}
			if runErr != nil {
				return fail(result.VerificationFailed, fmt.Errorf("verification-discovery agent: %w", runErr))
			}
			proposals, err := harness.ParseDiscovery(out)
			if err != nil {
				return fail(result.VerificationFailed, fmt.Errorf("invalid verification-discovery response: %w", err))
			}
			commands, err = verify.FromProposals(root, proposals)
			if err != nil {
				return fail(result.VerificationFailed, err)
			}
			verify.MarkDeclared(commands, discovered.Commands)
		}
		checks, err := verify.Execute(ctx, root, commands)
		res.Verification = append(res.Verification, checks...)
		res.VerificationCount = len(res.Verification)
		if err != nil {
			return fail(result.VerificationFailed, err)
		}
		res.Status, res.Message = result.Succeeded, "selected workflow stages and deterministic verification passed"
		return res
	}
	res.AgentInvocations++
	if _, err := implement.Run(ctx, agent.Request{Root: root, Role: "implement", Prompt: harness.ImplementPrompt(implementSkill, task, rules, diff)}); err != nil {
		return fail(result.ImplementationFailed, fmt.Errorf("implement agent: %w", err))
	}
	if options.Mode == "baseline" || options.Mode == "baseline-verify" {
		return finish()
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
		out, runErr := reviewer.Run(ctx, agent.Request{Root: root, Role: "review", Prompt: harness.ReviewPrompt(reviewSkill, task, rules, before.Diff)})
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
			return finish()
		}
		if pass == maxIterations {
			return fail(result.IterationLimit, fmt.Errorf("iteration limit reached with %d actionable finding(s): %s", len(actionable), harness.FormatFindings(actionable)))
		}
		res.RepairPasses++
		res.AgentInvocations++
		if _, err := implement.Run(ctx, agent.Request{Root: root, Role: "implement", Prompt: harness.RepairPrompt(implementSkill, task, rules, before.Diff, report)}); err != nil {
			return fail(result.ImplementationFailed, fmt.Errorf("implement agent repair: %w", err))
		}
	}
	panic("unreachable: positive review limit always returns from the loop")
}
