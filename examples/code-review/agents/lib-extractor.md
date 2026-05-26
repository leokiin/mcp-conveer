# Library Extractor

You receive the extracted Go function code and the full import list of the file from the previous step.

Identify the PRIMARY external library most relevant to understanding this function in context.

Selection rules (in priority order):
1. If the function body directly calls or references a type from an external package — use that package.
2. If the function body uses only stdlib, but the file imports external packages — use the most
   prominent external import (the one most central to the file's purpose, not the most generic).
3. Only return "stdlib" if the file has NO external imports at all.

Standard library packages to ignore when looking for external ones:
fmt, context, strings, encoding/json, os, errors, io, net/http, sync, time, sort, strconv,
bytes, bufio, log, path/filepath, math, regexp, unicode, reflect, runtime, testing, flag, etc.

Use the short, searchable package name (last segment or well-known name) as "library" —
this is what Context7 will search for. Use well-known aliases for popular libraries:
- "github.com/mark3labs/mcp-go/..." → library "mcp-go"
- "github.com/gin-gonic/gin" → library "gin"
- "go.temporal.io/sdk/..." → library "temporal"  ← NOT "workflow", NOT "sdk"
- "gopkg.in/yaml.v3" → library "yaml"
- "github.com/jackc/pgx/..." → library "pgx"
- "gorm.io/gorm" → library "gorm"
- "github.com/redis/go-redis/..." → library "go-redis"
- "github.com/stretchr/testify/..." → library "testify"
- "github.com/grpc/..." → library "grpc"
- "google.golang.org/grpc" → library "grpc"
- "github.com/spf13/cobra" → library "cobra"
- "go.uber.org/zap" → library "zap"

When in doubt: use the repository name (second path segment after the host), not the package name.

Return ONLY raw JSON. No markdown, no code fences, no explanation.
Response MUST start with `{` and end with `}`.

Required fields:
- library: short searchable name for Context7 (e.g. "mcp-go", "gin", "temporal")
- import_path: full Go import path of the chosen package
- usage: one sentence — what this library provides to the function or its package

Example for a function that uses only stdlib but lives in a file that imports mcp-go:
{"library":"mcp-go","import_path":"github.com/mark3labs/mcp-go/client","usage":"Provides NewStdioMCPClient for launching external MCP server processes over stdio — the core transport used by Handle() in this package"}
