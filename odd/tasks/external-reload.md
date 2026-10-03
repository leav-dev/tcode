# Feature: Recarga ante cambios externos

## Description
tcode ya detecta (`ChangedOnDisk`) y protege (`ErrFileChangedExternally` /
`SaveForce`): si otro proceso toca el archivo abierto, el guardado lo rechaza
hasta que el usuario decide pisar. Lo que quedó pendiente en
`external-change-detection.md` es la otra mitad: **recargar** el buffer desde
disco para quedarse con la versión nueva en lugar de seguir editando una basada
en la vieja.

Decisión de producto (usuario): **auto si limpio + Ctrl+R**.

- Si el archivo cambió en disco y el buffer **está limpio** (sin ediciones sin
  guardar), al detectar la actividad del usuario tcode recarga solo, sin
  preguntar, y lo anuncia con un mensaje transitorio.
- Si el buffer **tiene ediciones**, no recarga: sigue valiendo el aviso de
  `Ctrl+S` (en lugar de pisar los cambios ajenos). `Ctrl+R` recarga
  explícitamente, con la misma confirmación no modal del proyecto: la primera
  vez avisa que se pierden las ediciones, la segunda recarga; cualquier otra
  tecla desarma.

Riesgos que el diseño contempla:

- **SIGBUS:** un archivo encogido por fuera puede quedar más corto que el mapeo
  viejo; leer las páginas sobrantes levanta SIGBUS (lección de
  `save-to-disk.md`). `Reload` por eso **desmapea antes de leer nada nuevo**
  (`release()` primero, `openAndMap()` después): nunca se toca el contenido
  viejo.
- **Cursor colgado:** si el documento nuevo es más corto, el cursor de la vista
  puede quedar fuera de rango y `LineContent` paniquearía en el próximo dibujo.
  Se agrega `EditorView.ClampCursor()` (clamp + `ensureCursorVisible`) llamado
  tras cada recarga.
- **Costo del chequeo:** la detección automática hace un `stat` por buffer en
  cada evento de teclado/mouse. Barato en FS local; se documenta la limitación
  para montajes de red.

Sin deduplicación de Save As ni tope de historial: son las otras dos deudas,
fuera de esta unidad. Sin indicador visual persistente de "cambió en disco": el
aviso transitorio y el rechazo del guardado bastan en esta unidad.

## Tasks
- [ ] `model.PieceTable.Reload()`: `release()` → `openAndMap()` → historial
  vacío (`undo`/`redo` nil, `savedAt` 0); errores controlados (`ErrNoPath`,
  re-mapeo fallido) <!-- id: 0 -->
- [ ] Tests de modelo: Reload reemplaza contenido y descarta ediciones; undo
  vacío; archivo encogido recarga sin SIGBUS; `ChangedOnDisk` pasa false tras
  recargar; error con path vacío <!-- id: 1 -->
- [ ] `view.EditorView.ClampCursor()`: cursor y viewport dentro del documento
  tras recargar (doc vacío incluido) <!-- id: 2 -->
- [ ] Controller `Ctrl+R`: recarga el buffer activo; primera vez avisa si hay
  ediciones, segunda confirma; cualquier otra tecla desarma el armado <!-- id: 3 -->
- [ ] Controller detección automática: chequea los buffers abiertos en cada
  evento; el limpio recarga solo (mensaje "Cambios externos recargados"), el
  sucio no <!-- id: 4 -->
- [ ] Tests del controlador: recarga manual con confirmación, recarga
  automática de buffer limpio, buffer sucio que no recarga solo, cursor clamp
  tras recargar; ajustes de los tests existentes que afirmaban la semántica
  vieja <!-- id: 5 -->
- [ ] Verificación final: build/vet/gofmt y `go test ./...` sin fallos nuevos
  vs base <!-- id: 6 -->

## Design decisions

### `Reload` desmapea antes de leer
El archivo viejo ya no es confiable: el cambio externo pudo dejarlo más corto
que el mapeo, y leer las páginas sobrantes es SIGBUS (lección registrada en
save-to-disk). `Reload` llama `release()` (desmapea + cierra) y recién después
`openAndMap()` (mapea el estado actual del disco y reconstruye el índice de
líneas). El contenido viejo no se lee jamás. Si `openAndMap` falla (archivo
borrado), el buffer queda vacío y el error se reporta: recargar un archivo que
ya no existe no tiene alternativa.

### Auto si limpio — la detección es parte del bucle
La recarga automática solo es segura cuando no hay nada que perder. El chequeo
corre en cada evento de la actividad del usuario (teclado y mouse) contra todos
los buffers abiertos: `ChangedOnDisk()` es un `stat` (lo que `Save` ya paga) y
el buffer limpio se recarga sin preguntar. El buffer sucio no se toca: ahí
sigue mandando el aviso de `Ctrl+S` (pisar con `SaveForce` o recargar con
`Ctrl+R` son las dos decisiones explícitas). La cadencia por evento es la
definición más directa de "al detectar"; un montaje de red puede degradarse y
queda anotado como limitación.

### `Ctrl+R` con la confirmación no modal del proyecto
Recargar descarta las ediciones sin guardar, así que la primera `Ctrl+R` sobre
un buffer sucio solo avisa ("recargar descarta los cambios: Ctrl+R de nuevo
recarga") y la segunda ejecuta —el mismo patrón de dos toques que `Escape` y
`Ctrl+W`, sin modalidad y sin parsear y/n—. Cualquier otra tecla desarma el
armado. Sobre un buffer limpio recarga directo.

### La vista clampa, no reconstruye
Descartar la vista (`editors`) tras recargar sería lo más simple pero tira el
view scroll y el cursor: recargar un archivo grande en el medio volaría al
inicio. `ClampCursor()` conserva la posición si sigue existiendo (recorta la
línea al final y la columna al final de la línea, viewport incluido) y solo cae
a cero con un documento vacío.

## Falsificación (tests que escriben primero contra el código roto)
1. Sin `Reload` → `TestReloadReplacesContentAndDiscardsEdits` no compila
   (método inexistente) / no existe.
2. Recarga leyendo el mapeo viejo tras el cambio → el test de archivo encogido
   sería SIGBUS; `release` primero lo evita (verifiable por contenido correcto).
3. Sin `ClampCursor` → `TestReloadClampsCursor` no compila.
4. Sin `Ctrl+R` → `TestCtrlRReloadsWithConfirmation` falla (la tecla cae al
   documento / no hace nada).
5. Sin chequeo automático → `TestExternalChangeAutoReloadsCleanBuffer` falla
   (el contenido sigue siendo el viejo tras tipear).
6. Recarga automática sobre buffer sucio → `TestDirtyBufferDoesNotAutoReload`
   fallaría si el chequeo recargara igual.

## Evidence
(Rellenar por unidad: falsificaciones RED, verificación, commits.)