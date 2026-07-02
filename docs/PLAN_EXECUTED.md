# Plan — `github.com/tinywasm/router`: enrutado isomórfico + contrato de API de módulo

> Pieza de lego **nueva** con **responsabilidad única**: definir el contrato de
> enrutado (petición → handler) que es idéntico en el objetivo nativo y en el
> objetivo edge/wasm, y el contrato por el que un módulo publica su API montándose
> en un router. **No** implementa ningún servidor concreto: solo define la forma.
> Esta librería es autocontenida: no referencia ninguna otra pieza salvo el contrato
> de identidad (cero-dep).

---

## Reglas de Desarrollo

Las reglas del arnés (tipado sobre `any`, superficie mínima, fallar en compilación,
firmas autodescriptivas) viven en el **`AGENTS.md` de la raíz de esta librería** (se
crea al scaffolding de la pieza) — léelo antes de cualquier cambio. Este PLAN no las
repite.

Restricciones propias de esta pieza, que definen su razón de ser:

- **Responsabilidad única: solo enrutado.** No sabe de HTTP nativo, de Cloudflare,
  de archivos estáticos ni de UI. Esas son otras piezas que *implementan* o
  *consumen* este contrato.
- **Isomórfico.** El mismo `Context` y el mismo `Router` valen en servidor nativo
  (`!wasm`) y en runtime edge/`wasm`. Sin etiquetas de construcción y **sin
  `net/http` en la firma pública**.

---

## Motivación (por qué es su propia pieza)

El enrutado es una capacidad transversal: lo necesitan tanto un servidor de
desarrollo local como un runtime de borde. Si la abstracción vive dentro de una
librería de despliegue concreta, cualquier proyecto que solo quiera "recibir
peticiones y despachar handlers" se ve forzado a arrastrar ese runtime entero.
Como pieza de lego independiente, un runtime concreto pasa a ser **un implementador
más** del contrato, intercambiable, y los módulos escriben su API una sola vez
contra la interfaz, no contra un servidor específico.

---

## Contrato de enrutado

```go
package router

// Context es la abstracción mínima que ve un handler: la misma firma en el
// objetivo nativo y en el objetivo edge/wasm. Un handler nunca toca net/http.
type Context interface {
	Method() string
	Path() string
	Body() []byte
	GetHeader(key string) string
	SetHeader(key, value string)
	WriteStatus(code int)
	Write(b []byte) (int, error)
}

// HandlerFunc es la unidad de despacho: recibe un Context y responde sobre él.
type HandlerFunc func(Context)

// Router es aquello sobre lo que un módulo registra sus rutas. Un implementador
// concreto (servidor nativo, runtime edge) satisface esta interfaz; los módulos y
// los hosts solo la consumen.
type Router interface {
	Get(path string, h HandlerFunc)
	Post(path string, h HandlerFunc)
	Put(path string, h HandlerFunc)
	Delete(path string, h HandlerFunc)
	Options(path string, h HandlerFunc)
	Handle(method, path string, h HandlerFunc) // comodín para métodos arbitrarios
}
```

---

## Contrato de API de módulo

Un módulo publica su API **montándose** sobre un `Router` que el host le entrega.
El contrato reutiliza la identidad ya declarada por la pieza de identidad del
ecosistema (una interfaz externa con el método `ModelName() string`); este plan la
reexpresa aquí para ser autocontenido, pero en el código se **importa y embebe**,
no se redeclara:

```go
// Identidad reutilizada desde la pieza de identidad del ecosistema:
//     type Module interface { ModelName() string }

// APIModule es un módulo que expone una API de servidor. Lo consume el punto de
// entrada del servidor (!wasm): le pasa el Router del host y el módulo registra
// sus propias rutas/handlers. Como Router es isomórfico, el módulo nunca importa
// net/http para describir su API. El transporte concreto (subida binaria, un
// handler de otro protocolo montado como ruta) es decisión interna del módulo.
type APIModule interface {
	Module        // aporta ModelName() — identidad
	MountAPI(r Router)
}
```

**Por qué aquí y no en otra pieza:** `APIModule` nombra `Router`, que es el tipo
propio de esta librería; colocarlo junto a su tipo cumple "reutiliza los tipos ya
declarados" sin crear dependencias nuevas hacia esta pieza. La única dependencia
que esta librería adquiere es hacia el contrato de identidad (que es cero-dep).

---

