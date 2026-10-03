# Feature: Pestañas y explorador — U4: Menú de pestañas y sesión JSON

## Description
Última unidad de la feature. Dos piezas:

1. **Menú de pestañas (`Ctrl+T`)**: un superpuesto transitorio que lista las
   pestañas abiertas con cursor navegable; `Enter` cambia a la elegida, `Escape`
   cierra sin cambiar, cualquier otra tecla cierra y descarta. Mientras el menú
   está abierto, el teclado y el mouse son del menú (como el pedido de Save As).
2. **Sesión persistida en JSON**: al salir, el editor guarda el root de la
   sesión, las pestañas abiertas (rutas, en orden) y la activa en
   `<root>/.tcode/session.json`; al arrancar sobre un directorio (o sin
   argumentos) restaura esa sesión. El modo archivo explícito no guarda ni
   restaura: abrir un archivo por línea de comandos es una acción puntual de
   edición, no una sesión.

El archivo queda gitignored (`.tcode/`) y lo que no se persiste —estado de
undo/redo, cursor, viewport, foco, visibilidad del panel— es estado de sesión en
vivo, no de archivo.

## Tasks
- [x] `model.Session` (+ `LoadSession`/`SaveSession`): JSON con `version`, `root`,
  `tabs` y `active` (la activa como RUTA, no como índice: sobrevive a pestañas
  que ya no existen) <!-- id: 0 -->
- [x] `view.TabMenu`: lista transitoria con cursor y scroll mínimo; `Enter`
  activa, `Escape` cierra; `Draw(Surface)` con la fila del cursor resaltada y
  etiquetas reusando `tabLabel` <!-- id: 1 -->
- [x] `Ctrl+T` abre/cierra el menú; el menú posee el teclado y el mouse mientras
  está activo (guarda después del prompt, antes del explorador) <!-- id: 2 -->
- [x] `Enter` del menú cambia a la pestaña elegida y deja la composición
  consistente: TabBar reencuadrada, barra de estado, confirmaciones desarmadas
  <!-- id: 3 -->
- [x] `sessionPath(root)` = `<root>/.tcode/session.json`; `loadSession` en el
  arranque de modos directorio/sin-argumento (nunca en modo archivo), no fatal
  (pestañas que ya no existen se saltan y la activa se resuelve por ruta) <!-- id: 4 -->
- [x] `saveSession` al salir (`Run`), solo en sesión habilitada (dir/sin-arg); sin
  pestañas persistibles, se borra el archivo (estado limpio) <!-- id: 5 -->
- [x] Tests del modelo: roundtrip exacto, archivo inexistente, JSON inválido,
  creación de directorios <!-- id: 6 -->
- [x] Tests del controlador: `Ctrl+T` abre, `Enter` cambia, `Escape` cierra sin
  cambiar, otra tecla cierra; sesión: salir con `Run` guarda, arranque restaura
  (orden y activa), modo archivo ignora la sesión, pestaña muerta se salta,
  workspace vacío no rompe nada <!-- id: 7 -->
- [x] Aislar `TestStartupWithoutArguments` del estado del repo (chdir a un
  directorio temporal y restaurar): el repo es un destino legítimo de pruebas y
  puede tener `.tcode/session.json` <!-- id: 8 -->
- [x] Verificación: `go vet`, `gofmt -l` en las superficies, paquete `view` y
  tests de la unidad bajo `-race` con clang <!-- id: 9 -->
- [ ] Commit de unidad de trabajo <!-- id: 10 -->

## Design decisions

### La activa de la sesión se guarda como ruta, no como índice
Una sesión puede referir pestañas que ya no existen (archivo borrado). Si la
activa fuera un índice, una pestaña muerta ANTES de ella la correría de lugar.
Guardarla como ruta permite resolver la activa DESPUÉS de restaurar: se abren
todas las que existen y se busca la de esa ruta (normalizada, como las guarda
`Path()`), con la dedupe de `Open` haciendo el resto.

### El menú es una lista de una columna, como el explorador
Una paleta de pestañas "estilo fuzzy" exigiría índice de búsqueda y resaltado de
coincidencias. El menú de esta unidad es la versión mínima que cumple la función:
ver TODAS las pestañas de un vistazo y saltar a cualquiera con cursor + `Enter`.
Reusa la mecánica del explorador (cursor, scroll mínimo, clamp) y las etiquetas
de la TabBar (`tabLabel`: nombre base + `[+]` si está sucia, `(sin nombre)` sin
ruta). Mientras está abierto es dueño del teclado Y del mouse —como el pedido de
Save As—, así que el documento no recibe nada por accidente.

