# ViCoHa Eval v2

This sequential, standard-library Python suite runs coding workflows in fresh Git
repositories and scores their final patches with hidden deterministic oracles.
Requires Python 3.11+, Git, Go 1.22+ for the included cases, and executable agents.
No Python packages are required. Configure adapters using the [main README](../README.md).

```sh
go build -o bin/vicoha ./cmd/vicoha
go build -o bin/vicoha-codex ./cmd/vicoha-codex
python eval/run.py --list
python eval/run.py --vicoha ./bin/vicoha --implement-command ./bin/vicoha-codex --review-command ./bin/vicoha-codex --config-id luna-luna-medium --mode all --trials 5 --output results.jsonl
```

On Windows use `.exe` binaries and your installed Python interpreter. Model IDs
and effort settings remain adapter configuration. The evaluator never substitutes
models. Configure `VICOHA_CODEX_IMPLEMENT_MODEL`, `VICOHA_CODEX_REVIEW_MODEL`, and
`VICOHA_CODEX_VERIFY_DISCOVERY_MODEL` for the included adapter; the last is used
only when discovery cannot select commands confidently. Each role has an optional
matching `_EFFORT` variable. Other adapters use their own settings.

## Ablations

| Mode | Implementation | Review / repair | Verification |
| --- | --- | --- | --- |
| `baseline` | One fresh call | None | None |
| `baseline-verify` | One fresh call | None | Deterministic discovery/execution only |
| `independent-review` | Fresh call | Fresh independent review, bounded repair | None |
| `vicoha` | Fresh call | Fresh independent review, bounded repair | Full discovery, validation, execution |
| `self-review` | Unsupported | Generic executable contract cannot preserve implementation sessions | Unsupported |

`--mode both` is the default baseline/full comparison. `--mode all` runs the four
supported modes. Asking for self-review fails with a clear CLI usage error; no
fresh session is mislabeled as same-session review. A future provider extension
would need to preserve implementation context explicitly without weakening the
independent-review path. The included Codex adapter always starts fresh sessions.

Every supported mode goes through the same Go CLI and initial prompt builder.
Baseline and baseline-verify need only `--implement-command`; review modes also
need `--review-command`. `--verify-discovery-command` optionally selects a separate
executable; otherwise full ViCoHa invokes the review executable with the separate
discovery role. Baseline-verify fails if deterministic discovery is insufficient;
it never spends an extra model call to discover commands.

`--max-iterations` bounds review passes, default 2. Compare 1 (no repair) against 2
or more to study bounded repair. Comparing baseline with baseline-verify measures
the verification stage; independent-review versus full adds verification (and
possibly discovery calls). Baseline versus independent-review includes extra
calls as well as independent review. Without a supported self-review comparator,
this suite cannot attribute the difference solely to independence. It records
calls and time but neither controls tokens/cost nor claims equal compute.

## Repeated runs

Use `--case NAME` repeatedly to select cases. `--trials N` defaults to 1. Each
case/trial/mode gets a fresh committed repository. Six cases with five trials and
four modes produce 120 records. Runs are sequential, with no shared random seed.
Use a stable `--config-id` identifying models, settings and adapter version. The
explicit model/effort/command environment allowlist is recorded; credentials are
excluded. For custom adapters put otherwise invisible settings in the label.
Archive CLI/adapter versions alongside published results.

`--timeout 300` bounds each workflow or oracle command. It terminates the direct
subprocess, not an independently spawned process tree. `--work-dir NEW_DIRECTORY`
retains candidate repositories; otherwise they are removed after scoring. An
existing work directory is rejected. JSONL goes to stdout or `--output FILE`;
progress goes to stderr. Exit status is 0 only if every selected run has successful
execution and a passing oracle, 1 for failed runs, and 2 for invalid CLI usage.
Failures retain case/trial/configuration identity and later trials continue.

## Cases and hidden oracles