## Extensiones del contrato — streaming, WebSocket y middleware

Petición/respuesta básica no basta para todo el ecosistema: hay respuestas
incrementales de larga duración (SSE), conexiones que se *upgradean* (WebSocket) y
lógica transversal (auth). Para que **una sola forma** cubra también esos casos —y
nadie tenga que salirse a `net/http`— el contrato incorpora tres capacidades. La
regla de diseño en las tres: **el handler recibe la capacidad ya tipada en su
firma**, no la negocia con una aserción. Si una ruta es de streaming, su handler
*es* de streaming; el compilador lo exige. (Estados ilegales no representables.)

### Streaming (SSE)

```go
// Streamer es un Context que además empuja lo escrito de inmediato y mantiene la
// conexión abierta. No se obtiene por aserción: se registra una ruta de streaming
// cuyo handler ya lo recibe.
type Streamer interface {
	Context
	Flush() // envía al cliente lo escrito hasta ahora, sin cerrar la respuesta
}

type StreamFunc func(Streamer)

// Router gana:
//   Stream(path string, h StreamFunc)
```

### WebSocket (upgrade)

```go
// Socket es la conexión bidireccional ya upgradeada. Abstracción isomórfica: la
// misma firma en servidor nativo y en runtime edge/wasm; el handler no toca el
// mecanismo de upgrade.
type Socket interface {
	Read() ([]byte, error)
	Write(b []byte) error
	Close() error
}

type SocketFunc func(Socket)

// Router gana:
//   Socket(path string, h SocketFunc)   // hace el upgrade y entrega el Socket
```

### Middleware

```go
// Middleware envuelve un handler para añadir lógica transversal (auth, logging)
// operando SOLO sobre Context — nunca sobre http.Handler.
type Middleware func(HandlerFunc) HandlerFunc

// Router gana:
//   Use(m ...Middleware)   // aplica a las rutas registradas
```

Para que un middleware sirva de algo (p.ej. auth) debe poder **entregar datos al
handler siguiente** — la identidad autenticada. Por eso `Context` incorpora valores
de ámbito de petición, mínimos y tipados en su uso:

```go
// Añadido a Context:
//   SetValue(key string, v any)   // un middleware deja aquí lo que el handler leerá
//   Value(key string) any         // el `any` vive solo en este borde de paso, nunca en datos
```

Con esto, `sse` (streaming), `user` (middleware) y los hubs WebSocket estandarizan
sobre el contrato en lugar de sobre `net/http`. Cada capacidad la deben implementar
los *implementadores* concretos del `Router` (servidor nativo, runtime edge).

---

## Pasos de implementación

1. Crear la librería con la herramienta estándar de scaffolding del ecosistema,
   propietario `tinywasm`, nombre `router`.
2. Definir `Context`, `HandlerFunc` y `Router` en un archivo, sin etiquetas de
   construcción.
3. Añadir las extensiones: `Streamer`/`StreamFunc` (+ `Router.Stream`),
   `Socket`/`SocketFunc` (+ `Router.Socket`) y `Middleware` (+ `Router.Use`).
4. Añadir el contrato `APIModule` embebiendo el contrato de identidad externo.
5. `go.mod`: única dependencia, la pieza de identidad (cero-dep).

---

## Estrategia de pruebas y criterios de aceptación

- **Doble objetivo:** compila nativo y con `GOOS=js GOARCH=wasm` sin cambios.
- **Sin `net/http` en la superficie pública:** verificable revisando imports.
- **Aserción de contrato:** un `Router` de mentira (que registre en un mapa) y un
  handler que escriba en un `Context` de mentira demuestran que un handler se
  escribe guiado solo por la interfaz. `var _ Router = (*fakeRouter)(nil)` y
  `var _ APIModule = (*fakeModule)(nil)` fijan los contratos en compilación.
- **Intercambiabilidad:** documentar que un runtime concreto (servidor de
  desarrollo nativo, runtime de borde) es solo un implementador de `Router`, y que
  migrar entre ellos no toca ni módulos ni handlers.
- **Capacidades por firma, no por aserción:** un handler de `Stream` recibe un
  `Streamer` y uno de `Socket` recibe un `Socket` — no hay `ctx.(Streamer)` en el
  código de usuario. Verificable en los fakes: `var _ Streamer = (*fakeStream)(nil)`,
  `var _ Socket = (*fakeSocket)(nil)`.
