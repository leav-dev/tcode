# Feature: Salto por palabra (Ctrl+Left/Right)

## Description
Agregar desplazamiento por palabra con `Ctrl+Left` / `Ctrl+Right`, estilo VSCode,
más extensión de selección con `Ctrl+Shift+Left/Right`. Cierra el pendiente de
`cursor-and-movement.md` ("Sin salto de palabra (Ctrl+flechas)").

## Decisions
- Semántica elegida por el usuario: salto por palabra (letras/dígitos/`_`), no
  solo por espacios ni inicio/fin de línea.
- `Ctrl+Right`: si estoy dentro de palabra va al fin de esa palabra; si estoy en
  separador/espacio cruza separadores y va al fin de la próxima palabra. Cruza
  líneas (fin de línea → próxima línea).
- `Ctrl+Left`: espejo hacia atrás (al inicio de la palabra).
- `Ctrl+Shift+Left/Right`: misma navegación pero extiende selección
  (reusa `moveWithShift`).
- `Ctrl` solo en `Left/Right`; no toca `Up/Down`, `PgUp/PgDn`, `Home/End`.

## Tasks
- [ ] Navegación por palabra en la vista (`moveWord`) <!-- id: 0 -->
- [ ] Cablear `Ctrl+Left/Right` (+Shift) en `handleKey` <!-- id: 1 -->
- [ ] Tests: salto simple, símbolos, multilínea, con Shift <!-- id: 2 -->
- [ ] Verificación: `go vet`, `gofmt -l`, `go test` <!-- id: 3 -->
- [ ] Docs (README/constitution si listan atajos) + commit <!-- id: 4 -->

## Evidence
- `internal/view/editor_view.go`: `isWordRune` (letra/dígito/`_`), `moveWord`/`moveWordRight`/`moveWordLeft` con cruce de líneas, cableado en `handleKey` para `Ctrl+Left/Right` vía `moveWithShift` (con `Shift` extiende selección).
- `internal/view/word_jump_test.go`: 5 tests (fin de palabra, inicio, símbolos como separadores, cruce de líneas, `Ctrl+Shift` extiende `[0,3)`).
- Verificación: `go vet ./...` limpio, `gofmt -l internal/` limpio, `go test ./...` OK.
- Docs: `README.md` suma fila de salto por palabra y aclara wrap vs palabra.
