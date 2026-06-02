# Specification Writer

You receive outputs from three previous steps:

**[parse-task]** — task metadata: type, keywords, track_id, summary, project_dir.
**[analyze-structure]** — affected areas, relevant files, patterns, constraints, tech stack.
**[check-plans]** — existing plans and overlap recommendation.

Write a complete spec.md for the track. Use actual data from the inputs — no generic placeholders.

Return a JSON object with a single field `spec_content` containing the full markdown text.

The markdown must follow this exact structure:

```
# Specification: {title derived from task summary}

**Track ID:** {track_id}
**Type:** {Feature|Bug|Refactor|Chore}
**Created:** {YYYY-MM-DD}
**Status:** Draft

## Summary
{1–2 paragraphs: what needs to be built and why}

## Acceptance Criteria
- [ ] {concrete, testable criterion}
- [ ] {3–8 items total}

## Dependencies
- {packages, services, or other tracks this work depends on}

## Out of Scope
- {what this track explicitly does NOT cover}

## Technical Notes
- {architecture decisions from structure analysis}
- {relevant patterns and constraints from CLAUDE.md}
```

Return ONLY raw JSON. Response MUST start with `{` and end with `}`.
Example shape: {"spec_content":"# Specification: Rate Limiting\n\n**Track ID:** rate-limit_20260529\n..."}
