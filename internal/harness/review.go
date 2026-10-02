package harness

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Finding struct {
	Title      string `json:"title"`
	Category   string `json:"category"`
	Severity   string `json:"severity"`
	File       string `json:"file,omitempty"`
	Actionable *bool  `json:"actionable"`
	Details    string `json:"details"`
}
type Review struct {
	Findings []Finding `json:"findings"`
}

// ParseReview uses exact JSON property names, matching review.schema.json.
func ParseReview(out string) (Review, error) {
	var report Review
	var object map[string]json.RawMessage
	if err := StrictJSON(out, &object); err != nil {
		return report, err
	}
	if len(object) != 1 || object["findings"] == nil {
		return report, fmt.Errorf("review requires only the findings property")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(object["findings"], &items); err != nil {
		return report, err
	}
	if items == nil {
		return report, fmt.Errorf("findings must be a non-null array")
	}
	report.Findings = make([]Finding, 0, len(items))
	for i, item := range items {
		var properties map[string]json.RawMessage
		if err := json.Unmarshal(item, &properties); err != nil {
			return report, err
		}
		if err := fields(properties, []string{"title", "category", "severity", "actionable", "details"}, "file"); err != nil {
			return report, fmt.Errorf("finding %d: %w", i+1, err)
		}
		var f Finding
		if err := json.Unmarshal(item, &f); err != nil {
			return report, err
		}
		if strings.TrimSpace(f.Title) == "" || strings.TrimSpace(f.Severity) == "" || strings.TrimSpace(f.Details) == "" || f.Actionable == nil {
			return report, fmt.Errorf("finding %d requires non-empty title, severity, details and boolean actionable", i+1)
		}
		switch f.Category {
		case "functional_defect", "regression", "overengineering", "temporary_fix", "resource_waste", "excessive_tests", "scope_creep", "architecture_drift", "integration_problem", "other":
		default:
			return report, fmt.Errorf("finding %d has invalid category", i+1)
		}
		switch f.Severity {
		case "low", "medium", "high", "critical":
		default:
			return report, fmt.Errorf("finding %d has invalid severity", i+1)
		}
		if properties["file"] != nil && (string(properties["file"]) == "null" || strings.TrimSpace(f.File) == "") {
			return report, fmt.Errorf("file must be non-empty when present")
		}
		report.Findings = append(report.Findings, f)
	}
	return report, nil
}

func FormatFindings(findings []Finding) string {
	b, _ := json.MarshalIndent(findings, "", "  ")
	return string(b)
}

func (r Review) Actionable() []Finding {
	out := make([]Finding, 0)
	for _, f := range r.Findings {
		if f.Actionable != nil && *f.Actionable {
			out = append(out, f)
		}
	}
	return out
}
