package router

import (
	"testing"

	"github.com/tinywasm/fmt"
)

// fakeContext prueba que cualquier tipo implementando Context se escribe guiado
// por la interfaz — sin fallbacks a `net/http`.
type fakeContext struct {
	values  map[string]any
	cookies map[string]Cookie
}

func (f *fakeContext) Method() string                      { return "GET" }
func (f *fakeContext) Path() string                        { return "/" }
func (f *fakeContext) Body() []byte                        { return nil }
func (f *fakeContext) GetHeader(key string) string         { return "" }
func (f *fakeContext) SetHeader(key, value string)         {}
func (f *fakeContext) WriteStatus(code int)                {}
func (f *fakeContext) Write(b []byte) (int, error)         { return len(b), nil }
func (f *fakeContext) SetValue(key string, v any)          { if f.values == nil { f.values = make(map[string]any) }; f.values[key] = v }
func (f *fakeContext) Value(key string) any                { if f.values == nil { return nil }; return f.values[key] }
func (f *fakeContext) SetCookie(c Cookie)                  { if f.cookies == nil { f.cookies = make(map[string]Cookie) }; f.cookies[c.Name] = c }
func (f *fakeContext) Cookie(name string) (Cookie, bool)   { if f.cookies == nil { return Cookie{}, false }; c, ok := f.cookies[name]; return c, ok }

var _ Context = (*fakeContext)(nil)

// fakeStreamer prueba que Streamer se escribe con Flush().
type fakeStreamer struct{ *fakeContext }

func (f *fakeStreamer) Flush() {}

var _ Streamer = (*fakeStreamer)(nil)

// fakeSocket prueba que Socket se escribe tipado.
type fakeSocket struct{}

func (f *fakeSocket) Read() ([]byte, error) { return nil, nil }
func (f *fakeSocket) Write(b []byte) error  { return nil }
func (f *fakeSocket) Close() error          { return nil }

var _ Socket = (*fakeSocket)(nil)

// fakeRouter prueba que Router registra rutas tipadas con metadatos.
type fakeRouter struct {
	routes     map[string]HandlerFunc
	registered []RouteInfo
}

func (f *fakeRouter) registerRoute(method, path string) Route {
	if f.routes == nil {
		f.routes = make(map[string]HandlerFunc)
	}
	r := &fakeRoute{info: RouteInfo{Method: method, Path: path}}
	return r
}

func (f *fakeRouter) Get(path string, h HandlerFunc) Route {
	r := f.registerRoute("GET", path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Post(path string, h HandlerFunc) Route {
	r := f.registerRoute("POST", path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Put(path string, h HandlerFunc) Route {
	r := f.registerRoute("PUT", path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Delete(path string, h HandlerFunc) Route {
	r := f.registerRoute("DELETE", path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Options(path string, h HandlerFunc) Route {
	r := f.registerRoute("OPTIONS", path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Handle(method, path string, h HandlerFunc) Route {
	r := f.registerRoute(method, path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Stream(path string, h StreamFunc) Route {
	return f.registerRoute("GET", path)
}

func (f *fakeRouter) Socket(path string, h SocketFunc) Route {
	return f.registerRoute("GET", path)
}

func (f *fakeRouter) Use(m ...Middleware) {
}

func (f *fakeRouter) Routes() []RouteInfo {
	return f.registered
}

var _ Router = (*fakeRouter)(nil)

// fakeRoute implementa Route para grabar anotaciones de permiso.
type fakeRoute struct {
	info RouteInfo
}

func (r *fakeRoute) Requires(resource string, action string) Route {
	r.info.Resource = resource
	r.info.Action = action
	return r
}

var _ Route = (*fakeRoute)(nil)

// fakeModule prueba que APIModule embebe ModuleNaming sin tocar tipos de transporte.
type fakeModule struct{ name string }

func (f fakeModule) ModelName() string      { return f.name }
func (f fakeModule) MountAPI(r Router)      {}

var _ fmt.ModuleNaming = fakeModule{}
var _ APIModule = fakeModule{}

// TestRouterContracts verifica que los contratos se escriben tipados.
func TestRouterContracts(t *testing.T) {
	var ctx Context = &fakeContext{}
	if ctx.Path() != "/" {
		t.Fatalf("Context.Path() failed")
	}

	var stream Streamer = &fakeStreamer{&fakeContext{}}
	stream.Flush()

	var sock Socket = &fakeSocket{}
	sock.Close()

	var r Router = &fakeRouter{}
	r.Get("/api", func(ctx Context) {})

	var m APIModule = fakeModule{name: "test"}
	if m.ModelName() != "test" {
		t.Fatalf("APIModule.ModelName() = %q, want %q", m.ModelName(), "test")
	}
	m.MountAPI(r)
}

// TestCookies verifica ida y vuelta de cookies.
func TestCookies(t *testing.T) {
	ctx := &fakeContext{}

	// Escribe una cookie
	cookie := Cookie{
		Name:     "sid",
		Value:    "xyz123",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: SameSiteLax,
	}
	ctx.SetCookie(cookie)

	// Lee la cookie
	read, ok := ctx.Cookie("sid")
	if !ok {
		t.Fatalf("Cookie(\"sid\") should exist")
	}
	if read.Name != "sid" || read.Value != "xyz123" {
		t.Fatalf("Cookie mismatch: got %+v, want %+v", read, cookie)
	}
	if read.Secure != true || read.HttpOnly != true {
		t.Fatalf("Cookie flags mismatch: Secure=%v, HttpOnly=%v", read.Secure, read.HttpOnly)
	}

	// Cookie no existente
	_, ok = ctx.Cookie("nonexistent")
	if ok {
		t.Fatalf("Cookie(\"nonexistent\") should not exist")
	}
}

// TestRouteMetadata verifica que Requires() registra metadatos de permiso.
func TestRouteMetadata(t *testing.T) {
	r := &fakeRouter{}

	// Registra una ruta con metadatos de permiso
	route := r.Post("/orders", func(ctx Context) {})
	route.Requires("orders", "write")

	// Verifica que la ruta está anotada (en fakeRoute)
	if fakeRoute, ok := route.(*fakeRoute); ok {
		if fakeRoute.info.Resource != "orders" || fakeRoute.info.Action != "write" {
			t.Fatalf("Route metadata mismatch: got Resource=%q, Action=%q", fakeRoute.info.Resource, fakeRoute.info.Action)
		}
	}
}

// TestSameSiteConstants verifica que SameSite está bien tipado.
func TestSameSiteConstants(t *testing.T) {
	tests := []struct {
		name  string
		value SameSite
	}{
		{"Default", SameSiteDefault},
		{"Lax", SameSiteLax},
		{"Strict", SameSiteStrict},
		{"None", SameSiteNone},
	}

	for _, tt := range tests {
		if int(tt.value) < 0 || int(tt.value) > 3 {
			t.Fatalf("SameSite %s out of range: %d", tt.name, tt.value)
		}
	}
}
