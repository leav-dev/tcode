# Feature: Auto-cierre de pares + Enter entre pares

## Description
Cierre automático al tipear apertura (`()[]{}` más `""''```), con
sobreescritura al tipear el cierre, borrado doble con Backspace y Enter
inteligente entre pares (el cierre salta a la línea siguiente con indent).

## Decisions (usuario)
- Pares: brackets + comillas (con salto al tipear la misma comilla).
- `'` no duplica tras una runa de palabra (para no romper `don't`).
- Enter entre pares adyacentes: parte en dos líneas; nivel extra de indent
  solo para `([{`, las comillas parten al mismo nivel.
- Selección activa + apertura: envuelve (surround), estilo VSCode.
- Implementado en `handleKey` (no en `insertText`): el pegado no dispara pares.

## Tasks
- [ ] Auto-cierre + surround + overtype + borrado doble <!-- id: 0 -->
- [ ] Enter entre pares (split con indent) <!-- id: 1 -->
- [ ] Tests de pares <!-- id: 2 -->
- [ ] Verificación (`vet`, `gofmt`, `go test`) + docs <!-- id: 3 -->
- [ ] Commit de unidad de trabajo <!-- id: 4 -->

## Evidence
- `view/editor_view.go`: `autoClosePairs` + `typePair` (par, surround con
  selección, overtype del cierre, `'` simple tras palabra), `trySplitPair`
  (Enter entre pares adyacentes, indent extra solo para `([{`),
  borrado doble en `backspace()` — todo en un paso de deshacer por acción.
- `view/pairs_test.go`: 6 tests (par, overtype, borrado, split, comillas,
  surround). `auto_indent_test.go`: el test de brace con trailing spaces se
  adaptó al mundo con pares (Right salta el cierre, Backspace lo borra).
- Verificación: `go vet ./...` limpio, `gofmt -l` limpio, `go test ./...` OK.
- Docs: fila de pares en `README.md`.
