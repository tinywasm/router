> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.
> Orquestado por `tinywasm/docs/ROUTER_ADAPTER_MASTER_PLAN.md` — **Fase 0 (compuerta)**.

# Plan — `github.com/tinywasm/router`: cookies + metadatos de ruta en el contrato

> Extiende el contrato de enrutado (ya existente, ver `docs/PLAN_EXECUTED.md`) con dos
> capacidades isomórficas: **cookies** y **metadatos de ruta** (permiso RBAC e
> introspección). El contrato sigue siendo solo forma: **sin `net/http`**, sin stdlib de
> red, idéntico en objetivo nativo (`!wasm`) y edge/`wasm`. Declara; **no aplica** (la
> comprobación de RBAC es de cada implementador concreto). Esta es la **compuerta** del
> plan maestro: todos los implementadores concretos consumen estas firmas.
>
> **Fuera de alcance (decidido):** el rate limit NO vive en el contrato ni en el
> servidor de app — es concern de edge/gateway (un bucket in-memory no coordina entre
> réplicas → falsa seguridad). Si un despliegue single-instance lo necesitara, sería un
> middleware compuesto opcional, no metadato de ruta.

---

## Reglas de Desarrollo

Las reglas del arnés (tipado sobre `any`, superficie mínima, fallar en compilación,
firmas autodescriptivas) viven en el **`AGENTS.md` de la raíz de esta librería** —
léelo antes de cualquier cambio. Este PLAN no las repite.

Restricciones propias, inviolables:

- **Solo contrato.** No implementa servidor, ni HTTP nativo, ni archivos estáticos.
- **Isomórfico.** La misma firma vale en nativo y en edge/`wasm`. **Prohibido**
  `net/http` en la superficie pública y en imports.
- **Sin stdlib de Go** en el paquete (compila a WASM). Solo `github.com/tinywasm/fmt`
  y el contrato de identidad, tal como hoy.

---

## Problema

**Cookies.** `Context` hoy solo expone cabeceras crudas (`GetHeader`/`SetHeader`). Las
cookies son una capacidad **isomórfica** (existen en el servidor nativo y en el runtime
de borde), pero al no estar en el contrato, cada consumidor que las necesita
(autenticación por sesión: `user/server/middleware.go`) se sale a
`*http.Request`/`r.Cookie(...)`, rompiendo la promesa isomórfica del router. La cookie
debe ser un tipo del contrato.

**Metadatos de ruta.** El registro hoy es "dispara y olvida": `Get(path, h)` no devuelve
nada ni **registra** la ruta. Por eso hoy es imposible: (a) atar un permiso a una ruta
concreta, (b) listar las rutas expuestas. Middleware vía `Use` aplica a *todo* lo
posterior — sin granularidad ni introspección. La solución es una sola: **la ruta debe
ser un objeto que se describe y se anota**. El registro devuelve un `Route` anotable, y
el `Router` sabe enumerar sus `RouteInfo`. El contrato solo **declara** el metadato
(recurso, acción); la **aplicación** (comprobar RBAC) es de cada implementador concreto —
así el contrato sigue sin depender de un autorizador.

---

## Cambio de contrato (código objetivo)

Nuevo archivo `cookie.go` (sin etiquetas de construcción, isomórfico):

```go
package router

// Cookie es la representación isomórfica de una cookie HTTP. No referencia
// net/http: cada implementador concreto la mapea a su transporte (net/http.Cookie
// en nativo; cabecera Set-Cookie en edge/wasm).
type Cookie struct {
	Name     string
	Value    string
	Path     string
	Domain   string
	MaxAge   int  // >0 segundos; 0 = sesión; <0 = borrar ahora
	Secure   bool
	HttpOnly bool
	SameSite SameSite
}

// SameSite tipa la política SameSite — estado ilegal no representable (no un string).
type SameSite int

const (
	SameSiteDefault SameSite = iota
	SameSiteLax
	SameSiteStrict
	SameSiteNone
)
```

Ampliar la interfaz `Context` en `router.go` con dos métodos:

```go
type Context interface {
	Method() string
	Path() string
	Body() []byte
	GetHeader(key string) string
	SetHeader(key, value string)
	WriteStatus(code int)
	Write(b []byte) (int, error)
	SetValue(key string, v any)
	Value(key string) any
	// NUEVO — cookies isomórficas:
	SetCookie(c Cookie)             // escribe una cookie en la respuesta
	Cookie(name string) (Cookie, bool) // lee una cookie de la petición; ok=false si no está
}
```

**Ruptura controlada:** añadir métodos a `Context` obliga a que cada implementador los
provea. En el mismo repo, el `fakeContext` de `router_test.go` debe implementarlos. Los
implementadores en *otros* módulos (p. ej. `tinywasm/server`, `tinywasm/serverd`) no se
rompen hasta que suban su dependencia a esta versión de `router` — la adopción es por
versionado de módulo, no simultánea (ver plan maestro).

---

