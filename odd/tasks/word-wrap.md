# Feature: Salto de palabra (word wrap)

## Description
Deuda de `docs/memory.md` ("sin salto de palabra"). Las líneas que exceden el
ancho del editor hoy se cortan contra el borde y obligan a scrollear
horizontalmente. Esta unidad agrega **wrap visual**: las líneas largas se
envuelven a varias filas VISUALES dentro del ancho del viewport, sin tocar el
archivo ni el modelo (las líneas lógicas siguen intactas).

Decisiones de producto (usuario):

- **Configurable**: `var wordWrapEnabled` (mutable, la futura configuración la
  expondrá) + una tecla para alternarla en vivo: `Ctrl+Shift+W` (tcell la
  reporta como Ctrl+Shift, igual que el redo con z). Mensaje transitorio en la
  barra al alternar.
- **Corte por palabra** (tipo VSCode wordWrap): la fila se corta en el último
  límite de palabra que entra; una palabra que no cabe va entera a la fila
  siguiente; una palabra más larga que el ancho se parte por carácter (como
  VSCode).

Alcance v1 (modelo nano):

- El wrap es **visual puro**: el scroll, el cursor y el movimiento siguen en
  LÍNEAS LÓGICAS, como hoy. `Down`/`Up` mueven a la siguiente línea lógica (en
  una línea envuelta se "salta" el resto de la fila visual — limitación
  documentada; la navegación por unidades visuales queda como roadmap).
- El **cursor** se proyecta a su fila visual (física) para dibujarse y para
  `ensureCursorVisible`: si la línea lógica es más alta que el viewport, el
  cursor puede quedar fuera del área visible (nano igual); documentado.
- El **mouse** traduce la fila visual clicada a (línea lógica, bytecol) con el
  ancho real.

## Tasks
- [ ] `view/wrap.go`: `softLine`/`softLineCount`/`softLineAt`/`softLineToByte`
  — la línea lógica envuelta en filas visuales, corte por palabra con quiebre
  de palabras largas <!-- id: 0 -->
- [ ] Tests de wrap: corte por palabra, palabras largas partidas, tabulaciones,
  ancho justo, invariantes count/at/toByte <!-- id: 1 -->
- [ ] `Draw` envuelve: cada línea lógica se pinta en sus filas visuales; el
  clúster que no entra y no aplica wrap (desactivado) sigue cortando igual que
  hoy <!-- id: 2 -->
- [ ] Cursor: `drawCursor` proyectado a la fila visual física; movimiento
  vertical sin cambio (línea lógica) — tests del render del cursor envuelto
  <!-- id: 3 -->
- [ ] `ensureCursorVisible` vertical en unidades visuales (con el acomodo por
  líneas lógicas documentado); mouse traducido a (línea, bytecol) vía las
  filas visuales <!-- id: 4 -->
- [ ] Config: `var wordWrapEnabled` + toggle `Ctrl+Shift+W` en el controlador
  con mensaje; tests <!-- id: 5 -->
- [ ] Verificación: suite completa sin fallos nuevos vs base (parser limpio) +
  docs de `memory.md` <!-- id: 6 -->

## Design decisions

### Wrap visual, no de datos
El archivo y el modelo no cambian: `LineCount()` sigue siendo líneas lógicas y
el contenido se envuelve solo al pintar y al traducir entrada. Radicalmente más
simple que un modelo de unidades visuales y suficiente para leer líneas largas
sin scroll horizontal. El movimiento vertical por línea lógica (nano-like) se
documenta como límite; el salto a unidades visuales completas queda como
roadmap de la misma unidad futura.

### Corte por palabra con quiebre
La fila acumula clusters; al desbordar, si hubo un límite de palabra (un
espacio) dentro de la fila, la palabra que no cabía completa va a la fila
siguiente; si no (palabra gigante), se parte en el clúster exacto. Como VSCode.

### Config mutable + tecla
`wordWrapEnabled` vive en el paquete view (variable, como `indentUnit`); la
tecla del controlador la alterna y avisa. La futura configuración del editor
podrá escribir la misma variable.

## Falsificación (tests que escriben primero contra el código roto)
1. Sin `softLine*` → los tests del wrap no compilan.
2. Sin wrap en `Draw` → `TestRenderWrapsLongLine` falla (el texto sigue y el
   borde corta la fila 0).
3. Cursor sin proyectar → `TestDrawCursorProjectsToVisualRow` falla (el cursor
   se dibuja contra líneas lógicas).
4. Sin toggle → `TestWrapToggleKey` falla.

## Evidence
(Rellenar por unidad.)