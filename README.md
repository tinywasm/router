# tinywasm/router

<img src="docs/img/badges.svg">

Isomorphic routing contract — `Context`, `Router`, `HandlerFunc` identical on native and edge/wasm targets. Defines request handling, streaming (SSE), WebSocket upgrade, and middleware. Modules mount their API via `APIModule`. The router defines the shape; concrete servers implement it.

## Quick Start

```go
import "github.com/tinywasm/router"

type MyModule struct{ name string }

func (m MyModule) ModelName() string { return m.name }
func (m MyModule) MountAPI(r router.Router) {
    r.Get("/api/data", func(ctx router.Context) {
        ctx.WriteStatus(200)
        ctx.Write([]byte("hello"))
    })
}
```

## Caller — the call-side contract

```go
func (v *MyView) Refresh() {
    v.Caller.Call("list_services", nil, func(res []byte, err error) {
        if err != nil {
            v.HandleError(err)
            return
        }
        v.Update(res)
    })
}
```

Modules and views depend on `Caller` to invoke server operations without knowing the wire protocol or transport. Adapters live with each transport (e.g. `mcp.NewCaller` in `tinywasm/mcp` adapts a JSON-RPC client), while tests use a `mock.Caller`.

## Contracts

- **`Context`**: minimal I/O (read method/path/body, write headers/status) + cookies (SetCookie/Cookie) + identity (`SetUserID`/`UserID`) + typed codec (`Decode`/`Encode`)
- **`Cookie`**: isomorphic HTTP cookie type with SameSite policy (SameSiteDefault/Lax/Strict/None)
- **`HandlerFunc`**: `func(Context)` — the unit of dispatch
- **`Route`**: registration token; supports `Requires(resource, action)` for RBAC, `Public()` for explicit public access, and `Accepts(model.Fielder)` to declare the request-body schema
- **`RouteInfo`**: read-only view of a registered route with method, path, resource, action, public flag, and `Args` (the schema declared via `Accepts`)
- **`Router`**: register routes (Get/Post/Put/Delete/Handle) returning Route + register by logical name (`Op`) + streaming (Stream/Socket) + middleware (Use) + Routes() for introspection
- **`Streamer`**: Context + Flush() for SSE/streaming responses
- **`Socket`**: bidirectional connection (WebSocket)
- **`Middleware`**: `func(HandlerFunc) HandlerFunc` — transversal logic (auth, logging)
- **`APIModule`**: module + MountAPI(Router) — how modules publish APIs
- **`Caller`**: call-side contract — how a client-side view invokes a named server operation
- **`mock`**: subpackage with canonical test doubles (Router, Context, Route, Caller) — no `net/http`, WASM-safe

## Op — provider-side dispatch by name, with a typed codec at the edge

`Op` is the provider-side counterpart of `Caller.Call(name, args, cb)`: a module registers an
operation by **logical name**, never a path or an HTTP verb, so the SAME `APIModule` serves any
transport without importing it:

```go
func (m *Module) MountAPI(r router.Router) {
    r.Op("upsert_catalog_item", m.upsert).
        Requires("catalog_item", model.Create).
        Accepts(&CatalogItem{})
}

func (m *Module) upsert(ctx router.Context) {
    var in CatalogItem
    if err := ctx.Decode(&in); err != nil {
        ctx.WriteStatus(400)
        return
    }
    // … domain logic …
    ctx.Encode(&out)
}
```

- `Op` + `Accepts` let a transport (e.g. `mcp`) harvest the operation's name, RBAC and schema
  without the module ever importing that transport.
- `Context.Decode`/`Encode` let the handler work in typed `model.Decodable`/`Encodable` values —
  it never imports a codec package (`json`, `jsvalue`) directly; the transport supplies the codec.
- `router/conformance` covers all four additions (`op_route_*`, `context_decodes_and_encodes_typed_payload`); any implementation proves it the same way it proves the rest of the contract.

## Design

No `net/http` in the public API. Handlers never import Go's standard library HTTP types. All routing is self-describing via signatures — no runtime type assertions, no hidden machinery.


