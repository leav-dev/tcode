# Feature: Pestañas y explorador de archivos

## Description
Hoy `App` tiene **un** `*model.PieceTable` y **un** `*view.EditorView`, y `main.go`
toma `os.Args[1]`. El editor es single-buffer: no se puede abrir una carpeta, ni
seleccionar archivos, ni tener varios documentos abiertos a la vez.

Esta feature agrega cuatro cosas sobre la **Screen Architecture** que la
constitución ya declara (§2.B): explorador de archivos, pestañas dentro de la
misma terminal, menú de pestañas y sesión persistida en JSON. Se entrega en cuatro
unidades de trabajo encadenadas, para no inflar el review.

## Tasks

### U1 — Workspace multi-buffer (modelo puro, sin UI)
- [x] `Workspace` con root, buffers en orden de apertura y buffer activo <!-- id: 0 -->
- [x] `Open` deduplica por ruta normalizada y devuelve el buffer existente <!-- id: 1 -->
- [x] `Close` rechaza buffers sucios con `ErrBufferModified`; `CloseForce` los cierra <!-- id: 2 -->
- [x] `Active`, `SetActive`, `Next`, `Prev` circulares, `Len`, `AnyModified` <!-- id: 3 -->
- [x] `Open` rechaza directorios (`ErrIsDirectory`) y rutas vacías <!-- id: 4 -->
- [x] `Close` libera el mmap y el descriptor del buffer cerrado <!-- id: 5 -->
- [x] Tests: dedupe, orden de apertura, navegación circular, workspace vacío, cierre sucio <!-- id: 6 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race ./...` <!-- id: 7 -->
- [x] Commit de unidad de trabajo <!-- id: 8 -->

### U2 — Pestañas
Se parte en dos para que el review no mezcle refactor con feature.

- **U2a — Cablear el Workspace (`odd/tasks/workspace-wiring.md`).** `App` sostiene un
  `*model.Workspace`, un `EditorView` por buffer, y el estado hoy único
  (`confirmQuit`, `forceSave`, pedido de Save As) pasa a tener semántica por buffer.
  Sin UI nueva: es el refactor habilitante.
- **U2b — TabBar y navegación (`odd/tasks/tab-bar.md`).** Fila de pestañas,
  `Ctrl+PageUp`/`Ctrl+PageDown` para cambiar, `Ctrl+W` para cerrar con la misma
  confirmación no modal que la salida. Debe además: (a) borrar la entrada de
  `editors` del buffer que se cierra, porque la vista vieja conserva un puntero a un
  `PieceTable` ya liberado; (b) introducir la costura de superficie con desplazamiento;
  (c) arreglar el tamaño determinista de la pantalla en los tests (`SetSize`
  **después** de `NewAppWithScreen`). En la evidencia: `322cf62`.

### U3 — FileBrowser y modos de arranque (view + controller) — pendiente
### U4 — Menú de pestañas (`Ctrl+T`) y sesión JSON — pendiente

## Roadmap de decisión

### El explorador es un panel lateral fijo, con `Ctrl+B`
Decisión de quien usa el editor. El árbol ocupa una franja a la izquierda y el editor
el resto; `Ctrl+B` lo muestra u oculta para ganar ancho. Implica que el editor deja de
dibujar en la columna 0, y por eso U2b introduce la costura que lo hace sin ensuciar la
vista.

### La vista dibuja sobre una superficie con desplazamiento, no sobre la pantalla
`EditorView.Draw` recibe hoy un `tcell.Screen` y escribe en coordenadas propias desde
(0,0). Con un panel a la izquierda el editor tiene que empezar en la columna del panel.
La opción barata y equivocada es pasarle un desplazamiento a cada método de dibujo: eso
mete geometría de la composición dentro de la vista y obliga a tocar los cinco archivos
de test de la vista.

La costura correcta es una interfaz mínima con los pocos métodos que la vista realmente
usa (`Put`, `SetContent`, `ShowCursor`, `HideCursor`) más un adaptador que suma un
desplazamiento y recorta contra la región. `tcell.Screen` ya satisface esa interfaz, así
que **los tests de la vista no cambian**: siguen llamando `Draw(simScreen)` y dibujando
desde (0,0), porque el desplazamiento es responsabilidad de quien compone, no de quien
dibuja.

## Design decisions

### Un `PieceTable` por pestaña, `mmap` vivo solo mientras la pestaña existe
La constitución pide eficiencia extrema: *si algo no está en pantalla, no debe
estar cargado de forma pesada*. Un `PieceTable` abierto no contradice eso: el
`mmap` es una vista del page cache, no una copia, y el `newBuffer` solo crece con
lo editado. Lo que sí lo contradice es **recargar el buffer al cambiar de
pestaña**: eso perdería undo/redo, cursor y viewport, que es justamente el estado
que la pestaña representa.

Por eso: un `PieceTable` por archivo abierto, y `Close` desmapea y cierra el
descriptor. Cambiar de pestaña es cambiar un índice, no mapear nada.

### El cierre de un buffer sucio se rechaza en el modelo y se confirma en el controlador
El modelo no puede decidir perder trabajo. `Close` devuelve `ErrBufferModified` si
el buffer tiene cambios sin guardar, y `CloseForce` es la puerta que el
controlador abre recién después de que la persona confirmó. Es el mismo par
`Save`/`SaveForce` que ya existe, y la misma razón: la decisión de perder cambios
es de quien usa el editor.

### Dedupe por ruta normalizada
Abrir el mismo archivo por dos rutas distintas no puede producir dos buffers con
dos `mmap` del mismo inodo: dos historiales de undo sobre el mismo archivo, y el
que guarde segundo pisa al otro. `Open` normaliza con `filepath.Abs`, y además
`filepath.EvalSymlinks` cuando el archivo existe —un enlace y su destino son el
mismo archivo, y el guardado ya resuelve enlaces—. Si `EvalSymlinks` falla (ruta
inexistente), queda la ruta absoluta limpia.

### El `Workspace` no sabe de terminal
Root, buffers y activo son estado lógico: el modelo sigue aislado de `tcell`, como
pide la constitución §2.A. El cursor y el viewport **no** viven acá: son estado de
render y pertenecen a la vista. El workspace solo dice *qué documentos existen y
cuál está activo*.

### Un workspace vacío es un estado válido
Sin archivos abiertos no hay activo: `Active()` devuelve `nil` y la vista dibuja un
pane vacío. El arranque sin argumentos cae en el explorador, así que este estado es
el normal en el camino de entrada, no un caso raro.

### Cerrar una pestaña nunca cambia de archivo por sorpresa
El índice activo tiene que seguir nombrando **el mismo documento** después de
cerrar cualquier otra pestaña. La primera versión de esta unidad fijaba
`activeAt =` el índice cerrado, y eso rompía de dos maneras: cerrar una pestaña
anterior a la activa saltaba a otro archivo, y cerrar una posterior dejaba el
índice **fuera de rango**, con `Active()` devolviendo `nil`. El invariante correcto
es: cerrar antes de la activa corre el índice una posición a la izquierda; cerrar
después no lo mueve; cerrar la activa pasa al frente la que ocupó su lugar.

**Lección de test:** un caso de cierre con la activa a **una sola** posición de la
cerrada no distingue el invariante correcto del bug —con 3 pestañas los dos
comportamientos coinciden por casualidad—. Hace falta un hueco de dos posiciones
(`TestWorkspaceCloseWithGapBeforeActive`, cuatro pestañas) para que el test sea un
detector de verdad.

### El descriptor se verifica contra el sistema operativo, no contra el campo
`release()` limpia `pt.file` y `pt.originalBuffer`. Un test que solo lee esos campos
pasa igual si alguien borra el `file.Close()` real. `openFDsFor` cuenta los
descriptores que apuntan al archivo leyendo `/proc/self/fd` y comparando por
identidad (`os.SameFile`), así que el test distingue "el campo quedó en nil" de "el
sistema ya no tiene el archivo abierto".

## Evidence
- Rama: `feat/multi-buffer-workspace`.

### U1 — código, tests y verificación
- **Archivos:** `internal/model/workspace.go` (nuevo, 222 líneas),
  `internal/model/workspace_test.go` (nuevo, 23 tests, 667 líneas).
- **Verificación observada:** `go test ./... -race` verde en los tres paquetes
  (`controller` 1.734s, `model` 1.117s, `view` 1.078s), `go vet ./...` limpio,
  `go build ./...` limpio, `gofmt -l .` sin salida.
- **Falsificación:** con `closeImpl` revertido al bug y `release()` sin cerrar el
  descriptor, fallan cuatro tests: `TestWorkspaceCloseBeforeActiveKeepsSameActiveBuffer`,
  `TestWorkspaceCloseAfterActiveKeepsActive`, `TestWorkspaceCloseWithGapBeforeActive`
  y `TestWorkspaceCloseReleasesResources` (`descriptores abiertos para a.txt tras
  CloseForce = 1, se esperaba 0`). Restaurado y verde.
- **Verificación independiente (`gentle-ai-verify`, read-only):** PASS en aditividad
  (solo los dos archivos nuevos), traza del invariante del índice activo en todas
  las ramas, camino de liberación en `Open` fallido, y dedupe. Señaló dos huecos que
  se cerraron en esta misma unidad: los casos de la pestaña del medio y la
  observación real del descriptor.
- **Pendiente conocido:** si `SaveAs` mueve un buffer a una ruta ya abierta en otra
  pestaña, el dedupe deja de verlos como el mismo archivo. Es decisión de U2/U4
  (dueño de la TabBar), no de U1.
- **Commit de unidad de trabajo:** `646a4c3`.
