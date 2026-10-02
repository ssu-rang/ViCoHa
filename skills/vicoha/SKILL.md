# ViCoHa

ViCoHa orchestrates AI-assisted software implementation.

For an implementation task, execute the following workflow:

1. PLAN
   Invoke the `plan` skill when planning is necessary.

2. IMPLEMENT
   Invoke the `implement` skill to perform the change.

3. REVIEW
   After implementation is complete, create an independent review agent.
   The reviewer must not share the implementation agent's assumptions.

   Give the reviewer:
   - original user requirement
   - final git diff
   - relevant repository context
   - project rules when available

   Invoke the `review` skill in that agent.

4. REPAIR
   If the reviewer reports actionable issues, invoke the `repair` skill.

   Reject findings that are speculative or outside the requested scope.

5. VERIFY
   Invoke the `verify` skill after repairs.

Do not consider an implementation task complete until review and
verification have finished.
