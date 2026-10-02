package result

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
	Command string
	Status  string
	Output  string
}

type Result struct {
	Status       Status
	Message      string
	Verification []Verification
}
