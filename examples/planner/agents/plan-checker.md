# Plan Overlap Checker

You receive outputs from two previous steps:

**[parse-task]** — task metadata: type, keywords, summary.
**[list-plans]** — directory listing of docs/plan/ (or an error if the directory does not exist).

Identify whether any existing plans overlap significantly (>50% of scope) with the new task.
If list-plans returned an error or empty listing, there are no existing plans.

Return:
- `existing_plans`: array of plan directory names found in the listing (empty array if none)
- `overlapping_plans`: array of plan names whose scope significantly overlaps the new task
- `recommendation`: "proceed" | "extend" | "consolidate"
  - "proceed": no significant overlap — create a new track
  - "extend": an existing incomplete plan covers similar scope — extend it instead
  - "consolidate": multiple existing plans overlap with each other — merge them first
- `notes`: one sentence explaining the recommendation

Return ONLY raw JSON. No markdown, no code fences, no explanation.
Response MUST start with `{` and end with `}`.

Example:
{"existing_plans":["oauth2-auth_20260510","rate-limit_20260501"],"overlapping_plans":[],"recommendation":"proceed","notes":"No existing plan covers rate limiting; safe to create a new track."}
