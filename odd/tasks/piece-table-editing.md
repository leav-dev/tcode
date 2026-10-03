# Feature: Edición en la Piece Table

## Description
Implementar las primitivas de edición de la Piece Table: `Insert` y `Delete`.
Es lo que convierte a `tcode` de un visor en un editor.

## El problema de fondo

Hasta ahora el documento era un único tramo del `mmap`: `GetRange` indexaba
**directo** en `originalBuffer` usando offsets de línea. Con ediciones eso se rompe
porque el documento pasa a ser una mezcla de dos buffers.

Hay que separar dos coordenadas que antes coincidían:

- **Offset de documento:** posición lógica del byte dentro del texto actual.
- **Offset de buffer:** posición del byte dentro de `originalBuffer` o `newBuffer`.

La resolución pasa a ser un recorrido de piezas, y el índice de líneas tiene que
vivir en coordenadas de documento.

## Tasks
- [x] Reestructurar `PieceTable` con `docLen` y piezas resueltas por recorrido <!-- id: 0 -->
- [x] Implementar `Insert(offset, text)` partiendo piezas cuando el offset cae adentro <!-- id: 1 -->
- [x] Implementar `Delete(start, end)` recortando y descartando piezas <!-- id: 2 -->
- [x] Mantener el índice de líneas de forma incremental en ambas operaciones <!-- id: 3 -->
- [x] Reescribir `GetRange` en coordenadas de documento, con camino rápido cero-copia <!-- id: 4 -->
- [x] Tests: insert/delete en inicio, medio y fin; bordes; varias piezas; multi-línea <!-- id: 5 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race` <!-- id: 6 -->
- [ ] Commit de unidad de trabajo — pendiente de autorización del usuario <!-- id: 7 -->

## Evidence

### Implementación
- `PieceTable` ahora mantiene `docLen` y resuelve offsets de documento recorriendo
  las piezas (`locate`, `slice`). Se separaron explícitamente las dos coordenadas
  que antes coincidían: offset de documento y offset de buffer.
- `Insert`: `newBuffer` es **append-only**, así que una pieza es inmutable una vez
  creada. Si el offset cae adentro de una pieza, se parte en dos y la nueva va en
  el medio (la tabla crece en 2 y se corre la cola con `copy`).
- `Delete`: recorre las piezas una sola vez y conserva los tramos que sobreviven;
  puede recortar una pieza por la izquierda, por la derecha o por ambos lados.
- `GetRange` quedó en coordenadas de documento, con **camino rápido cero-copia**
  cuando no hay ediciones (una sola pieza del `mmap`). El contrato de "a veces
  vista, a veces copia" quedó documentado en el método.
- `GetContent` se reimplementó sobre `slice(0, docLen)`.

### El bug que encontró el test diferencial
`TestEditsMatchReferenceString` (400 iteraciones contra un `string` de referencia,
comparando contenido, `Len`, `LineCount`, `lineOffsets` y `GetRange` línea por
línea) falló en la **iteración 91**:

```
iter 91: LineCount() = 6, se esperaba 5 (contenido "\n\nYb¥\n\n日b")
```

**Causa:** al borrar `[start, end)`, un inicio de línea que caía justo en `end` se
mapeaba a `start`. Eso solo es válido si `start` ya era un inicio de línea. Mi
dedupe tapaba el duplicado pero no el caso inválido.

Reproducción mínima: `"a\n\nb"` con arranques `[0,2,3]`, borrando `[1,2)`. `start=1`
**no** es inicio de línea, así que el arranque en 2 debía descartarse; el código
producía `[0,1,2]`, con un offset 1 no precedido por `'\n'`.

**Fix:** la condición pasó de `l >= end` a `l > end`. Un inicio en `end` se
descarta **siempre**: o duplica a `start` (que ya se conservó), o es inválido. La
corrección además elimina la necesidad del dedupe, porque los conservados quedan
`<= start` y los corridos `> start`, sin colisión posible.

### Falsificación
Se reintrodujo la condición defectuosa (`l >= end`) y se corrió la suite:
**5 tests fallan**, incluido el diferencial en la iteración 91 exacta.

| Test | Síntoma con la condición defectuosa |
|---|---|
| `TestDeleteMiddleNewlineKeepsLineStartsValid` | `lineOffsets = [0 1 2]`, se esperaba `[0 2]` |
| `TestDeleteCollapsingLineStartDoesNotDuplicate` | `lineOffsets = [0 2 2]` — duplicado |
| `TestInsertThenDeleteRoundTrips` | `lineOffsets = [0 4 4 8]` — duplicado |
| `TestGetRangeAfterEdits` | `GetRange(1,2) = ""` — índice corrido |
| `TestEditsMatchReferenceString` | iter 91: `LineCount() = 6`, se esperaba 5 |

### Dos errores míos en los tests
- `TestDeleteInMiddleTrimmsPiece` esperaba `Delete(4,6)` → `"holamundo"`, pero eso
  borra `" m"`. El correcto es `Delete(4,5)`.
- `TestGetRangeAfterEdits` borraba `Delete(9,12)`, que quita `"dos"`, no `"uno"`.
  El rango correcto para quitar `"uno\n"` es `Delete(5,9)`.

### Verificación
```
go vet ./...              → limpio
gofmt -l .                → limpio
go test -race -count=1    → 50 tests OK (28 modelo + 22 vista)
```

El test `TestEditingDoesNotMutateTheMappedFile` comprueba además que las ediciones
nunca escriben el archivo mapeado: solo crean piezas sobre `newBuffer`.

## Known Limitations
- El índice de líneas es **O(cantidad de líneas)** por edición por el corrimiento.
  Aceptable para archivos chicos y medianos; para archivos enormes conviene un árbol
  de Fenwick o un buffer de huecos sobre `lineOffsets`.
- `locate` recorre las piezas linealmente: **O(cantidad de piezas)** por edición.
  Correcto primero; un árbol balanceado de piezas es la optimización siguiente.
- Sin undo/redo todavía. La estructura ya lo permite (las piezas viejas no se
  destruyen cuando se inserta), pero no hay pila de deshacer.
- Sin guardar a disco: no hay `Save`. Editar y escribir es la próxima unidad.
