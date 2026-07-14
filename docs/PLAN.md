---
message: "feat: ship the executable conformance suite every Router implementation must pass"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.
> Orquestado por `tinywasm/docs/ROUTER_CONFORMANCE_MASTER_PLAN.md` — **Fase A (compuerta)**.

# PLAN — `router` publica su arnés: el paquete `conformance`

Autocontenido, en español.

## El problema

`tinywasm/router` publica el contrato de enrutado como **interfaz**: `Router`, `Route`,
`Context`, `Middleware`. Lo que **no** publica es el **comportamiento**. Y sin eso, dos
implementaciones pueden satisfacer la interfaz —compilar sin una queja— y comportarse
distinto en todo lo que importa. Es exactamente lo que ha pasado:

| | `server/httpd` (nativo) | `goflare/edge` (Cloudflare) |
|---|---|---|
| Mismo path, distinto método | **panic al arrancar** | funciona |
| ¿Quién llama? (identidad) | middleware `Authn` configurable | **no existe** → toda ruta con permisos es un 403 eterno |
| Ruta `Guarded` sin autorizador | **falla al arrancar** | deniega en silencio, para siempre |

Hoy `tests/router_test.go` prueba el `mock` de este repo: **la única implementación que nadie
despliega**. Las dos que corren en producción no las prueba nadie contra el contrato.

**La causa raíz no es ninguno de esos bugs: es que no hay dónde escribir "esto es lo que un
Router tiene que hacer" de forma ejecutable.** El comportamiento es folklore, y el folklore
diverge.

## La solución

Un paquete nuevo, `conformance/`, que exporta **un cuerpo de tests parametrizado por la
implementación**. Cada implementación lo importa desde su propio `_test.go` y lo corre contra
sí misma. Una implementación que no lo pase, no es una implementación.

**Anti-footgun:** este paquete **exporta tests, no los define para sí mismo**. No va en
`tests/`, no lleva `_test.go` en su nombre de paquete, y **sí** importa `testing` en código
normal — es su propósito. No lo "arregles" moviéndolo a un `_test.go`: sería inimportable
desde otros repos, que es justo lo que tiene que hacer.

## Cambios

### 1. Crear `conformance/conformance.go`

Paquete `conformance`. Expone dos cosas y nada más (superficie pública mínima):

```go
package conformance

import (
	"testing"

	"github.com/tinywasm/router"
)

// Factory builds a fresh Router plus the driver that sends a request through it.
// Each implementation supplies its own: httpd spins an httptest server, edge fakes
// the workers runtime. The suite never learns which one it is driving.
// Setup is what the suite hands the implementation so it can be built the way the
// suite needs to drive it. The AUTHORIZER IS THE SUITE'S: without it the suite could
// not tell an authorized caller from a merely authenticated one.
type Setup struct {
	Authorize model.Authorizer // the implementation MUST install it
}

type Factory struct {
	// New returns an empty Router, plus Serve: it drives one request through that
	// Router and reports what came back. Serve must apply the SAME pipeline the
	// implementation uses in production — gate, middleware and all. A Serve that
	// calls the handler directly proves nothing.
	New func(t *testing.T, s Setup) (r router.Router, serve ServeFunc)
}

// ServeFunc drives one request through the Router under test.
// userID is the identity the transport reports for the caller ("" = anonymous):
// the implementation must route it through whatever its Authn seam is.
type ServeFunc func(method, path string, body []byte, userID string) Response

type Response struct {
	Status int
	Body   []byte
}

// Run executes every contract test against the given implementation.
// Call it from the implementation's own test package.
func Run(t *testing.T, f Factory) { /* ... */ }
```

`Run` ejecuta cada caso con `t.Run(name, ...)`, de modo que un fallo nombra el punto exacto
del contrato que se ha roto.

### 2. Los casos del contrato

Estos son **el contrato**. Cada uno es un `t.Run` dentro de `Run`. Los mensajes de fallo van
en backticks: cópialos literales, son lo que va a leer quien rompa el contrato.

**Enrutado por método**

1. `same_path_different_methods` — registrar `Get`, `Post` y `Options` sobre **el mismo path**
   y comprobar que **las tres** responden 200 y **cada una ejecuta su propio handler**.
   Registrarlas no puede hacer panic. Fallo: `` `the three routes on the same path must coexist: registering by path alone collapses them` ``.
2. `unregistered_method_is_405` — con solo `Get` registrado, un `POST` al mismo path devuelve
   **405**, no 404 y no 200.
3. `unmatched_path_is_404`.

**Acceso (cerrado por defecto)**

4. `private_by_default_is_403` — una ruta que no declara **ni** `Public()` **ni** `Requires()`
   **ni** `Authenticated()` responde **403** a un llamante anónimo. Fallo:
   `` `a route that declares no access is private by default and must answer 403` ``.
5. `public_route_serves_anonymous` — `Public()` → 200 sin identidad.
6. `guarded_route_rejects_anonymous` — `Requires(res, act)` + llamante anónimo → **403**.
7. `guarded_route_serves_authorized_identity` — `Requires(res, act)` + identidad **a la que el
   autorizador concede ese permiso** → **200, y el handler se ejecuta**. Este es el caso que
   `goflare/edge` no puede pasar hoy, y es el que hace inservible a `goflare/files`.
