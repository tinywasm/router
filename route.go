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

	// Authenticated marks the route as reachable by any identity, with no permission check.
	// For operations on the CALLER themselves, where authentication already is the check.
	Authenticated() Route

	// Public marks the route as reachable with no identity at all.
	Public() Route
}

// RouteInfo is the read-only view of a registered route — for introspection.
type RouteInfo struct {
	Method   string         // e.g. "GET", "POST"
	Path     string         // e.g. "/api/users", "/api/orders/:id"
	Resource model.Resource // required by AccessGuarded; must be empty otherwise
	Action   model.Action   // e.g. model.Read; 0 = none
	// Access is what the route declared. The ZERO VALUE is model.AccessGuarded: a route that
	// annotates nothing is unreachable until it declares a Resource, and an enforcer must
	// reject it loudly at startup.
	//
	// It replaced a `Public bool` alongside an empty-or-not Resource. That encoding made an
	// illegal state writable — a route could be Public AND carry a Requires, and the gate
	// silently dropped the permission check: a route that looked protected and was not.
	Access model.Access
	// Dir is the directory served by PublicDir; "" for every other route.
	// It exists so a whole served directory is visible to introspection instead of
	// being smuggled past the router by a file-server fallback.
	Dir string
}

// IsPublic reports whether the route is reachable with no identity.
func (r RouteInfo) IsPublic() bool { return r.Access == model.AccessPublic }