## Cambio de contrato — metadatos de ruta (código objetivo)

El registro deja de ser `void`: **devuelve un `Route`** anotable. Reutiliza el modelo de
autorización del ecosistema — `(resource string, action byte)` — idéntico al de
`mcp.Authorizer.Can(userID, resource string, action byte)` y `user.HasPermission`. **No
se inventa un tipo de acción nuevo:** la acción es `byte`, como en todo el ecosistema.

Nuevo archivo `route.go` (isomórfico, sin `net/http`):

```go
package router

// Route describe una ruta ya registrada y permite anotarla. Lo devuelve cada método de
// registro del Router. Las anotaciones son declarativas: el contrato no las aplica —
// cada implementador concreto (serverd nativo, runtime edge) las hace cumplir.
type Route interface {
	// Requires ata un permiso RBAC a la ruta. Mismo modelo que el resto del ecosistema:
	// (resource string, action byte), idéntico a mcp.Authorizer.Can / user.HasPermission.
	Requires(resource string, action byte) Route
}

// RouteInfo es la vista de solo lectura de una ruta registrada — para introspección.
type RouteInfo struct {
	Method   string
	Path     string
	Resource string // "" = ruta pública (sin RBAC)
	Action   byte
}
```

Y `Router` (en `router.go`) cambia las firmas de registro para **devolver `Route`** y gana
`Routes()`:

```go
type Router interface {
	Get(path string, h HandlerFunc) Route
	Post(path string, h HandlerFunc) Route
	Put(path string, h HandlerFunc) Route
	Delete(path string, h HandlerFunc) Route
	Options(path string, h HandlerFunc) Route
	Handle(method, path string, h HandlerFunc) Route
	Stream(path string, h StreamFunc) Route
	Socket(path string, h SocketFunc) Route
	Use(m ...Middleware)
	// NUEVO — introspección: enumera las rutas registradas y sus metadatos.
	Routes() []RouteInfo
}
```

**Ruptura controlada:** cambiar el tipo de retorno de los métodos de registro (de nada a
`Route`) y añadir `Routes()` rompe a los implementadores en compilación. En este repo, el
`fakeRouter` de `router_test.go` debe devolver un `Route` (basta un fake que registre las
anotaciones en el `RouteInfo`) y proveer `Routes()`. Los implementadores en otros módulos
adoptan al subir la dependencia (ver plan maestro). El código de usuario que hoy hace
`r.Get(path, h)` **sigue compilando** — ignorar el `Route` devuelto es válido en Go.

---

## Mock del contrato — subpaquete `mock` (doble de test oficial)

Hoy cada consumidor que testea contra el contrato hand-rollea su propio fake (el
`fakeRouter`/`fakeContext` de `router_test.go` está *unexported*; `app`, `server`,
`goflare` repetirían el suyo). Peor: un consumidor como **`app`** que expone
`ServerInterface.RegisterRoutes(router.Router)` no puede probar el registro de rutas sin
(a) un fake casero por repo, (b) levantar un servidor real, o (c) degradar la firma a
`any` — que rompe la verificación en compilación. La solución es **un doble canónico**.

Nuevo subpaquete `github.com/tinywasm/router/mock` (compila también a WASM; **sin
`net/http`**; misma restricción de stdlib que el paquete raíz):

```go
package mock

import "github.com/tinywasm/router"

// Router graba las rutas registradas y permite dispararlas en un test.
type Router struct {
	Registered []router.RouteInfo // introspección: qué se registró
	// handlers internos indexados por (method, path) para Invoke
}
func (r *Router) Get(path string, h router.HandlerFunc) router.Route  { /* graba + devuelve *Route */ }
// ...Post/Put/Delete/Options/Handle/Stream/Socket/Use/Routes idénticos...
func (r *Router) Invoke(method, path string, ctx router.Context)      // ejecuta el handler registrado

// Context bufferiza la respuesta y deja fijar la petición desde el test.
type Context struct {
	InMethod, InPath string
	InBody           []byte
	Status           int
	Body             []byte
	// headers, cookies, values...
}

var _ router.Router  = (*Router)(nil)
var _ router.Context = (*Context)(nil)
var _ router.Route   = (*Route)(nil)
```

Uso (p. ej. en `app`, evitando el `any`):

```go
mr := &mock.Router{}
assetMin.RegisterRoutes(mr)   // firma nueva: router.Router
// assert mr.Registered contiene GET /assets, GET /app.wasm, ...
```

**Por qué en un subpaquete y no en `router_test.go`:** un `_test.go` no es importable; un
subpaquete sí. Al vivir en `mock`, el contrato raíz mantiene su superficie mínima y el
doble se comparte. El propio `router_test.go` puede usar `mock` en vez de duplicar fakes.

---

## Pasos de implementación

