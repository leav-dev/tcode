# Feature: Cursor y Movimiento

## Description
Introducir un cursor de edición en `EditorView` y todo lo que necesita para
comportarse como el de un editor real: movimiento por *grapheme cluster*,
movimiento vertical con columna deseada, y desplazamiento automático del viewport
para que el cursor nunca quede fuera de pantalla.

Es el paso previo a conectar el teclado con `Insert`/`Delete`: sin cursor no hay
dónde insertar.

## Design decisions

### Coordenadas del cursor: (línea, byte dentro de línea)
No se usa offset de documento crudo porque el movimiento horizontal tiene que ser
consciente de los *grapheme clusters* y de los caracteres anchos. Con
`(línea, byteCol)` el movimiento horizontal avanza un cluster por vez y la
conversión a offset de documento es `LineStart(line) + byteCol`.

### Columna deseada (sticky column)
Al moverse verticalmente, el cursor no debe "comerse" columnas cuando pasa por
líneas más cortas. Se guarda la columna de pantalla deseada y se recalcula el byte
en cada línea destino. Es el comportamiento que la gente espera de un editor.

### El viewport sigue al cursor
`ensureCursorVisible` corre después de todo movimiento o edición y ajusta
`TopLine` y `LeftColumn`. El scroll manual con flechas se elimina: ahora las
flechas mueven el cursor y el viewport lo acompaña.

### Se eliminan las teclas de scroll estilo vim
`j`, `k`, `h`, `l`, `g` y `G` dejan de scrollear. Un editor no modal tiene que
insertar esas letras como texto: `tcode` apunta a funcionar como un editor de GUI
dentro de la terminal. La navegación queda en flechas, `PgUp`/`PgDn`,
`Home`/`End`, `Ctrl+Home`/`Ctrl+End` y la rueda del mouse.

## Tasks
- [x] Accesores de línea en el modelo: `LineStart`, `LineContent`, `LineBreakLen`, `LineAt` <!-- id: 0 -->
- [x] Helpers de cluster en la vista: `columnAt`, `offsetAtColumn`, `nextCluster`, `prevCluster` <!-- id: 1 -->
- [x] `Cursor{Line, ByteCol, desiredCol}` e integración en `EditorView` <!-- id: 2 -->
- [x] Movimiento: horizontal por cluster, vertical con columna deseada, Home/End, PgUp/PgDn, Ctrl+Home/End <!-- id: 3 -->
- [x] `ensureCursorVisible` para mantener el cursor dentro del viewport <!-- id: 4 -->
- [x] Dibujar el cursor con `ShowCursor` en la celda correcta <!-- id: 5 -->
- [x] Hit testing del mouse: el clic mueve el cursor respetando el ancho real <!-- id: 6 -->
- [x] Tests: movimiento, clusters anchos, columna deseada, seguimiento del viewport, clic <!-- id: 7 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race` <!-- id: 8 -->
- [x] Commit de unidad de trabajo <!-- id: 9 -->

## Evidence

### Modelo — accesores de línea
`LineStart`, `LineContent`, `LineBreakLen` y `LineAt`, todos resueltos sobre el
índice de líneas en coordenadas de documento, no sobre el archivo original. Un
helper interno `lineSpan` evita duplicar el cálculo del rango de línea.
`LineContent` descuenta el salto completo, así que `\r\n` no queda como contenido.
7 tests nuevos, incluido `TestLineAccessorsSurviveEdits`, que comprueba que los
accesores sigan al índice después de insertar y borrar.

### Vista — el cursor
`Cursor{Line, ByteCol, desiredCol}` en coordenadas `(línea, byte dentro de línea)`.
No se usa offset de documento crudo porque el movimiento horizontal tiene que ser
consciente de los clusters y del ancho.

- **Columna deseada:** al moverse verticalmente el cursor no se "come" columnas
  por pasar por líneas cortas. `TestVerticalMovementKeepsDesiredColumn` lo cubre:
  baja de una línea de 8 a una de 2 (queda en 2) y al volver a una larga recupera
  la columna 6.
- **Comparación en columnas de pantalla, no en bytes:**
  `TestVerticalMovementUsesDisplayColumnsForWideCharacters` verifica que desde
  `"日b"` (3 columnas, 4 bytes) bajar a `"xyz"` deje el cursor en la columna 3.
- **El viewport sigue al cursor** con el mínimo desplazamiento; el scroll con rueda
  **no** arrastra el cursor (`TestMouseWheelScrollDoesNotDragTheCursor`).
- **Se eliminaron las teclas de scroll estilo vim** (`j`, `k`, `h`, `l`, `g`, `G`).
  Un editor no modal tiene que insertar esas letras como texto. La navegación queda
  en flechas, `PgUp`/`PgDn`, `Home`/`End`, `Ctrl+Home`/`Ctrl+End` y la rueda.
  Esto rompió a propósito 5 tests viejos de scroll, que se reescribieron a la
  semántica nueva.

### Bug encontrado al escribir los tests del mouse
`offsetAtColumn` comparaba `at >= col` **antes** de sumar el ancho del cluster
actual, así que un clic en la mitad de un carácter ancho caía **después** del
carácter en lugar de anclar en su inicio.

Con `"日ab"`, un clic en la columna 1 (mitad de `日`) devolvía el offset 3 (el
carácter siguiente) en vez del 0.

**Fix:** comparar `col < at+width` para detectar que la columna cae dentro del
cluster y devolver su inicio.

**Falsificación:** reintroducir la condición vieja hace fallar
`TestMouseClickOnWideCharacterSnapsToItsStart` con `ByteCol = 3, se esperaba 0`.
Los otros dos tests de mouse pasan en ambas versiones: **no discriminan este
defecto**, solo lo fija el primero.

### Verificación
```
go vet ./...              → limpio
gofmt -l .                → limpio
go test -race -count=1    → 76 tests OK (35 modelo + 41 vista)
```

## Known Limitations
- Sin selección de texto todavía.
- Sin salto de palabra (`Ctrl+flechas`).
- Sin edición: esta unidad agrega el cursor y el movimiento; insertar y borrar es
  la unidad siguiente. Por eso una tecla de texto sin manejar devuelve `false`.
- El clic no inicia selección por arrastre (`drag`).
