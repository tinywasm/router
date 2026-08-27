---
PLAN: "feat: path parameters and a router-owned introspection endpoint"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 14506525275357653895
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — path parameters, and one introspection endpoint every router shares

## Why this exists

Two measured problems, one root: **the route table does not describe the API.**

**1. There are no path parameters.** `router.Context` has no `Param`, and the
edge implementation matches by prefix only. So a real API is registered like
this (from `veltylabs/misitio`):

```go
sites := HandleSitesRoute(sm, sc, bucket, ids)     // dispatches by suffix INSIDE the handler
r.Get(PathSitesPrefix, sites).Requires(ResourceSite, model.Read)
r.Put(PathSitesPrefix, sites).Requires(ResourceSiteContent, model.Update)
r.Post(PathSitesPrefix, sites).Requires(ResourceSiteAsset, model.Create)
r.Delete(PathSitesPrefix, sites).Requires(ResourceSiteAsset, model.Delete)
```

`Routes()` then reports `/api/sites/` four times and hides the operations that
actually exist — `/api/sites/{id}/content` and `/api/sites/{id}/assets`. Every
consumer of `Routes()` (an operator, an MCP tool list, the API explorer this
ecosystem is about to grow) is reading a table that is not the API.

**2. Only one implementation can be introspected.** `tinywasm/server/httpd`
serves `/_routes`; the edge runtime does not. The deployment you most need to
interrogate is the one that cannot answer.

And a third thing the endpoint cannot say today: **who holds the permission a
route requires.** In `veltylabs/misitio`, six of eleven routes require a
`(Resource, Action)` pair granted to no role at all — every one answers `403` to
everybody while looking perfectly declared. `tinywasm/model` now exposes
`PolicyDescriber`/`RolesFor` so that question is answerable; this plan is what
puts the answer on the wire.

## Dependency — read before you start

This plan requires **`github.com/tinywasm/model`** at the version that exports
`RoleGrant`, `PolicyDescriber` and `RolesFor`. That version is **already
published** when this plan is dispatched.

```
go get github.com/tinywasm/model@latest
go mod tidy
```

**Never** add a `replace` directive, never hand-write a version number, and
never stub `PolicyDescriber` locally "until it lands". If `go get` does not
produce a `model` that has `RolesFor`, stop and report it — do not work around
it.

## Anti-footguns

- This repo is the **contract**, not an implementation. It ships the interface,
  the shared pattern helpers, and the introspection handler. The actual matching
  in a deployed transport lives in `tinywasm/cloudflare` (edge) and
  `tinywasm/server` (httpd), which are dispatched separately.
- This package compiles into WASM binaries. Use **`tinywasm/fmt`**, never
  `strings`/`strconv`/`errors`/`fmt`. Both `tinywasm/fmt` and `tinywasm/json`
  are already in `go.mod`.
- **No `map[K]V`** in the root package or in `mock/`. Parameter names and values
  travel as two parallel slices.
- Adding a method to `router.Context` **breaks every implementer**. That is
  intended and coordinated: `tinywasm/cloudflare`, `tinywasm/server` and this
  repo's own `mock` are updated in the same wave. Do not add a default
  implementation, an embedded base struct, or an `interface{ Param }` type
  assertion to "keep compatibility" — this ecosystem has no compatibility
  layers.

---

## Stage 1 — Pattern syntax: `{name}`, and only `{name}`

New file: **`pattern.go`**.

### The syntax decision, and why it is not `:name`

The parameter syntax is **`{name}`**, matching `net/http.ServeMux` (Go 1.22+).

`tinywasm/server/httpd` registers each route straight into a `ServeMux` as
`"GET /api/sites/{id}"`. With `{name}`, the standard library does the matching
and the extraction, with its own precedence rules, for free and correctly. With
`:name`, ServeMux would treat `:id` as a **literal segment** — httpd and the
edge runtime would silently disagree about what a route matches, which is the
single worst failure mode a router contract can have.

The doc comment on `RouteInfo.Path` currently says `"/api/orders/:id"`. **Fix
it** to `"/api/orders/{id}"` as part of this stage.

### Rules

1. A segment that is exactly `{name}` is a parameter. It matches **one
   non-empty** path segment.
2. `name` must be non-empty and unique within the pattern.
3. A pattern containing **no** `{` behaves exactly as it does today: a trailing
   `/` matches its whole subtree, anything else matches exactly. Do not change
   this.
