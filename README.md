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

- **`Context`**: minimal I/O (read method/path/body, write headers/status)
- **`HandlerFunc`**: `func(Context)` — the unit of dispatch
- **`Router`**: register routes (Get/Post/Put/Delete/Handle) + streaming (Stream/Socket) + middleware (Use)
- **`Streamer`**: Context + Flush() for SSE/streaming responses
- **`Socket`**: bidirectional connection (WebSocket)
- **`Middleware`**: `func(HandlerFunc) HandlerFunc` — transversal logic (auth, logging)
- **`APIModule`**: module + MountAPI(Router) — how modules publish APIs

## Design

No `net/http` in the public API. Handlers never import Go's standard library HTTP types. All routing is self-describing via signatures — no runtime type assertions, no hidden machinery.

See [docs/PLAN_EXECUTED.md](docs/PLAN_EXECUTED.md) for implementation details.
