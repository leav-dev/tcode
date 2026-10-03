# Feature: Selección y portapapeles

## Description
Deuda de `docs/memory.md` ("sin selección ni portapapeles"). Esta unidad suma
selección visual (teclado y mouse) y copiar/pegar del portapapeles del SISTEMA.

Decisiones de producto (usuario):

- **Estilo VSCode**: `Shift+flechas` extienden la selección, `Ctrl+A`
  selecciona todo, `Ctrl+C` copia y `Ctrl+V` pega. Cualquier movimiento sin
  Shift descarta la selección.
- **El arrastre del mouse selecciona**: presionar el botón 1 ancla, arrastrar
  (ButtonMotion con el botón) extiende la selección desde el ancla; soltar
  termina; el clic (sin arrastre) sigue posicionando como hoy.
- **Portapapeles del sistema**: dependencia nueva y ligera
  `github.com/atotto/clipboard` (Windows: user32, sin cgo, en línea con la
  dependencia mínima). El paquete de tests la reemplaza con mocks (el host de
  tests no puede escribir el portapapeles real).

Diseño:

- **Selección en la vista, no en el modelo**: el archivo es texto; la selección
  es un rango `[Start, End)` de offsets de documento como estado del
  `EditorView` (como el cursor). El modelo solo expone el texto del rango
  (`TextRange`, público, para el copy).
- **Ancla**: al mover con Shift desde sin-selección, se ancla la posición
  previa del cursor; el extremo es el cursor que se mueve. Sin selección
  previa, el ancla se fija; con selección previa, extiende desde el ancla.
- **Escribir reemplaza la selección**: teclear, Backspace o Delete con
  selección borran el rango (y el tipeo inserta donde quedó el ancla), como
  VSCode. `Ctrl+A` mueve el cursor al final con el rango 0..docLen.
- **Ctrl+C sin selección NO copia** y conserva el comportamiento de salida del
  editor (el controller decide: con selección activa, `Ctrl+C` es copiar y no
  sale; sin selección, sale como hoy).
- **Render**: los clusters dentro del rango se pintan con el rol `Selection`
  del tema (default: Reverse + acento, como los roles activos; clave JSON
  `"selection"`). La selección gana sobre el fondo de la línea del cursor.

Qué NO cambia: la edición sin selección, el movimiento, el modelo. El
portapapeles se prueba por mocks; un smoke real se salta si el sistema no lo
permite.

## Tasks
- [x] Tema: rol `Selection` (default + JSON) <!-- id: 0 --> · `bae5bf0`
- [x] Vista: `Selection` (rango), ancla con Shift+movimiento (Horizontal,
  Vertical, PgUp/Dn, Home/End, DocStart/End), limpieza sin Shift, `Ctrl+A`
  <!-- id: 1 --> · `bae5bf0`
- [x] Render de la selección (el cluster en rango → rol Selection) <!-- id: 2 --> · `49cfc69`
- [x] Edición con selección: teclear/Backspace/Delete reemplazan el rango
  <!-- id: 3 --> · `49cfc69`
- [x] Clipboard: `TextRange` en el modelo; mocks; `Ctrl+C` copia (la vista),
  `Ctrl+V` pega (reemplazando la selección) <!-- id: 4 --> · `49cfc69`
- [x] Controller: `Ctrl+C` con selección copia y no sale; sin selección sale
  (comportamiento actual) <!-- id: 5 --> · `49cfc69`
- [x] Mouse: presionar ancla, arrastrar extiende, soltar termina <!-- id: 6 --> · `49cfc69`
- [x] Tests por unidad + suite completa sin fallos nuevos <!-- id: 7 -->

## Design decisions

### La selección es estado de la vista
`[Start, End)` de offsets en el `EditorView` (como el cursor) que el modelo no
conoce. Copiar solo pide el texto del rango (`TextRange` público nuevo en el
modelo): el archivo sigue siendo puro texto y la vista dibuja el marcado.

### El ancla de VSCode
Con Shift se extiende desde el ÚLTIMO ancla; sin selección previa, el ancla es
la posición del cursor antes del primer movimiento con Shift. Cualquier
movimiento sin Shift limpia el rango y el ancla (salir del estado de selección
es la acción explícita que nunca se hace sola).

### Ctrl+C pelea con la salida
Hoy `Ctrl+C` siempre sale del editor. Con la selección, `Ctrl+C` copia cuando
hay rango activo y SOLO sale cuando no — el controller decide mirando la vista,
y el viejo comportamiento de salida se conserva en el caso sin selección.

## Falsificación (tests que escriben primero contra el código roto)
1. Sin `Selection` → los tests del movimiento con Shift no compilan.
2. Sin render de selección → `TestSelectionStyledInRender` falla.
3. Sin reemplazo → `TestTypingReplacesSelection` falla (el texto se inserta
   junto al rango).
4. Sin copy/paste → `TestCopyPutsMarkedTextOnClipboard` no compila.
5. Sin el guard del controller → `TestCtrlCCopiesInsteadOfQuitting` falla
   (Ctrl+C con selección sale o no copia).
6. Sin drag → `TestMouseDragSelects` falla.

## Evidence

### Selección · `bae5bf0` + `49cfc69`
- **RED:** los tests de movimiento con Shift, render, reemplazo y clipboard.
- **Lección del coalesce:** `breakTypingGroup` en `insertText` genérico rompió
  la fusión de tipeo (2 tests); el corte corresponde SOLO al reemplazo de
  selección, no al tipeo plano.
- **Gotcha tcell:** no existe `ButtonMotion` en v2.13: el arrastre llega como
  `Button1` repetido con posiciones distintas; el flag propio distingue de un
  clic. El release es `ButtonNone`.
- **Gotcha de edición repetido:** los `\r` literales pasados por los heredocs
  de edición se corrompían en LF real; se resolvió comparando el código 13
  sin string literal.
- **Dependencia nueva:** `atotto/clipboard` (ligera, sin cgo); los tests la
  reemplazan con mocks inyectables.
- **Verificación:** suite completa con 34 fallos ambientales y 0 nuevos.