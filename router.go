package router

import "github.com/tinywasm/model"

// Context is the minimal abstraction seen by a handler: request → response.
// Same interface signature for both native (!wasm) and edge/wasm targets.
//
// Ownership: a Context belongs to ONE goroutine (the handler's);
// implementations are not required to be safe for concurrent use
// (same contract as http.ResponseWriter). To feed it from other
// goroutines, send the data over a channel to the owning goroutine —
// never share the Context itself.
type Context interface {
	Method() string
	Path() string
	Body() []byte
	GetHeader(key string) string
	SetHeader(key, value string)
	WriteStatus(code int)
	Write(b []byte) (int, error)
	// Request-scoped values (middleware passes data to the next handler).
	SetValue(key string, v any)
	Value(key string) any
	// Isomorphic cookies.
	SetCookie(c Cookie)             // writes a cookie to the response
	Cookie(name string) (Cookie, bool) // reads a cookie from the request; ok=false if not found
	// Request-scoped identity. An auth middleware records the caller;
	// handlers and mounted modules read it.
	SetUserID(id string) // records the authenticated identity (id "" = anonymous)
	UserID() string      // reads the identity; "" if no valid session
}

// HandlerFunc is the dispatch unit: receives a Context and responds to it.
type HandlerFunc func(Context)

// Streamer is a Context that also flushes writes immediately.
// Used for incremental responses (SSE, streaming).
//
// Ownership: same single-goroutine contract as Context. A push loop
// (SSE hub, broker) must deliver messages to the handler's goroutine
// via a channel; only that goroutine calls Write/Flush.
type Streamer interface {
	Context
	Flush() // sends to the client what has been written so far, without closing the response
}

// StreamFunc is a handler that receives a typed Streamer.
type StreamFunc func(Streamer)

// Socket is the bidirectional upgraded connection (WebSocket).
// Isomorphic abstraction: does not touch concrete upgrade mechanisms.
type Socket interface {
	Read() ([]byte, error)
	Write(b []byte) error
	Close() error
}

// SocketFunc is a handler that receives a typed Socket.
type SocketFunc func(Socket)

// Middleware wraps a handler to add cross-cutting logic (auth, logging).
// Operate ONLY on Context — never on concrete transport types.
type Middleware func(HandlerFunc) HandlerFunc

// Router is what a module registers its routes on.
// A concrete implementer (native server, edge runtime) satisfies this interface;
// modules and hosts only consume it.
type Router interface {
	Get(path string, h HandlerFunc) Route
	Post(path string, h HandlerFunc) Route
	Put(path string, h HandlerFunc) Route
	Delete(path string, h HandlerFunc) Route
	Options(path string, h HandlerFunc) Route
	Handle(method, path string, h HandlerFunc) Route
	Stream(path string, h StreamFunc) Route
	Socket(path string, h SocketFunc) Route

	// PublicAsset registers ONE route serving ONE file to the browser: generated
	// content such as index.html, the stylesheet, the JS bundle or the wasm binary.
	//
	// It is public by construction — a browser fetching an asset has no identity
	// yet. It returns no Route: there is no permission to attach, so an asset can
	// neither be left private by accident (a silent 403 on a blank page) nor be
	// wrongly gated. Serving a file that DOES need permissions is a normal route:
	// Get(path, h).Requires(resource, action) — which fails closed if forgotten.
	PublicAsset(path string, h HandlerFunc)

	// PublicDir serves a whole directory under a prefix (e.g. "web/public").
	// Same contract as PublicAsset: public by construction, no Route to gate.
	PublicDir(prefix string, dir string)

	Use(m ...Middleware)
	// Routes enumerates the registered routes and their metadata.
	Routes() []RouteInfo
}

// APIModule is a module that exposes a server API.
// It is consumed by the server entry point (!wasm): which passes it the host's Router,
// and the module registers its own routes/handlers. Since Router is isomorphic,
// the module never imports net/http to describe its API. The concrete transport
// (binary upload, another protocol mounted as a route) is the module's internal decision.
type APIModule interface {
	model.ModuleNaming // provides ModelName() — identity
	MountAPI(r Router)
}