1. Crear `cookie.go` con `Cookie` y `SameSite` + sus constantes.
2. Añadir `SetCookie(Cookie)` y `Cookie(name string) (Cookie, bool)` a `Context`.
3. Crear `route.go` con `Route` (`Requires`) y `RouteInfo`.
4. En `router.go`, cambiar los métodos de registro para **devolver `Route`** y añadir
   `Routes() []RouteInfo` a `Router`.
5. Crear el subpaquete `mock/` (`package mock`): `Router` (graba `Registered` + handlers,
   `Invoke`), `Context` (buffers + petición fijable) y `Route`, implementando el contrato
   **completo** (incl. cookies/`Requires`/`Routes()`). Fijar `var _ router.Router =
   (*Router)(nil)`, `var _ router.Context = (*Context)(nil)`, `var _ router.Route =
   (*Route)(nil)`. Sin `net/http`; compila WASM.
6. `router_test.go`: usar el `mock` (o los fakes locales) para las cookies y verificar
   que registrar y disparar una ruta funciona guiado solo por la interfaz.
7. Tests: (a) ida/vuelta de cookie; (b) registrar `r.Post("/x", h).Requires("res", 'w')`
   y verificar que `Routes()` devuelve el `RouteInfo` con `Method=="POST"`, `Path=="/x"`,
   `Resource=="res"`, `Action=='w'`; (c) `mock.Router.Invoke` ejecuta el handler y el
   `mock.Context` captura status/body.
8. Actualizar `README.md`: añadir `Cookie`/`SameSite`, `Route`/`RouteInfo`, `Routes()` y el
   subpaquete `mock` a la lista de contratos.
9. `docs/PLAN_EXECUTED.md` **no** se toca — es histórico; el contrato vigente se
   documenta en `README.md`.

---

## Code Quality Checklist (obligatorio, el agente no tiene contexto)

- **Sin literales repetidos.** Los valores de `SameSite` son constantes tipadas, no
  enteros mágicos ni strings.
- **Tipado sobre `any`.** `SameSite` es un tipo, no un `string`. `Cookie` y `RouteInfo`
  son structs con campos nombrados, no `map[string]string`.
- **Reutiliza el modelo del ecosistema.** El permiso de ruta es `(resource string,
  action byte)` — el mismo de `mcp.Authorizer.Can`/`user.HasPermission`. **No** declares
  un tipo `Action` nuevo ni un enum propio.
- **El contrato declara, no aplica.** `Route.Requires` solo registra el metadato; aquí no
  hay `Authorizer`. La aplicación es del implementador.
- **Superficie mínima.** El paquete raíz exporta `Cookie`, `SameSite` (+ constantes),
  `Route`, `RouteInfo`, los métodos de cookie y `Routes()`. Nada más. **No** hay rate
  limit. El doble de test vive **aparte** en el subpaquete `mock` (no ensucia el contrato
  raíz) e implementa el contrato completo sin `net/http`.
- **Sin rate limit en el contrato.** Es concern de edge/gateway; no declares `Rate`,
  constructores de tope ni un método `RateLimit`.
- **Sin `cmd/`** en esta librería (es un contrato puro).
- **Sin `net/http`, sin stdlib de red.** Verificable por imports.

---

## Estrategia de pruebas y criterios de aceptación

- `gotest` (nunca `go test`). Doble objetivo: compila nativo y con
  `GOOS=js GOARCH=wasm` sin cambios.
- **Sin `net/http` en la superficie ni en imports:** verificable por búsqueda.
- **Contrato fijado en compilación:** `var _ Context = (*fakeContext)(nil)`,
  `var _ Router = (*fakeRouter)(nil)`, `var _ Route = (*fakeRoute)(nil)`.
- **Ida y vuelta de cookie:** un test escribe `SetCookie(Cookie{Name:"sid",Value:"x"})`
  y otro lee `Cookie("sid")` devolviendo `ok==true` y `Value=="x"` sobre el fake.
- **Metadatos registrados:** `r.Post("/x", h).Requires("res", 'w')` produce un
  `RouteInfo{Method:"POST", Path:"/x", Resource:"res", Action:'w'}` en `r.Routes()`.

---

## Tabla de etapas

| Etapa | Archivo | Acción | Rompe API |
|---|---|---|---|
| 1 | `cookie.go` | Crear `Cookie` + `SameSite` + constantes | No (adición) |
| 2 | `router.go` | Añadir `SetCookie`/`Cookie` a `Context` | Sí — nuevos métodos de interfaz |
| 3 | `route.go` | Crear `Route` (`Requires`) + `RouteInfo` | N/A — nuevo |
| 4 | `router.go` | Registro devuelve `Route`; añadir `Routes()` a `Router` | Sí — cambio de firma |
| 5 | `mock/` (subpaquete) | `Router`/`Context`/`Route` doble de test (grabar + `Invoke`), WASM-safe | N/A — nuevo |
| 6 | `router_test.go` | Cookies + registro/disparo vía `mock`; aserciones de contrato | No |
| 7 | `README.md` | Indexar `Cookie`/`SameSite`/`Route`/`RouteInfo`/`Routes()`/`mock` | No |
