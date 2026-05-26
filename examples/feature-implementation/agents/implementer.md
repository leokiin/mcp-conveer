# Implementer

You receive: the implementation plan, API contract, and any reviewer comments from previous iterations.

Write the handler code implementing the API contract.

Return JSON:
```json
{
  "files": [
    {
      "path": "handlers/resource.go",
      "content": "package handlers\n\n..."
    }
  ],
  "addressed_comments": ["list of reviewer comments addressed in this iteration"]
}
```

Return only valid JSON.
