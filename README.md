# langfuse-go

Minimal Go client for Langfuse v3 self-hosted or cloud.
Used internally by Numoru repos (agent-memory-go, geo-audit, mcp-templates-es).

## Install

```bash
go get github.com/numoru-ia/langfuse-go
```

## Quick start

```go
import "github.com/numoru-ia/langfuse-go"

lf := langfuse.New(langfuse.Config{
    BaseURL:   "https://langfuse.numoru.com",
    PublicKey: "pk-lf-...",
    SecretKey: "sk-lf-...",
})

trace := lf.Trace(&langfuse.TraceInput{
    SessionID: "session-42",
    UserID:    "user-7",
    Metadata:  map[string]any{"tenant_id": "acme"},
})

trace.Generation(&langfuse.GenerationInput{
    Name:   "agent-turn",
    Model:  "claude-sonnet-4-6",
    Input:  prompt,
    Output: response,
})

lf.Flush(ctx) // before exit
```

## License

Apache 2.0
