# Function Describer

You receive one input from the previous pipeline step:

**[summarize-context]** — a compact JSON object with fields:
- `function_name`, `signature`, `code` — the exact Go function verbatim
- `imports` — only the imports used in the function body
- `library` — primary external library name
- `library_id` — resolved Context7 library ID
- `relevant_docs` — a short summary of what the library provides for this function

Write a COMPREHENSIVE technical description of the function.

Requirements:
- Quote parameter names, types, and field names EXACTLY as written in the source code.
- Describe each branch of logic (if/else, early returns) in the behavior section.
- Use the library name and relevant_docs from summarize-context to explain what the external API provides.
- The "notes" field must contain concrete observations: what can go wrong, what the caller must know,
  non-obvious constraints or defaults.
- Do not add information that is not in the source. If something is unclear, say so.

Return ONLY raw JSON. No markdown, no code fences, no explanation.
Response MUST start with `{` and end with `}`.

Required fields:
- function: exact function name
- file_location: "relative/path/file.go:line" (make path relative to project root if possible)
- signature: complete function signature line
- summary: one precise sentence — what this function constructs or does
- purpose: 2–4 sentences — role of this function in the larger system, why it exists
- parameters: array of objects, one per parameter:
    {name, type, description, default_value} — default_value is "" if none
- returns: {type, description}
- behavior: numbered array of strings — step-by-step walk through the function body
- library_context: what the external library (from fetch-docs) provides to this function,
    citing real names from the code (types, functions, methods used or referenced)
- notes: array of concrete observations — defaults applied, edge cases, constraints, potential issues

Example shape (fill every field with real content from the source):
{
  "function": "New",
  "file_location": "internal/agents/mcpclient/agent.go:28",
  "signature": "func New(name, command, toolName, inputKey string, extraArgs map[string]string) *Agent",
  "summary": "Constructor that creates a configured MCP client agent ready to launch an external MCP server process and invoke a specific tool.",
  "purpose": "...",
  "parameters": [
    {"name": "name", "type": "string", "description": "...", "default_value": ""},
    {"name": "extraArgs", "type": "map[string]string", "description": "...", "default_value": ""}
  ],
  "returns": {"type": "*Agent", "description": "..."},
  "behavior": ["1. Checks if inputKey is empty ...", "2. Returns &Agent{...} with all fields set"],
  "library_context": "...",
  "notes": ["inputKey silently defaults to 'query' ...", "command is later split by spaces in Handle() ..."]
}
