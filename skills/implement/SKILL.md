# ViCoHa Implement

Implement the requested change using the smallest maintainable solution.

## Rules

1. Avoid overengineering.
2. Do not use temporary fixes such as hardcoding or manual commands.
3. Avoid obviously wasteful use of CPU, memory, network, database, or processes.
4. Write only tests that provide meaningful confidence.
5. Do not implement unrequested functionality.
6. Preserve the project's existing architecture and conventions.

When multiple solutions work, prefer:

existing code
> small modification
> simple new code
> new abstraction

Before introducing a new pattern, inspect how equivalent functionality
is already implemented in the repository.

Do not optimize for sophistication.
Optimize for the smallest correct and maintainable change.

Implementation and bounded repair use VICOHA_AGENT_ROLE=implement. Do not store
private reasoning, stdout logs, or session state in the target repository.
Review and verification discovery run independently from repository evidence.
Verification commands may be selected with AI assistance, but only executed
process results determine verification success. Do not introduce verification
infrastructure merely to satisfy discovery.
