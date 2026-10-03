# Feature: Pestañas y explorador — U2a: cablear el Workspace

## Description
U1 entregó un `Workspace` multi-buffer, pero nadie lo usa: `App` sigue teniendo un
`*model.PieceTable` suelto. Esta unidad es el **refactor habilitante**: `App` pasa a
sostener un `*model.Workspace` y el estado que hoy es único (confirmación de salida,
permiso de pisar, pedido de Save As) pasa a tener semántica por buffer.

No agrega UI: no hay TabBar, no hay explorador, no hay teclas nuevas. Deja el terreno
listo para que U2b (TabBar) y U3 (explorador) sean cambios chicos. Es el "hacé fácil
el cambio, después hacé el cambio fácil".

## Tasks
- [x] `Workspace.NewUntitled()` en el modelo: buffer vacío sin ruta <!-- id: 0 -->
- [x] `App` sostiene `*model.Workspace` en lugar de `*model.PieceTable` <!-- id: 1 -->
- [x] Un `EditorView` por buffer, creado bajo demanda y guardado en un mapa <!-- id: 2 -->
- [x] `activeEditor()` resuelve la vista del buffer activo <!-- id: 3 -->
- [x] `EventResize` redimensiona **todas** las vistas, no solo la activa <!-- id: 4 -->
- [x] `confirmQuit` consulta `ws.AnyModified()` <!-- id: 5 -->
- [x] `forceSave` pasa a ser por buffer, sin borrarse al cambiar de pestaña <!-- id: 6 -->
- [x] Save As captura el buffer destino cuando se abre el pedido <!-- id: 7 -->
- [x] `Run` libera todo con `ws.CloseAll()`; `NewAppWithScreen` carga la ruta en el workspace <!-- id: 8 -->
- [x] `redraw` con workspace vacío no entra en pánico <!-- id: 9 -->
- [x] Actualizar los tests existentes del controlador sin debilitar ninguna aserción <!-- id: 10 -->
- [x] Tests nuevos: guardar la pestaña activa, salir con cualquier buffer sucio, Save As al buffer capturado <!-- id: 11 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race ./...` <!-- id: 12 -->
- [x] Commit de unidad de trabajo <!-- id: 13 -->
- [ ] `editors` pierde la entrada del buffer al cerrarlo (tarea de U2b) <!-- id: 14 -->

## Design decisions

### El buffer es la unidad de estado, no la aplicación
`forceSave` hoy es un `bool` de `App`: "el próximo Ctrl+S pisa cambios externos". Con
pestañas eso es una autorización sobre **un** archivo, y llevarla puesta al cambiar de
pestaña haría que un permiso dado para un archivo autorice pisar otro sin que nadie lo
decidiera. Pasa a ser un conjunto de buffers autorizados, y cambiar de pestaña no lo
limpia: lo mantiene por buffer, que es donde tiene sentido.

### Una vista por buffer, en un mapa y creada bajo demanda
Cambiar de pestaña no puede perder cursor, viewport ni scroll: eso es justamente lo que
la pestaña representa. Por eso cada buffer tiene su propio `EditorView` y cambiar de
pestaña es cambiar de puntero.

Se guardan en un mapa con la clave puesta en el puntero del buffer, no en un slice
paralelo: el workspace ya deduplica por ruta y el mapa no se puede desincronizar del
workspace como sí podría hacerlo un slice mantenido a mano. La creación es bajo demanda
—la constitución pide no inicializar lo que no se usa— y la entrada se borra cuando el
buffer se cierra, en U2b.

### Redimensionar tiene que alcanzar a todas las vistas
Una terminal que cambia de tamaño cambia el viewport de **todos** los buffers abiertos.
Si sólo se redimensiona la activa, al volver a otra pestaña la vista queda con el alto
viejo y dibuja mal hasta el próximo resize. Es el tipo de bug que sólo aparece cuando
ya hay varias pestañas y cuesta atribuir.

### Save As se ata al buffer cuando el pedido se abre
El pedido de Save As vive en la barra de estado, que es compartida. Si el destino se
resolviera al apretar Enter, lo que se guarda dependería de qué pestaña esté activa
cuando la persona termina de tipear la ruta. Se captura al abrir el pedido: la ruta que
alguien escribió pertenece al documento que estaba mirando cuando empezó a escribirla.

### Un workspace vacío es un estado válido, también para el controlador
Hoy `App` siempre tiene un buffer, así que `redraw` puede dibujar sin preguntar. Con
pestañas, cerrar la última deja `Active() == nil`. El controlador tiene que sostener ese
estado sin entrar en pánico: el guard va en esta unidad aunque el cierre de pestañas
llegue en U2b.

### `NewUntitled` pertenece al modelo
El arranque sin argumentos tiene que seguir funcionando: hoy da un documento sin ruta,
que es exactamente para lo que existe Save As. Un buffer sin ruta es una cosa legítima
del modelo, no una excepción del controlador, así que la fábrica va en `Workspace`.

### `editors` tiene que olvidar los buffers cerrados
El mapa de vistas vive más que el buffer: al cerrar una pestaña, el workspace libera el
`mmap` y el descriptor, pero la entrada del mapa seguiría apuntando a un `PieceTable`
ya liberado. Hoy no se puede llegar ahí porque **no hay forma de cerrar una pestaña
hasta U2b**, y por eso esta unidad no borra entradas. Es una deuda con vencimiento
corto y con consecuencia real: la vista vieja conserva un puntero a memoria desmapeada,
y dibujarla sería leer memoria liberada. Queda anotado como tarea de U2b, no como
detalle opcional.

### Trampa de test: `SetSize` antes de construir la App no sirve
`NewAppWithScreen` llama `s.Init()`, y la `SimulationScreen` de tcell reinicia su
tamaño a **80x25** dentro de `Init` (`simulation.go`: `s.back.Resize(80, 25)`). Por eso
el patrón de los helpers —`s.Init()`, `s.SetSize(40, 10)`, `NewAppWithScreen`— corre en
realidad a 80x25: ese `SetSize` es letra muerta.

Lo delató el mensaje de fallo de `TestResizeUpdatesEveryEditor` al falsificarlo:
"quedó con 80x24" (25 de alto menos la barra de estado). Ninguna aserción de esta
unidad depende del tamaño inicial —todas fijan el tamaño después, con `SetSize` +
`EventResize`—, así que no se toca acá. Pero U2b necesita un ancho determinista para
verificar dónde caen las pestañas, y ahí hay que poner el `SetSize` **después** de
construir la App.

## Evidence
- Rama: `feat/multi-buffer-workspace`.

### U2a — código, tests y verificación
- **Archivos:** `internal/controller/app.go` (reescrito), `internal/controller/app_test.go`,
  `internal/controller/smoke_test.go`, `internal/model/workspace.go` (+`NewUntitled`),
  `internal/model/workspace_test.go`, `internal/view/editor_view.go` (+`Size()`).
- **Verificación observada:** `go test ./... -race -count=1` verde (controller 1.955s,
  model 1.136s, view 1.082s), `go vet ./...` limpio, `go build ./...` limpio,
  `gofmt -l .` sin salida.
- **Falsificación (4 experimentos, todos fallan contra el código roto):**
  1. resize solo de la vista activa → `TestResizeUpdatesEveryEditor`
     (`la vista de …/a.txt quedó con 80x24, se esperaba 30x4`).
  2. Save As resuelto al apretar Enter en vez de al abrir el pedido →
     `TestSaveAsTargetsTheBufferCapturedAtPromptOpen` (`destino = "dos"`).
  3. `forceSave` global en vez de por buffer → `TestForceSavePermissionDoesNotCrossBuffers`.
  4. `save()` escribiendo todos los buffers → `TestCtrlSSavesOnlyTheActiveBuffer`
     (`Ctrl+S escribió el buffer 1, que no era el activo: "Ydos"`).
- **Una línea fuera de superficie, aceptada con criterio:** `EditorView.Size()` (7 líneas,
  aditiva, sin cambio de comportamiento). El worker la agregó sin autorización porque el
  test de resize necesita ver la geometría desde el paquete `controller`. Se conserva:
  la alternativa sería una aserción frágil sobre el dibujo, y los tests de la vista
  (10 llamadas a `Draw`) no cambian gracias a que `tcell.Screen` sigue siendo el
  parámetro. Queda como cambio revisado, no como accidente.
- **Verificación independiente (`gentle-ai-verify`, read-only):** PASS en los diez puntos;
  confirmó que **ninguna aserción existente se debilitó** (las 32 líneas eliminadas son
  migraciones de acceso o el chequeo de `forceSave`), que el permiso por buffer no puede
  cruzar de pestaña por construcción, y que la entrada huérfana de `editors` es
  inalcanzable hoy. Señaló dos tests débiles y un hueco que se cerraron en esta unidad:
  `TestCtrlSSavesOnlyTheActiveBuffer` no podía detectar un guardado de más (el buffer 1
  nunca se ensuciaba), `TestEmptyWorkspaceDoesNotPanic` no ejercía los atajos con
  punteros nulos, y faltaba el test de que el permiso de pisar no cruza de buffer.
- **Detalle de performance corregido:** `clearForceSave` reusa el mapa con `clear()` en
  lugar de asignar uno nuevo. Ese camino corre en cada tecla que no sea un atajo, así que
  asignar un mapa por tecla era un costo por nada.
- **Commit de unidad de trabajo:** `8433084`.

### Ajuste posterior — el aviso de la confirmación siempre se ve (reporte del usuario)
- **Síntoma reportado:** «no se valida que presione Escape dos veces para
  cerrar»: el editor parecía salir con una sola pulsación.
- **Causa:** la validación de la doble pulsación existía y funcionaba, pero
  `StatusBar.Draw` solo dibujaba el mensaje si entraba SIN pisar la etiqueta.
  En terminales angostas —o con nombres largos— el aviso de «Cambios sin
  guardar» nunca se dibujaba: el primer Escape no producía ninguna señal
  visible y el segundo cerraba, así que la confirmación se percibía inexistente.
- **Fix:** la barra de estado le da prioridad al MENSAJE: si no entra a la
  derecha de la etiqueta, se recorta la etiqueta (y el mensaje con `…` si
  tampoco entra en la fila) en lugar de descartarlo. Un aviso invisible es un
  aviso perdido.
- **Test que dejaba pasar el bug:** `TestEscapeWarnsWhenAnyBufferIsDirty`
  aseveraba `statusBar.Label() != ""` —la etiqueta, que nunca está vacía— en
  vez del mensaje. Ahora asevera que «Cambios sin guardar» esté DIBUJADO en la
  fila, y `TestTheQuitWarningSurvivesANarrowTerminal` lo exige con 30 columnas
  (recortado, nunca ausente).
- **Verificación:** paquete `view` completo y bajo `-race` con clang en verde,
  tests de la confirmación de salida en verde, `go vet`/`gofmt` limpios.
