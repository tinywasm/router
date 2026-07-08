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
