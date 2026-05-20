# MCP SDK Gotchas

Observations from integrating `github.com/modelcontextprotocol/go-sdk` v1.6.0.
These are reference-material notes, not one-time commit observations.

---

## `CallToolParams.Arguments` is typed `any`, not `json.RawMessage`

`pkg.go.dev` shows `Arguments json.RawMessage` in `CallToolParamsRaw`, but the
struct actually used by client code is `CallToolParams` (in `protocol.go`), which has:

```go
type CallToolParams struct {
    Arguments any `json:"arguments,omitempty"`
}
```

**Symptom**: passing pre-marshaled `[]byte` as `Arguments` causes the entire value
to be base64-encoded in the wire JSON (because `json.Marshal([]byte{...})` produces
a base64 string).

**Fix**: pass a Go map or struct literal directly — the SDK marshals it internally:

```go
// Wrong — passes []byte, which gets base64-encoded:
argsJSON, _ := json.Marshal(map[string]any{"reference": "VAN5022058"})
params := &sdkmcp.CallToolParams{Name: "track_shipment", Arguments: argsJSON}

// Correct — pass the value directly:
params := &sdkmcp.CallToolParams{
    Name:      "track_shipment",
    Arguments: map[string]any{"reference": "VAN5022058"},
}
```

---

## `omitempty` on JSON tags controls whether a field is `required` in the schema

The SDK uses `github.com/google/jsonschema-go` to derive the input schema from
the handler's `In` type parameter. By default, **all struct fields are emitted as
`required`**, even pointer types such as `*string`.

To make a field optional (absent from the `required` array), add `omitempty` to
the JSON tag:

```go
// Wrong — schema emits reference_type as required:
ReferenceType *string `json:"reference_type"`

// Correct — schema treats reference_type as optional:
ReferenceType *string `json:"reference_type,omitempty"`
```

**Why this matters**: callers that omit an optional field will get a schema
validation error (`required: missing properties: ["reference_type"]`) before
the handler is ever invoked.

---

## `errors.Is` through `*UpstreamError` works without modification

The cache layer returns fetcher errors directly (no wrapping) when there is no
stale entry. `*domain.UpstreamError.Unwrap()` returns the sentinel error, so
`errors.Is(err, domain.ErrShipmentNotFound)` works correctly at the MCP tool
boundary without any string-matching fallback.

Do **not** use `.Error()` string-matching to discriminate domain error classes.
The sentinel errors and `Unwrap()` chain are the contract.

---

## `AddReceivingMiddleware` intercepts all MCP methods, not just `tools/call`

The `Server.AddReceivingMiddleware` hook wraps every incoming JSON-RPC method
(`initialize`, `tools/list`, `tools/call`, notifications, etc.). Filter on
`method == "tools/call"` before recording tool-call metrics; otherwise every
`initialize` handshake increments the counter.

See `internal/mcp/server.go` `observingMiddleware` for the pattern.
