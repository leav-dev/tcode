# Feature: Pestañas y explorador — U2b: TabBar y navegación

## Description
U2a dejó el `App` con workspace y una vista por buffer, pero sin forma de ver qué
pestañas están abiertas, cambiar entre ellas ni cerrarlas. Esta unidad agrega la
**fila de pestañas** y la navegación por teclado:

- `Ctrl+PageUp` / `Ctrl+PageDown` cambian de pestaña (circular, con wrap).
- `Ctrl+W` cierra la pestaña activa con la **misma confirmación no modal** que la
  salida: la primera vez avisa si hay cambios sin guardar, la segunda cierra.
- La fila de pestañas se desplaza horizontalmente cuando no entran todas; la
  activa queda siempre visible.

Además cancela tres deudas anotadas:

- **(a)** al cerrar un buffer, el controlador borra su entrada de `editors` y su
  permiso en `forceSave`: la vista vieja conservaba un puntero a un `PieceTable`
  ya desmapeado.
- **(b)** introduce la **costura de superficie**: una interfaz mínima
  (`Surface`) que la vista usa para dibujar, más un adaptador (`OffsetSurface`)
  que suma un desplazamiento y recorta contra una región. Quien compone —el
  controlador— elige dónde empieza cada pane; la vista nunca se entera del
  layout. Es lo que permite en U3 que el editor arranque en la columna del
  explorador sin tocar la vista.
- **(c)** los tests que necesitan un tamaño determinista llaman `SetSize`
  **después** de `NewAppWithScreen`: `Init()` reinicia la pantalla simulada a
  80x25, así que un `SetSize` anterior es letra muerta.

Sin menú de pestañas ni sesión JSON: eso es U4. Sin explorador: eso es U3.

## Tasks
- [x] `model.Workspace.BufferAt(i)`: acceso por índice sin copiar el slice; la TabBar itera con `Len()` + `BufferAt` sin alocar por redibujo <!-- id: 0 -->
- [x] `view.Surface`: interfaz mínima (`Put`, `SetContent`, `ShowCursor`, `HideCursor`); `tcell.Screen` la satisface <!-- id: 1 -->
- [x] `view.OffsetSurface`: adaptador con origen `(ox, oy)` y región `(w, h)`; traduce y recorta (y esconde el cursor fuera de la región); `SetRegion` para reusar la instancia <!-- id: 2 -->
- [x] `EditorView.Draw` y `drawCursor` pasan de `tcell.Screen` a `Surface`; los tests de la vista no cambian <!-- id: 3 -->
- [x] `EditorView` ignora `Ctrl+PageUp`/`Ctrl+PageDown` (son del controlador, no scroll de página) <!-- id: 4 -->
- [x] `view.TabBar`: fila de pestañas, activa resaltada, marca `[+]` para sucias, scroll horizontal con `<`/`>` en los bordes, `EnsureActive` <!-- id: 5 -->
- [x] El controlador compone: pestañas en la fila 0, editor en la región desplazada desde la fila 1, barra de estado al final; la superficie del editor se reusa (sin alocar por tecla) <!-- id: 6 -->
- [x] `editorHeight` reserva la fila de pestañas; `redraw` esconde el cursor con el workspace vacío o sin editor <!-- id: 7 -->
- [x] `Ctrl+PageUp`/`Ctrl+PageDown` cambian de pestaña y reencuadran la TabBar; el mouse se traduce restando la fila de pestañas antes de llegar al editor <!-- id: 8 -->
- [x] `Ctrl+W`: confirmación no modal por pestaña (armado/desarmado, cancelado por cualquier otra tecla o cambio de pestaña); al cerrar borra `editors` y `forceSave` del buffer <!-- id: 9 -->
- [x] Tests nuevos del modelo: `BufferAt` en rango, fuera de rango, workspace vacío <!-- id: 10 -->
- [x] Tests nuevos de la vista: `surface_test.go` (traducción, recorte, cursor) y `tab_bar_test.go` (etiquetas, activa, `[+]`, scroll, flechas, `EnsureActive`) <!-- id: 11 -->
- [x] Tests nuevos del controlador: navegación circular, cierre limpio, cierre con confirmación, cancelar la confirmación, cierre de la última pestaña, limpieza de `editors`/`forceSave`, composición (contenido fila 1, cursor fila 1+, pestaña fila 0), mouse traducido <!-- id: 12 -->
- [x] Actualizar los tres tests que dependen de la geometría vieja sin debilitar aserciones: cursor `(3,1)->(3,2)`, `30x4->30x3`, `editorHeight 10->8` <!-- id: 13 -->
- [x] Verificación: `gofmt -l .`, `go vet ./...`, tests de la unidad en verde (véase Evidence) <!-- id: 14 -->
- [x] Commit de unidad de trabajo: `322cf62` <!-- id: 15 -->