4. **`{name...}` (trailing wildcard) is NOT supported** and must be rejected at
   registration. ServeMux honours it; the edge runtime would have to reimplement
   it; a syntax only one implementation honours is worse than no syntax. Reject
   it loudly now rather than discover the divergence in production.
5. A parameter segment may not be combined with literal text in the same segment
   (`/v{n}` is not a parameter — it is a literal segment named `v{n}`, and
   `ValidatePattern` rejects it).

### API

```go
// ParamNames returns the parameter names pattern declares, left to right.
// nil when the pattern declares none.
func ParamNames(pattern string) []string

// ValidatePattern reports why a pattern cannot be registered, or nil.
// A router MUST call it at registration and fail loudly — a bad pattern that
// silently matches nothing is a route that exists in the table and answers 404
// forever.
func ValidatePattern(pattern string) error

// MatchPattern matches pathname against pattern. values holds one entry per
// name ParamNames(pattern) reports, in the same order. ok is false when the
// pattern does not match.
//
// A pattern with no parameters is matched by the pre-existing rule: trailing
// "/" matches the subtree, otherwise exact equality.
func MatchPattern(pattern, pathname string) (values []string, ok bool)

// MoreSpecific reports whether a should win over b when both match the same
// path. See the ordering rule below.
func MoreSpecific(a, b string) bool
```

### Error messages — verbatim, as typed constants

```go
const (
	ErrMsgWildcardUnsupported = "router: pattern %q: {name...} wildcards are not supported"
	ErrMsgEmptyParamName      = "router: pattern %q: empty parameter name"
	ErrMsgDuplicateParamName  = "router: pattern %q: duplicate parameter name %q"
	ErrMsgMixedSegment        = "router: pattern %q: segment %q mixes literal text and a parameter"
)
```

Build them with `fmt.Errf` from `tinywasm/fmt`.

### The ordering rule — state it once, here

When several patterns match the same path, the winner is decided by, in order:

1. Compare segments left to right. At the **first** position where one pattern
   has a literal and the other has a parameter, **the literal wins**
   (`/api/sites/new` beats `/api/sites/{id}`).
