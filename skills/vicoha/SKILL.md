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
4. VERIFY: after a clean review, discover commands deterministically from project
   configuration. If evidence is insufficient, invoke a fresh read-only
   verification-discovery role with repository configuration/guidance only. It
   proposes argv arrays with reasons and evidence, never a pass/fail judgment.
   Validate the complete selection, then execute commands deterministically.
   Process exit results alone determine success. Empty selections fail.

`--max-iterations` bounds review passes (default 2, minimum 1). With the default,
there can be one initial implementation, two reviews and one repair. Actionable
findings on the last pass yield `iteration_limit`. Agent errors, malformed structured
JSON and read-only agent mutations fail the workflow. Verification failures terminate
the workflow without another repair loop; no discovered checks also fails.

The runner's Git checks cover HEAD/ref, index, tracked files and non-ignored
untracked content. They do not detect ignored/external files or transient changes
restored before the read-only agent exits. Executable adapters must enforce fresh sessions and
may add a sandbox; they must not store private agent state in the target repo.

Completion requires both a clean independent review and deterministic verification.
There are no separate plan or repair agents. Repair uses the implementation role.
The generic executable contract signals roles with VICOHA_AGENT_ROLE: implement,
review, verify-discovery. Prompt prefixes do not select roles. Discovery does not
receive implementation/review session state, private reasoning or stdout.

Command validation reduces accidental risk; it is not a sandbox against malicious
local agents or repository scripts. Generic adapters must preserve fresh sessions
and enforce read-only roles. The Codex adapter uses a fresh temporary home and
read-only sandbox for review and discovery. Evaluation modes may intentionally
omit stages; only the default full workflow requires review and verification.