## Design decisions

### La composición es dueña del layout; la vista dibuja desde (0,0)
`EditorView.Draw` recibe hoy un `tcell.Screen` y escribe en coordenadas propias.
Con una fila de pestañas arriba, el editor debe dibujar desde la fila 1. La
opción barata y equivocada es pasarle un desplazamiento a cada método de dibujo:
eso mete la geometría de la composición dentro de la vista.

La costura correcta es una interfaz mínima con los pocos métodos que la vista
realmente usa —`Put`, `SetContent`, `ShowCursor`, `HideCursor`— más un adaptador
que suma un origen y recorta contra una región. `tcell.Screen` ya satisface esa
interfaz, así que los tests de la vista no cambian: siguen llamando
`Draw(simScreen)` y dibujando desde (0,0).

La traducción de entrada es parte de la misma costura: el mouse llega en
coordenadas de pantalla y el controlador le resta la fila de pestañas (y en U3
le restará la columna del explorador) antes de pasárselo al editor. El clic
dentro de la pestaña que se ve *en el lugar correcto* es justamente el caso que
esteá compuesto: lo observable —en qué línea cae el cursor— es lo que cambia.

### Una `Surface` reusada, no una por redibujo
`redraw` corre en cada tecla. Alocar la superficie por redibujo (un struct con
origen y región) era el mismo tipo de costo por nada que el mapa por tecla que
ya se corrigió en U2a. El controlador guarda una instancia y le actualiza la
región con `SetRegion(x, y, w, h)` cuando cambia el tamaño de la terminal.

### `BufferAt` en el modelo, no `Buffers()` en el camino caliente
`Buffers()` copia el slice interno a propósito (mutarla no debe afectar al
workspace), pero la TabBar dibuja en cada redibujo: la copia sería una
alocación por tecla. `BufferAt(i)` + `Len()` iteran sin copiar, con el mismo
contrato de solo lectura. La copia de `Buffers()` sigue existiendo para los
llamadores que quieren tomar posesión del slice (p. ej. `saveAs`).

### La confirmación de cierre es no modal y se desarma sola
`Ctrl+W` sobre una pestaña sucia no cierra: avisa en la barra y arma una
confirmación. La segunda vez cierra. Cualquier otra tecla, cambiar de pestaña,
guardar, deshacer, rehacer o Save As la cancela: es el mismo patrón que
`confirmQuit`. La confirmación NO es por buffer: como `Ctrl+W` siempre cierra la
activa y cambiar de pestaña la desarma, nunca puede caer sobre otro buffer.

El cierre consulta al modelo (`Close` → `ErrBufferModified`) y solo usa
`CloseForce` cuando el humano ya confirmó: el modelo sigue siendo la única
fuente de verdad sobre el estado sucio, igual que `Save`/`SaveForce`.

### Cerrar una pestaña borra la vista y el permiso del buffer cerrado
La deuda (a) de U2a: la entrada de `editors` seguía apuntando a un
`PieceTable` liberado, y dibujarla sería leer memoria desmapeada. Al cerrar se
borran `editors[buf]` y `forceSave[buf]`. El permiso de pisar (`forceSave`)
también: autorizó a un archivo que ya no está abierto.

