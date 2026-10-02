package result

import "vicoha/internal/harness"

type Status string

const (
	Succeeded            Status = "success"
	ImplementationFailed Status = "implementation_failed"
	ReviewFailed         Status = "review_failed"
	VerificationFailed   Status = "verification_failed"
	FindingsRemain       Status = "actionable_findings"
	IterationLimit       Status = "iteration_limit"
)

type Verification struct {
	Command string `json:"command"`
	Status  string `json:"status"`
	Output  string `json:"output"`
}

type ReviewPass struct {
	Pass     int               `json:"pass"`
	Findings []harness.Finding `json:"findings"`
}

// Counts include attempted invocations, including those that fail.
// Reviews contains only successfully parsed, read-only review responses.
type Result struct {
	Status           Status         `json:"status"`
	Message          string         `json:"message"`
	ReviewPasses     int            `json:"review_passes"`
	RepairPasses     int            `json:"repair_passes"`
	AgentInvocations int            `json:"agent_invocations"`
	DurationMS       int64          `json:"duration_ms"`
	Reviews          []ReviewPass   `json:"reviews"`
	Verification     []Verification `json:"verification"`
}
