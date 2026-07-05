# Plan — `router`: identidad tipada en `Context` (`SetUserID`/`UserID`)

> Autocontenido, en español. Rige el **arnés de construcción** (reglas en el
> `AGENTS.md` de la raíz de esta librería): un solo camino tipado, sin "clave mágica
> que recordar", el mal uso no compila.
>
> Prerrequisito explícito de
> [`user/docs/ROUTER_REFACTOR_PLAN.md`](../../user/docs/ROUTER_REFACTOR_PLAN.md)
> (su "nota de dependencia": el middleware de auth necesita **dónde dejar la identidad**
> autenticada para el handler siguiente) y de
> [`mcp/docs/PLAN.md`](../../mcp/docs/PLAN.md) (MCP lee la identidad del `Context`).

---

## El problema

Un middleware de autenticación (`user`) valida la credencial y obtiene un `userID`.
Debe **entregárselo** al handler y a los módulos montados (RBAC de ruta en `httpd`,
RBAC por-tool en `mcp`). Hoy el único canal es `Context.SetValue(key, v any)` /
`Value(key) any`:

- **Hueco de arnés #1 — clave mágica.** Emisor y lector deben acordar un `string`
  (`"userID"`) fuera del sistema de tipos. Si no coinciden, no compila mal: falla en
  runtime, en silencio (identidad vacía → RBAC pasa). Es exactamente lo que el arnés
  prohíbe ("cosas que hay que recordar", "fallos silenciosos").
- **Hueco de arnés #2 — `any` en los datos.** La identidad viaja como `any` y se
  castea; el contrato no expresa que "una petición tiene un usuario (posiblemente
  anónimo)".

## La corrección

`Context` gana **un accesor de identidad tipado** — un solo camino, explícito:

```go
// package router — añadir a la interfaz Context:
type Context interface {
	// ...lo existente (Method, Path, Body, Header, WriteStatus, Write,
	//    SetValue/Value, SetCookie/Cookie)...

	// Identidad de ámbito de petición. Un middleware de autenticación registra
	// quién es el llamador; los handlers y módulos montados la leen.
	SetUserID(id string) // registra la identidad autenticada (id "" = anónimo)
	UserID() string      // lee la identidad; "" si no hay sesión válida
}
```

- **Tipado, no `any`.** `UserID() string` — sin cast, sin clave.
- **Una sola forma.** No hay dos maneras de propagar identidad; `SetValue("userID",…)`
  deja de ser el patrón.
- **Isomórfico.** Nativo (`httpd`) y edge implementan `SetUserID`/`UserID`; su firma es
  idéntica en ambos objetivos (igual criterio que las cookies de la Fase 0).
- **Estado ilegal difícil de escribir.** El lector no puede "olvidar la clave": el
  autocompletado ofrece `ctx.UserID()` y nada más.

> **Decisión de alcance — el nombre dice ID, y es solo el ID.** El método se llama
> `UserID()` (no `User()`) porque devuelve un **identificador**, no un objeto de dominio;
> el nombre no debe mentir sobre lo que retorna (arnés: explícito, autodescriptivo). Se
> guarda **solo el `userID`** (string), no el `*User` (name, email, roles). Razones:
> - **Responsabilidad / superficie mínima.** `router` es un contrato de transporte; no
>   debe conocer el tipo `User`. Meter `Name`/`Email`/`Roles` acopla el contrato al
>   dominio y obliga a cada implementador (nativo, edge, mock) a cargarlo.
> - **Costo por petición.** El `Context` se puebla en **cada** request; el 99% solo
>   necesita el ID para el RBAC. Cargar el perfil por request es caro para nada.
> - **Es identidad para *autorizar*, no para *mostrar*.** El perfil (nombre, avatar,
>   roles para UI) es **dato de dominio**: el front lo pide por una tool `me`/`whoami`
>   (dueño: `user`) una vez tras login y lo cachea; no viaja en el `Context`, que además
>   es server-side y el front ni lo ve. Ver el plan de `user` (tool `me`).

---

## Cambios

| Archivo | Cambio |
|---|---|
| `router.go` | Añadir `SetUserID(id string)` y `UserID() string` a `Context`. |
| `docs/` (mock) | El doble de test de `Context` (subpaquete `mock`, si existe) implementa los dos métodos con un campo `userID`. |

Implementadores concretos que deben satisfacer la interfaz ampliada:
`tinywasm/server/httpd` (`httpContext`), el runtime edge, y cualquier `mock`.
La adopción es por **versionado de módulo** (como la Fase 0): nada se rompe de golpe;
cada implementador sube su dependencia a esta versión de `router` y añade los métodos.

---

## Estrategia de pruebas y criterios de aceptación

- Compila nativo y `GOOS=js GOARCH=wasm` (contrato isomórfico, sin `net/http`).
- `var _ router.Context = (*mock.Context)(nil)` obliga a que el doble implemente
  `SetUserID`/`UserID` — el arnés fija el contrato en compilación.
- Un test de ida y vuelta: `ctx.SetUserID("u1")` → `ctx.UserID() == "u1"`; por defecto
  `ctx.UserID() == ""`.
- Ningún símbolo nuevo expone `any` en los datos de identidad.

---

## Endurecimiento de seguridad (cerrado por defecto) — con test

El contrato debe permitir declarar "público" como un **acto explícito y tipado**; la
ausencia del marcador = privado (requiere identidad/permiso). Hoy no existe forma de
expresarlo, así que "público" se confunde con "olvidé conceder" — un default abiertо.

- **`Route` gana `Public()`**, hermano de `Requires(resource, action)`: marca una ruta
  como accesible sin identidad. El implementador (p. ej. `httpd`) **aplica**: sin
  `Public()` ni permiso concedido ⇒ deniega.

  ```go
  // package router — en la interfaz Route:
  Requires(resource, action string) Route // gated por RBAC
  Public() Route                          // explícitamente sin auth; ausencia = privado
  ```
  **Test:** `RouteInfo` refleja `Public` solo cuando se llamó `Public()`; una ruta sin
  `Public()` ni `Requires(...)` queda marcada como privada por defecto (el contrato no
  produce rutas "abiertas por omisión"). `var _ router.Route` fija la firma.

> El marcador de tool público equivalente para el endpoint MCP (una sola ruta, muchas
> tools) vive en `mcp.Tool.Public` — ver el plan de `mcp`.

---

## Relación con el ecosistema

- Desbloquea a [`user`](../../user/docs/ROUTER_REFACTOR_PLAN.md): su middleware hace
  `ctx.SetUserID(userID)` en vez de `SetValue` con clave acordada.
- Desbloquea a [`mcp`](../../mcp/docs/PLAN.md): `MountAPI` lee `ctx.UserID()` y lo
  inyecta en el `context.Context` del protocolo antes de `HandleMessage`.
- `httpd` puede simplificar `Config.Identify` (hoy `func(router.Context) string`)
  reduciéndolo a `ctx.UserID()` — ver [`server/docs/PLAN_IDENTITY_WIRING.md`](../../server/docs/PLAN_IDENTITY_WIRING.md).
