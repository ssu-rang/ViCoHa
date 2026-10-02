package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Exec consumes a prompt on stdin. Paths with separators are relative to the
// caller, not the target repository. Provider flags belong in an adapter.
type Exec struct{ Command string }

type Request struct{ Root, Prompt string }

func (a Exec) Run(ctx context.Context, req Request) (string, error) {
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
	cmd.Stdin = strings.NewReader(req.Prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
