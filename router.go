package router

import "github.com/tinywasm/fmt"

// Context es la abstracción mínima que ve un handler: petición → respuesta.
// Idéntica firma en el objetivo nativo (!wasm) y en el objetivo edge/wasm.
type Context interface {
	Method() string
	Path() string
	Body() []byte
	GetHeader(key string) string
	SetHeader(key, value string)
	WriteStatus(code int)
	Write(b []byte) (int, error)
	// Valores de ámbito de petición (middleware pasa datos al handler siguiente).
	SetValue(key string, v any)
	Value(key string) any
}

// HandlerFunc es la unidad de despacho: recibe un Context y responde sobre él.
type HandlerFunc func(Context)

// Streamer es un Context que además empuja lo escrito de inmediato.
// Usada para respuestas incrementales (SSE, streaming).
type Streamer interface {
	Context
	Flush() // envía al cliente lo escrito hasta ahora, sin cerrar la respuesta
}

// StreamFunc es un handler que recibe un Streamer tipado.
type StreamFunc func(Streamer)

// Socket es la conexión bidireccional ya upgradeada (WebSocket).
// Abstracción isomórfica: no toca mecanismos de upgrade concretos.
type Socket interface {
	Read() ([]byte, error)
	Write(b []byte) error
	Close() error
}

// SocketFunc es un handler que recibe un Socket tipado.
type SocketFunc func(Socket)

// Middleware envuelve un handler para añadir lógica transversal (auth, logging).
// Operar SOLO sobre Context — nunca sobre tipos concretos de transporte.
type Middleware func(HandlerFunc) HandlerFunc

// Router es aquello sobre lo que un módulo registra sus rutas.
// Un implementador concreto (servidor nativo, runtime edge) satisface esta interfaz;
// los módulos y los hosts solo la consumen.
type Router interface {
	Get(path string, h HandlerFunc)
	Post(path string, h HandlerFunc)
	Put(path string, h HandlerFunc)
	Delete(path string, h HandlerFunc)
	Options(path string, h HandlerFunc)
	Handle(method, path string, h HandlerFunc)
	Stream(path string, h StreamFunc)
	Socket(path string, h SocketFunc)
	Use(m ...Middleware)
}

// APIModule es un módulo que expone una API de servidor.
// Lo consume el punto de entrada del servidor (!wasm): le pasa el Router del host
// y el módulo registra sus propias rutas/handlers. Como Router es isomórfico,
// el módulo nunca importa net/http para describir su API. El transporte concreto
// (subida binaria, otro protocolo montado como ruta) es decisión interna del módulo.
type APIModule interface {
	fmt.ModuleNaming // aporta ModelName() — identidad
	MountAPI(r Router)
}