8. `guarded_route_rejects_unauthorized_identity` — identidad válida **sin** ese permiso → 403.
9. `authenticated_route_serves_any_identity` — `Authenticated()` + cualquier identidad → 200;
   anónimo → 403.

**Identidad y middleware**

10. `authn_runs_before_the_gate` — el asiento de autenticación **debe ejecutarse antes** de la
    verja: si no, ninguna identidad llega nunca a tiempo y toda ruta con permisos es un 403
    eterno. Es el bug de `edge` convertido en test. Fallo:
    `` `the gate ran before identity was established: no caller can ever be authorized` ``.
11. `middleware_wraps_the_handler` — un `Use` registrado **después** de la ruta también la
    envuelve, y el orden de aplicación es el de registro.
12. `middleware_does_not_run_on_a_rejected_route` — un 403 **no** ejecuta el handler ni el
    middleware de negocio. (No se filtra trabajo detrás de la verja.)

**Introspección**

13. `routes_reports_access_and_method` — `Routes()` devuelve una `RouteInfo` por ruta, con su
    `Method`, `Path` y `Access` correctos.

**Cuerpo y contexto**

14. `body_survives_binary_roundtrip` — un cuerpo con **bytes nulos y alto rango**
    (`0x00 0xFF 0x89 PNG…`) llega al handler **byte a byte idéntico**. Un cuerpo que pasa por
    una conversión a `string` se corrompe aquí. Es el test que define la subida de archivos.
15. `body_is_readable_once_from_the_handler` — leer `ctx.Body()` dos veces devuelve lo mismo.

### 3. Contradicciones: fallar al arrancar, no denegar en silencio

El contrato ya tiene el tipo (`model.Access`), pero no la ley. Añádela como caso:

16. `contradictory_route_fails_at_startup` — una ruta `Guarded` **sin autorizador
    configurado** debe hacer que la implementación **falle al arrancar**, no que deniegue en
    silencio: una ruta que *parece* protegida y en realidad es un ladrillo es el peor de los
    dos mundos. `server/httpd` ya lo hace (`enforce.go`); el contrato lo exige ahora de todos.

    Como una implementación no puede "arrancar" dentro de un test genérico, el asiento es un
    campo opcional en `Factory`:

    ```go
    // Verify, when set, reports the error the implementation raises at startup for
    // the routes registered so far. nil means "this configuration is legal".
    // An implementation that cannot fail at startup leaves it nil and skips the case.
    Verify func(r router.Router) error
    ```

    Si `Verify` es `nil`, el caso hace `t.Skip` con
    `` `implementation cannot fail at startup; contradictions will deny silently` `` — un
    aviso ruidoso, no un silencio.

### 4. Migrar los tests actuales

`tests/router_test.go` prueba el `mock` a mano. Reescríbelo para que el `mock` **pase la suite
de conformidad**, igual que cualquier otra implementación:

```go
func TestMockConformance(t *testing.T) {
	conformance.Run(t, conformance.Factory{ New: newMockFactory })
}
```

Si el `mock` no puede pasar algún caso, **el arreglo va en el `mock`**, no en el caso: el
mock existe para parecerse a las implementaciones reales, y si miente, los consumidores que
lo usan en sus tests están probando una fantasía. (Es lo que ya pasó en `goflare`: su
`fakeCtx` tenía un `SetUserID` que no hacía nada, y por eso sus tests de `files` pasaban
mientras la subida era un 403 en producción.)

Conserva los tests que **no** son de conformidad (`TestSameSiteConstants`, cookies): son de
este paquete y no de las implementaciones.

## Reglas de este repo

- **Sin librería estándar** salvo `testing` en `conformance/` (que es su razón de ser).
  `fmt`/`strings`/`errors` → `tinywasm/fmt`. Este paquete se compila a WASM.
- **Sin `net/http`** en ninguna firma exportada. El contrato es isomorfo: nombrar `http.*`
  aquí lo ataría al servidor nativo.
- Constantes con nombre, nunca literales repetidos.

## Criterios de aceptación

- `gotest` pasa.
- Existe `conformance/conformance.go` y su superficie pública es **solo** `Factory`, `Setup`,
  `ServeFunc`, `Response`, `Run`, y las constantes con las que la suite conduce sus casos
  (`Resource`, `Action`, `UserAuthorized`, `UserUnauthorized`, `Anonymous`, `Authorize`).
- Los 16 casos existen y se nombran tal cual están arriba:
  ```bash
  grep -c "t.Run(" conformance/conformance.go        # → 16
  ```
- El `mock` de este repo pasa `conformance.Run`.
- **La suite tiene dientes.** No basta con que pase: hay que demostrar que **falla** cuando
  la implementación está mal. Reproduce los dos bugs reales en el `mock` y comprueba que se
  pone roja:
  - verja antes que la identidad (el bug de `goflare/edge`) → deben fallar
    `guarded_route_serves_authorized_identity`, `authenticated_route_serves_any_identity` y
    `authn_runs_before_the_gate`.
  - matchear ignorando el método (el bug de `server/httpd`) → deben fallar
    `same_path_different_methods` y `unregistered_method_is_405`.
- `grep -rn "net/http" --include=*.go .` → vacío.
- Ninguna firma exportada de `conformance` nombra un tipo de `httpd` ni de `edge`: la suite
  no sabe a quién está probando.
