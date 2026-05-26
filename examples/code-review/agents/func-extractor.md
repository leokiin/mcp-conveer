# Function Extractor

You receive two inputs from previous steps:

**[parse-task]** — parsed file location:
{"file": "/path/to/file.go", "line": 28}

**[file-reader]** — the full Go source file content as text in the "response" field.

Your job:
1. Count lines in the file content to find the function at the given line number.
   - If line is 0, pick the first exported function in the file.
   - The function starts at or nearest to the given line (look ±5 lines if needed).
2. Extract the COMPLETE function body — from `func` keyword to the matching closing `}`.
   Include every line: signature, body, nested blocks, closing brace.
3. Extract ALL import paths from the file's import block.
4. Note the package name from the `package` declaration.
5. Determine if it is a method (has a receiver) or a plain function.

Return ONLY raw JSON. No markdown, no code fences, no explanation.
Response MUST start with `{` and end with `}`.

Required fields — use exact values from the source, do not paraphrase:
- package: Go package name
- function_name: the function or method name
- receiver: receiver type string if method, empty string if plain function
- is_method: true/false
- line: line number where the function signature starts
- signature: the full first line of the function (func keyword through opening brace)
- code: the complete function source including signature and closing brace
- imports: array of all import paths found in the file (both stdlib and external)

Example shape:
{"package":"mcpclient","function_name":"New","receiver":"","is_method":false,"line":28,"signature":"func New(name, command, toolName, inputKey string, extraArgs map[string]string) *Agent {","code":"func New(name, command, toolName, inputKey string, extraArgs map[string]string) *Agent {\n\tif inputKey == \"\" {\n\t\tinputKey = \"query\"\n\t}\n\treturn &Agent{name: name, command: command, toolName: toolName, inputKey: inputKey, extraArgs: extraArgs}\n}","imports":["context","encoding/json","fmt","strings","github.com/mark3labs/mcp-go/client","github.com/mark3labs/mcp-go/mcp"]}
