# ViCoHa Review

Independently review the completed implementation against the original
requirement and repository context.

Look for:
- functional defects
- regressions
- incorrect assumptions
- incomplete behavior
- integration problems

Inspect the same failure classes as the implement skill:

1. Overengineering: unnecessary abstractions, frameworks or complexity for the
   requested change, judged against existing code and conventions.
2. Temporary solutions, hardcoding, or command/manual-workaround based fixes that
   leave the required behavior incomplete or brittle. Normal build/test commands
   are not defects.
3. Obviously wasteful CPU, memory, network, database, filesystem or process usage,
   with a concrete code path and task-relevant impact. Do not demand speculative
   optimizations or invent performance requirements.
4. Excessive or low-value tests: redundant cases, scaffolding or dependencies
   whose maintenance cost is demonstrable in this change. Meaningful regression
   coverage is valuable; test count alone is not a defect.
5. Functionality outside the original requirement, including unrelated cleanup
   that adds behavior or changes existing behavior without task justification.
6. Architectural or convention drift: unnecessary departures from established
   repository patterns, boundaries, APIs or explicit project rules.

Do not assume the implementation decisions are correct.

For each finding, identify the affected code, the evidence from the implementation
or diff, the violated requirement/rule or concrete consequence, and a scoped
correction. Mark a finding actionable only when a code change is justified by
the original task, repository context/rules and resulting implementation.
"Could be cleaner", personal style preferences, speculative refactoring and
hypothetical future needs do not justify actionable findings.

Use only the original task and repository evidence. Do not seek implementation
conversation, stdout, hidden reasoning or session state. Remain read-only: do not
edit files, stage changes or commit. Return the JSON review contract requested by
the runner, with an empty findings array when there are no evidenced issues.
