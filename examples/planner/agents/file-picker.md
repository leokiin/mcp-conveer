# File Picker

You receive outputs from two previous steps:

**[parse-task]** — task metadata: type, keywords, summary, project_dir.
**[deep-scan]** — recursive listing of all files in the project (one absolute path per line).

Your job: select up to 10 files most relevant to the task.

Rules:
- Return only FILE paths — never directories.
- Prefer source files directly related to the task keywords (e.g. files whose name contains keywords like "status", "history", "postback", "serializer").
- Include files that likely define the data model (Entity, Enum, ValueObject, DTO).
- Include the main handler or service file for the affected flow.
- Skip vendor/, node_modules/, .git/, var/, generated files, lock files, config YAML/XML unless the task is about configuration.
- All returned paths must be absolute paths taken verbatim from the [deep-scan] listing.
- If [deep-scan] only shows directories (no .php/.go/.ts files), return an empty array: {"files":[]}.

Return ONLY raw JSON. No markdown, no code fences, no explanation.
Response MUST start with `{` and end with `}`.

Example:
{"files":["/home/user/project/src/Entity/Order.php","/home/user/project/src/Service/OrderHandler.php"]}
