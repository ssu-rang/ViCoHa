# Future SWE-bench adapter

The custom baseline/ViCoHa comparison in `../run.py` is the current evaluator.
No SWE-bench dataset, harness, or heavyweight dependencies are bundled here.

A future adapter should prepare each upstream task and base revision, invoke the
same baseline executable or ViCoHa CLI, export the resulting patch, and submit it
to the official SWE-bench test environment. Preserve task identity, executable
configuration, workflow JSON, and upstream test results for comparison. Dataset
provisioning and execution isolation belong to that integration, not the Go runner.
