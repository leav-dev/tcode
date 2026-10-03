# Feature: Save As

## Description
Un documento sin ruta no tenía forma de llegar al disco: `Save` devolvía
`ErrNoPath`. `SaveAs` guarda en una ruta elegida y pasa a trabajar sobre ella.

Commit: `dd141c7`.

## Tasks
- [x] `SaveAs` en el modelo: escribir, cambiar la ruta, respetar permisos <!-- id: 0 -->
- [x] Escribir siempre, aunque no haya cambios <!-- id: 1 -->
- [x] Separar el volcado y la recarga de `save` para poder reusarlos <!-- id: 2 -->
- [x] Modo de pedido de texto en la barra de estado <!-- id: 3 -->
- [x] `Ctrl+Shift+S` para abrir el pedido, prellenado con la ruta actual <!-- id: 4 -->
- [x] Enter acepta, Escape cancela sin escribir <!-- id: 5 -->
- [x] Tests del modelo y del controlador <!-- id: 6 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race` <!-- id: 7 -->
- [x] Commit de unidad de trabajo <!-- id: 8 -->

## Design decisions

### Siempre escribe, aunque el documento esté limpio
Elegir una ruta es una decisión explícita. Un `Save` normal se saltea la escritura
cuando no hay cambios; `SaveAs` no puede hacerlo, porque entonces sería imposible
copiar un archivo sin editarlo. Por eso el volcado y la recarga se separaron en
`writeAndReload`, que no tiene ni el atajo de "no hay nada que guardar" ni la
verificación de cambios externos —esa marca pertenecía a la ruta anterior.

### Copia, no mueve
El archivo original queda intacto. Es la semántica que la gente espera de Save As y
además es la más segura: si alguien se equivoca de ruta, no perdió el archivo.

### Permisos: del destino si existe, 0644 si es nuevo
`os.CreateTemp` crea con `0600`. Copiar un archivo nuevo con esos permisos lo dejaría
privado y el usuario no entendería por qué. Si el destino ya existe, se respetan sus
permisos.

### El pedido vive en la barra de estado, y le roba el teclado al documento
Mientras el pedido está abierto, las teclas alimentan el pedido y no el documento.
Sin esa separación, tipear una ruta editaría el archivo que se está guardando. Es la
razón por la que el modo existe como estado propio del controlador y no como una
consulta al vuelo.

El pedido se prellena con la **ruta actual** para poder editarla en lugar de
reescribirla entera, que es el uso habitual.

### `Ctrl+Shift+S`
tcell entrega los control con Shift como `KeyRune` con `ModCtrl` y `ModShift`, no
como el código `KeyCtrl*`. Es la misma particularidad que ya tenía `Ctrl+Shift+Z`, y
por eso el reconocimiento va por runa y no por código de tecla.

## Evidence

### Once tests en el modelo
Cubren: escritura al destino, cambio de ruta de trabajo, escritura sin cambios,
documento sin ruta, ruta vacía, destino existente, permisos del destino, permisos de
archivo nuevo, limpieza del aviso de cambio externo, e historial usable después.

`TestSaveAsKeepsTheHistoryUsable` es el que cruza las dos unidades anteriores:
después de Save As se tiene que poder deshacer, y deshacer no debe tocar el archivo.

### Siete tests en el controlador
Uno de punta a punta cubre el caso que motivó la unidad: **abrir el editor sin
archivo, escribir y guardar en una ruta nueva**. Los demás verifican el pedido:
prellenado, que las teclas no editen el documento, aceptar, cancelar con Escape y no
hacer nada con ruta vacía.

### Verificación
```
go vet ./...              → limpio
gofmt -l .                → limpio
go test -race -count=1    → 192 tests OK (27 controlador + 90 modelo + 75 vista)
```

## Known Limitations
- **El pedido no tiene cursor visible ni historial de tipeo.** Se puede escribir y
  borrar, pero no moverse dentro de la línea ni aceptar sugerencias.
- **El borrado del pedido va por runa**, no por *grapheme cluster*. Alcanza para
  rutas, que en la práctica son ASCII, pero es una inconsistencia con el editor.
- **No hay completado de rutas** ni validación de que el directorio exista antes de
  intentar escribir: un directorio inexistente se reporta como error del `SaveAs`.
- **Sin confirmación al pisar un destino existente.** El usuario nombró la ruta, así
  que se asume la intención, pero un archivo con contenido se pierde sin aviso.
