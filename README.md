# ViCoHa (Vibe Coding Harness)

An experimental coding-agent quality harness. This repository contains only an
initial scaffold; no harness or evaluation behavior is implemented yet.

## Layout

- `cmd/vicoha/`: Go CLI entry point (currently a no-op).
- `internal/`: Future Go runtime and deterministic tooling: `harness` for harness
  execution, `project` for project context, `runner` for task execution, and
  `result` for result handling.
- `skills/`: Short Markdown descriptions of intended agent responsibilities.
- `eval/`: Future Python evaluation and benchmarking tooling; `swebench/` reserves
  space for future SWE-bench integration.
- `schemas/`: JSON Schema placeholders for project profiles, plans, and reviews.
  These currently accept any object and define no fields or validation contract.

## Development

Requires Go 1.22 or later. From the repository root:

```sh
go run ./cmd/vicoha
go test ./...
```

The CLI intentionally performs no work. Python tooling has no dependencies or
implementation yet; `pyproject.toml` reserves project metadata for Python 3.11+.

Model/API integrations, MCP, agent orchestration, model routing, self-improvement,
and benchmark execution are outside this scaffold.

## License

A license has not yet been selected; see `LICENSE`.
