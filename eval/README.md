# ViCoHa evaluations

This suite compares an implementation agent alone with the same agent running
under ViCoHa. It requires Python 3.11+, Git, Go 1.22+, and executable agents. It
uses only Python's standard library; `pyproject.toml` is the sole dependency
configuration. Agent adapters supply any provider-specific configuration.

From the repository root:

```sh
go build -o bin/vicoha ./cmd/vicoha
go build -o bin/vicoha-codex ./cmd/vicoha-codex
python eval/run.py --list
python eval/run.py --vicoha ./bin/vicoha --implement-command ./bin/vicoha-codex --review-command ./bin/vicoha-codex --config-id luna-luna-medium --output results.jsonl
```

First configure the adapter and authentication as described in the
[main README](../README.md#included-codex-adapter-luna--sol).
On Windows use `bin/vicoha.exe`, `bin/vicoha-codex.exe`, the native Codex binary,
and your Python executable (`python` or `py`).
Use `--case NAME` repeatedly to select cases. `--mode baseline`, `--mode vicoha`,
or `--mode both` (default) controls the comparison. `--max-iterations 2` bounds
ViCoHa review passes; `--timeout 300` limits each workflow or oracle command in
seconds. Timeout handling terminates the direct subprocess, not an entire tree
of independently spawned agent processes.

`--config-id LABEL` is required for runs (not `--list`). Use a label that identifies
the implement/review models, settings and agent version. It is metadata, not an
execution setting. The included adapter's model/effort/command environment is
also captured automatically from an explicit allowlist; credentials are excluded.
For custom adapters, the label must identify configuration not visible in their
executable paths. Archive adapter/CLI versions and any custom wrapper alongside
published results. Do not change configuration during a run.

## Repeated trials and model roles

`--trials N` defaults to 1 and must be positive. Each case/trial/mode gets a fresh
repository and fresh agent execution. With six cases, `--trials 5 --mode both`
produces 60 JSONL records. Trials are numbered from 1 within each invocation.
Runs are sequential in case, trial, baseline/ViCoHa order; this is not a paired
random seed or a guarantee of identical stochastic outputs. Failures retain their
trial/configuration identity and do not prevent subsequent trials.

For a single baseline/ViCoHa pair on one case, after setting model roles:

```sh
python eval/run.py --case scope-creep --vicoha ./bin/vicoha --implement-command ./bin/vicoha-codex --review-command ./bin/vicoha-codex --config-id luna-luna-medium --output pair.jsonl
```

Run each of the following in a POSIX shell for the three role combinations. The
model IDs are passed directly to Codex; account access and CLI support are
prerequisites. Keep the implementation effort identical across both modes:

```sh
export VICOHA_CODEX_IMPLEMENT_EFFORT=medium
export VICOHA_CODEX_REVIEW_EFFORT=medium

# Luna implement -> Luna review
export VICOHA_CODEX_IMPLEMENT_MODEL=gpt-5.6-luna
export VICOHA_CODEX_REVIEW_MODEL=gpt-5.6-luna
python eval/run.py --vicoha ./bin/vicoha --implement-command ./bin/vicoha-codex --review-command ./bin/vicoha-codex --config-id luna-luna-medium --trials 5 --output luna-luna.jsonl

# Sol implement -> Luna review
export VICOHA_CODEX_IMPLEMENT_MODEL=gpt-5.6-sol
export VICOHA_CODEX_REVIEW_MODEL=gpt-5.6-luna
python eval/run.py --vicoha ./bin/vicoha --implement-command ./bin/vicoha-codex --review-command ./bin/vicoha-codex --config-id sol-luna-medium --trials 5 --output sol-luna.jsonl

# Sol implement -> Sol review
export VICOHA_CODEX_IMPLEMENT_MODEL=gpt-5.6-sol
export VICOHA_CODEX_REVIEW_MODEL=gpt-5.6-sol
python eval/run.py --vicoha ./bin/vicoha --implement-command ./bin/vicoha-codex --review-command ./bin/vicoha-codex --config-id sol-sol-medium --trials 5 --output sol-sol.jsonl
```

PowerShell equivalents after the Windows setup in the main README:

```powershell
$env:VICOHA_CODEX_IMPLEMENT_EFFORT = 'medium'
$env:VICOHA_CODEX_REVIEW_EFFORT = 'medium'
$env:VICOHA_CODEX_IMPLEMENT_MODEL = 'gpt-5.6-luna'
$env:VICOHA_CODEX_REVIEW_MODEL = 'gpt-5.6-luna'
python eval/run.py --vicoha ./bin/vicoha.exe --implement-command ./bin/vicoha-codex.exe --review-command ./bin/vicoha-codex.exe --config-id luna-luna-medium --trials 5 --output luna-luna.jsonl
$env:VICOHA_CODEX_IMPLEMENT_MODEL = 'gpt-5.6-sol'
python eval/run.py --vicoha ./bin/vicoha.exe --implement-command ./bin/vicoha-codex.exe --review-command ./bin/vicoha-codex.exe --config-id sol-luna-medium --trials 5 --output sol-luna.jsonl
$env:VICOHA_CODEX_REVIEW_MODEL = 'gpt-5.6-sol'
python eval/run.py --vicoha ./bin/vicoha.exe --implement-command ./bin/vicoha-codex.exe --review-command ./bin/vicoha-codex.exe --config-id sol-sol-medium --trials 5 --output sol-sol.jsonl
```

Each mode starts from a fresh committed copy of the case repository. Baseline
receives the same initial implement skill, task, context, and empty diff as
ViCoHa. It invokes the implementation executable once. Keep the executable,
environment, model configuration, and task identical across both modes; adapters
must start fresh sessions. ViCoHa gets extra review/repair calls by design. The
comparison measures that workflow, not equal compute budgets. Run multiple trials
to assess stochastic agents. Local tests verify initial prompt byte parity.

JSONL goes to stdout or `--output FILE`; human per-case summaries go to stderr.
`--work-dir NEW_DIRECTORY` retains repositories for inspection; otherwise they
are removed after the run. Existing work directories are rejected to protect
earlier runs. Exit status is 0 only when every selected run has both a successful
execution and a passing oracle, 1 when a run fails, and 2 for CLI usage errors.

## Cases and oracles

Each `cases/NAME/` contains:

- `repo/`: small initial repository, copied for each mode.
- `task.md`: original task, shared unchanged by both modes.
- `oracle.json`: explicit deterministic acceptance constraints.
- `testdata/`: immutable Go tests copied into a separate scoring repository after
  collecting the agent's changes. Go excludes this directory from root tests.

| Case | Functional check | Additional constraint |
| --- | --- | --- |
| scope-creep | Blank-name greeting and preservation of non-blank names | Billing files unchanged |
| hardcoding | Unicode normalization over varied identifiers | Existing API, small change |
| architecture-drift | New formatter available through the established registry | Domain model unchanged |
| overengineering | Integer clamp boundary behavior | Existing file updated, at most two changed files |
| excessive-tests | HTTP retry status boundaries | At most 40 added test lines |
| resource-efficiency | EOF/error handling and first-line contents | At most 8192 bytes consumed for a short line before a large tail |

Supported oracle fields are intentionally limited to what these cases use:

- `must_pass`: arrays of command arguments, executed without a shell. All commands
  must succeed. Initial cases run Go tests and build.
- `must_not_touch`: case-sensitive `fnmatch` patterns over repository-relative
  paths with `/` separators. `*` also matches `/`; `**` has no special extra meaning.
- `must_change`: exact repository-relative paths required to change.
- `max_changed_files`: maximum number of changed paths.
- `forbidden_patterns`: literal substrings forbidden in added/replaced lines.
- `max_added_test_lines`: maximum added/replaced lines in `*_test.go` files.

The resource check counts bytes read; it uses no wall-clock performance threshold.
File and test budgets are transparent, task-specific proxies for scope and test
volume. They do not prove architectural quality, and fewer lines are not generally
better. These constraints apply equally to baseline and ViCoHa. No AI judge is
required. Oracle tests cover existing behavior as well as the new requirement;
they run alongside the candidate's tests, so broken test edits also fail scoring.
Oracle data never enters agent prompts.
Agents run with the caller's permissions; this is not a hostile-agent sandbox.

## Result records

Each JSONL record includes `case`, `mode`, `trial` (1-based), `configuration`,
`functional_success` (oracle commands),
`oracle_success` (functional checks plus constraints), `execution_success`,
`changed_files` (paths), `changed_files_count`, `added_lines`, `deleted_lines`,
`added_test_lines`, violations, command results,
errors, elapsed milliseconds, and invocation metadata. `workflow` is null for
baseline and contains ViCoHa's decoded JSON result for ViCoHa: status, review and
repair counts, agent calls, verification results, duration, and findings by pass.
Setup/scoring failures emit a failure record with case, mode, trial,
configuration, elapsed time and error; unavailable metrics are omitted, not zero.
`configuration` records the label, executable commands, skills directory,
review limit, timeout and included Codex adapter environment. Baseline records
retain the pair's review configuration for grouping, but invoke no reviewer.

Diff metrics compare file contents before implementation and after the entire
workflow, including agent commits and non-ignored untracked files. They are
collected before oracle tests are installed. Text line metrics use a sequence
diff, not Git's rename heuristics; fixture files are text. Elapsed time includes
setup, execution, and oracle checks; workflow duration is also available separately.
Functional success is independent of execution status: a correct final patch can
still have a failed review or invocation. Inspect both when comparing runs.

Compare success rates per `(configuration.id, case, mode)` across trials, and
report sample counts, failures and elapsed time alongside patch metrics. Inspect
ViCoHa's `workflow.agent_invocations`, `review_passes`, `repair_passes`, `reviews`,
`verification`, `status` and `duration_ms` to explain outcomes. Baseline has one
attempted agent call when setup succeeds; ViCoHa deliberately spends extra calls.
There is no token/cost accounting or equal-compute claim in this evaluator.
Small synthetic cases and explicit file/test budgets do not establish general
coding quality, architectural quality, or statistical significance from one run.

## Local validation

```sh
python eval/test_run.py -v
```

This checks that all original repositories fail their oracles, reference edits
pass, an unrelated billing change is rejected, and the CLI comparison works with
a deterministic executable fixture. Reference edits in `test_solutions.json` and
`testdata/agent.go` are tooling-test data only; they are never baseline agents in
a reported model evaluation. Passing these checks makes no claim that ViCoHa
outperforms a real agent. Use real adapters and repeated trials for that question.

SWE-bench integration is reserved under [`swebench/`](swebench/README.md) and is
not a dependency of this suite.
