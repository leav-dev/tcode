# Feature: Edición por Teclado

## Description
Conectar el teclado con `Insert`/`Delete` del modelo. Es lo que convierte a `tcode`
en un editor de verdad: hasta acá el modelo editaba pero la interfaz era de solo
lectura.

## Tasks
- [x] `insertText`: escribir runas, Enter y Tab en la posición del cursor <!-- id: 0 -->
- [x] `backspace`: borrar el cluster anterior y fusionar líneas al inicio <!-- id: 1 -->
- [x] `deleteForward`: borrar el cluster del cursor y fusionar líneas al final <!-- id: 2 -->
- [x] Respetar los modificadores: Ctrl y Alt quedan libres para atajos, no insertan <!-- id: 3 -->
- [x] Corregir la semántica de `LineCount` para que la línea vacía final sea direccionable <!-- id: 4 -->
- [x] Tests de edición: inserción, borrado, fusión de líneas, ida y vuelta <!-- id: 5 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race` <!-- id: 6 -->
- [x] Commit de unidad de trabajo <!-- id: 7 -->

## Evidence

### Lo que se implementó
- `insertText(s)`: inserta en la posición del cursor y lo deja después del texto.
- `backspace`: con `ByteCol > 0` borra el *grapheme cluster* anterior; al inicio de
  línea borra el **salto completo** de la línea anterior, que es lo que fusiona las
  dos líneas y deja el cursor en la unión.
- `deleteForward`: con `ByteCol < len(content)` borra el cluster del cursor; al
  final de línea borra el salto y fusiona con la siguiente.
- Enter y Tab se resuelven por `Key` (no por runa), y una runa solo inserta si no
  tiene `ModCtrl` ni `ModAlt`, para que un atajo nunca escriba texto por accidente.

### Corrección de semántica: la línea fantasma
`LineCount()` **excluía** la línea vacía final que deja un `\n`. Eso servía para un
visor, pero con cursor se rompe: tras `Enter` al final de `"uno"` el documento es
`"uno\n"` y el cursor queda en la línea 1, que `LineCount()` no reconocía. La
flecha abajo desde ahí **saltaba hacia arriba**, porque el tope era la línea 0.

**Corrección:** `LineCount()` devuelve `len(lineOffsets)` (un documento vacío sigue
teniendo 0 líneas). Un archivo que termina en `\n` tiene una línea vacía final
direccionable, igual que en cualquier editor. Lo exige el modelo de cursor
`(línea, byte dentro de línea)`: sin esa línea no hay dónde poner el cursor.

Esto obligó a actualizar `expectedLines` del test diferencial, que era justamente
la referencia contra la que se comparaba.

### Hallazgo: `KeyLF` y los saltos de línea
Al escribir el test de ida y vuelta, el `\n` no se insertaba pero el `\t` sí. La
causa está en `tcell.NewEventKey`:

- `Key(0x09)` es `KeyTab`, que está en su lista de teclas "directamente tipeables",
  así que la deja sin modificadores.
- `Key('\n')` es `KeyLF` (10), que **no** está en esa lista, así que tcell la marca
  con `ModCtrl` y la reporta como `KeyLF`, no como `KeyRune`.

Con la rama de runas exigiendo sin modificadores, el salto se perdía **en
silencio**. Se agregó `KeyLF` junto a `KeyEnter`: hay terminales y modos de línea
que mandan LF crudo, y perder el Enter es un fallo invisible.

Nota: en un terminal real Enter llega como `KeyEnter` (CR), así que este camino es
de robustez, no el habitual. El test sintético que usé al principio alimentaba un
evento que ningún terminal produce para Enter; el test de ida y vuelta ahora usa
`KeyEnter` y hay un test dedicado para el alias de LF.

### Dos errores míos
Usé `End` dos veces esperando llegar al final del documento, cuando `End` es **de
línea** desde la unidad anterior. El correcto es `Ctrl+End`. Fueron dos tests con
el mismo error.

### Falsificación
| Cambio revertido | Tests que fallan |
|---|---|
| `LineCount` vuelve a excluir la línea fantasma | `TestLineCountIncludesTrailingEmptyLine`, `TestEnterAtTheEndCreatesAnAddressableEmptyLine` y el diferencial `TestEditsMatchReferenceString` |
| Se quita `KeyLF` de la rama de salto de línea | `TestLineFeedAlsoInsertsANewline` |

`TestEnterSplitsTheLineAndMovesCursorToTheNewOne` y
`TestTypingThenBackspacingIsARoundTrip` pasan en ambas versiones: usan `KeyEnter`,
así que **no discriminan** el alias de LF.

### Verificación
```
go vet ./...              → limpio
gofmt -l .                → limpio
go test -race -count=1    → 98 tests OK (35 modelo + 63 vista)
```

## Known Limitations
- Sin `Save` a disco: se edita en memoria y no se puede escribir todavía. Es la
  unidad siguiente y la más urgente, porque sin eso el editor pierde el trabajo.
- Sin indicador de "modificado" ni confirmación al salir.
- Sin auto-indentación: Enter inserta un `\n` pelado.
- Sin undo/redo. La estructura de la Piece Table ya lo permite (las piezas viejas
  no se destruyen), pero no hay pila de deshacer.
- Sin selección, sin portapapeles, sin salto de palabra.
- Sin paréntesis/llaves automáticos.
