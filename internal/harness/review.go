package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Finding struct {
	Title      string `json:"title"`
	Severity   string `json:"severity"`
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
	decoder := json.NewDecoder(strings.NewReader(out))
	if err := decoder.Decode(&object); err != nil {
		return report, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return report, fmt.Errorf("expected exactly one JSON object")
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
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(item, &fields); err != nil {
			return report, err
		}
		if len(fields) != 4 || fields["title"] == nil || fields["severity"] == nil || fields["actionable"] == nil || fields["details"] == nil {
			return report, fmt.Errorf("finding %d requires exactly title, severity, actionable and details", i+1)
		}
		var f Finding
		if err := json.Unmarshal(item, &f); err != nil {
			return report, err
		}
		if strings.TrimSpace(f.Title) == "" || strings.TrimSpace(f.Severity) == "" || strings.TrimSpace(f.Details) == "" || f.Actionable == nil {
			return report, fmt.Errorf("finding %d requires non-empty title, severity, details and boolean actionable", i+1)
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
