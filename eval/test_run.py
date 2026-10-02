"""Local deterministic checks; no model or network required."""

import json
import contextlib
import io
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

# Also works with Windows' isolated embedded Python distribution.
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import run


class EvaluationTests(unittest.TestCase):
    def test_oracles_reject_original_and_accept_reference_edits(self):
        solutions = json.loads((HERE / "test_solutions.json").read_text(encoding="utf-8"))
        for name, edits in solutions.items():
            with self.subTest(case=name), tempfile.TemporaryDirectory() as tmp:
                case = HERE / "cases" / name
                root = Path(tmp) / "repo"
                run.prepare(case, root)
                before = run.snapshot(root)
                original = run.assess(case, root, before, 120)
                self.assertFalse(original["functional_success"], original)
                self.assertFalse(original["oracle_success"], original)
                for filename, content in edits.items():
                    (root / filename).write_text(content, encoding="utf-8")
                scored = run.assess(case, root, before, 120)
                self.assertTrue(scored["oracle_success"], scored)
                if name == "scope-creep":
                    (root / "billing/rate.go").write_text("package billing\nfunc Rate() int { return 99 }\n")
                    scored = run.assess(case, root, before, 120)
                    self.assertTrue(scored["functional_success"])
                    self.assertFalse(scored["oracle_success"])
                    self.assertIn("protected file changed: billing/rate.go", scored["violations"])
                    (root / "greet_test.go").write_text("invalid Go test source")
                    self.assertFalse(run.assess(case, root, before, 120)["functional_success"])

    def test_black_box_pair_and_prompt_parity(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            suffix = ".exe" if os.name == "nt" else ""
            cli, agent = tmp / ("vicoha" + suffix), tmp / ("agent" + suffix)
            for output, source in [(cli, "./cmd/vicoha"), (agent, "./eval/testdata/agent.go")]:
                built = run.execute(["go", "build", "-o", str(output), source], HERE.parent, timeout=120)
                self.assertEqual(built["returncode"], 0, built)
            edits = json.loads((HERE / "test_solutions.json").read_text())["scope-creep"]
            env = {**os.environ, "EVAL_EDITS": json.dumps(edits), "EVAL_PROMPTS": str(tmp)}
            proc = subprocess.run([sys.executable, str(HERE / "run.py"), "--case", "scope-creep",
                                   "--vicoha", str(cli), "--implement-command", str(agent),
                                   "--review-command", str(agent), "--trials", "2", "--mode", "all",
                                   "--config-id", "scripted-fixture-v1",
                                   "--work-dir", str(tmp / "repos")], env=env, text=True,
                                  encoding="utf-8", capture_output=True, timeout=180)
            self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
            records = [json.loads(line) for line in proc.stdout.splitlines()]
            self.assertEqual([(r["trial"], r["mode"]) for r in records],
                             [(trial, mode) for trial in (1, 2) for mode in
                              ("baseline", "baseline-verify", "independent-review", "vicoha")])
            self.assertTrue(all(r["oracle_success"] for r in records))
            self.assertTrue(all(r["configuration"]["id"] == "scripted-fixture-v1" for r in records))
            self.assertTrue(all(r["configuration"]["implement_command"] == str(agent) for r in records))
            self.assertEqual(len(list((tmp / "repos").iterdir())), 8)
            expected = {"baseline": (1, 0, 0), "baseline-verify": (1, 0, 2),
                        "independent-review": (2, 1, 0), "vicoha": (2, 1, 2)}
            for record in records:
                self.assertEqual(record["record_version"], 2)
                self.assertEqual((record["agent_invocation_count"], record["review_count"],
                                  record["verification_count"]), expected[record["mode"]])
                self.assertEqual(record["repair_count"], 0)
                self.assertEqual(record["discovery_count"], 0)
                self.assertNotIn("private implementation output", json.dumps(record))
            workflow = records[3]["workflow"]
            self.assertEqual(workflow["status"], "success")
            self.assertEqual(workflow["agent_invocations"], 2)
            self.assertEqual(workflow["review_passes"], 1)
            self.assertEqual(len(workflow["verification"]), 2)
            initial = (tmp / "scope-creep-trial-1-baseline-implement.txt").read_bytes()
            for trial in (1, 2):
                for mode in expected:
                    self.assertEqual(initial, (tmp / f"scope-creep-trial-{trial}-{mode}-implement.txt").read_bytes())
                review = (tmp / f"scope-creep-trial-{trial}-vicoha-review.txt").read_text()
                self.assertNotIn("private implementation output", review)
                self.assertIn("Overengineering", review)
                self.assertIn("filesystem", review)
                self.assertIn("low-value tests", review)
                self.assertIn("Architectural or convention drift", review)

            # Deterministic fixture proves that independent review/repair can
            # remove an oracle violation that functional verification misses.
            proc = subprocess.run([sys.executable, str(HERE / "run.py"), "--case", "scope-creep",
                                   "--vicoha", str(cli), "--implement-command", str(agent),
                                   "--review-command", str(agent), "--mode", "all",
                                   "--config-id", "scripted-bad-change"],
                                  env={**env, "EVAL_BAD_CHANGE": "1"}, text=True,
                                  encoding="utf-8", capture_output=True, timeout=180)
            self.assertEqual(proc.returncode, 1, proc.stdout + proc.stderr)
            records = [json.loads(line) for line in proc.stdout.splitlines()]
            self.assertEqual([r["oracle_success"] for r in records], [False, False, True, True])
            self.assertEqual([r["repair_count"] for r in records], [0, 0, 1, 1])
            self.assertEqual([r["agent_invocation_count"] for r in records], [1, 1, 4, 4])

    def test_trial_failures_keep_identity_and_continue(self):
        out = io.StringIO()
        with mock.patch.object(run, "run_case", side_effect=RuntimeError("setup failed")) as case_run, \
                mock.patch.dict(os.environ, {"VICOHA_CODEX_IMPLEMENT_MODEL": "example-model",
                                             "CODEX_API_KEY": "never-record-this"}), \
                contextlib.redirect_stdout(out), contextlib.redirect_stderr(io.StringIO()):
            code = run.main(["--case", "scope-creep", "--trials", "2", "--config-id", "failure-check",
                             "--implement-command", "implement", "--review-command", "review"])
        self.assertEqual(code, 1)
        self.assertEqual(case_run.call_count, 4)
        records = [json.loads(line) for line in out.getvalue().splitlines()]
        self.assertEqual([r["trial"] for r in records], [1, 1, 2, 2])
        for record in records:
            self.assertFalse(record["execution_success"])
            self.assertEqual(record["errors"], ["setup failed"])
            self.assertEqual(record["configuration"]["id"], "failure-check")
            self.assertEqual(record["configuration"]["codex_adapter_environment"]
                             ["VICOHA_CODEX_IMPLEMENT_MODEL"], "example-model")
        self.assertNotIn("never-record-this", out.getvalue())

    def test_cli_trial_and_configuration_validation(self):
        base = ["--implement-command", "unused", "--mode", "baseline", "--config-id", "fixture"]
        for flags in (["--trials", "0"], ["--trials", "-1"], ["--trials", "1.5"],
                      ["--config-id", " "], ["--timeout", "nan"], ["--timeout", "inf"],
                      ["--mode", "self-review"]):
            with self.subTest(flags=flags), contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit) as exc:
                    run.main(base + flags)
                self.assertEqual(exc.exception.code, 2)
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as exc:
            run.main(["--implement-command", "unused", "--mode", "baseline"])
        self.assertEqual(exc.exception.code, 2)

    def test_diff_metrics_include_new_files(self):
        metrics, added = run.changes({"a.go": b"old\n"}, {"a.go": b"new\n", "a_test.go": b"test\n"})
        self.assertEqual(metrics, {"changed_files": ["a.go", "a_test.go"], "changed_files_count": 2, "added_lines": 2,
                                   "deleted_lines": 1, "added_test_lines": 1})
        self.assertEqual(added, "new\ntest")

    def test_external_case_collection_and_task_wording(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "external-case").mkdir()
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                self.assertEqual(run.main(["--cases-dir", str(root), "--list"]), 0)
            self.assertEqual(out.getvalue().strip(), "external-case")
        for case in (HERE / "cases").iterdir():
            task = (case / "task.md").read_text().lower()
            for warning in ("do not overengineer", "40 lines", "do not hardcode", "no configuration",
                            "billing behavior is unrelated", "do not add a parallel dispatcher"):
                self.assertNotIn(warning, task)


if __name__ == "__main__":
    unittest.main()
