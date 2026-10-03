# Feature: Tope del historial de deshacer

## Description
Deuda anotada en la unidad de undo/redo: "el historial no tiene tope". Los
`Change` guardan los strings removidos/insertados; en una sesión larga (o con
borrados grandes) el historial crece sin límite, contra la constitución de RAM
mínima. Esta unidad pone un tope.

Decisión de diseño:

- **Tope SOLO en el historial de deshacer** (`undo`), con
  `maxUndoHistory = 1000` (mismo default que VSCode). Al apilar, si el
  historial excede, se descarta el cambio **más viejo** (el del frente: los
  descartados ya no se pueden deshacer, el documento conserva su efecto).
- **El redo NO se topa** y no hace falta: es una pila de aplicación LIFO donde
  truncar por un borde rompe el orden de restauración (reharía cambios fuera
  de secuencia sobre un documento corrupto), y además se autorregula —solo
  crece deshaciendo, y con el undo topeado a `max` el redo tampoco puede pasar
  de ese orden—.
- **El punto de guardado se ajusta con el descarte**: `savedAt` es un índice en
  `undo`; al dropear del frente, si el guardado era el más viejo (índice 0) deja
  de ser alcanzable (`noSavedAt`), si no, corre uno. `Modified()` sigue siendo
  `len(undo) != savedAt`, sin flags extra.
- El corte por el frente NO toca el grupo de tipeo (el merge solo toca el
  último elemento) ni la rama de redo (se descarta al editar, como siempre).

Qué NO cambia: semántica de undo/redo dentro del historial vivo, el grupo de
tipeo, la marca de modificado, el guardado. La única diferencia observable es
que deshacer no puede volver más atrás de los últimos 1000 pasos.

## Tasks
- [ ] `model`: `var maxUndoHistory` (mutable para tests) + `pushUndo` con el
  corte por el frente y el ajuste de `savedAt`; reemplazar los dos appends de
  `undo` (record y Redo) <!-- id: 0 -->
- [ ] Tests de modelo: el tope descarta los más viejos (documento tras deshacer
  todo conserva el efecto de los descartados); `savedAt` se ajusta hasta
  `noSavedAt` (Modified vuelve a true al perder el punto de guardado); redo
  dentro del tope sigue funcionando <!-- id: 1 -->
- [ ] Verificación: `go vet`, `gofmt`, `go test ./internal/model/` y suite
  completa sin fallos nuevos vs base <!-- id: 2 -->

## Design decisions

### Solo undo se topa; el redo se autorregula
El redo es una pila LIFO de aplicación: rehacer exige aplicar exactamente la
secuencia inversa de los deshechos. Truncarlo por cualquier borde deja
restaurables cambios que ya no se pueden aplicar en orden sobre el estado
actual (documento corrupto). Por eso no se topa; su tamaño ya está acotado por
el undo: solo crece deshaciendo, y el undo no puede darme más de `max` pasos.
Si el undo se corta por el frente mientras hay redo, el redo igual queda
acotado por los últimos `max` deshechos.

### `savedAt` viaja con el descarte
`savedAt` es un índice sobre `undo` (o `noSavedAt`). Al dropear del frente el
historial entero corre uno: si el guardado era el índice 0 se pierde para
siempre y pasa a `noSavedAt` (el documento queda legítimamente modificado); si
no, se decrementa. No hay estado nuevo: solo la aritmética correcta del índice
que ya existía.

## Falsificación (tests que escriben primero contra el código roto)
1. Sin tope → `TestUndoHistoryIsCapped` falla (el historial crece sin límite;
   con `max` chico el documento tras deshacer todo NO puede haber vuelto al
   inicio — los cambios viejos ya no están).
2. Descarte sin ajustar `savedAt` → `TestUndoCapLosesTheSavedPoint` falla
   (`Modified()` no vuelve a true cuando el punto de guardado cae del frente).
3. Tope en el redo (si alguien lo intentara) → `TestRedoWithinCapStillWorks`
   no alcanza para cazarlo; la decisión documentada es no topar el redo.

## Evidence
(Rellenar por unidad.)