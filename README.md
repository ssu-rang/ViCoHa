# ViCoHa (Vibe Coding Harness)

ViCoHa runs an implementation agent, an independent review agent, bounded
repair cycles, and deterministic project checks. The Go CLI accepts any
executable agent backend that reads its prompt from stdin; implement and review
must be separate invocations. Agent commands run in the target repository.

```sh
go run ./cmd/vicoha --implement-command <implement-agent> --review-command <review-agent> --repo <project-dir> "<task>"
```

The command names can also be set with `VICOHA_IMPLEMENT_COMMAND` and
`VICOHA_REVIEW_COMMAND`. Run from the ViCoHa directory, or pass
`--skills-dir /path/to/ViCoHa/skills`; skills are independent of `--repo`.
The target must be a Git repository with an initial commit.
`--max-iterations` defaults to 2 review passes (at most one repair). Review
agents return a JSON object with a `findings` array; each finding has `title`,
`severity`, `actionable`, and `details` fields. All fields are required, and
text fields must be non-empty. Review is read-only; detected repository changes
cause a review failure. Only actionable findings are
sent back to the implementation agent. The runner supplies the original task,
repository guidance, and the relevant skill. Review receives the task, current
diff, project guidance including directory-scoped `AGENTS.md` files, and review
skill, without implementation output. The diff includes agent commits since
the workflow began, staged/unstaged changes, and untracked files.

After a clean review, the runner executes checks supported by project files:
`go test ./...` and `go build ./...` when `go.mod` exists, declared npm scripts
for test/build/lint/typecheck when `package.json` defines them, and pytest when
`pytest.ini` or `[tool.pytest.ini_options]` explicitly configures it. A project
with no supported checks produces a verification failure. Missing or failed
agent commands, malformed review output, remaining findings, and failed checks
produce nonzero exit status and distinct result statuses. Actionable findings
remaining on the final review pass produce `iteration_limit`, including the
findings in the result message. Agent executables must consume prompts from
stdin; provider-specific CLI flags require an executable adapter.

## Layout

- `cmd/vicoha/`: CLI entry point.
- `internal/runner/`: agent lifecycle, review/repair loop, and verification.
- `internal/project/`: repository context collection.
- `internal/result/`: workflow result and status types.
- `skills/`: concise agent behavior instructions.
- `schemas/`: review response contract and reserved project/plan schemas.
- `eval/`: reserved for future evaluation tooling.

Requires Go 1.22 or later.
