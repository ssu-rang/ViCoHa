package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Exec consumes a prompt on stdin. Paths with separators are relative to the
// caller, not the target repository. Provider flags belong in an adapter.
type Exec struct{ Command string }

type Request struct{ Root, Prompt, Role string }

func (a Exec) Run(ctx context.Context, req Request) (string, error) {
	switch req.Role {
	case "implement", "review", "verify-discovery":
	default:
		return "", fmt.Errorf("invalid agent role %q", req.Role)
	}
	executable := a.Command
	if strings.ContainsAny(executable, `/\`) && !filepath.IsAbs(executable) {
		path, err := filepath.Abs(executable)
		if err != nil {
			return "", err
		}
		executable = path
	}
	cmd := exec.CommandContext(ctx, executable)
	cmd.Dir = req.Root
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "VICOHA_AGENT_ROLE") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "VICOHA_AGENT_ROLE="+req.Role)
	cmd.Stdin = strings.NewReader(req.Prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