### El menú se dibuja sobre la región del editor
El menú pertenece al área de trabajo, no al panel lateral: se compone sobre la
región del editor (después de todo, es un overlay) y no borra la fila de
pestañas ni la barra de estado. Reusa la `editorSurf` con `SetRegion` como los
otros panes.

### El modo archivo no tiene sesión
`tcode <archivo>` abre un documento puntual; guardar `.tcode/` en el directorio
padre de cualquier archivo que se abra por línea de comandos sería ensuciar
directorios ajenos. La sesión existe para los modos explorador (dir/sin-arg):
ahí el root es un lugar de trabajo y `.tcode/session.json` es su memoria. El
flag `sessionEnabled` se define en el arranque y cubre guardado y restauración.

### Guardar al salir, no en cada cambio
Escribir la sesión en cada open/close/switch sería churn de disco en el camino
caliente. Se guarda en el `defer` de `Run` (último defer, corre antes de
`CloseAll`): la sesión refleja el último estado consistente al salir. Un crash
pierde —como cualquier editor— los cambios de sesión no guardados; documentado.

### Sin pestañas persistibles, la sesión se borra
Guardar `{tabs: []}` dejaría un archivo mentiroso. Si no hay ninguna pestaña con
ruta (todo sin nombre o cerró todo), `saveSession` elimina el archivo: el estado
por defecto del directorio vuelve a ser "sin sesión".

### La restauración nunca es fatal
Una pestaña de la sesión que ya no existe se salta (`ws.Open` falla → seguir).
Un JSON corrupto se descarta. El arranque no puede morir por una sesión vieja:
el estado por defecto (explorador, sin buffers) es siempre el respaldo.

## Falsificación (tests que escriben primero contra el código roto)
1. Sin handler de `Ctrl+T` → `TestCtrlTOpensTheTabMenu` falla (el menú no abre).
2. `Enter` sin cambiar → `TestTabMenuEnterSwitchesToTheSelectedTab` (la activa no cambia).
3. `Escape` sin descartar → `TestTabMenuEscapeDismissesWithoutSwitching`.
4. Sin guardado → `TestRunSavesTheSession` (salir con `Run` + tecla inyectada no deja archivo).
5. Sin restauración → `TestStartupRestoresTheSession` (arranque sobre dir con sesión no abre las pestañas).
6. Modo archivo restaurando igual → `TestFileStartupIgnoresTheSession`.
7. Pestaña muerta abortando → `TestSessionLoadSkipsMissingTabs`.

## Evidence
- Rama: `feat/multi-buffer-workspace`.

### U4 — código, tests y verificación
- **Archivos:** `internal/model/session.go` (nuevo: `Session`, `LoadSession`,
  `SaveSession`), `internal/view/tab_menu.go` (nuevo), `internal/controller/app.go`
  (menú, sesión, `sessionEnabled`, defer en `Run`), más los tests: `session_test.go`
  (5), `tab_menu_test.go` (9), `app_test.go` (10 + aislamiento de
  `TestStartupWithoutArguments`).
- **Interrupción del worker resuelta por el orquestador:** el contrato era
  auto-contradictorio en el cierre del menú (`Escape → (true,false)` chocaba con el
  guard que solo cierra con `!handled` y con el test que exige que Escape cierre).
  Se resolvió con la opción consistente con el precedente: `Escape`/`Ctrl+C` NO los
  maneja el menú (`(false,false)`, igual que `FileBrowser`) y el guard los cierra por
  la rama `!handled` —"cualquier tecla ajena cierra y descarta" cubre a Escape— sin
  ramas extra. Autorizado además el campo interno `count` capturado en `Open(ws)`.
- **Falsificaciones observadas (RED):** los símbolos no existían (build failed con
  `undefined: Session/TabMenu/app.menu`), y los 10 tests del controlador fijaron el
  comportamiento (apertura, cambio, descarte, sesión guardada/restaurada/ignorada,
  pestaña muerta, borrado del archivo).
- **Verificación observada (host Windows):** `go vet ./...` limpio, `gofmt -l` sin
  salida en las 6 superficies, paquete `view` completo bajo
  `CGO_ENABLED=1 CC=clang go test -race` en verde (2.14s), los 24 tests de la unidad
  en verde (model 5, view 9, controller 10). El controller completo solo falla los
  ambientales preexistentes de Windows (renombres sobre mmap y smoke), verificados
  idénticos contra el HEAD limpio.
- **Decisión del orquestador:** `.gitignore` gana `.tcode/` (el archivo de sesión
  vive en `/.tcode/session.json` del root y el repo es el destino de pruebas).
- **Pendiente conocido (de U1, sigue fuera de scope):** un Save As a una ruta ya
  abierta en otra pestaña no deduplica entre buffers; es el último pendiente de la
  feature y quedaría para una unidad futura.
- **Commit de unidad de trabajo:** (se registra en el commit de docs siguiente).