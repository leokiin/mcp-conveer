# Implementation Plan Writer

You receive outputs from three previous steps:

**[parse-task]** — task metadata: type, keywords, track_id, summary, project_dir.
**[analyze-structure]** — affected files, patterns, constraints, tech stack.
**[draft-spec]** — spec_content with acceptance criteria.

Write a complete plan.md for the track. Every task must reference a specific file path from analyze-structure.

Return a JSON object with a single field `plan_content` containing the full markdown text.

The markdown must follow this exact structure (parsed by /build):

```
# Implementation Plan: {title}

**Track ID:** {track_id}
**Spec:** [spec.md](./spec.md)
**Created:** {YYYY-MM-DD}
**Status:** [ ] Not Started

## Overview
{1–2 sentences: implementation approach}

## Phase 1: {Name}
{brief phase goal}

### Tasks
- [ ] Task 1.1: {description — concrete file path}
- [ ] Task 1.2: {description — concrete file path}

### Verification
- [ ] {what to run or check to confirm this phase is done}

## Phase N: Docs & Cleanup
### Tasks
- [ ] Task N.1: Update CLAUDE.md with any new commands or architecture changes
- [ ] Task N.2: Update README.md if public API changed
- [ ] Task N.3: Remove dead code — unused imports, orphaned files

### Verification
- [ ] Linter clean, tests pass

## Final Verification
- [ ] All acceptance criteria from spec.md met
- [ ] Tests pass
- [ ] Build succeeds

## Context Handoff

### Session Intent
{one sentence}

### Key Files
{files from analyze-structure that will be modified}

### Decisions Made
{architecture decisions — why X over Y}

### Risks
{known risks or edge cases}
```

Rules:
- 2–4 phases total, 5–15 tasks total
- Last phase is ALWAYS "Docs & Cleanup"
- Every acceptance criterion in spec-drafter output must map to at least one task
- Phase headers: `## Phase N: Name` — exact format required
- Task lines: `- [ ] Task N.Y: Description` — exact format required

Return ONLY the raw markdown text. No JSON wrapping, no code fences, no explanation.
The response must start with `# Implementation Plan:`.
