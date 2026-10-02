"""Local deterministic checks; no model or network required."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

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
                                   "--review-command", str(agent)], env=env, text=True,
                                  encoding="utf-8", capture_output=True, timeout=180)
            self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
            records = [json.loads(line) for line in proc.stdout.splitlines()]
            self.assertEqual([r["mode"] for r in records], ["baseline", "vicoha"])
            self.assertTrue(all(r["oracle_success"] for r in records))
            workflow = records[1]["workflow"]
            self.assertEqual(workflow["status"], "success")
            self.assertEqual(workflow["agent_invocations"], 2)
            self.assertEqual(workflow["review_passes"], 1)
            self.assertEqual(len(workflow["verification"]), 2)
            self.assertEqual((tmp / "scope-creep-baseline-implement.txt").read_bytes(),
                             (tmp / "scope-creep-vicoha-implement.txt").read_bytes())
            self.assertNotIn("private implementation output", (tmp / "scope-creep-vicoha-review.txt").read_text())

    def test_diff_metrics_include_new_files(self):
        metrics, added = run.changes({"a.go": b"old\n"}, {"a.go": b"new\n", "a_test.go": b"test\n"})
        self.assertEqual(metrics, {"changed_files": ["a.go", "a_test.go"], "added_lines": 2,
                                   "deleted_lines": 1, "added_test_lines": 1})
        self.assertEqual(added, "new\ntest")


if __name__ == "__main__":
    unittest.main()
