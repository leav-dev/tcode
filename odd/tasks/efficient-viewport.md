# Feature: Viewport Eficiente y Scroll

## Description
Implementar la proyección visible del documento (Viewport) y la navegación
básica por teclado y mouse, garantizando que solo se procese el rango visible
del archivo (regla de la `constitution.md`: "lo que no está en pantalla no se carga").

## Tasks
- [x] Implementar `GetRange` y `LineCount` en `PieceTable` <!-- id: 0 -->
- [x] Definir la estructura `Viewport` en `internal/view` <!-- id: 1 -->
- [x] Actualizar `editor_view` para renderizar solo el rango visible <!-- id: 2 -->
- [x] Implementar scroll por teclado (flechas, j/k, PgUp/PgDn, Home/End, h/l) <!-- id: 3 -->
- [x] Implementar scroll por mouse (rueda vertical y horizontal) <!-- id: 4 -->
- [x] Soportar `EventResize` y reajustar el viewport <!-- id: 5 -->
- [x] Tests del modelo (8) y de la vista con `SimulationScreen` (12) <!-- id: 6 -->
- [ ] Commit de unidad de trabajo (`feat(view): ...`) — **pendiente de autorización del usuario**

## Evidence
- `internal/model/piece_table.go`
  - `lineOffsets []int` construido una sola vez por carga; `GetRange` devuelve una
    **vista zero-copy del mmap** (verificado por `TestGetRangeIsZeroCopyViewIntoMmap`).
  - `LineCount()` no cuenta la línea fantasma cuando el archivo termina en `\n`.
  - **Bug real detectado por test:** `mmap.Map` falla con `invalid argument` en
    archivos vacíos (`TestEmptyFileHasNoLines`). Corregido con un atajo que no mapea
    0 bytes.
- `internal/view/editor_view.go`
  - `Viewport{TopLine, LeftColumn, Height, Width}` centraliza el estado de proyección.
  - `Draw` decodifica UTF-8 con `utf8.DecodeRune` sin alocar y respeta el scroll horizontal.
  - `HandleEvent` devuelve `bool`: el controlador solo redibuja si el viewport cambió.
- `internal/view/editor_view_test.go`
  - Renderizado verificado con `tcell.NewSimulationScreen("UTF-8")`: sin TTY, sin `/dev/tty`.
  - Nota: `GetContents()` lee el *front buffer*, por lo que el test debe llamar `Show()`.
- Comandos verificados:
  - `go vet ./...` → limpio
  - `gofmt -l .` → limpio
  - `go test -race -count=1 ./...` → **20 tests, OK**
- `internal/controller/app.go`
  - `NewApp(path string)` carga el archivo opcional y devuelve error en lugar de `panic`.
  - Loop de eventos síncrono (sin goroutine) con un único camino de redibujado.
- `main.go` acepta la ruta del archivo como argumento.

## Known Limitations (próxima iteración)
- **Ancho de caracteres:** se dibuja 1 celda por runa. Los caracteres anchos (CJK,
  emoji) y los *grapheme clusters* desalinearán las columnas. Requiere `uniseg`.
- **Edición:** la `PieceTable` todavía no implementa `Insert`/`Delete`; el editor es solo lectura.
- **`GetContent()`:** existe solo para tests y usos no interactivos; no debe usarse en el render.
