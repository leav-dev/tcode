# Feature: Pestañas y explorador — U3: FileBrowser y modos de arranque

> **Superada por U3b (`odd/tasks/tree-explorer.md`):** el explorador dejó de ser
> una lista plana con descenso y `..` y es ahora un **árbol expandible anclado al
> root de la sesión**, con carga perezosa por nivel. La navegación detallada acá
> (Enter desciende, `..` sube) se complementa con enter→expandir y ←→colapsar.

## Description
U2b dejó la composición lista: pestañas en la fila 0, editor en una región
compuesta con la costura (`OffsetSurface`) y mouse traducido. Esta unidad agrega
el **explorador de archivos** como panel lateral a la izquierda y los **modos de
arranque**:

- `tcode` (sin argumentos) → explorador sobre el directorio actual, sin buffers.
- `tcode <dir>` → explorador sobre ese directorio, sin buffers.
- `tcode <archivo>` → editor con el archivo, explorador oculto (arranque clásico);
  el root de la sesión es el directorio padre del archivo.
- `Ctrl+B` muestra u oculta el panel. Mientras está visible, el editor arranca en
  la columna del panel: es exactamente lo que la costura de U2b habilitó.
- Navegación con `Up`/`Down`/`PageUp`/`PageDown`/`Home`/`End`, `Enter` desciende a
  directorios y abre archivos, `..` sube (bordeado por el root de la sesión).
  `Tab` alterna el foco entre explorador y editor; abrir un archivo devuelve el
  foco al editor. El clic selecciona y enfoca; la rueda scrollea la lista.
- La lista se lee con `os.ReadDir` (el controlador la lee y la pasa a la vista);
  directorios primero, luego archivos, alfabético; los archivos ocultos se ven.

## Tasks
- [x] Al arranque se distingue el argumento: sin argumento → root = cwd, explorador
  visible, sin buffers; directorio → root = ese dir, visible; archivo → root =
  padre, `Open`, oculto <!-- id: 0 -->
- [x] `view.FileBrowser`: entradas (nombre, es dir, ruta), cursor, scroll con la
  activa visible, `SetEntries` con clamp, cursor reseteado, páginas, `Draw(Surface)`
  <!-- id: 1 -->
- [x] Panel con ancho determinista (`panelWidth` = `min(24, max(1, ancho-16))`) y
  región propia en la composición; el editor pasa a región `(panelW, tabBarHeight,
  ancho-panelW, alto editor)` y sus vistas se redimensionan al togglear <!-- id: 2 -->
- [x] `Ctrl+B` toggle show/hide; show enfoca el explorador, hide enfoca el editor
  <!-- id: 3 -->
- [x] Foco explícito: el explorador enfocado consume `Up/Down/PgUp/PgDn/Home/End/Enter`;
  lo no consumido cae al flujo normal (atajos y documento) <!-- id: 4 -->
- [x] `Enter` sobre un directorio desciende (re-lectura); sobre un archivo lo abre
  (`ws.Open`, dedupe incluida) y enfoca el editor; `..` sube hasta el root de la
  sesión <!-- id: 5 -->
- [x] Mouse: clic/rueda con x dentro del panel van al explorador (y traducido por la
  fila de pestañas); x fuera van al editor traducido por `(panelW, tabBarHeight)`;
  el clic en el panel lo enfoca <!-- id: 6 -->
- [x] Tests de la vista: dibujo (dirs con "/"), cursor resaltado, scroll con la
  activa visible, clamp al cambiar la lista, páginas <!-- id: 7 -->
- [x] Tests del controlador: modos de arranque (los tres), toggle con `Ctrl+B`,
  `Enter` abre/desciende/sube, foco, traducción del mouse y geometría del editor
  desplazada por el panel, arranque sin argumentos con workspace vacío sin romper
  nada <!-- id: 8 -->
- [x] Verificación: `go vet`, `gofmt -l` en las superficies, tests de la unidad y
  paquete `view` bajo `-race` con clang <!-- id: 9 -->
- [x] Commit de unidad de trabajo: `2355846` <!-- id: 10 -->

