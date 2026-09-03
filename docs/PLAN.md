---
PLAN: "feat(query): add router.QueryParam, the single correct query-string parser"
EXECUTOR: jules
REVIEWER: none
---

> Este plan se despacha con el flujo CodeJob. Ver skill: `agents-workflow`.
> No ejecutes `gopush` ni `codejob` — son herramientas del desarrollador local.

# PLAN — `tinywasm/router`: el parser de query string vive acá

## Contexto

Auditoría de seguridad de `veltylabs/iam` (2026-09-02). `router.Context`
expone `Path()` completo (con query string) y `Param(name)` para parámetros de
ruta, pero **no hay contrato para leer un parámetro de query**. Consecuencia:
dos consumidores lo rehicieron a mano, ambos mal.

Doctrina obligatoria: [CONSTRUCTION_HARNESS.md](https://github.com/tinywasm/app-releases/blob/main/docs/CONSTRUCTION_HARNESS.md).
Los principios que gobiernan este plan:

- **"A missing contract at a boundary is a defect in the library, not in the consumer."**
- **"The glue is written once, in the library that owns it."** Si toda
  aplicación escribiría el mismo wiring, ese wiring pertenece a la pieza.
- **6 · Nunca fallo silencioso.**

## Hallazgo Q-1 (Medio) · dos parsers duplicados y rotos

`tinywasm/auth/oauth2/oauth.go`:

```go
func queryParam(path, key string) (string, bool) {
	if !fmt.Contains(path, "?") { return "", false }
	query := fmt.Split(path, "?")[1]
	for _, part := range fmt.Split(query, "&") {
		kv := fmt.Split(part, "=")
		if len(kv) == 2 && kv[0] == key { return kv[1], true }
	}
	return "", false
}
```

`veltylabs/iam/modules/admin/handler.go` tiene una variante casi idéntica.
Defectos compartidos:

1. **`len(kv) == 2` descarta cualquier valor que contenga `=`.** Un `state`
   en base64 con relleno, o un `redirect_uri=https://x/?a=b`, se pierden en
   silencio: la validación no rechaza, simplemente no ve el parámetro.
2. **No decodifica percent-encoding.** Un `redirect_uri` correctamente
   codificado (`https%3A%2F%2Fmisitio.velty.cl`) llega ilegible y falla la
   validación de dominio — el camino correcto es el que no funciona.
3. **No maneja el fragmento `#`.** Todo lo que sigue a `#` es del cliente y
   nunca debería entrar al valor.
4. **No maneja `+` como espacio** en `application/x-www-form-urlencoded`.
5. Un parámetro sin `=` (`?flag`) no se distingue de uno ausente.

Ninguno de estos es explotable por sí solo, pero el #1 y el #2 están en el
camino de validación del `redirect_uri` de OAuth: una validación que no ve el
valor es una validación que no existe.

## Etapa 1 · `router.QueryParam` y `router.QueryUnescape`

Archivo nuevo: `query.go` (raíz del módulo, junto a `pattern.go`).

`pattern.go` ya es el hogar de las funciones de parsing de ruta
(`ParamNames`, `MatchPattern`, `MoreSpecific`) como **funciones de paquete**,
no métodos de interfaz. `QueryParam` sigue esa convención: **NO agregues un
método a `router.Context`** — eso rompería a los cinco implementadores
(`cloudflare/edge`, `server/httpd`, `router/mock`, `app/sse_adapter`,
`mcp/harvest`) por un problema que una función de paquete resuelve sin tocar
ninguno.

```go
package router

// QueryParam devuelve el valor de key en la query string de path, ya
// percent-decodificado. ok es false cuando la clave no aparece; un parámetro
// presente pero vacío ("?k=") devuelve "" con ok true — ausente y vacío son
// cosas distintas y el llamador debe poder distinguirlas.
//
// path es lo que devuelve Context.Path(): puede traer o no query string, y
// puede traer un fragmento. Todo lo que sigue al primer "#" se descarta antes
// de parsear: el fragmento nunca viaja al servidor y su presencia en un valor
// sólo puede venir de un llamador tratando de confundir a un validador.
//
// El valor NO se valida ni se sanea: es entrada del llamante como cualquier
// otra. Esta función sólo garantiza que lo que devuelve es lo que el llamante
// realmente envió — que es exactamente lo que un validador necesita para
// poder hacer su trabajo.
func QueryParam(path, key string) (value string, ok bool)

// QueryUnescape decodifica una cadena percent-encoded de una query string:
// "%XX" pasa a su byte y "+" pasa a espacio. Una secuencia "%" malformada se
// deja tal cual — devolver un error obligaría a cada llamador a decidir qué
// hacer con un byte roto, y la respuesta correcta siempre es "trátalo como
// dato opaco y déjaselo al validador".
func QueryUnescape(s string) string
```

Reglas de implementación:

1. Cortar en el primer `#` **antes** de buscar el `?`.
2. Cortar en el primer `?`; si no hay, no hay query → `"", false`.
3. Separar por `&`. Para cada parte, buscar el **primer** `=` con
   `fmt.Index(part, "=")` — **nunca `fmt.Split` sobre `=`**, que es el bug #1.
4. Sin `=` en la parte: la clave es la parte entera y el valor es `""`.
5. Comparar la clave **ya decodificada** con `key` (una clave puede venir
   percent-encoded).
6. Primera coincidencia gana; no acumular repetidas.

`QueryUnescape` es un recorrido byte a byte sobre el string. `+` → `" "`.
`%` seguido de dos dígitos hexadecimales válidos → ese byte. `%` sin dos
hexadecimales válidos detrás → se copia el `%` literal y se sigue. Escribí un
helper `unhex(c byte) (byte, bool)` no exportado; nada de `strconv`.

Constantes nuevas para lo que hoy serían literales:

```go
const (
	queryStart    = '?'
	queryFragment = '#'
	querySep      = '&'
	queryEq       = '='
	queryPlus     = '+'
	queryEscape   = '%'
)
```

## Etapa 2 · Tests

Archivo nuevo: `tests/query_test.go` (los tests del repo viven bajo `tests/`).

Tabla obligatoria en `TestQueryParam`, cada fila con `path`, `key`, `want`,
`wantOK`:

| path | key | want | ok | Fija |
|---|---|---|---|---|
| `/x` | `a` | `""` | false | sin query |
| `/x?` | `a` | `""` | false | query vacía |
| `/x?a=1` | `a` | `1` | true | caso base |
| `/x?a=1&b=2` | `b` | `2` | true | segunda clave |
| `/x?a=1&b=2` | `c` | `""` | false | ausente |
| `/x?a=` | `a` | `""` | true | **presente y vacío ≠ ausente** |
| `/x?a` | `a` | `""` | true | sin `=` |
| `/x?a=b=c` | `a` | `b=c` | true | **regresión del bug #1** |
| `/x?s=YWJj==` | `s` | `YWJj==` | true | base64 con relleno |
| `/x?u=https%3A%2F%2Fm.velty.cl%2Fp` | `u` | `https://m.velty.cl/p` | true | **regresión del bug #2** |
| `/x?q=a+b` | `q` | `a b` | true | `+` es espacio |
| `/x?a=1#b=2` | `b` | `""` | false | **regresión del bug #3** |
| `/x?a=1#frag` | `a` | `1` | true | fragmento descartado |
| `/x?a=1&a=2` | `a` | `1` | true | primera gana |
| `/x?%61=1` | `a` | `1` | true | clave codificada |
| `/x?a=%ZZ` | `a` | `%ZZ` | true | `%` malformado se deja |
| `/x?a=100%` | `a` | `100%` | true | `%` al final |

`TestQueryUnescape` con su propia tabla: `""`→`""`, `"a+b"`→`"a b"`,
`"%20"`→`" "`, `"%2F"`→`"/"`, `"%2f"`→`"/"` (hex minúscula),
`"%"`→`"%"`, `"%A"`→`"%A"`, `"100%25"`→`"100%"`.

**Test consumer-shaped obligatorio** (regla de oro del harness: *an API is not
published until a consumer-shaped test, inside the library itself, proves it*).
En `tests/query_test.go`:

```
TestQueryParam_ValidatesAnOAuthRedirectURI
```

Debe reproducir el uso real de `tinywasm/auth`: dado
`/oauth/google?redirect_uri=https%3A%2F%2Fmisitio.velty.cl%2Fpanel&state=YWJj==`,
`QueryParam` devuelve **ambos** parámetros íntegros, y un validador de
dominio de juguete definido en el propio test (prefijo `https://` + sufijo
`.velty.cl`) los acepta. Con el parser viejo ninguno de los dos llegaba.

## Restricciones de código (leer antes de escribir)

| Regla | Detalle |
|---|---|
| **Sin mapas** | Prohibido `map[K]V` en librería y en tests. Slices + búsqueda lineal. |
| **Sin stdlib** | Nada de `fmt`, `errors`, `strconv`, `strings`, `net/url`, `log`, `os`. Usa `github.com/tinywasm/fmt`. Este repo compila para TinyGo/wasm. |
| **`error` sí, `errors` no** | `fmt.Err(...)`, nunca `errors.New`. |
| **Sin `reflect`** | Ni transitivo. |
| **Sin literales repetidos** | Los delimitadores van como constantes nombradas (arriba). |
| **Sin `internal/`** | No crees carpetas `internal/`. |
| **No cambies `router.Context`** | Agregar un método a esa interfaz rompe cinco implementadores. `QueryParam` es una función de paquete, como `MatchPattern`. |

Idioma: **código e identificadores en inglés**; **comentarios de prosa y
documentación en español**.

## Etapa 3 · Documentación

- `README.md`: agregar `QueryParam` a la lista de helpers de parsing junto a
  `MatchPattern`/`ParamNames`, con un ejemplo de dos líneas.
- Si existe `docs/ARCHITECTURE.md`, agregar un párrafo en la sección de
  routing explicando **por qué es una función de paquete y no un método de
  `Context`**: el contrato de `Context` lo implementan cinco transportes, y
  el parsing de query string no depende del transporte — sale de `Path()`,
  que ya está en la interfaz.

## Criterios de aceptación

1. `go vet ./...` y `go test ./...` verdes.
2. `grep -rn "map\[" query.go` → vacío.
3. `grep -rn "strconv\|net/url\|strings" query.go` → vacío.
4. `git diff router.go` → **sin cambios** (la interfaz `Context` no se toca).
5. `QueryParam` y `QueryUnescape` exportados con comentario en español.
6. Las 17 filas de `TestQueryParam` y `TestQueryParam_ValidatesAnOAuthRedirectURI` pasan.
7. Compila para wasm: `GOOS=js GOARCH=wasm go build ./...`.

## Etapas

| # | Archivo | Entrega |
|---|---|---|
| 1 | `query.go` | `QueryParam`, `QueryUnescape`, constantes de delimitador |
| 2 | `tests/query_test.go` | Tablas + test consumer-shaped |
| 3 | `README.md`, `docs/ARCHITECTURE.md` | Por qué función de paquete, no método |