### Desplazamiento de la TabBar: la activa siempre visible
Las pestañas se dibujan de izquierda a derecha mientras entren. Si no entran,
la ventana se corre y las flechas `<` y `>` marcan que hay más a cada lado.
`EnsureActive` (llamado al cambiar, abrir/cerrar y tras resize) corre la ventana
lo mínimo: si la activa entra entera, no mueve nada; si no, muestra su inicio
(truncado con `…` si es más ancha que la fila). Es idempotente.

### Etiquetas y marcadores consistentes con la barra de estado
La pestaña muestra el nombre base de la ruta y `[+]` si el buffer tiene cambios
sin guardar, igual que la barra. La activa va con estilo invertido (como la
barra) y las no activas normales. Si no hay buffer (sin nombre), `(sin nombre)`.

### Tamaño determinista en los tests: `SetSize` después de `NewAppWithScreen`
`NewAppWithScreen` llama `s.Init()`, y la `SimulationScreen` reinicia su tamaño
a **80x25** dentro de `Init` (`simulation.go`). Los tests nuevos que dependen
del ancho (scroll de pestañas) y de la fila de pestañas usan un helper que llama
`SetSize` + dispara `EventResize` **después** de construir la app.

### `Ctrl+PageUp`/`Ctrl+PageDown` no scrollean la página
tcell entrega esas combinaciones como `KeyPgUp`/`KeyPgDn` con `ModCtrl`. El
controlador las intercepta antes de caer en la vista; defensivamente, la vista
también ignora `PgUp`/`PgDn` con modificadores para que ninguna variante de
terminal escrolle por accidente mientras se cambia de pestaña.

## Falsificación (tests que escriben primero contra el código roto)
1. Sin handler de cambio de pestaña → `TestCtrlPageDownSwitchesTabs` falla
   (`ActiveIndex` no cambia).
2. `Ctrl+W` cerrando sin avisar sobre una pestaña sucia → `TestCtrlWAsksBeforeClosingADirtyTab` falla.
3. Cerrar sin borrar la entrada de `editors` (deuda a) → `TestClosingATabRemovesItsEditorAndPermission` falla.
4. Editor dibujando en la fila 0 (sin fila de pestañas ni costura) → `TestRedrawComposesTabsAboveTheEditor` falla (contenido en (0,0)).
5. Mouse sin traducir (la fila de pestañas cuenta como fila de documento) → `TestMouseClickIsTranslatedPastTheTabBar` falla (X cae en otra línea).
6. Sin `Surface`/`OffsetSurface` → los tests de la costura no compilan (no existen aún).

## Evidence
- Rama: `feat/multi-buffer-workspace`.

### U2b — código, tests y verificación
- **Archivos:** `internal/model/workspace.go` (+`BufferAt`), `internal/view/surface.go`
  (nuevo: `Surface` + `OffsetSurface`), `internal/view/tab_bar.go` (nuevo), `internal/view/editor_view.go`
  (`Draw`/`drawCursor` sobre `Surface`, guard `ModCtrl` en `PgUp`/`PgDn`),
  `internal/controller/app.go` (tabBarHeight, composición, navegación, cierre, mouse
  traducido), más los tests: `surface_test.go`, `tab_bar_test.go`, `workspace_test.go` (+3),
  `app_test.go` (+8 y los 3 valores geométricos actualizados).
- **Falsificaciones observadas (RED antes de implementar, por gent-ai-worker):**
  1. Sin handler de cambio de pestaña → `TestCtrlPageDownSwitchesTabs` (ActiveIndex no cambia).
  2. `Ctrl+W` cerrando sin avisar → `TestCtrlWAsksBeforeClosingADirtyTab` (Len queda en 1 sin aviso).
  3. Cierre sin limpiar `editors` (deuda a) → `TestClosingATabRemovesItsEditorAndPermission`.
  4. Editor en la fila 0 → `TestRedrawComposesTabsAboveTheEditor` (contenido en (0,0)).
  5. Mouse sin traducir → `TestMouseClickIsTranslatedPastTheTabBar` (X caía en la línea 2).
  6. Costura inexistente → los tests de `Surface`/`OffsetSurface` no compilaban.
  7. Geometría vieja → los tres tests actualizados fallaban con los valores viejos ((3,1), 30x4, 9).
