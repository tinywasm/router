package mock

import "github.com/tinywasm/router"

// Route implementa router.Route para el mock, grabando las anotaciones de permiso.
type Route struct {
	info router.RouteInfo
}

func (r *Route) Requires(resource string, action string) router.Route {
	r.info.Resource = resource
	r.info.Action = action
	return r
}

var _ router.Route = (*Route)(nil)