Eval v2 keeps the six synthetic fixtures but removes warnings about hidden failure
classes from user tasks. Requirements remain explicit; local conventions live in
AGENTS.md and existing code. For example, retry tests extend an existing compact
table, formatters extend an existing registry, and stream buffering has a visible
8 KiB repository constraint. Numeric file/test budgets remain scorer heuristics,
not universal quality measures; they can reject otherwise reasonable patches.

Each `cases/NAME/` contains:

- `repo/`: the initial repository visible to agents.
- `task.md`: the realistic software request supplied unchanged to every mode.
- `oracle.json`: acceptance commands, protected paths and optional change budgets.
- `testdata/`: immutable oracle files installed only in a separate scoring copy.

| Case | Functional requirement | Additional oracle constraint |
| --- | --- | --- |
| scope-creep | Blank-name greeting, preserved nonblank names | Billing unchanged |
| hardcoding | Unicode identifier normalization | Stable API, small patch |
| architecture-drift | Lower formatter through registry | Domain unchanged |
| overengineering | Integer clamp boundaries | Existing helper file, at most two changed files |
| excessive-tests | HTTP 429 and existing retry range | At most 40 added test lines |
| resource-efficiency | First line, EOF and error handling | At most 8192 bytes read for a short line |

Oracle fields are `must_pass` (argv arrays), `must_not_touch` (case-sensitive
fnmatch patterns), `must_change` (exact paths), `max_changed_files`,
`max_added_test_lines`, and `forbidden_patterns` (literals in added/replaced lines).
Paths use `/`; fnmatch `*` can match `/`, and `**` has no extra meaning. All oracle
commands must succeed. Resource checks count bytes, not elapsed time. Candidate
tests run alongside immutable oracle tests in the scoring copy. Oracle results
are never sent back to repair or discovery. Oracles and reference solutions are
not copied into agent repositories or prompts. This is separation for honest
agents, not protection against a malicious executable with local filesystem access.

## Result format and larger repositories

Each record has `record_version: 2`, case, mode, 1-based trial, configuration,
`agent_invocation_count`, `review_count`, `repair_count`, `verification_count`,
`discovery_count`, elapsed milliseconds, execution success, functional success,
oracle success, violations, oracle command results, errors and invocation metadata.
`workflow` contains the Go result for every mode, including its structured findings
and verification provenance. Raw model stdout is not stored. Invocation counts
include attempted calls; verification count means attempted check processes.
Unavailable counts on setup/transport failures are null, not invented zeroes.

Patch metrics include changed paths/count, added/deleted lines, and added test
lines. They compare contents before implementation and after the entire workflow,
including commits and untracked files, before oracle files are installed. Text
line metrics use a sequence diff; the current test-line metric recognizes Go's
`*_test.go` convention. Setup/scoring failures can omit unavailable patch metrics.
Elapsed time includes setup, workflow and oracle execution; workflow duration is
also recorded separately. A patch can pass its oracle despite workflow failure,
so compare execution and oracle outcomes separately.

`--cases-dir PATH` loads another collection with the same layout and record
format. Repositories and oracle commands need not be Go; `testdata/` can contain
any immutable files. Preparation, workflow execution, metrics and scoring are
separate functions. This supports additional fixtures without hardcoded case
names; it is not a dataset checkout/build system. SWE-bench remains a future
[integration point](swebench/README.md), with no adapter or claimed results.

Report trial counts, failures, model settings, calls, time and patch metrics
alongside success rates per configuration/case/mode. The small fixtures and change
budgets do not establish general coding quality or statistical significance.

## Local validation

```sh
python eval/test_run.py -v
```

Checks cover original failing repositories, passing reference patches, protected
billing changes, all supported CLI modes/counts, prompt parity, hidden-state
exclusion, unsupported self-review and external case discovery. A scripted fixture
also introduces a functional-but-out-of-scope billing change: verification accepts
it, while its scripted reviewer requests a bounded repair. This tests harness and
oracle plumbing, not actual model quality. `test_solutions.json` and
`testdata/agent.go` are local test fixtures, never reported model agents.
