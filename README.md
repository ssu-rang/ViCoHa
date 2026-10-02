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
5. After a clean review, discover verification commands deterministically. If the
   evidence is ambiguous, a fresh read-only discovery agent may propose commands.
   Validate the selection, then execute checks; process results alone decide success.

`--max-iterations` limits **review passes**, defaults to 2, and must be at least 1.
Thus the default permits one initial implementation, two reviews, and one repair.
Remaining actionable findings on the final pass yield `iteration_limit`.
Verification failures terminate the workflow; they do not start another repair loop.

Review and verification discovery are read-only. Changes to HEAD, the index,
tracked content, or non-ignored untracked content during either agent invocation
fail the workflow. This is a Git state check,
not an OS sandbox: ignored artifacts, external files, and transient changes that
are restored before the agent exits are outside its detection boundary. Agents run
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
- The runner sets `VICOHA_AGENT_ROLE=implement`, `review`, or `verify-discovery`.
  Repair uses `implement`. Adapters must select roles from this variable; prompt
  wording is not a machine protocol. Unknown roles must fail explicitly.
- The complete prompt arrives on stdin. Implementation may modify files or commit.
- Agent stdout is captured. Implementation stdout is not part of review input.
- A nonzero exit is an agent failure; stderr is included in the result message.
- A command is a single executable, without embedded shell arguments. Use an
  executable adapter for provider-specific flags or configuration.
- Bare command names resolve through PATH. Relative paths containing separators
  resolve from ViCoHa's launch directory, before switching to the target repo.
- The same executable may serve all roles if each call starts independently and
  follows the supplied role. Agent-side session reuse must not share private
  implementation reasoning with review.

Review stdout must contain exactly one JSON object matching
[`schemas/review.schema.json`](schemas/review.schema.json):

```json
{"findings":[{"title":"Concrete defect","category":"functional_defect","severity":"high","actionable":true,"details":"Evidence and the necessary change"}]}
```

`findings` must be a non-null array; `{"findings":[]}` is a clean review. Every
finding requires `title`, `category`, `severity`, `actionable`, and `details`.
`file` is an optional non-empty path; line numbers are not parsed. Severity is
`low`, `medium`, `high`, or `critical`. Categories are `functional_defect`,
`regression`, `overengineering`, `temporary_fix`, `resource_waste`, `excessive_tests`,
`scope_creep`, `architecture_drift`, `integration_problem`, and `other`.
Text must be non-empty; actionable must be a boolean. Unknown fields, duplicate
keys, null required values, wrong enum values, and extra output fail explicitly.
Only findings marked actionable are included in repair prompts. Review is
instructed to mark only evidenced defects within task scope; repair is also
instructed to reject speculative or out-of-scope suggestions. ViCoHa does not
pretend that a JSON parser can determine whether an agent's judgment is correct.

### Included Codex adapter (Luna / Sol)

`cmd/vicoha-codex` is a thin executable adapter. The runner still accepts any
executable implementing the contract above; no provider SDK or model-specific
runner logic is required. Runtime model IDs are configuration, with no Astra
dependency or automatic model substitution.

Build both binaries and configure roles independently (POSIX shell):

```sh
go build -o bin/vicoha ./cmd/vicoha
go build -o bin/vicoha-codex ./cmd/vicoha-codex
export VICOHA_CODEX_IMPLEMENT_MODEL=gpt-5.6-luna
export VICOHA_CODEX_REVIEW_MODEL=gpt-5.6-luna
export VICOHA_CODEX_VERIFY_DISCOVERY_MODEL=gpt-5.6-luna
export VICOHA_CODEX_IMPLEMENT_EFFORT=medium
export VICOHA_CODEX_REVIEW_EFFORT=medium
./bin/vicoha --implement-command ./bin/vicoha-codex --review-command ./bin/vicoha-codex --repo /path/to/project "Implement the requested change"
```

