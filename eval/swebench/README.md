# Future real-repository evaluation

SWE-bench integration is not implemented. The current evaluator accepts additional
case collections through `eval/run.py --cases-dir PATH`, using repo/, task.md,
oracle.json and testdata/ and emitting the same version 2 records across modes.
Oracle commands are argv arrays and immutable oracle files are installed only in
a scoring copy. A future dataset importer can prepare that layout or reuse the
prepare/run_case/assess boundaries without changing record consumers.

Dataset checkout, environment provisioning, containers, patches from a hosted
benchmark and token/cost controls are intentionally outside the current scope.
