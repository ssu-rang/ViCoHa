"""Small, sequential black-box baseline/ViCoHa evaluation. Standard library only."""

import argparse
import difflib
import fnmatch
import json
import math
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time

HERE = Path(__file__).resolve().parent


def execute(args, root, *, prompt=None, timeout=300):
    start = time.monotonic()
    try:
        proc = subprocess.run(args, cwd=root, input=prompt.encode("utf-8") if prompt is not None else None,
                              capture_output=True,
                              timeout=timeout)
        return {"command": args, "returncode": proc.returncode,
                "stdout": proc.stdout.decode("utf-8", errors="replace"),
                "stderr": proc.stderr.decode("utf-8", errors="replace"),
                "elapsed_ms": round((time.monotonic() - start) * 1000)}
    except (OSError, subprocess.TimeoutExpired) as exc:
        return {"command": args, "returncode": None, "stdout": "",
                "stderr": str(exc), "elapsed_ms": round((time.monotonic() - start) * 1000)}


def git(root, *args):
    result = execute(["git", *args], root)
    if result["returncode"] != 0:
        raise RuntimeError(f"git {' '.join(args)}: {result['stderr']}")
    return result["stdout"]


def prepare(case, root):
    shutil.copytree(case / "repo", root)
    git(root, "init", "-q")
    git(root, "-c", "core.autocrlf=false", "add", ".")
    git(root, "-c", "user.name=ViCoHa Eval", "-c", "user.email=eval@example.invalid",
        "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")


def snapshot(root):
    names = git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
    return {name: (root / name).read_bytes() for name in set(names.split("\0"))
            if name and (root / name).is_file()}


def changes(before, after):
    changed = sorted(name for name in before.keys() | after.keys()
                     if before.get(name) != after.get(name))
    added, deleted, test_added = 0, 0, 0
    introduced = []
    for name in changed:
        old = before.get(name, b"").decode("utf-8", errors="replace").splitlines()
        new = after.get(name, b"").decode("utf-8", errors="replace").splitlines()
        for tag, i, j, k, l in difflib.SequenceMatcher(None, old, new, autojunk=False).get_opcodes():
            if tag in ("replace", "delete"):
                deleted += j - i
            if tag in ("replace", "insert"):
                added += l - k
                introduced.extend(new[k:l])
                if name.endswith("_test.go"):
                    test_added += l - k
    return {"changed_files": changed, "changed_files_count": len(changed),
            "added_lines": added, "deleted_lines": deleted,
            "added_test_lines": test_added}, "\n".join(introduced)


def baseline_prompt(case, root, skills):
    parts = []
    for name in ("AGENTS.md", "README.md", "go.mod", "package.json", "pyproject.toml", "Makefile"):
        path = root / name
        if path.exists():
            parts.append(f"--- {name} ---\n{path.read_bytes().decode('utf-8')}")
    names = git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
    for name in names.split("\0"):
        if name != "AGENTS.md" and Path(name).name == "AGENTS.md":
            parts.append(f"--- {name} (applies to that directory) ---\n"
                         + (root / name).read_bytes().decode("utf-8"))
    skill = (skills / "implement" / "SKILL.md").read_bytes().decode("utf-8")
    task = (case / "task.md").read_text(encoding="utf-8")
    context = "\n\n".join(parts)
    # Matches the public executable prompt contract for these clean fixtures.
    return (f"You are the Implement Agent. Follow this skill exactly:\n{skill}\n\n"
            f"Original user task:\n{task}\n\nRepository context and project rules:\n{context}\n\n"
            "Current git diff:\n\n")


def assess(case, root, before, timeout):
    oracle = json.loads((case / "oracle.json").read_text(encoding="utf-8"))
    metrics, introduced = changes(before, snapshot(root))
    changed = metrics["changed_files"]
    violations = []
    for pattern in oracle.get("must_not_touch", []):
        for name in changed:
            if fnmatch.fnmatchcase(name, pattern):
                violations.append(f"protected file changed: {name}")
    for name in oracle.get("must_change", []):
        if name not in changed:
            violations.append(f"required file unchanged: {name}")
    if len(changed) > oracle.get("max_changed_files", len(changed)):
        violations.append("changed file budget exceeded")
    if metrics["added_test_lines"] > oracle.get("max_added_test_lines", metrics["added_test_lines"]):
        violations.append("test addition budget exceeded")
    for pattern in oracle.get("forbidden_patterns", []):
        if pattern in introduced:
            violations.append(f"forbidden literal introduced: {pattern}")

    # Run immutable oracle tests alongside the candidate's tests in a separate
    # copy, after measuring the diff. Oracles also cover existing API behavior.
    with tempfile.TemporaryDirectory(prefix="vicoha-oracle-") as tmp:
        checked = Path(tmp) / "repo"
        shutil.copytree(root, checked, ignore=shutil.ignore_patterns(".git"))
        shutil.copytree(case / "testdata", checked, dirs_exist_ok=True)
        checks = [execute(command, checked, timeout=timeout) for command in oracle["must_pass"]]
    functional = bool(checks) and all(check["returncode"] == 0 for check in checks)
    return {**metrics, "functional_success": functional,
            "oracle_success": functional and not violations,
            "violations": violations, "oracle_checks": checks}


