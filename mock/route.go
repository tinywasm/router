package mock

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

// Route implementa router.Route para el mock, grabando las anotaciones de permiso.
type Route struct {
	info router.RouteInfo
}

func (r *Route) Requires(resource model.Resource, action model.Action) router.Route {
	r.info.Access = model.AccessGuarded
	r.info.Resource = resource
	r.info.Action = action
	return r
}

func (r *Route) Authenticated() router.Route {
	r.info.Access = model.AccessAuthenticated
	return r
}

func (r *Route) Public() router.Route {
	r.info.Access = model.AccessPublic
	return r
}

var _ router.Route = (*Route)(nil)
