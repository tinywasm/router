package router

// Route describe una ruta ya registrada y permite anotarla. Lo devuelve cada método de
// registro del Router. Las anotaciones son declarativas: el contrato no las aplica —
// cada implementador concreto (serverd nativo, runtime edge) las hace cumplir.
type Route interface {
	// Requires ata un permiso RBAC a la ruta: el par (resource, action). action es string,
	// coincidiendo con la fuente de verdad user.Permission.Action string. Legible y
	// extensible: "write", "read", "orders:export" — no un byte críptico.
	Requires(resource string, action string) Route
	// Public marca la ruta como accesible sin identidad. La ausencia de este
	// marcador (y de Requires) implica que la ruta es privada por defecto.
	Public() Route
}

// RouteInfo es la vista de solo lectura de una ruta registrada — para introspección.
type RouteInfo struct {
	Method   string
	Path     string
	Resource string // "" = ruta pública (sin RBAC)
	Action   string
	Public   bool // true = accesible sin identidad
}