2. Otherwise, more segments wins.
3. Otherwise, the longer pattern string wins (today's rule, unchanged).

`tinywasm/server/httpd` does **not** call `MoreSpecific` — it delegates matching
to `ServeMux`, whose "most specific pattern wins" rule already agrees with the
three clauses above. `MoreSpecific` exists for the implementations that match by
hand (`tinywasm/cloudflare`'s edge runtime, this repo's `mock`) so they cannot
drift from it independently. What guarantees all three agree is the conformance
clause in Stage 4, not this function.

## Stage 2 — `Context.Param`

File: **`router.go`**, in the `Context` interface, immediately after
`SetValue`/`Value`:

```go
	// Param returns the value of a path parameter the matched route declared
	// with {name}. It returns "" when the route declares no such parameter —
	// an absent parameter is not an error, it is a handler asking for
	// something this route never had.
	//
	// Values are NOT decoded beyond the transport's own URL decoding, and they
	// are never trusted: a parameter is caller input like any other.
	Param(name string) string
```

Update the `Context` doc comment to mention parameters alongside request-scoped
values, and make clear the two are different things: `Value` is what a
middleware wrote, `Param` is what the URL carried.

## Stage 3 — `MountIntrospection`

New file: **`introspection.go`**. This is the code being moved out of
`tinywasm/server/httpd/routes_endpoint.go` so every implementation shares one
copy. Preserve the reasoning in that file's comments — it records a real bug
(reflection encoding `Access: 0`, the most protected route reporting itself as
"nothing declared") and that reasoning must not be lost in the move.

```go
// IntrospectionPath is where this ecosystem serves its route table.
const IntrospectionPath = "/_routes"

// MountIntrospection registers a read-only endpoint at path reporting every
// route registered on r, plus what the policy says about each one.
//
// It returns the Route so the CALLER declares the access, which is never
// optional: a route that annotates nothing is AccessGuarded and refuses to
// start. Callers write either
//
//	router.MountIntrospection(r, router.IntrospectionPath, policy).Public()
//
// in development, or .Requires(resource, action) in a deployment where the
// permission map is not something to hand out anonymously.
//
// policy may be nil. The response then reports every route's required
// permission and marks the roles UNKNOWN — never as "nobody", which is a
// different and much more alarming fact (see model.RolesFor).
//
// The route table is read when a request arrives, not at mount time, so this
// may be called before or after the routes it reports.
func MountIntrospection(r Router, path string, policy model.PolicyDescriber) Route
```

### Wire shape

Unexported types in the same file. `RouteInfo` already encodes
method/path/resource/action through its own `EncodeFields`; the view adds what
the policy knows.

`RouteInfo.Args` (set by `Route.Accepts`) is the schema a caller must send. It is
deliberately **not** part of `RouteInfo.EncodeFields` — but it is exactly what a client
building a request form needs, so the view serializes it here, where a transport's wire
concern belongs.

```go
type routeView struct {
	info         RouteInfo
	roles        []model.RoleCode
	policyKnown  bool
}

func (v routeView) IsNil() bool { return false }

func (v routeView) EncodeFields(w model.FieldWriter) {
	v.info.EncodeFields(w)          // method, path, resource, action, access
	w.Bool("policy_known", v.policyKnown)
	arr := w.Array("roles", len(v.roles))
	for _, r := range v.roles {
		arr.String(string(r))
	}
	arr.Close()

	// args: the field names and kinds Route.Accepts declared; omitted entirely
	// when Args is nil ("no args", per Route.Accepts). An empty array would
	// claim the route takes an empty body, which is a different statement.
	if v.info.Args != nil {
		fields := v.info.Args.Schema()
		fa := w.Array("args", len(fields))
		for i := range fields {
			fa.Object(argField{f: &fields[i]})
		}
		fa.Close()
	}
}
```

`argField` is an unexported wrapper writing `{"name":…,"kind":…,"required":…}` from a
`model.Field`. Read `model.Field` before writing it and use the names it already has — do not
invent a parallel vocabulary for something the schema already states.

Envelope, same shape httpd already serves so existing consumers keep working:

```json
{"routes":[{"method":"GET","path":"/api/sites/{id}/content","resource":"site_content",
            "action":"r","access":"guarded","policy_known":true,"roles":[]}]}
```

`"policy_known":true` with `"roles":[]` is the finding: **a permission no role
holds.** `"policy_known":false` means the app did not describe its policy and
the column says nothing.

Check `tinywasm/json`'s array writer for the exact method that appends a bare
string to an array; if it only accepts objects, encode each role through a tiny
unexported `roleCode` wrapper implementing `EncodeFields` rather than
hand-building JSON. **Never concatenate JSON by hand in this repo.**

On encode failure, respond `500` with the body `encoding routes failed` — never
an empty `200`, which reads as "no routes" to whoever is auditing the server.

## Stage 4 — Conformance clauses

File: **`conformance/conformance.go`**. This package is the executable contract
both deployed implementations run; new behaviour is not real until it is pinned
here. Add these clauses to `Run`:

| Clause | Setup | Assert |
|---|---|---|
| `path parameter is extracted` | `GET /api/items/{id}`, handler writes `ctx.Param("id")` | request to `/api/items/42` → body `42` |
| `two parameters in one pattern` | `GET /api/sites/{site}/pages/{page}` | `/api/sites/a/pages/b` → `a` and `b`, each under its own name |
| `parameter does not match across a separator` | `GET /api/items/{id}` | `/api/items/42/extra` → **404** |
| `parameter does not match an empty segment` | `GET /api/items/{id}` | `/api/items/` → **404** |
| `literal beats parameter` | both `/api/items/new` and `/api/items/{id}` registered | `/api/items/new` reaches the literal handler |
| `unknown parameter reads empty` | `GET /api/items/{id}`, handler reads `ctx.Param("nope")` | body is empty, status `200` — not a panic |
| `parameters do not leak into Value` | handler reads `ctx.Value("id")` | empty — `Param` and `Value` are separate stores |

Every clause must drive a request through the implementation's **real**
pipeline via `ServeFunc`, never call the handler directly. The existing suite
documents why; do not weaken it.

## Stage 5 — `mock/`

Files: **`mock/context.go`**, **`mock/router.go`**, **`mock/route.go`**.

1. `mock.Context` implements `Param`, backed by two parallel slices
   (`paramNames`, `paramValues`) — **no map**.
2. `mock/router.go`'s `patternMatches` is replaced by a call to
   `router.MatchPattern`, and its longest-wins tie-break in `match` by
   `router.MoreSpecific`. Delete `patternMatches` and the now-unused `strings`
   import; `grep -n "patternMatches" mock/` must come back empty.
3. `registerRoute` calls `router.ValidatePattern` and panics with the returned
   error. A mock that accepts a pattern production rejects is a mock that
   certifies a broken app.
4. The matched route's parameter values are written onto the `Context` before
   the gate runs, so a middleware can read them too.

**Also in this stage:** `mock/router.go` currently carries several long comment
blocks in Spanish (`patternMatches`, `match`). This is a public `tinywasm`
library — code, comments and docs are English. Translate them, preserving the
full reasoning; do not shorten them to fit.

## Stage 6 — Tests

Under **`tests/`** (this repo's convention).

`tests/pattern_test.go`, table-driven:

| Pattern | Path | ok | values |
|---|---|---|---|
| `/api/items` | `/api/items` | true | none |
| `/api/items` | `/api/items/1` | false | — |
| `/api/items/` | `/api/items/1/2` | true | none (subtree, unchanged) |
| `/api/items/{id}` | `/api/items/42` | true | `["42"]` |
| `/api/items/{id}` | `/api/items/42/x` | false | — |
| `/api/items/{id}` | `/api/items/` | false | — |
| `/api/items/{id}` | `/api/items` | false | — |
| `/s/{a}/p/{b}` | `/s/x/p/y` | true | `["x","y"]` |

`ValidatePattern` cases, each asserting the **exact** message:
`/x/{a...}`, `/x/{}`, `/x/{a}/y/{a}`, `/v{n}`.

`MoreSpecific` cases covering all three clauses of the ordering rule, including
the tie where neither is more specific (`MoreSpecific(a,b)` and
`MoreSpecific(b,a)` must not both be true).

`tests/introspection_test.go`, driving `mock.Router`:

- Mount with a policy that grants nothing for a guarded route → that route's
  `roles` is `[]` and `policy_known` is `true`.
- Mount with `policy = nil` → `policy_known` is `false` for every route.
- Mount with a policy granting the route → `roles` names the role.
- A public route reports `"access":"public"` and an empty resource.
- A route with `Accepts(&fixture{})` reports its field names under `args`; a route that never
  called `Accepts` has **no** `args` key at all.
- Mounting before the routes are registered still reports them.

## Stage 7 — Documentation

- **`README.md`** — a short "Path parameters" section with the `{name}` syntax,
  the one-segment rule, the explicit non-support of `{name...}`, and a
  three-line example using `ctx.Param`.
- **`docs/`** — create **`docs/INTROSPECTION.md`**: the endpoint, the JSON
  shape, and one paragraph on how to read `policy_known`/`roles` (including the
  empty-roles finding and what it means). Reference it from `README.md`.
- **`AGENTS.md`** — one line under the harness principles: patterns are
  validated at registration, never at request time.
- Do **not** link `docs/PLAN.md` from any permanent document.

## Acceptance criteria

- [ ] `go build ./...`, `go vet ./...` clean; `gotest ./...` green.
- [ ] `grep -rn "map\[" pattern.go introspection.go router.go mock/` → empty.
- [ ] `grep -rn "\"strings\"\|\"strconv\"\|\"errors\"\|\"fmt\"" *.go mock/*.go conformance/*.go`
      → empty (only `tinywasm/fmt`).
- [ ] `grep -rn "patternMatches" .` → empty.
- [ ] `grep -n ":id" route.go` → empty; the `RouteInfo.Path` example says
      `{id}`.
- [ ] `go get github.com/tinywasm/model@latest` resolved a version exporting
      `RolesFor`, and `go.mod` contains **no** `replace` directive.
- [ ] `Context` has `Param`, and `mock.Context` implements it.
- [ ] The seven conformance clauses of Stage 4 exist in
      `conformance/conformance.go` and pass against `mock`.
- [ ] `grep -rn "[áéíóúñ¿¡]" mock/*.go` → empty (Stage 5 translation done).
- [ ] `docs/INTROSPECTION.md` exists and `README.md` links it.

## Out of scope

- Implementing `Param` in `tinywasm/cloudflare` and `tinywasm/server` — each has
  its own plan in this wave and both are gated on this one being published.
- Any HTML rendering. This repo serves JSON and nothing else; the explorer UI is
  a separate package that fetches it.
- `{name...}` wildcards, regex constraints, and typed parameters.

## Stages

| # | Stage | Files |
|---|---|---|
| 1 | Pattern syntax + helpers | `pattern.go`, `route.go` |
| 2 | `Context.Param` | `router.go` |
| 3 | `MountIntrospection` | `introspection.go` |
| 4 | Conformance clauses | `conformance/conformance.go` |
| 5 | mock + English translation | `mock/context.go`, `mock/router.go`, `mock/route.go` |
| 6 | Tests | `tests/pattern_test.go`, `tests/introspection_test.go` |
| 7 | Documentation | `README.md`, `docs/INTROSPECTION.md`, `AGENTS.md` |