## Design decisions

### El explorador es un panel de una sola columna navegable, no un árbol
El roadmap dice "árbol", pero un árbol recursivo exige cargar subdirectorios por
nodo y recortar contra el alto disponible del panel. Un listado del directorio
actual con `Enter` para descender, `..` para subir y scroll cumple la misma
función —navegar y abrir archivos— con un viewport virtual simple (el mismo
patrón que el editor). La entrada sintética `..` aparece solo cuando el
directorio actual no es el root de la sesión: el panel no escapa de su límite.

### Quién lee el directorio: el controlador, no el modelo
El modelo es PieceTable y texto; `os.ReadDir` es E/S de archivos, no estado de
buffer. El controlador (el cerebro que une Model y View) lee la lista y la
deposita en la vista con `SetEntries`. La vista solo dibuja y mueve el cursor;
abrir un archivo es decisión de `ws.Open` (dedupe por ruta normalizada incluida).
El `Workspace` no cambia: `SetRoot` ya existe y lo usa el arranque.

### El panel se compone con la misma costura que el editor
El explorador dibuja sobre una `Surface` en coordenadas propias —igual que el
editor— y el controlador lo compone en la región `(0, tabBarHeight, panelW, h)`.
Sus tests llaman `Draw(simScreen)` directo, como los del editor: la superficie se
reusa en el redibujo (se pinta el panel, se reencuadra con `SetRegion` y se pinta
el editor), sin alocar por tecla.

### El toggle redimensiona las vistas del editor
Mostrar u ocultar el panel cambia su ancho: las vistas ya creadas reciben el
mismo tratamiento que un resize (`ed.Resize(anchoNuevo, editorHeight)`) para
todas, y el `TabBar`/`StatusBar` siguen a todo el ancho (solo el editor se
desplaza).

### Foco explícito, no panes en paralelo
Dos panes no pueden ser dueños del mismo `Up`/`Down` sin un foco. `Ctrl+B` (al
mostrar) y el clic enfocan el explorador; `Tab` SOLO lo devuelve al editor
—nunca al revés: con el foco en el editor, `Tab` sigue insertando tabulación en
el documento, porque la indentación es una tecla básica de la edición y el
panel no puede robársela—. Abrir un archivo devuelve el foco al editor; volver
al panel es con clic o re-mostrando con `Ctrl+B`. El explorador enfocado
consume solo lo que entiende (movimiento, `Enter`); cualquier otra tecla cae al
flujo normal (atajos, documento, salida). Navegar el panel es "seguir
trabajando": desarma las confirmaciones pendientes y el permiso de pisar, como
cualquier otra tecla del documento. El teclado del panel funciona también con
el workspace vacío: la rama del explorador va ANTES del guard `buf == nil` en
`handleEvent`, igual que `Ctrl+B` —ocultar el panel para ganar ancho no puede
exigir abrir primero un archivo—.

### Traducción del mouse: un origen, dos destinatarios
El mouse llega en coordenadas de pantalla. Si el panel está visible y `x < panelW`,
va al explorador (solo se traduce `y - tabBarHeight`); si no, al editor
(`x - panelW`, `y - tabBarHeight`). Es la misma composición dueña del layout de
U2b, con un eje más.

### Modos de arranque y defaults
`tcode <archivo>` arranca con el explorador oculto: abrir un archivo por la línea
de comandos es una acción de edición, no de navegación. Desde `Ctrl+B` se puede
mostrar igual. Sinclair argumento (o directorio) el explorador es el camino de
entrada natural —el estado a propósito vacío que la feature declaró en U1— y el
foco ya está en él para navegar de inmediato.

### Ancho del panel determinista
`panelWidth(ancho) = min(explorerWidth, max(1, ancho-16))` con
`explorerWidth = 24`: el editor conserva al menos 16 columnas y el valor es
determinista para los tests (en 80 → 24, en 30 → 16, en 20 → 4).

