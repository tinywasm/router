package router

import (
	"testing"

	"github.com/tinywasm/fmt"
)

// fakeContext prueba que cualquier tipo implementando Context se escribe guiado
// por la interfaz — sin fallbacks a `net/http`.
type fakeContext struct {
	values map[string]any
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

// fakeRouter prueba que Router registra rutas tipadas.
type fakeRouter struct {
	routes map[string]HandlerFunc
}

func (f *fakeRouter) Get(path string, h HandlerFunc)        { if f.routes == nil { f.routes = make(map[string]HandlerFunc) }; f.routes[path] = h }
func (f *fakeRouter) Post(path string, h HandlerFunc)       { f.Get(path, h) }
func (f *fakeRouter) Put(path string, h HandlerFunc)        { f.Get(path, h) }
func (f *fakeRouter) Delete(path string, h HandlerFunc)     { f.Get(path, h) }
func (f *fakeRouter) Options(path string, h HandlerFunc)    { f.Get(path, h) }
func (f *fakeRouter) Handle(method, path string, h HandlerFunc) { f.Get(path, h) }
func (f *fakeRouter) Stream(path string, h StreamFunc)      {}
func (f *fakeRouter) Socket(path string, h SocketFunc)      {}
func (f *fakeRouter) Use(m ...Middleware)                   {}

var _ Router = (*fakeRouter)(nil)

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
