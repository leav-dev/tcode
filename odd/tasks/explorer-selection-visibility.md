# Feature: Visibilidad de la selección del explorador de archivos

## Description
El cursor del árbol de archivos del panel lateral es difícil de identificar:
con `~/.tcode/theme.json` seteado (`"treeCursor": "237"`), `LoadTheme` aplica
el rol como **color de frente sin fondo ni Reverse**, y la fila activa queda
gris oscuro sobre fondo oscuro — casi invisible. El default (`active(ColorDefault)`)
depende de los colores "default" de la terminal y es frágil.

Objetivo: que la posición en el árbol se identifique de inmediato, en
cualquier terminal y con cualquier tema:
- `treeCursor` pasa a ser el **color de fondo** de la fila activa (semántica
  como `cursorLine`): la barra de selección a todo el ancho siempre lleva un
  fondo explícito y el texto en el color por defecto. Default: azul oscuro
  (paleta 24) con texto claro (15), el mismo acento de la barra de estado.
- La fila activa de **archivos** lleva un marcador `> ` (los directorios ya
  tienen su caret `▸`/`▾`, que se conserva).

## Tasks
- [x] `Theme.TreeCursor`: default `on(15, 24)` (blanco sobre azul oscuro);
  `LoadTheme` trata `treeCursor` como color de **fondo** (`Background(c)`,
  como `cursorLine`), texto por defecto; `"default"` o valor inválido
  conserva la barra por defecto <!-- id: 0 -->
- [x] `FileBrowser.Draw`: la fila activa de archivos usa el prefijo `> `
  (misma semántica de 2 celdas que `  `); los dirs conservan `▸`/`▾`;
  comentario del método actualizado <!-- id: 1 -->
- [x] Tests de la vista: `cellBg` helper; `TreeCursor` verificado como fondo
  (bg 237 en `LoadTheme`, bg 24 en default); resaltado por barra de acento
  (ya no `Reverse`); test nuevo del marcador `> ` en la fila activa de
  archivos y de que los dirs lo conservan con caret <!-- id: 2 -->
- [x] Test del controlador `TestStartupWithDirectoryArgument`: la fila activa
  de archivo espera `"> doc.txt"` <!-- id: 3 -->
- [x] Docs: `docs/editor-theme.md` — rol `treeCursor` como color de fondo de
  la fila activa; párrafo de roles "activos" corregido <!-- id: 4 -->
- [x] Verificación: `gofmt -l`, `go vet ./...`, `go test ./...` (suite
  completa, vía verifier) <!-- id: 5 -->
- [x] Verificación: `gofmt -l`, `go vet ./...`, `go test ./...` (suite
  completa, vía verifier) — gofmt marcado por line endings CRLF del host
  (artefacto preexistente, probado con `main.go` pristino); `internal/model` y
  3 tests de `internal/controller` son fallos ambientales de Windows,
  verificados idénticos en HEAD con el cambio ausente <!-- id: 5 -->
- [x] Commit de unidad de trabajo en rama `feat/explorer-selection-visibility`
  con Conventional Commit en inglés: `2872d16` <!-- id: 6 -->

## Design decisions
- **`treeCursor` es un color de fondo, no un estilo.** La fila activa se pinta
  como barra: `StyleDefault.Background(c)` con el texto en el fg por defecto
  (que se adapta a terminales claras y oscuras). Misma semántica que
  `cursorLine`; el usuario que escribió `"237"` obtiene una barra gris clara
  visible sin cambiar su tema.
- **El marcador `> ` solo en archivos.** Los dirs ya señalan expansión con
  `▸`/`▾`; reemplazarlo en la fila activa perdería el estado expandido. El
  marcador de archivo es un refuerzo de la barra sin costo de alineación
  (prefijo sigue siendo de 2 celdas) ni de hit-testing (los carets no se
  mueven).
- **Sin gutter dedicado:** el panel mide a lo sumo 24 celdas; una columna de
  gutter comería ancho. La barra de fondo + marcador sobre la fila bastan.
- **Reverse sigue solo en `tabActive`.** El contrato de `Reverse` de los roles
  "activos" se corrige en la doc: `treeCursor` ya no depende de los colores
  default de la terminal.