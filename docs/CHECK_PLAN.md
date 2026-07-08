# PLAN — `router.Caller`: the client-side call contract (mirror of `APIModule`)

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

## Context (zero-context summary)

`tinywasm/router` is the isomorphic communication contract of the tinywasm
ecosystem: no `net/http` in the public API, everything self-descriptive via
signatures. Today it only covers the **server side**: a module implements
`APIModule.MountAPI(router.Router)` and never sees the concrete transport.

There is **no client-side counterpart**. A view that wants to invoke a named
server operation has no contract to depend on, so consumers ended up importing
`tinywasm/mcp` directly and hand-writing the JSON-RPC envelope. This already
produced a real, silent bug in a consumer (`veltylabs/mjosefa-cms`): the view
passed a tool name as the JSON-RPC `method`, the server only dispatches
`tools/call`, and every call died with `METHOD_NOT_FOUND` — it compiled fine and
no test caught it.

**Goal:** add the `Caller` interface — the call-side contract, symmetric to
`APIModule` (mount-side). Adapters live elsewhere (`tinywasm/mcp` ships
`NewCaller`, see that repo's plan); this repo only owns the contract.

## Change — new file `caller.go` (additive, no break)

```go
package router

import "github.com/tinywasm/model"

// Caller is how a client-side view invokes a named server operation without
// knowing the wire protocol or transport. It mirrors APIModule: APIModule is
// the mount-side contract, Caller is the call-side contract.
//
// op is the logical operation name (e.g. "list_services") — NEVER a wire-level
// method. Translating op to the concrete envelope is the adapter's job (e.g.
// mcp.NewCaller adapts *mcp.Client). Test doubles satisfy Caller with canned
// bytes and no transport.
type Caller interface {
	// Call invokes op; result/err arrive asynchronously via callback
	// (works for wasm fetch and for in-process test doubles alike).
	// Implementations MUST propagate every error — never swallow.
	Call(op string, args model.Encodable, callback func(result []byte, err error))

	// Dispatch is fire-and-forget (no response expected).
	Dispatch(op string, args model.Encodable)
}
```

Design decisions (final, do not re-litigate):

- **`args model.Encodable`, not `any`.** Ecosystem rule "zero `any`"; adapters
  already require `Encodable` by type-assertion internally — with the type in
  the signature, misuse does not compile instead of failing silently at runtime.
  This adds the (already ubiquitous) `tinywasm/model` dependency to `router`.
- **Callback-based, not blocking.** Matches wasm `fetch` reality and the
  existing `mcp.Client.Call` shape; a test double calls the callback inline.
- **No context parameter.** Client-side wasm has no cancellation story today;
  adapters own whatever internal context their transport needs. Adding one later
  is the adapter's concern, not this contract's.

## Mock support

Add a `Caller` fake to the existing `mock` package (same pattern as the other
mocks there): records `op`/`args`, replies with configurable canned bytes or
error. This is what consumer view-tests inject.

## Tests

- Compile-time contract test: the mock satisfies `Caller`.
- Behavior test on the mock: recorded op/args, canned reply delivered, error
  path delivered (never swallowed).
- Run with `gotest ./...` (never `go test`). Prerequisite:
  `go install github.com/tinywasm/devflow/cmd/gotest@latest`.

## Documentation (mandatory)

- `README.md`: new short section "Caller — the call-side contract", mirroring
  the existing `APIModule` explanation (one code block + one paragraph: modules
  depend on `Caller`, adapters live with each transport, e.g. `mcp.NewCaller`).
- `docs/ARCHITECTURE.md`: does not exist today — do NOT create it for this
  change; the README section is the record.

## Harness checklist (mandatory)

- No stdlib imports in library code (`tinywasm/fmt` only if needed).
- No `any`, no `map`, no generics in the public API.
- No hardcoded repeated strings; none should be needed here.
- Additive only: existing `Router`/`Context`/`APIModule` signatures untouched.

## Acceptance criteria

1. `caller.go` compiles for both native and wasm targets; `gotest ./...` green.
2. `mock` package exposes a `Caller` fake usable by downstream view tests.
3. README documents the contract and names `mcp.NewCaller` as the reference
   adapter.
4. No existing public symbol changed (verified by compiling a downstream
   consumer against the new version without edits).

## Stages

| Stage | File | Action |
|---|---|---|
| 1 | `caller.go` | add `Caller` interface (spec above, verbatim semantics) |
| 2 | `mock/caller.go` | fake `Caller`: recorded calls + canned replies/errors |
| 3 | `router_test.go` or `caller_test.go` | contract + mock behavior tests |
| 4 | `README.md` | document the call-side contract |
