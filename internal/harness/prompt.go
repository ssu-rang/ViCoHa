package harness

import "fmt"

func ImplementPrompt(skill, task, context, diff string) string {
	return fmt.Sprintf("You are the Implement Agent. Follow this skill exactly:\n%s\n\nOriginal user task:\n%s\n\nRepository context and project rules:\n%s\n\nCurrent git diff:\n%s\n", skill, task, context, diff)
}

// Review input is reconstructed from task and project state, never implement output.
func ReviewPrompt(skill, task, context, diff string) string {
	return fmt.Sprintf("You are an independent Review Agent. Follow this skill exactly:\n%s\n\nOriginal user task:\n%s\n\nRepository context and project rules:\n%s\n\nFinal git diff to review:\n%s\n\nReview is read-only: do not modify repository files, stage changes, or create commits. Return exactly one JSON object matching {\"findings\":[{\"title\":string,\"category\":string,\"severity\":string,\"file\":string,\"actionable\":boolean,\"details\":string}]}. file is optional; other fields are required. Text must be non-empty and unknown fields are forbidden. category must be one of functional_defect, regression, overengineering, temporary_fix, resource_waste, excessive_tests, scope_creep, architecture_drift, integration_problem, other. severity must be low, medium, high or critical. findings must be a non-null array. Mark actionable only concrete defects within the original task scope; exclude speculative suggestions.", skill, task, context, diff)
}

func RepairPrompt(skill, task, context, diff string, report Review) string {
	return fmt.Sprintf("You are the Implement Agent continuing the original task. Follow this implement skill:\n%s\n\nOriginal user task:\n%s\n\nCurrent repository context and project rules:\n%s\n\nCurrent git diff:\n%s\n\nActionable review findings:\n%s\n\nFix only findings that are concrete and within the original task scope. Ignore speculative or out-of-scope suggestions. Make the changes in the repository.", skill, task, context, diff, FormatFindings(report.Actionable()))
}
