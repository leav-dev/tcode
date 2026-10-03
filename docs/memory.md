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

## 4. Aprendizajes y Notas
- **Nota de rendimiento:** Evitar `fmt.Scan` o métodos de entrada estándar; usar exclusivamente `tcell` para no corromper el buffer de pantalla.
- **Gotchas:** Cuidado con los caracteres Unicode/Graphemes al manipular el buffer; siempre validar la longitud en bytes vs. caracteres visuales.
- **Gotcha de `tcell`:** `SimulationScreen.GetContents()` lee el *front buffer*; si el test no llama a `Show()` después de `Draw()`, la pantalla se ve vacía.
- **Gotcha de `mmap`:** no se puede mapear un archivo de 0 bytes (`invalid argument`). Verificar `Stat().Size()` antes de mapear.
- **Gotcha de `mmap` (segundo):** `[]byte` no tiene método `Unmap`; hay que conservar el `mmap.MMap` original para poder desmapear.
- **Deuda técnica registrada:** el render asume 1 celda por runa, por lo que los caracteres anchos (CJK, emoji) y los *grapheme clusters* desalinean las columnas. Pendiente: `github.com/rivo/uniseg` (ya está en el árbol de dependencias de `tcell`).
