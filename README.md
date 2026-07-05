# router

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

## Contracts

- **`Context`**: minimal I/O (read method/path/body, write headers/status) + cookies (SetCookie/Cookie) + identity (`SetUserID`/`UserID`)
- **`Cookie`**: isomorphic HTTP cookie type with SameSite policy (SameSiteDefault/Lax/Strict/None)
- **`HandlerFunc`**: `func(Context)` — the unit of dispatch
- **`Route`**: registration token; supports `Requires(resource, action)` for RBAC and `Public()` for explicit public access
- **`RouteInfo`**: read-only view of a registered route with method, path, resource, action, and public flag
- **`Router`**: register routes (Get/Post/Put/Delete/Handle) returning Route + streaming (Stream/Socket) + middleware (Use) + Routes() for introspection
- **`Streamer`**: Context + Flush() for SSE/streaming responses
- **`Socket`**: bidirectional connection (WebSocket)
- **`Middleware`**: `func(HandlerFunc) HandlerFunc` — transversal logic (auth, logging)
- **`APIModule`**: module + MountAPI(Router) — how modules publish APIs
- **`mock`**: subpackage with canonical test doubles (Router, Context, Route) — no `net/http`, WASM-safe

## Design

No `net/http` in the public API. Handlers never import Go's standard library HTTP types. All routing is self-describing via signatures — no runtime type assertions, no hidden machinery.

See [docs/PLAN_EXECUTED.md](docs/PLAN_EXECUTED.md) for implementation details.
