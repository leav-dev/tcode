# Feature: Explorer Create Buttons

## Description
El explorador ya crea archivos y carpetas con `Ctrl+N` / `Ctrl+Shift+N`, en el
destino **contextual** del cursor (`CursorDir()`: la carpeta bajo el cursor, la
carpeta que contiene el archivo, o la raíz de la sesión). Faltaba el camino
VISIBLE: se agrega un **pie de panel** con dos botones clicables —`+ Archivo` y
`+ Carpeta`— que disparan los mismos flujos. El teclado sigue funcionando igual.

## Decisiones de diseño
- **El pie ocupa la última fila del panel** (`listRows()` = alto − 1): el árbol
  pierde una fila visible y el scroll/página se recalculan con ella. La fila
  queda justo arriba de la barra de estado —el "abajo" del explorador—.
- **El destino es el del cursor**, sin cambios: el botón llama a
  `promptCreateEntry`, que ya resuelve y captura `CursorDir()` antes del prompt.
- **Mismo dibujo y mismo hit-test**: `buttonRanges()` calcula los dos rangos
  [x0,x1) y lo usan el `Draw` y el mouse, para que no se desincronicen.
- **Rol de tema `Button`** (chips en video inverso, como la pestaña activa):
  `Button` en las 7 paletas + `LoadTheme` (`button`), según la regla de que
  ningún módulo hardcodea un color.
- **Sin teclado nuevo**: los botones son la afordancia de mouse (y la pista
  visual); `Ctrl+N` / `Ctrl+Shift+N` siguen siendo el camino de teclado.

## Tasks
- [x] View: `ActionNewFile`/`ActionNewFolder`, `listRows()` en clamp/
  ensureCursorVisible/page/Draw, pie (`buttonRanges` + `drawFooter`) + tests <!-- id: 1 -->
- [x] Theme: rol `Button` en 7 paletas + `LoadTheme` <!-- id: 2 -->
- [x] View: hit-test del pie en el mouse → las acciones nuevas + tests <!-- id: 3 -->
- [x] Controller: wiring de las acciones (teclado y mouse) + tests <!-- id: 4 -->
- [x] Docs: `README.md` (fila de features y tabla de teclas) <!-- id: 5 -->
- [x] Verificación (gofmt/vet/test -race) y commit de unidad del trabajo <!-- id: 6 -->

## Bug adyacente corregido
El clic en el espacio vacío del panel (menos nodos que filas) indexaba el
aplanado fuera de rango: `fb.nodes[fb.top+y]` con `top+y >= len(nodes)`. Se
corrige con el chequeo de rango, cubierto por
`TestFileBrowserClickBelowTheNodesIsConsumed`.

## Verificación
- `gofmt -l` limpio en los archivos tocados.
- `go vet ./...` limpio.
- `go test -race ./...` ok (controller, ext, model, view).
- Tests: 5 nuevos de la vista (pie dibujado, clics, vacío, sin alto) + 3 del
  controlador (botón de archivo, de carpeta, clic fuera); 4 tests existentes
  actualizados a la geometría nueva (el pie ocupa una fila).
