package harness

import (
	"strings"
	"testing"
)

func TestReviewContract(t *testing.T) {
	for _, input := range []string{`null`, `{}`, `{"findings":null}`, `{"findings":[{}]}`, `{"findings":[],"unexpected":true}`, `{"findings":[]} {}`, `{"findings":[{"title":"bug","category":"functional_defect","severity":"high","details":"defect"}]}`, `{"findings":[{"title":"bug","category":"functional_defect","severity":"high","actionable":null,"details":"defect"}]}`} {
		if _, err := ParseReview(input); err == nil {
			t.Errorf("accepted invalid review: %s", input)
		}
	}
	for _, input := range []string{`{"findings":[]}`, `{"findings":[{"title":"bug","category":"functional_defect","severity":"high","actionable":false,"details":"defect"}]}`} {
		if _, err := ParseReview(input); err != nil {
			t.Errorf("rejected valid review: %v", err)
		}
	}
}

func TestStrictFindingFieldsAndRepair(t *testing.T) {
	valid := `{"title":"bug","category":"functional_defect","severity":"high","actionable":true,"details":"concrete defect"}`
	for _, finding := range []string{
		strings.Replace(valid, `"category":"functional_defect",`, "", 1),
		strings.Replace(valid, `"functional_defect"`, `"unknown"`, 1),
		strings.Replace(valid, `"functional_defect"`, `"functional_defect|regression"`, 1),
		strings.Replace(valid, `"high"`, `"urgent"`, 1),
		strings.Replace(valid, `"title":`, `"file":null,"title":`, 1),
		strings.Replace(valid, `"title":`, `"file":" ","title":`, 1),
		strings.Replace(valid, `"title":`, `"title":"duplicate","title":`, 1),
		strings.Replace(valid, `"title":"bug",`, "", 1),
		strings.Replace(valid, `"category":"functional_defect","severity":"high",`, "", 1),
		strings.Replace(valid, `,"details":"concrete defect"`, "", 1),
		strings.Replace(valid, `"title"`, `"Title"`, 1),
		strings.Replace(valid, `"bug"`, `"  "`, 1),
		strings.Replace(valid, `true`, `"true"`, 1),
		strings.Replace(valid, `"title":`, `"unknown":0,"title":`, 1),
	} {
		if _, err := ParseReview(`{"findings":[` + finding + `]}`); err == nil {
			t.Errorf("accepted %s", finding)
		}
	}
	if _, err := ParseReview(`{"Findings":[]}`); err == nil {
		t.Fatal("accepted unknown case variant")
	}
	report, err := ParseReview(`{"findings":[` + valid + `,{"title":"speculation","category":"functional_defect","severity":"low","actionable":false,"details":"unrelated suggestion"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Actionable()) != 1 {
		t.Fatal("incorrect actionable filtering")
	}
	prompt := RepairPrompt("skill", "original task", "rules", "diff", report)
	for _, required := range []string{"skill", "original task", "rules", "diff", "concrete defect"} {
		if !strings.Contains(prompt, required) {
			t.Errorf("missing %s", required)
		}
	}
	if strings.Contains(prompt, "speculation") || strings.Contains(prompt, "unrelated suggestion") {
		t.Fatal("repair leaked non-actionable findings")
	}
}
