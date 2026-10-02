package harness

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Proposal struct {
	Argv     []string `json:"argv"`
	Reason   string   `json:"reason"`
	Evidence []string `json:"evidence"`
}

func ParseDiscovery(out string) ([]Proposal, error) {
	var object map[string]json.RawMessage
	if err := StrictJSON(out, &object); err != nil {
		return nil, err
	}
	if err := fields(object, []string{"commands"}); err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(object["commands"], &items); err != nil {
		return nil, err
	}
	proposals := make([]Proposal, 0, len(items))
	for _, item := range items {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(item, &object); err != nil {
			return nil, err
		}
		if err := fields(object, []string{"argv", "reason", "evidence"}); err != nil {
			return nil, err
		}
		var p Proposal
		if err := json.Unmarshal(item, &p); err != nil {
			return nil, err
		}
		if len(p.Argv) == 0 || len(p.Evidence) == 0 || strings.TrimSpace(p.Reason) == "" {
			return nil, fmt.Errorf("command requires argv, reason and evidence")
		}
		for _, s := range append(append([]string{}, p.Argv...), p.Evidence...) {
			if strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("argv and evidence entries must be non-empty")
			}
		}
		proposals = append(proposals, p)
	}
	return proposals, nil
}

func DiscoveryPrompt(evidence string) string {
	return `Determine the repository's existing verification commands. This is a fresh,
read-only verification-discovery role, independent of implementation and review.
Use repository evidence only. Inspect additional repository files when needed.
Priority: documented verification entrypoints, existing CI commands, existing
build-tool/package-manager commands, then conventions justified by repository evidence.
Do not invent infrastructure, install dependencies, create tests, modify files or
configuration, execute verification, use network operations, sudo, destructive
commands or arbitrary shell pipelines. Proposed verification commands must not
use shell interpreters. Do not seek agent logs or session state.
You select commands; the runner alone determines pass/fail from process results.
Return exactly one JSON object, no prose or fences:
{"commands":[{"argv":["tool","test"],"reason":"Brief evidence-based justification","evidence":["README.md"]}]}
Only these fields are allowed. argv is a non-empty array of non-empty strings,
never a shell string. reason is non-empty. evidence is a non-empty array of
repository-relative existing file paths. Return {"commands":[]} if insufficient
evidence exists; this will fail verification, not report success.

Repository configuration and guidance:
` + evidence
}