## Falsificación (tests que escriben primero contra el código roto)
1. Sin handler de `Ctrl+B` → `TestCtrlBTogglesTheExplorer` (la visibilidad no cambia).
2. Editor ignorando el panel (región x=0 aunque esté visible) → `TestEditorStartsAtThePanelColumn` (contenido en (0,1) en vez de (24,1)).
3. Sin modos de arranque → `TestStartupWithDirectoryArgument` (`ErrIsDirectory`) y `TestStartupWithoutArguments` (root = cwd, visible, sin buffer).
4. Mouse sin traducir por el panel → `TestMouseClickInTheEditorIsTranslatedPastThePanel` (la X cae en otra línea/columna).
5. `Enter` sin abrir → `TestEnterOpensTheSelectedFile` (el buffer no existe).
6. Sin foco → `TestTabCyclesFocus` / `TestClickInThePanelFocusesIt`.
7. FileBrowser sin scroll ni clamp → tests de la vista.

## Evidence
- Rama: `feat/multi-buffer-workspace`.

### U3 — código, tests y verificación
- **Archivos:** `internal/view/file_browser.go` (nuevo: `Entry`, `FileBrowser`,
  `SetEntries` con clamp, scroll mínimo, páginas, mouse, `Draw(Surface)`),
  `internal/view/status_bar.go` (solo la firma de `writeString` → `Surface`, aditivo),
  `internal/controller/app.go` (modos de arranque, `explorerWidth`/`panelWidth`,
  `Ctrl+B`, foco, listado en el controlador, mouse traducido por `(panelW,
  tabBarHeight)`), más los tests: `file_browser_test.go` (9), `app_test.go` (12 + B1).
- **Interrupciones del worker resueltas por el orquestador:** (A1) `writeString` pasa
  a `Surface` en vez de duplicar el helper; (B1) `TestSaveAsForADocumentWithoutPath`
  crea su buffer sin ruta con `ws.NewUntitled()` (el arranque `""` ya no crea
  buffer, como manda U3); (C1) la fórmula literal `min(24, max(1, ancho-16))`, el
  ejemplo "30→16" del borrador era una errata (es 30→14, el editor conserva 16).
- **Tres defectos encontrados en la revisión del orquestador y corregidos (con test de
  falsificación cada uno):** (1) `Ctrl+B` estaba dentro del switch, inalcanzable con
  el workspace vacío —el arranque por directorio es el camino de entrada de U3—;
  ya va antes del guard `buf == nil` (`TestCtrlBTogglesWithAnEmptyWorkspace`).
  (2) Las teclas consumidas por el panel no desarmaban las confirmaciones; ahora
  navegar el panel es "seguir trabajando" y las desarma como cualquier tecla del
  documento (`TestExplorerNavigationCancelsThePendingConfirmations`). (3) El `Tab`
  simétrico del primer contrato le robaba al documento su inserción de tabulación
  con el panel visible; quedó unidireccional: `Tab` solo devuelve el foco del
  explorador al editor, y con el foco en el editor inserta (`TestTabInsertsInTheDocumentWhileTheExplorerIsVisible`).
- **Falsificaciones observadas por el worker (RED):** los símbolos del panel no
  existían (build failed con `undefined: Entry/NewFileBrowser/explorerVisible/panelWidth`),
  y la geometría vieja se observó con los tests nuevos tras la implementación.
- **Verificación observada (host Windows):** `go vet ./...` limpio, `gofmt -l` sin
  salida en las 5 superficies, paquete `view` completo bajo
  `CGO_ENABLED=1 CC=clang go test -race` en verde (1.90s), y los 21+ tests de la
  unidad en verde. El controller completo solo falla los ambientales preexistentes
  de Windows (renombres sobre mmap y smoke, `Acceso denegado`), idénticos a la
  línea base.
- **Decisión de diseño ajustada en la implementación (revisada, no desvío):** el
  listado del root se lee SIEMPRE al arrancar —también con el panel oculto— para
  que mostrarlo con `Ctrl+B` aparezca poblado; el panel oculto no dibuja, así que
  ninguna geometría existente cambia.
- **Commit de unidad de trabajo:** `2355846`.