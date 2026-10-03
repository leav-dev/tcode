# Registro de Memoria del Proyecto: tcode

Este archivo registra las decisiones arquitectónicas clave, cambios estructurales y aprendizajes para mantener el estado del proyecto a lo largo del tiempo.

## 1. Bases Fundamentales
- **Lenguaje:** Go 1.21+.
- **Arquitectura:** MVC + Screen Architecture.
- **Data Structure:** Piece Table (implementada para eficiencia de memoria).
- **Entrada/Salida:** `tcell/v2` para TUI y manejo avanzado de mouse.

## 2. Decisiones Arquitectónicas
- **MVC Separado:** El código se encuentra en `internal/model`, `internal/view`, `internal/controller`.
- **Carga de Archivos:** Uso estricto de `mmap` (via `edsrzf/mmap-go`) para evitar la carga de archivos completos en RAM.
- **Manejo de Componentes:** Cada parte de la UI será una `Screen` que se carga bajo demanda, evitando inicializaciones pesadas innecesarias.
- **Repositorio:** `git@github.com:leav-dev/tcode.git`. La rama por defecto es `main` (no `master`) por convención del autor.
- **Commits:** Conventional Commits en inglés. Una unidad de trabajo por commit, con tests y docs junto al código. El push es siempre una decisión explícita del usuario.

## 3. Registro de Cambios (Changelog de Memoria)
- *2024-05-23:* Inicialización del proyecto, `go.mod` y estructura de directorios MVC. Implementación del esqueleto `tcell` con soporte de mouse. Definición de `constitution.md`.
- *2024-05-23:* Creación de `agents.md` para estandarizar la interacción con agentes externos y `memory.md` para el registro de estado.
- *2024-05-23:* **Viewport eficiente + scroll.** `PieceTable.GetRange` devuelve una vista *zero-copy* del `mmap` y `LineCount` maneja el final de archivo. La `View` renderiza solo el rango visible. Scroll completo por teclado y rueda del mouse, más soporte de `EventResize`. Se agregaron 20 tests (`internal/model`, `internal/view`) usando `tcell.SimulationScreen` para testear el render sin TTY.
  - **Bug encontrado por test:** `mmap.Map` falla con `invalid argument` en archivos de 0 bytes. Se resolvió con un atajo para el archivo vacío.
  - **Decisión:** `HandleEvent` devuelve `bool` para que el controlador solo redibuje cuando el viewport cambió (evita repintados inútiles).
  - **Decisión:** el loop de eventos es síncrono, sin goroutine; el único camino de redibujado es `App.redraw()`.
- *2024-05-23:* **Preparación del repositorio y primer commit.** Rama renombrada de `master` a `main`, remoto `origin` apuntando a `git@github.com:leav-dev/tcode.git`, `.gitignore` extendido para Go y artefactos de editor. Commit raíz **`fc07fa0`** (`feat: bootstrap tcode with MVC scaffold and efficient viewport`, 14 archivos, 1009 líneas) pusheado a `origin/main`. Verificado con el worktree limpio: `go vet`, `gofmt -l` y `go test -race` sobre el snapshot exacto.
  - **Estado del switch RDD:** `gentle-ai review mode status` → **off** (global y clone-local unset), por lo que no aplica el preflight de native review en este candidato.
- *2026-10-03:* **Ancho de caracteres correcto (grapheme clusters).** `Draw` dejó de asumir 1 celda por runa: ahora itera con `uniseg` y avanza la columna lógica por `Width()`, medida en celdas de terminal. Se cubren CJK, acentos combinantes, emoji ZWJ, tabulaciones, CRLF/CR y el borde derecho. 10 tests nuevos, **9 de 10 verificados como detectores de la regresión** (se revirtió `Draw` con `git stash` y fallan contra la implementación vieja).
  - **Hallazgo:** `tcell.Screen.SetContent` está deprecado y hace `string(append([]rune{mainc}, combc...))`, o sea dos *allocations* por celda. `tcell.Screen.Put` recibe el cluster como `string`, es *grapheme-aware* por dentro y evita ambas. `Draw` ahora usa `Put`.
  - **Hallazgo:** `uniseg.Graphemes.Bytes()` aloca (`[]byte(g.cluster)`); `Str()` devuelve un substring sin alocar. Se usa `Str()`.
  - **Decisión:** el texto visible se expone con `unsafe.String` sobre los bytes del `mmap` para no copiar la ventana en cada frame. Seguro porque el mapeo es de solo lectura y nadie retiene la vista más allá de `Draw`.
  - **Decisión:** `uniseg` promovido de dependencia indirecta a directa (`go mod tidy`).
