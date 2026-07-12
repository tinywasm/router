package mock

import "github.com/tinywasm/router"

// Router graba las rutas registradas y permite dispararlas en un test.
//
// Guarda *Route, no RouteInfo: las anotaciones de permiso (Public/Requires) se
// encadenan DESPUÉS de registrar la ruta, así que una copia por valor tomada en
// el registro nunca las vería y el mock afirmaría que toda ruta es privada.
type Router struct {
	registered []*Route
	handlers   map[string]map[string]router.HandlerFunc // [method][path]handler
	streams    map[string]map[string]router.StreamFunc  // [method][path]handler
	sockets    map[string]map[string]router.SocketFunc  // [method][path]handler
}

func (r *Router) ensureHandlers() {
	if r.handlers == nil {
		r.handlers = make(map[string]map[string]router.HandlerFunc)
		r.streams = make(map[string]map[string]router.StreamFunc)
		r.sockets = make(map[string]map[string]router.SocketFunc)
	}
}

func (r *Router) registerRoute(method, path string, resource, action string) *Route {
	r.ensureHandlers()
	route := &Route{
		info: router.RouteInfo{
			Method:   method,
			Path:     path,
			Resource: resource,
			Action:   action,
		},
	}
	r.registered = append(r.registered, route)
	return route
}

func (r *Router) Get(path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute("GET", path, "", "")
	r.ensureHandlers()
	if r.handlers["GET"] == nil {
		r.handlers["GET"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["GET"][path] = h
	return route
}

func (r *Router) Post(path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute("POST", path, "", "")
	r.ensureHandlers()
	if r.handlers["POST"] == nil {
		r.handlers["POST"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["POST"][path] = h
	return route
}

func (r *Router) Put(path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute("PUT", path, "", "")
	r.ensureHandlers()
	if r.handlers["PUT"] == nil {
		r.handlers["PUT"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["PUT"][path] = h
	return route
}

func (r *Router) Delete(path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute("DELETE", path, "", "")
	r.ensureHandlers()
	if r.handlers["DELETE"] == nil {
		r.handlers["DELETE"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["DELETE"][path] = h
	return route
}

func (r *Router) Options(path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute("OPTIONS", path, "", "")
	r.ensureHandlers()
	if r.handlers["OPTIONS"] == nil {
		r.handlers["OPTIONS"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["OPTIONS"][path] = h
	return route
}

func (r *Router) Handle(method, path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute(method, path, "", "")
	r.ensureHandlers()
	if r.handlers[method] == nil {
		r.handlers[method] = make(map[string]router.HandlerFunc)
	}
	r.handlers[method][path] = h
	return route
}

func (r *Router) Stream(path string, h router.StreamFunc) router.Route {
	route := r.registerRoute("GET", path, "", "")
	r.ensureHandlers()
	if r.streams["GET"] == nil {
		r.streams["GET"] = make(map[string]router.StreamFunc)
	}
	r.streams["GET"][path] = h
	return route
}

func (r *Router) Socket(path string, h router.SocketFunc) router.Route {
	route := r.registerRoute("GET", path, "", "")
	r.ensureHandlers()
	if r.sockets["GET"] == nil {
		r.sockets["GET"] = make(map[string]router.SocketFunc)
	}
	r.sockets["GET"][path] = h
	return route
}

func (r *Router) Use(m ...router.Middleware) {
	// middleware is not recorded in this mock
}

// Routes proyecta las rutas registradas en el momento de la consulta, ya con sus
// anotaciones de permiso aplicadas.
func (r *Router) Routes() []router.RouteInfo {
	out := make([]router.RouteInfo, 0, len(r.registered))
	for _, route := range r.registered {
		out = append(out, route.info)
	}
	return out
}

// Invoke ejecuta el handler registrado para un método y ruta.
func (r *Router) Invoke(method, path string, ctx router.Context) {
	r.ensureHandlers()
	if handlers, ok := r.handlers[method]; ok {
		if h, ok := handlers[path]; ok {
			h(ctx)
			return
		}
	}
}

var _ router.Router = (*Router)(nil)
