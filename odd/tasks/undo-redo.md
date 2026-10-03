# Feature: Undo / Redo

## Description
Historial de deshacer y rehacer. La Piece Table ya tenía la propiedad que lo
habilita: editar nunca muta los buffers, solo crea y descarta piezas.

Se agrega `Ctrl+Z` para deshacer, `Ctrl+Y` y `Ctrl+Shift+Z` para rehacer, y el
cursor salta al lugar donde ocurrió el cambio.

## Tasks
- [x] Tipo `Change` y pilas de historial en el modelo <!-- id: 0 -->
- [x] Separar las operaciones crudas (`insertRaw`/`deleteRaw`) de las públicas <!-- id: 1 -->
- [x] `Undo`, `Redo`, `CanUndo`, `CanRedo` <!-- id: 2 -->
- [x] Deducir el marcador de modificado del historial en lugar de un flag aparte <!-- id: 3 -->
- [x] `MoveCursorToOffset` en la vista y cableado en el controlador <!-- id: 4 -->
- [x] Tests: operaciones básicas, LIFO, rama de rehacer, guardado, diferencial <!-- id: 5 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race` <!-- id: 6 -->
- [x] Commit de unidad de trabajo <!-- id: 7 -->

## Design decisions

### Historial de cambios, no instantáneas
Una instantánea del documento tendría que copiar `pieces` y sobre todo
`lineOffsets`, que es **O(cantidad de líneas)**: un archivo de 100k líneas serían
~800 KB por paso. Inviable para una pila.

En cambio se guarda un `Change{Offset, Removed, Inserted}` por edición, así que la
memoria depende de **lo que se editó** y no del tamaño del archivo. La inversa se
aplica con las mismas operaciones crudas que usa la edición, así que el índice de
líneas se mantiene solo.

```
Insert(offset, text)  →  Change{Offset: offset, Inserted: text}
Delete(start, end)    →  Change{Offset: start,  Removed: texto quitado}
```

Deshacer es "quitar lo insertado y reponer lo borrado" en la misma posición. El
orden importa: primero quitar, después reponer.

### Guardar no borra el historial
Si `Save` vaciara las pilas, deshacer después de guardar sería imposible y eso es
exactamente lo que más se extraña. Se conservan las pilas y lo único que cambia es
un `savedAt` que registra `len(undo)` en el momento del guardado. Como los cambios
guardan **texto** y no referencias a piezas, siguen siendo válidos aunque `Save`
haya reconstruido la tabla desde cero.

### El marcador de modificado se deduce del historial
`Modified()` es `len(undo) != savedAt`. Una sola fuente de verdad en lugar de un
flag que hay que acordarse de limpiar en cada camino. Así:

- deshacer un cambio ya guardado vuelve a marcar el documento como sucio;
- rehacer hasta el punto guardado lo vuelve a dejar limpio;
- y no hay forma de que el flag y el historial se desincronicen.

### El orden de la comparación en `record` es sutil y crítico
Al editar después de deshacer se descarta la rama de rehacer. Si el punto de
guardado vivía en esa rama, queda **inalcanzable**. La comparación tiene que ser
contra la longitud **actual**, antes de apilar el cambio nuevo:

```go
if pt.savedAt > len(pt.undo) {   // antes de append
    pt.savedAt = noSavedAt
}
pt.undo = append(pt.undo, c)
```

Comparar después del `append` hace que el caso "guardar, editar, deshacer, editar"
deje el documento marcado como **limpio** sin serlo: la edición nueva repone la
misma longitud que tenía el historial al guardar, pero con otro contenido.

Se falsificó: invirtiendo el orden, `TestBranchingAfterUndoKeepsTheDocumentDirty`
falla con "el documento NO está limpio: el estado guardado quedó en la rama
descartada". Los tests de modificado más obvios pasan en ambas versiones, así que
**no discriminan** este orden.

### Editar y rehacer no son lo mismo que una edición nueva
`insertRaw` y `deleteRaw` no tocan el historial; solo lo hacen las operaciones
públicas. Sin esa separación, deshacer apilaría el propio deshacer.

## Evidence

### Un test que no era del modelo
El test diferencial falló en el primer deshacer con contenido divergente. El bug
**no estaba en el modelo** sino en el test: el generador producía borrados de
longitud cero y el test agregaba un estado de referencia igual.

`Delete(start, start)` es un no-op que **no registra historial**, que es lo
correcto: una edición que no cambia nada no debe ser deshacible. Pero entonces las
pilas quedaban desfasadas: más estados de referencia que entradas de historial, y
el deshacer aplicaba el cambio equivocado. El rango del generador ahora se fuerza
no vacío, con el motivo escrito en el propio test.

### Cobertura del diferencial
`TestUndoRedoMatchesReference` aplica 120 ediciones deterministas guardando el texto
después de cada una, después **deshace todo comparando hacia atrás** y **rehace todo
comparando hacia adelante**. En cada paso verifica contenido, `Len`, `LineCount` y el
invariante de `lineOffsets`. Es el test que atrapa inversas inexactas.

### Verificación
```
go vet ./...              → limpio
gofmt -l .                → limpio
go test -race -count=1    → 150 tests OK (15 controlador + 60 modelo + 75 vista)
```

## Known Limitations
- **Sin agrupación de tipeo:** cada carácter es un paso de deshacer. Escribir una
  palabra y deshacerla requiere un `Ctrl+Z` por letra. Es la mejora más urgente de
  esta unidad. Agrupar tiene reglas finas: cortar en el salto de línea, en el
  movimiento del cursor y al cambiar de línea.
- **Historial sin límite:** crece con lo editado. En una sesión larga conviene un
  tope con descarte del más viejo (y hay que ajustar `savedAt` al descartar).
- **La selección y el portapapeles no existen**, así que deshacer no cubre esos
  caminos porque no hay tales.
- Sin auto-indentación, sin `Save As`, sin detección de cambios externos.
- El índice de líneas sigue siendo O(cantidad de líneas) por edición y `locate`
  O(cantidad de piezas).