- *2026-10-03:* **Edición en la Piece Table.** `Insert(offset, text)` y `Delete(start, end)` funcionando, con `GetRange` en coordenadas de documento. Se agregaron 20 tests (28 en el modelo).
  - **Decisión:** `newBuffer` es *append-only*, así que una pieza es **inmutable** una vez creada. Editar es crear y descartar piezas, nunca mutarlas. Eso es lo que deja intactos el `mmap` y las piezas viejas, y lo que va a regalar el undo/redo.
  - **Decisión:** se separaron explícitamente dos coordenadas que antes coincidían: *offset de documento* (lógico, el que usan vista y edición) y *offset de buffer* (interno a cada pieza). Resolverlas mezcladas era la trampa central de implementar edición.
  - **Decisión:** `GetRange` mantiene un **camino rápido cero-copia** cuando no hay ediciones (una sola pieza del `mmap`) y materializa solo cuando el rango abarca varios buffers. El contrato queda documentado en el método porque devolver a veces una vista y a veces una copia es una trampa si no se aclara.
  - **Bug real encontrado por el test diferencial** (`TestEditsMatchReferenceString`, 400 iteraciones contra un `string` de referencia) en la iteración 91: al borrar `[start, end)`, un inicio de línea en `end` se mapeaba a `start` sin verificar que `start` fuera inicio de línea. Reproducción mínima: `"a\n\nb"` borrando `[1,2)` producía `lineOffsets [0,1,2]` con un offset no precedido por `'\n'`. El fix (`l >= end` → `l > end`) además elimina la necesidad del dedupe.
  - **Falsificación:** reintroducir la condición defectuosa hace fallar 5 tests, incluido el diferencial en la iteración 91 exacta.
  - **Gotcha:** el índice de líneas es O(cantidad de líneas) por edición por el corrimiento, y `locate` es O(cantidad de piezas). Correcto primero; la optimización siguiente es un árbol de Fenwick / piezas balanceadas.

## 4. Aprendizajes y Notas
- **Nota de rendimiento:** Evitar `fmt.Scan` o métodos de entrada estándar; usar exclusivamente `tcell` para no corromper el buffer de pantalla.
- **Gotchas:** Cuidado con los caracteres Unicode/Graphemes al manipular el buffer; siempre validar la longitud en bytes vs. caracteres visuales.
- **Gotcha de `tcell`:** `SimulationScreen.GetContents()` lee el *front buffer*; si el test no llama a `Show()` después de `Draw()`, la pantalla se ve vacía.
- **Gotcha de `mmap`:** no se puede mapear un archivo de 0 bytes (`invalid argument`). Verificar `Stat().Size()` antes de mapear.
- **Gotcha de `mmap` (segundo):** `[]byte` no tiene método `Unmap`; hay que conservar el `mmap.MMap` original para poder desmapear.
- **Deuda técnica registrada:** el render asume 1 celda por runa, por lo que los caracteres anchos (CJK, emoji) y los *grapheme clusters* desalinean las columnas. Pendiente: `github.com/rivo/uniseg` (ya está en el árbol de dependencias de `tcell`).
  - **RESUELTO** en `odd/tasks/character-width.md`.
- **Gotcha de `tcell`:** `SetContent` está deprecado y aloca dos veces por celda. Usar `Put(x, y, cluster, style)`, que además es *grapheme-aware*.
- **Gotcha de `uniseg`:** `Graphemes.Bytes()` aloca; `Graphemes.Str()` no.
- **Deuda técnica abierta:** no existe el mapeo inverso celda → offset del documento, que el hit testing del mouse va a necesitar cuando se implemente el click.
- **Deuda técnica abierta:** el modelo ya soporta `Insert`/`Delete`, pero el controlador todavía no los invoca: la interfaz sigue siendo de solo lectura. No hay `Save` a disco tampoco.
