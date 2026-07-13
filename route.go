package router

import "github.com/tinywasm/model"

// Route describes a registered route and allows annotating it. It is returned by
// each Router registration method. Annotations are declarative: the contract does
// not enforce them — each concrete implementer (native server, edge runtime) enforces them.
type Route interface {
	// Requires binds an RBAC permission to the route: the (resource, action) pair.
	//
	// Both are typed (model.Resource, model.Action). They used to be two bare strings in a
	// row, so swapping them compiled — and the failure was not an error but a SILENT denial
	// at runtime, in the one place where silence is unacceptable. Now it does not compile.
	//
	// The resource is open vocabulary: the app declares its own ("service_catalog"). The
	// action is a closed CRUD set (model.Read, model.Update, …): persistence has four verbs
	// and no tool in this ecosystem ever needed a fifth.
	Requires(resource model.Resource, action model.Action) Route
	// Public marks the route as accessible without identity. The absence of this
	// marker (and Requires) means the route is private by default.
	Public() Route
}

// RouteInfo is the read-only view of a registered route — for introspection.
type RouteInfo struct {
	Method   string         // e.g. "GET", "POST"
	Path     string         // e.g. "/api/users", "/api/orders/:id"
	Resource model.Resource // e.g. "users", "orders"; "" = no RBAC declared
	Action   model.Action   // e.g. model.Read; 0 = none
	Public   bool           // true = accessible without identity
	// Dir is the directory served by PublicDir; "" for every other route.
	// It exists so a whole served directory is visible to introspection instead of
	// being smuggled past the router by a file-server fallback.
	Dir string
}
