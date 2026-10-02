# ViCoHa

ViCoHa's Go runner orchestrates executable agents in an existing Git repository.
The runtime reads the implement and review skills. This file describes that
workflow; it does not introduce additional runtime skill invocations.

1. IMPLEMENT: capture the initial HEAD, repository guidance and current diff.
   Start a fresh implementation execution with the implement skill and original
   user task. Planning may be useful within implementation but is not a required
   separate phase or protocol.
2. REVIEW: start a fresh independent review execution with the review skill,
   original task, relevant repository guidance and current diff, including
   changes committed since workflow start and non-ignored untracked files.
   Never forward implementation stdout, conversation, hidden reasoning or private
   assumptions. Do not resume or fork the implementation session for review.
   Review is read-only. The runner compares Git state before and after review;
   any detected mutation fails the workflow, even if the reviewer reports success.
3. REPAIR: when actionable findings remain, start another implementation execution
   with the implement skill, original task, current guidance/diff and actionable
   findings. Continue the task through repository state, not a shared session.
   Reject speculative or out-of-scope findings. Review independently again.
4. VERIFY: after a review with no actionable findings, Go code discovers and runs
   deterministic project checks. No verify agent or verify skill is invoked.

`--max-iterations` bounds review passes (default 2, minimum 1). With the default,
there can be one initial implementation, two reviews and one repair. Actionable
findings on the last pass yield `iteration_limit`. Agent errors, malformed review
JSON and reviewer mutations fail the workflow. Verification failures terminate
the workflow without another repair loop; no supported checks also fails.

The runner's Git checks cover HEAD/ref, index, tracked files and non-ignored
untracked content. They do not detect ignored/external files or transient changes
restored before review exits. Executable adapters must enforce fresh sessions and
may add a sandbox; they must not store private agent state in the target repo.

Completion requires both a clean independent review and deterministic verification.
There are no separate plan, repair or verify subsystems in the current runner.