- **Riesgo de revisión analizado y cerrado con evidencia:** una pestaña intermedia más
  ancha que la fila no puede cortar antes de la activa: si la activa entra según
  `EnsureActive`, el prefijo completo hasta ella entra y toda intermedia (más corta que
  ese prefijo) entra entera. `TestTabBarWideMiddleTabStillDrawsTheActive` (la activa
  visible con la intermedia ancha en la fila) y `TestTabBarSlidesPastAWideMiddleTab`
  (la ventana salta la intermedia cuando la activa no entra con ella) fijan la
  concordancia `EnsureActive`/`Draw` en la frontera.
- **Verificación observada (host Windows):** `go vet ./...` limpio, `go build ./...`
  limpio, `gofmt -l` sin salida para los 9 archivos de la unidad, `go test ./internal/view/`
  verde completo, y los tests nuevos de modelo y controlador en verde.
- **`-race` resuelto tras la instalación del compilador:** el usuario instaló MSYS2
  (`mingw-w64` gcc 15.2.0), pero el `ld` de binutils resultó roto en este host (falla
  incluso ante un error forzado, en silencio y también desde `cmd`). Se resolvió
  instalando `mingw-w64-ucrt-x86_64-clang` + `lld` y corriendo con
  `CGO_ENABLED=1 CC=clang`. Verificación observada: los 30 tests de la unidad en verde
  bajo `-race` (controller 1.92s, model 1.91s, view 1.90s) y el paquete `view` completo
  bajo `-race` en verde.
- **Fallos ambientales preexistentes de Windows, ajenos a la unidad:** los fallos de
  `go test ./...` (con y sin `-race`) son los mismos de siempre —renombres (
  `Acceso denegado` sobre archivos mapeados) y limpieza de TempDir (`mmap` vivo), con
  un subconjunto que varía de corrida en corrida—, verificados idénticos contra el
  HEAD limpio en un worktree aparte; ninguna aserción de esta unidad falla.
- **Cambios revisados tras la delegación:** dos tests frontera de la TabBar agregados
  por el orquestador (riesgo de revisión del worker, ver arriba); su primera variante
  contaba mal el ancho de la etiqueta (21 vs 20) y asumía un `>` derecho inexistente
  cuando la ventana cabe entera sin desborde —corregido en la aserción, no en el código.
- **Commit de unidad de trabajo:** `322cf62`.

### Ajustes posteriores — layout y mouse (reportes del usuario)
- **Layout:** la fila de pestañas dejó de ocupar todo el ancho (se dibujaba sobre
  el árbol) y se renderiza SOLO sobre el área del editor, arrancando en la columna
  del panel; su ancho de encuadre (`tabBarWidth`) es el del editor.
- **Mouse:** clic sobre una pestaña → la activa; rueda sobre la fila → cambia de
  pestaña (arriba = anterior, abajo = siguiente, con wrap), como en un navegador;
  clic en `<`/`>` → corre la ventana del strip. El hit box de cada pestaña se come
  el separador siguiente para no dejar zonas muertas.
- **Aritmética unificada:** `layout()` es la única fuente de la geometría del
  strip (posiciones, flechas, truncado) y la usan `Draw` y `HandleMouse`, para que
  el dibujo y lo que se puede clickear no puedan divergir.
- **Coherencia modal:** con el menú (`Ctrl+T`) o el pedido de Save As abiertos el
  mouse es de ellos y no toca nada por debajo.
- **Tests:** `TestTabBarClickOnATabReturnsItsIndex`,
  `TestTabBarArrowClicksScrollTheStrip`, `TestTabBarWheelSwitchesTabs` (vista) y
  `TestClickOnATabSwitchesToIt`, `TestClickOnATabWithThePanelVisible`,
  `TestWheelOverTheTabBarSwitchesTabs`, `TestSaveAsPromptOwnsTheMouse`
  (controlador).