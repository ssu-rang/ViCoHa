# ViCoHa (Vibe Coding Harness)

ViCoHa improves the reliability of AI-assisted coding through independent review,
bounded repairs, and deterministic project checks. It is a small Go CLI that runs
provider-agnostic executable agents in a target Git repository.

## Workflow

1. Capture the initial HEAD and collect project guidance and the current diff.
2. Run the implementation agent with the implement skill and original task.
3. Invoke a separate review agent with the review skill, original task, project
   guidance, and current diff. Implementation stdout, prompt, and private reasoning
   are never forwarded to review.
4. If actionable findings remain, ask the implementation agent to repair them
   within the original task scope, then review again.
5. After a review with no actionable findings, run deterministic verification.

`--max-iterations` limits **review passes**, defaults to 2, and must be at least 1.
Thus the default permits one initial implementation, two reviews, and one repair.
Remaining actionable findings on the final pass yield `iteration_limit`.
Verification failures terminate the workflow; they do not start another repair loop.

Review is read-only. Changes to HEAD, the index, tracked content, or non-ignored
untracked content during review fail the workflow. This is a Git state check,
not an OS sandbox: ignored artifacts, external files, and transient changes that
are restored before review exits are outside its detection boundary. Agents run
with the caller's permissions. Detected mutations are left available for inspection.

## Usage

Requires Go 1.22+ to build, Git on PATH, and the target project's verification tools.
The target must have an initial Git commit.

```sh
go build -o bin/vicoha ./cmd/vicoha
./bin/vicoha --implement-command /path/to/implement-agent --review-command /path/to/review-agent --repo /path/to/project "Implement the requested change"
```

On Windows, build `bin/vicoha.exe` and use executable adapters suitable for Windows.
Alternatively run `go run ./cmd/vicoha` with the same flags. Agent paths may also be
set using `VICOHA_IMPLEMENT_COMMAND` and `VICOHA_REVIEW_COMMAND`.

Run from the ViCoHa directory or provide `--skills-dir /path/to/ViCoHa/skills`.
Skills are resolved relative to the caller, independently of `--repo`.
Place flags before the task; use `--` before a task beginning with a dash.

### Executable-agent contract

- Each invocation starts a new process in the target repository directory.
- The complete prompt arrives on stdin. Implementation may modify files or commit.
- Agent stdout is captured. Implementation stdout is not part of review input.
- A nonzero exit is an agent failure; stderr is included in the result message.
- A command is a single executable, without embedded shell arguments. Use an
  executable adapter for provider-specific flags or configuration.
- Bare command names resolve through PATH. Relative paths containing separators
  resolve from ViCoHa's launch directory, before switching to the target repo.
- The same executable may serve both roles if each call starts independently and
  follows the supplied role. Agent-side session reuse must not share private
  implementation reasoning with review.

Review stdout must contain exactly one JSON object matching
[`schemas/review.schema.json`](schemas/review.schema.json):

```json
{"findings":[{"title":"Concrete defect","severity":"high","actionable":true,"details":"Evidence and the necessary change"}]}
```

`findings` must be a non-null array; `{"findings":[]}` is a clean review. Every
finding requires all four fields. Text must be non-empty; actionable must be a
boolean. Unknown fields and extra output are rejected deterministically.
Only findings marked actionable are included in repair prompts. Review is
instructed to mark only evidenced defects within task scope; repair is also
instructed to reject speculative or out-of-scope suggestions. ViCoHa does not
pretend that a JSON parser can determine whether an agent's judgment is correct.

### Project context and verification

Context includes root `AGENTS.md`, `README.md`, `go.mod`, `package.json`,
`pyproject.toml`, and `Makefile` when present, plus Git-listed directory-scoped
`AGENTS.md` files. The diff includes commits since workflow start, staged and
unstaged edits, and non-ignored untracked files. Existing local changes are also
included; ViCoHa does not reset the working tree.

After a clean review, checks run in this order and stop on the first failure:

| Configuration | Commands |
| --- | --- |
| `go.mod` | `go test ./...`, `go build ./...` |
| Declared `package.json` scripts | `npm run test`, `npm run build`, `npm run lint`, `npm run typecheck`, only when declared and non-empty |
| `pytest.ini` or a `[tool.pytest.ini_options]` table in `pyproject.toml` | `python -m pytest` |

Windows npm commands run through `cmd.exe`. A project with no supported checks
fails verification. ViCoHa does not install project dependencies.

### JSON output

```sh
./bin/vicoha --json --implement-command /path/to/agent --review-command /path/to/reviewer --repo /path/to/project "Fix the bug" > result.json
```

Stdout contains one structured result with `status`, `message`, `review_passes`,
`repair_passes`, `agent_invocations`, `duration_ms`, `reviews` (findings per valid
review), and `verification` (command, status, captured output). Counts include
attempted invocations that fail. Invalid or mutating reviews count as attempts
but do not enter `reviews`. Empty lists are JSON arrays.

Workflow statuses are `success`, `implementation_failed`, `review_failed`,
`verification_failed`, and `iteration_limit`. The reserved legacy status
`actionable_findings` remains defined but is not emitted by this workflow.
Exit codes: 0 for success, 1 for a failed workflow, 2 for invalid CLI usage or an
output error. Command-level diagnostics go to stderr; usage errors produce no
workflow JSON. Human output remains the default.

## Evaluation

The standard-library Python suite under [`eval/`](eval/README.md) compares the same
implementation executable on the same task in fresh repositories:

- **Baseline:** one implementation invocation, using the same initial prompt.
- **ViCoHa:** implementation, independent review, bounded repair, verification.

Six small cases exercise scope creep, hardcoded fixes, architecture drift,
overengineering, excessive tests, and stream resource efficiency. Immutable
functional checks and explicit deterministic constraints score both modes equally.
The evaluator uses the CLI and JSON results, without importing Go internals.
These are narrow regression cases, not evidence of universal coding quality.

```sh
python eval/run.py --list
python eval/run.py --vicoha ./bin/vicoha --implement-command /path/to/agent --review-command /path/to/reviewer --output results.jsonl
```

Use `--case scope-creep` to select a case or `--mode baseline` / `--mode vicoha` to
run one side. Python 3.11+ is required; no Python packages need installation.
See the eval README for oracle semantics, metrics, and limitations.
`eval/swebench/` is reserved for future SWE-bench integration.

## Layout and development

- `internal/agent/`: executable transport.
- `internal/harness/`: skills, prompts, review parsing and filtering.
- `internal/project/`: context and Git inspection.
- `internal/runner/`: explicit workflow orchestration.
- `internal/verify/`: deterministic command discovery and execution.
- `internal/result/`: structured results shared by CLI and tooling.
- `cmd/vicoha/`: human and JSON CLI output.
- `skills/`: project behavior instructions.
- `schemas/`: runtime review contract; plan/project-profile remain documented placeholders.

Run the Go checks and the deterministic local eval tooling checks:

```sh
go test ./...
go build ./...
python eval/test_run.py -v
```

The local eval checks use a scripted executable fixture, verify each oracle
against original and corrected repositories, and exercise baseline/ViCoHa CLI
transport and prompt parity. They do not measure real model performance.
