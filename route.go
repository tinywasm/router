package router

// Route describes a registered route and allows annotating it. It is returned by
// each Router registration method. Annotations are declarative: the contract does
// not enforce them — each concrete implementer (native server, edge runtime) enforces them.
type Route interface {
	// Requires binds an RBAC permission to the route: the (resource, action) pair.
	// action is a string, matching the source of truth user.Permission.Action string.
	// Readable and extensible: "write", "read", "orders:export" — not a cryptic byte.
	Requires(resource string, action string) Route
	// Public marks the route as accessible without identity. The absence of this
	// marker (and Requires) means the route is private by default.
	Public() Route
}

// RouteInfo is the read-only view of a registered route — for introspection.
type RouteInfo struct {
	Method   string // e.g. "GET", "POST"
	Path     string // e.g. "/api/users", "/api/orders/:id"
	Resource string // e.g. "users", "orders"; "" = public route (no RBAC)
	Action   string // e.g. "read", "write", "orders:export"
	Public   bool   // true = accessible without identity
}