def run_case(case, mode, args, workspace, trial=1):
    start = time.monotonic()
    root = workspace / f"{case.name}-trial-{trial}-{mode}"
    prepare(case, root)
    before = snapshot(root)
    workflow = None
    errors = []
    if mode == "baseline":
        invocation = execute([args.implement_command], root,
                             prompt=baseline_prompt(case, root, args.skills_dir), timeout=args.timeout)
    else:
        invocation = execute([args.vicoha, "--json", "--repo", str(root),
                              "--skills-dir", str(args.skills_dir),
                              "--implement-command", args.implement_command,
                              "--review-command", args.review_command,
                              "--max-iterations", str(args.max_iterations),
                              (case / "task.md").read_text(encoding="utf-8")],
                             root, timeout=args.timeout)
        try:
            workflow = json.loads(invocation["stdout"])
            if not isinstance(workflow, dict) or "status" not in workflow:
                raise ValueError("expected a workflow result object")
        except (ValueError, TypeError) as exc:
            errors.append(f"invalid ViCoHa JSON: {exc}")
            workflow = None
        if workflow is not None:
            # The decoded result is retained once, without duplicate raw JSON.
            del invocation["stdout"]
    if invocation["returncode"] != 0:
        errors.append(f"agent/workflow exit: {invocation['returncode']}: {invocation['stderr']}")
    scored = assess(case, root, before, args.timeout)
    return {"case": case.name, "mode": mode, **scored,
            "execution_success": invocation["returncode"] == 0 and not errors,
            "workflow": workflow, "errors": errors,
            "elapsed_ms": round((time.monotonic() - start) * 1000),
            "invocation": invocation}


def executable(value):
    # Resolve before switching cwd, matching ViCoHa's executable path behavior.
    return str(Path(value).resolve()) if "/" in value or "\\" in value else value


def configuration(args):
    # Explicit allowlist: never serialize credentials or the entire environment.
    adapter_keys = ("VICOHA_CODEX_COMMAND", "VICOHA_CODEX_IMPLEMENT_MODEL",
                    "VICOHA_CODEX_REVIEW_MODEL", "VICOHA_CODEX_IMPLEMENT_EFFORT",
                    "VICOHA_CODEX_REVIEW_EFFORT")
    return {"id": args.config_id, "implement_command": args.implement_command,
            "review_command": args.review_command, "vicoha": args.vicoha,
            "skills_dir": str(args.skills_dir), "max_iterations": args.max_iterations,
            "timeout_seconds": args.timeout,
            "codex_adapter_environment": {key: os.environ[key] for key in adapter_keys
                                          if key in os.environ}}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=["baseline", "vicoha", "both"], default="both")
    parser.add_argument("--case", action="append", dest="cases", help="case name; repeat to select several")
    parser.add_argument("--list", action="store_true", help="list cases without running agents")
    parser.add_argument("--trials", type=int, default=1, help="fresh trials per case and mode (default: 1)")
    parser.add_argument("--config-id", help="required experiment label identifying models, settings and agent version")
    parser.add_argument("--implement-command")
    parser.add_argument("--review-command")
    parser.add_argument("--vicoha", default="vicoha")
    parser.add_argument("--skills-dir", type=Path, default=HERE.parent / "skills")
    parser.add_argument("--max-iterations", type=int, default=2)
    parser.add_argument("--timeout", type=float, default=300, help="seconds per workflow or oracle command")
    parser.add_argument("--output", type=Path, help="JSONL file; defaults to stdout")
    parser.add_argument("--work-dir", type=Path, help="new directory to retain run repositories")
    args = parser.parse_args(argv)
    cases = sorted(path for path in (HERE / "cases").iterdir() if path.is_dir())
    if args.cases:
        unknown = set(args.cases) - {case.name for case in cases}
        if unknown:
            parser.error(f"unknown cases: {', '.join(sorted(unknown))}")
        cases = [case for case in cases if case.name in args.cases]
    if args.list:
        print("\n".join(case.name for case in cases))
        return 0
    if not args.implement_command or (args.mode != "baseline" and not args.review_command):
        parser.error("--implement-command is required; --review-command is required for ViCoHa")
    if not args.config_id or not args.config_id.strip():
        parser.error("--config-id is required to identify the implement/review setup")
    if args.trials < 1 or args.max_iterations < 1 or not math.isfinite(args.timeout) or args.timeout <= 0:
        parser.error("trials, max-iterations and timeout must be positive and finite")
    args.skills_dir = args.skills_dir.resolve()
    args.implement_command = executable(args.implement_command)
    args.review_command = executable(args.review_command) if args.review_command else None
    args.vicoha = executable(args.vicoha)
    modes = ["baseline", "vicoha"] if args.mode == "both" else [args.mode]
    config = configuration(args)
    # TemporaryDirectory cleans only directories created by this invocation.
    with tempfile.TemporaryDirectory(prefix="vicoha-eval-") as tmp:
        workspace = Path(tmp)
        if args.work_dir:
            workspace = args.work_dir.resolve()
            workspace.mkdir(parents=True, exist_ok=False)
        output = args.output.open("w", encoding="utf-8") if args.output else sys.stdout
        failed = False
        try:
            for case in cases:
                for trial in range(1, args.trials + 1):
                    for mode in modes:
                        start = time.monotonic()
                        try:
                            record = run_case(case, mode, args, workspace, trial)
                        except (OSError, ValueError, RuntimeError) as exc:
                            record = {"case": case.name, "mode": mode, "execution_success": False,
                                      "functional_success": False, "oracle_success": False,
                                      "elapsed_ms": round((time.monotonic() - start) * 1000),
                                      "workflow": None, "errors": [str(exc)]}
                        record.update(trial=trial, configuration=config)
                        output.write(json.dumps(record, ensure_ascii=False) + "\n")
                        output.flush()
                        passed = record["oracle_success"] and record["execution_success"]
                        failed |= not passed
                        print(f"{case.name:22} trial={trial} {mode:8} functional={record['functional_success']} "
                              f"oracle={record['oracle_success']} execution={record['execution_success']}", file=sys.stderr)
        finally:
            if args.output:
                output.close()
    return int(failed)


if __name__ == "__main__":
    sys.exit(main())