Install Codex CLI separately, with support for `exec --ephemeral`, `--sandbox`,
`--output-schema` and `--output-last-message` (local CLI inspected: 0.159.3).
Authenticate through `CODEX_API_KEY` in your environment or an existing file-based
Codex login (`$CODEX_HOME/auth.json`, otherwise `~/.codex/auth.json`). Keyring-only
login is not copied; use an API key or `codex -c 'cli_auth_credentials_store="file"' login`.
For repeated experiments prefer `CODEX_API_KEY`: login token refreshes in the
temporary home are discarded, so an expired file-based login requires logging
in again outside the adapter.
Your account/provider must expose the selected model ID. Unsupported models or
options fail the invocation; the adapter never silently falls back to another model.

| Environment variable | Meaning |
| --- | --- |
| `VICOHA_CODEX_COMMAND` | Codex executable, default `codex` on PATH; paths must be absolute |
| `VICOHA_CODEX_IMPLEMENT_MODEL` | Required for initial implementation and repair |
| `VICOHA_CODEX_REVIEW_MODEL` | Required for review |
| `VICOHA_CODEX_VERIFY_DISCOVERY_MODEL` | Required only when AI discovery is invoked |
| `VICOHA_CODEX_IMPLEMENT_EFFORT` | Optional implementation/repair reasoning effort, passed verbatim |
| `VICOHA_CODEX_REVIEW_EFFORT` | Optional review reasoning effort, passed verbatim |
| `VICOHA_CODEX_VERIFY_DISCOVERY_EFFORT` | Optional discovery reasoning effort |

The adapter selects its role from `VICOHA_AGENT_ROLE`, including repair as
implementation. It forwards stdin unchanged and starts a fresh `codex exec`
with a temporary `CODEX_HOME` on **every** invocation. Only file-based credentials
are copied; user configuration, conversation, memories and logs are not copied.
The home and final-response file are removed when the adapter exits normally,
including reported errors. Forced termination can leave temporary files behind.
Do not put private agent logs or session state in the target repository.

