# Agent Guide — `tinywasm/router`

Constraints for agents working on this library. Read this before any change.

---

## Construction Harness — the TinyWasm way (read first)

The typed, explicit code **is** the harness. Whoever writes against this library is
often an agent that does **not** know it; they must produce correct code guided only
by the signatures, and the compiler must **reject** whatever is wrong. Correctness
lives in the compiler and the signatures, not in a manual you must remember. A
harness moves correctness to the compiler; a manual moves it to the reader — the
first is orders of magnitude more reliable for someone with no context.

Every public API must hold to these principles:

1. **Typed over `any`.** No generic holes (`func(...any)`, `interface{}`) in the
   API — intent-typed methods, like the `tinywasm/json` writer (`String`, `Int`,
   `Bool`, `Object`, `Array`). `any` is allowed **only** at the I/O edge, never in
   the data. **Reuse already-declared types** (e.g. `fmt.KeyValue`) instead of
   inventing new ones. Generics with an `any` constraint are the same hole in
   disguise: a signature that does not name its real type is not self-describing.
2. **Explicit over implicit.** The name declares the intent; reading the call is
   enough to know what it does, without opening the implementation.
3. **Illegal states unrepresentable.** If something must not happen, it must not be
   writable. One intent = one path, typed to demand exactly what it needs.
4. **One way to do each thing.** A single construction pattern, with no alternatives
   that force a choice or a trip to the docs.
5. **Minimal public surface.** Export exactly what the author uses; internal
   machinery stays unexported — you cannot misuse what you cannot see.
6. **Fail at compile time, not at runtime.** Order of preference to catch an error:
   compile error → noisy dev-mode diagnostic → (never) silent failure.
7. **Self-describing signatures.** Autocomplete must be enough to build. If using
   the API needs a long document, the API is incomplete.

**Litmus test:** if an agent with no context produces correct code guided only by
autocomplete and a few-line example, the harness is closed. If it needs a manual to
avoid mistakes, something is still untyped.

---

## Scope — single responsibility

This library declares **only** the isomorphic routing contract — `Context`,
`HandlerFunc`, `Router` — plus the `APIModule` contract by which a module publishes
its API by mounting onto a `Router`. It **implements no concrete server**: it only
defines the shape.

- **Routing only.** It knows nothing of native HTTP, Cloudflare, static files or UI.
  Those are other pieces that *implement* (a concrete server, an edge runtime) or
  *consume* this contract. A concrete router is an interchangeable implementor.
- **Isomorphic.** The same `Context` and the same `Router` hold on the native target
  (`!wasm`) and the edge/`wasm` target. No build tags, and **no `net/http` in the
  public surface** — a handler never touches the standard net stack directly.
- **One dependency only.** The identity contract (zero-dep), embedded by `APIModule`
  so that every API-capable module also carries `ModelName()`. Nothing else.

---

## Testing

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
gotest
```

- `gotest`, never `go test`. Stdlib assertions only. Dual WASM/stdlib.
- A fake `Router` (recording into a map) and a fake `Context` (buffering the write)
  prove a handler is writable guided only by the interface. Fix the contracts at
  compile time: `var _ Router = (*fakeRouter)(nil)`, `var _ APIModule =
  (*fakeModule)(nil)`. Publish with `gopush 'message'`.

---

## Documentation First

Update docs **before** code and before `gopush`: keep `docs/PLAN.md` and (when
present) `docs/ARCHITECTURE.md` in sync with the contract, and re-index `README.md`
so every `docs/` file is linked. Diagrams use `flowchart TD`, no `subgraph`, `<br/>`
for line breaks.
