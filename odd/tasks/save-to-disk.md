# Feature: Guardar a Disco

## Description
Implementar `Save`. Es la deuda más urgente del proyecto: hasta acá el editor
editaba en memoria y todo el trabajo se perdía al salir.

Incluye lo mínimo para que guardar sea usable y seguro: marca de documento
modificado, barra de estado con el nombre del archivo, `Ctrl+S`, y confirmación
antes de salir con cambios sin guardar.

## Tasks
- [x] `Save` atómico: archivo temporal en el mismo directorio y renombre <!-- id: 0 -->
- [x] Preservar los permisos del archivo original <!-- id: 1 -->
- [x] Escribir sobre el destino de un enlace simbólico, no sobre el enlace <!-- id: 2 -->
- [x] Recargar el documento después de guardar (el `mmap` apunta al inodo viejo) <!-- id: 3 -->
- [x] Marca de modificado en el modelo, limpiada al guardar <!-- id: 4 -->
- [x] Barra de estado: archivo, marca de modificado y mensajes <!-- id: 5 -->
- [x] `Ctrl+S` en el controlador con mensaje de resultado <!-- id: 6 -->
- [x] Confirmación antes de salir con cambios sin guardar <!-- id: 7 -->
- [x] Tests de atomicidad, permisos, enlaces, barra de estado y controlador <!-- id: 8 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race` <!-- id: 9 -->
- [x] Commit de unidad de trabajo <!-- id: 10 -->

## Design decisions

### Guardado atómico, nunca en el lugar
Se escribe a un temporal en el **mismo directorio** y recién entonces se renombra
sobre el original. Un renombre dentro del mismo sistema de archivos es atómico, así
que en disco siempre queda el contenido viejo o el nuevo, nunca uno a medias.

Escribir en el lugar parecía más simple y es **directamente inviable**, no solo
inseguro. Ver la sección de falsificación: produce un `SIGBUS`.

### Recargar después de guardar
El `mmap` y el descriptor siguen apuntando al inodo viejo, que el renombre dejó
reemplazado. Tras guardar hay que desmapear, reabrir y remapear, y el documento
queda otra vez en una sola pieza. `openAndMap` es la operación que comparten la
carga inicial y esta recarga.

Si el archivo ya está en disco pero falla la recarga, se devuelve un error
envuelto que lo aclara: el trabajo está a salvo, lo que falló es la reapertura.

### Enlaces simbólicos
Renombrar encima de un enlace lo reemplazaría por un archivo común y el enlace se
perdería. La ruta se resuelve con `filepath.EvalSymlinks` y el renombre va sobre el
destino. `TestSaveFollowsSymlinks` lo cubre.

### Permisos
`os.CreateTemp` crea con `0600`. Sin restaurar los permisos del original, guardar
un archivo `0644` lo dejaría privado. Se guarda el modo al cargar y se reaplica con
`Chmod` antes del renombre.

### Confirmación de salida sin modal
Salir con cambios sin guardar avisa en vez de salir: la primera `Escape` solo
muestra el mensaje, la segunda sale. Cualquier otra tecla cancela la confirmación
pendiente, así una `Escape` posterior vuelve a avisar en lugar de cerrar por
accidente. Evita tener que parsear un `y`/`n` modal.

### La barra de estado
Es una `Screen` más. Ocupa la última fila, que el editor descuenta de su alto
(`editorHeight`). Muestra el nombre base, `[+]` si hay cambios sin guardar, y el
mensaje alineado a la derecha **solo si entra sin pisar la etiqueta**.

## Evidence

### La falsificación que define el diseño
Se reemplazó `Save` por la versión ingenua —`os.OpenFile(O_WRONLY|O_TRUNC)` y
escribir ahí— y se corrió la suite. El resultado no fue corrupción de datos: fue
un **corte del proceso**.

```
unexpected fault address 0x7458c3029004
fatal error: fault
[signal SIGBUS: bus error code=0x2 addr=0x7458c3029004]
runtime.memmove()
bufio.(*Writer).Write(...)
tcode/internal/model.(*PieceTable).writeContent(...)
tcode/internal/model.(*PieceTable).Save(...)
    → TestSaveWritesEditsToDisk
```

`O_TRUNC` invalida las páginas del `mmap` **antes** de que las leamos, así que
`writeContent` lee de memoria desmapeada y el proceso muere. No es un happy path que
falla con elegancia: es un editor que se cae al guardar.

Y lo detectó el test **más básico** (`TestSaveWritesEditsToDisk`), no el de
permisos. La interacción con el `mmap` es tan fundamental que la implementación
ingenua no logra guardar ni un documento.

Nota honesta: `TestSaveFollowsSymlinks` **no** discrimina este diseño, porque
escribir en el lugar sigue el enlace y lo preserva. Ese test cubre el manejo del
enlace, no la atomicidad.

### La prueba de seguridad
`TestSaveLeavesTheOriginalIntactOnFailure` deja el directorio sin permiso de
escritura y comprueba que `Save` falle **y** que el archivo original quede intacto,
el documento en memoria no se pierda y el estado modificado siga puesto. Se saltea
si corre como root, porque ahí los permisos no aplican.

### Verificación
```
go vet ./...              → limpio
gofmt -l .                → limpio
go test -race -count=1    → 127 tests OK (9 controlador + 46 modelo + 72 vista)
```

`NewAppWithScreen` permite inyectar una pantalla, así que el controlador tiene
tests reales (`Ctrl+S` guarda, `Escape` confirma, cualquier tecla cancela la
confirmación, un guardado fallido no limpia el estado modificado).

## Known Limitations
- **Sin `Save As`**: solo se guarda sobre la ruta del archivo abierto. Un documento
  sin archivo devuelve `ErrNoPath`.
- **Sin detección de cambios externos**: si otro proceso modifica el archivo,
  guardar lo pisa sin avisar.
- **Sin recuperación ante corte**: si el proceso muere entre el `Flush` y el
  `Rename`, queda un temporal `.tcode-*.tmp` en el directorio.
- **Sin undo/redo**: la Piece Table ya lo permite (las piezas viejas no se
  destruyen), pero no hay pila de deshacer. Deshacer después de guardar sería lo
  natural para el siguiente paso.
- Sin auto-indentación, sin selección, sin portapapeles, sin salto de palabra.
- El índice de líneas sigue siendo O(cantidad de líneas) por edición y `locate`
  O(cantidad de piezas).
