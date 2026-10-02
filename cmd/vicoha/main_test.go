package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"vicoha/internal/result"
)

func TestOutput(t *testing.T) {
	for _, jsonMode := range []bool{true, false} {
		var stdout, stderr bytes.Buffer
		args := []string{"--implement-command", "unused", "--review-command", "unused", "--skills-dir", filepath.Join(t.TempDir(), "missing"), "task"}
		if jsonMode {
			args = append([]string{"--json"}, args...)
		}
		if code := run(args, &stdout, &stderr); code != 1 || stderr.Len() != 0 {
			t.Fatalf("exit=%d stderr=%s", code, &stderr)
		}
		if jsonMode {
			var res result.Result
			if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
				t.Fatal(err)
			}
			if res.Status != result.ImplementationFailed || res.Reviews == nil || res.Verification == nil {
				t.Fatalf("result=%+v", res)
			}
		} else if !strings.HasPrefix(stdout.String(), "status: implementation_failed\n") {
			t.Fatal(stdout.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--json", "--max-iterations", "0"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("invalid command: exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
}