Implementation and repair use `--sandbox workspace-write`; review and discovery use
`--sandbox read-only`. Approval policy is `never`, so a blocked operation fails
instead of waiting for interactive approval. The adapter does not disable the
sandbox. Use a Codex installation with working sandbox support on your platform.
On native Windows it selects `windows.sandbox="unelevated"`, the restricted-token
sandbox, because a fresh home has no persistent elevated-sandbox credentials.
This provides weaker network isolation than the elevated backend; managed policy
requiring elevated mode may reject it. See the official
[Windows sandbox documentation](https://learn.chatgpt.com/docs/windows/windows-sandbox).
Repository guidance remains available through the prompt and repository files.
The runner's Git mutation checks still apply independently of the sandbox.

Only the file produced by `--output-last-message` becomes adapter stdout.
Codex progress goes to stderr. Review and discovery use their embedded JSON
schemas and validate the final response. Extra prose or invalid JSON fails;
the adapter does not extract or repair JSON heuristically. No session is resumed
or forked, including for repair. See the official
[Codex CLI reference](https://developers.openai.com/codex/cli/reference) for these options.

On Windows, use compiled `.exe` adapters and the native Codex binary, avoiding
PowerShell execution-policy and `.cmd` shell quoting issues. For an npm installation:

```powershell
go build -o bin/vicoha.exe ./cmd/vicoha
go build -o bin/vicoha-codex.exe ./cmd/vicoha-codex
Get-ChildItem (Join-Path (npm.cmd root -g) '@openai') -Recurse -Filter codex.exe | Select-Object -ExpandProperty FullName
$env:VICOHA_CODEX_COMMAND = 'C:\absolute\path\from\the\listing\codex.exe'
$env:VICOHA_CODEX_IMPLEMENT_MODEL = 'gpt-5.6-luna'
$env:VICOHA_CODEX_REVIEW_MODEL = 'gpt-5.6-luna'
$env:VICOHA_CODEX_VERIFY_DISCOVERY_MODEL = 'gpt-5.6-luna'
$env:VICOHA_CODEX_IMPLEMENT_EFFORT = 'medium'
$env:VICOHA_CODEX_REVIEW_EFFORT = 'medium'
./bin/vicoha.exe --implement-command ./bin/vicoha-codex.exe --review-command ./bin/vicoha-codex.exe --repo C:/path/to/project "Implement the requested change"
```

Other providers or custom Codex configuration can use their own executable
adapters. Keep fresh sessions and the same stdout contract, and give review only
repository evidence. The included adapter deliberately exposes only model and
effort settings, without importing personal Codex configuration into experiments.

### Project context and verification

Context includes root `AGENTS.md`, `README.md`, `go.mod`, `package.json`,
`pyproject.toml`, and `Makefile` when present, plus Git-listed directory-scoped
`AGENTS.md` files. The diff includes commits since workflow start, staged and
unstaged edits, and non-ignored untracked files. Existing local changes are also
included; ViCoHa does not reset the working tree.

Verification has four steps: deterministic discovery, optional AI discovery,
validation, and deterministic execution. ViCoHa may use AI to discover appropriate
verification commands, but AI does not judge whether verification succeeded.
Commands are validated and executed; process results determine success.

Deterministic discovery recognizes common `verify`, `check`, and `test` targets in
Makefiles, simple justfiles and Taskfiles, preferring that order. Package scripts
`verify`/`check` are preferred over separate `test`, `build`, `lint`, and `typecheck`
scripts. The package manager comes from `packageManager` or lockfiles, defaulting
to npm. An npm placeholder test is not treated as verification. Obvious defaults
include Go test/build, Cargo test, existing Gradle/Maven wrappers with `test`, and
explicit pytest configuration. Repository entrypoints take precedence over defaults.

Discovery also inspects README/AGENTS guidance, CI workflows, other build metadata
(including CMake, tox and pubspec), and environment/lockfiles. CI, verification
instructions in documentation, unsupported orchestration, or insufficient evidence
make discovery conservative: a language marker alone does not override those
instructions. Simple root entrypoints are treated as project-wide; ambiguous
monorepo layouts may require discovery. This is deliberately not an exhaustive
language or configuration parser.

When needed, `--verify-discovery-command` (or `VICOHA_VERIFY_DISCOVERY_COMMAND`)
starts a fresh agent with its own `verify-discovery` role. By default it uses the
review **executable**, with a fresh invocation and the discovery role; it does not
reuse the reviewer or any session. Its prompt contains configuration/guidance,
not the task, diff, implementation output, or review findings. It may inspect
additional repository files read-only. Selection priority is documented commands,
CI, existing tool commands, then justified conventions. Set `--no-ai-discovery`
to fail when deterministic discovery is insufficient.

Discovery stdout must match
[`schemas/verify-discovery.schema.json`](schemas/verify-discovery.schema.json):

```json
{"commands":[{"argv":["go","test","./..."],"reason":"Existing Go test workflow","evidence":["go.mod"]}]}
```

Reasons and evidence are required; evidence must name existing files inside the
repository. An empty command array fails verification. Malformed output is never
extracted or repaired. Discovery must not run checks, install dependencies, change
files/configuration, create tests, use network discovery, or invent infrastructure.

The runner validates the entire selection before execution. It rejects obvious
privilege escalation, deletion/system commands, Git commands, network tools,
installation commands, shell interpreters, inline interpreter code, and paths
outside the repository (including detectable symlink escapes). Commands use argv
and direct process execution. Windows npm/pnpm/yarn and Gradle/Maven batch shims
use `cmd.exe` with restricted simple arguments. Other shell strings are unsupported.
Commands execute in order and stop on the first failure; failures do not trigger
another repair cycle. Dependencies must already be available.

Validation prevents accidental dangerous proposals; repository scripts and build
tools are still trusted code and can have side effects or access networks during
execution. Neither command validation nor Git mutation detection is a full OS
sandbox. ViCoHa is not safe against a malicious local executable agent. Generic
adapters must enforce read-only roles and fresh sessions; the Codex adapter adds
its provider sandbox. Do not store private logs or session state in the target repo.

### JSON output

```sh
./bin/vicoha --json --implement-command /path/to/agent --review-command /path/to/reviewer --repo /path/to/project "Fix the bug" > result.json
```

Stdout contains one structured result with `status`, `message`, `review_passes`,
`repair_passes`, `agent_invocations`, `discovery_invocations`, `verification_count`,
`mode`, `duration_ms`, `reviews` (findings per valid review), and `verification`.
Each check records display `command`, exact `argv`, `source` (`deterministic` or
`ai`), `repository_declared` when recognized, supporting `evidence` paths, `status`,
`output`, `duration_ms`, and `exit_code` (null when the process could not start).
`repository_declared=false` means not identified, rather than proof of a convention.
Model justifications/private reasoning are not included in command results.
Counts include attempted invocations that fail. Invalid or mutating reviews count as attempts
but do not enter `reviews`. Empty lists are JSON arrays.

Workflow statuses are `success`, `implementation_failed`, `review_failed`,
`verification_failed`, and `iteration_limit`. The reserved legacy status
`actionable_findings` remains defined but is not emitted by this workflow.
Exit codes: 0 for success, 1 for a failed workflow, 2 for invalid CLI usage or an
output error. Workflow errors and captured agent failure diagnostics are included
in the result message. Usage/output errors go to stderr; usage errors produce no
workflow JSON. Human output remains the default.

## Evaluation

The standard-library Python Eval v2 suite under [`eval/`](eval/README.md) uses
fresh repositories and the same initial implementation prompt for these modes:

| Mode | Stages |
| --- | --- |
| `baseline` | Implementation only |
| `baseline-verify` | Implementation and deterministic verification; no AI discovery/review |
| `independent-review` | Implementation, fresh review, bounded repair; no verification |
| `vicoha` | Full workflow with independent review, bounded repair and verification |
| `self-review` | Unsupported: generic adapters cannot resume implementation context |

The Go CLI accepts the four supported modes through `--mode`, defaulting to
`vicoha`. For repair ablations compare `--max-iterations 1` (no repair) with 2 or
more, keeping other settings fixed. Extra calls, discovery and repair are counted;
this is not equal-compute evaluation and does not isolate independence from extra
calls without a future supported self-review comparison.

Six synthetic cases retain hidden immutable oracles. User tasks now express
realistic requirements; conventions and scoped budgets live in repository guidance
and scorer files. Oracles and reference edits are never copied into agent repos
or prompts. These are narrow regression cases, not evidence of general coding quality.

```sh
python eval/run.py --list
python eval/run.py --vicoha ./bin/vicoha --implement-command ./bin/vicoha-codex --review-command ./bin/vicoha-codex --config-id luna-luna-medium --mode all --trials 5 --output results.jsonl
```

Python 3.11+ is required, with no extra packages. `--mode both` remains the default
baseline/full pair; `all` selects all four supported modes. Six cases, five trials,
and four modes produce 120 fresh repositories. Records include configuration,
trial, stage counts, elapsed time, oracle results and patch metrics. Credentials
are not recorded. `--cases-dir` accepts another collection with the same
repo/task/oracle/testdata layout; arbitrary argv oracle commands support future
larger repositories without a new result format. SWE-bench remains unimplemented.
See the [eval README](eval/README.md) for oracle limits and experiment guidance.

## Layout and development

- `internal/agent/`: executable transport.
- `internal/harness/`: skills, prompts, review parsing and filtering.
- `internal/project/`: context and Git inspection.
- `internal/runner/`: explicit workflow orchestration.
- `internal/verify/`: discovery, command validation and deterministic execution.
- `internal/result/`: structured results shared by CLI and tooling.
- `cmd/vicoha/`: human and JSON CLI output.
- `cmd/vicoha-codex/`: optional Codex CLI executable adapter with fresh sessions.
- `skills/`: project behavior instructions.
- `schemas/`: runtime review/discovery contracts; plan/project-profile remain documented placeholders.

Run the Go checks and the deterministic local eval tooling checks:

```sh
go test ./...
go build ./...
python eval/test_run.py -v
```

The local eval checks use a scripted executable fixture, verify each oracle
against original and corrected repositories, and exercise baseline/ViCoHa CLI
transport, ablation counts and initial prompt parity. They do not measure real model performance.

GitHub Actions runs these same three checks on pushes and pull requests, using
Go 1.26.x and Python 3.13 on Ubuntu. Local minimums remain Go 1.22 and Python 3.11.
