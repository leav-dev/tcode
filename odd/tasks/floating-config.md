# Feature: Ventana flotante de configuración básica (Ctrl+,)

## Description
El usuario pide una "ventana flotante" (overlay, como el menú de pestañas
Ctrl+T) desde la que se manejan configuraciones básicas del editor. Alcance
acordado: **tamaño del tab**, **salto de palabra on/off** y **ancho del panel
lateral**, abriendo/cerrando con **Ctrl+,** y persistiendo en
**`~/.tcode/config.json`** (leído al arranque, escrito al cambiar un ajuste).

Estado de la config hoy: `view.indentUnit` (string, var), `view.wordWrapEnabled`
(var con `WordWrapEnabled()`/`ToggleWordWrap()`), `controller.explorerWidth`
(const). La config se muda a vars de `view` con setters/getters exportados y el
`ConfigMenu` (del paquete view) muta esas vars en vivo; el controlador persiste.

## Tasks
- [x] `view/settings.go`: `view.explorerWidth = 24` (movido de controller),
  `ExplorerWidth()`/`SetExplorerWidth(n)`, `IndentSize()`/`SetIndentSize(n)`
  (n espacios → `indentUnit`), `SetWordWrapEnabled(b)` <!-- id: 0 -->
- [x] `view/config_menu.go`: `ConfigMenu` overlay (patrón TabMenu: cursor/top/
  clamp/ensureCursorVisible), items fijos (Tab size int 1..8 paso 1; Word wrap
  bool; Panel width int 16..48 paso 2) con closures get/set sobre las vars;
  `HandleEvent` devuelve `(handled, changed)` (Left/Right/Enter mutan la fila,
  Escape y toda tecla ajena cierran); `Draw` con marco `┌─┐│└┘`, título
  "Configuración" y valor alineado a la derecha; fila activa con TreeCursor
  a todo el ancho interior <!-- id: 1 -->
- [x] Controller: mover `explorerWidth` fuera del const block; `panelWidth`
  usa `view.ExplorerWidth()`; campo `configMenu` + `configActive`; `SetTheme`
  del menú junto al resto; `toggleConfig()` centrado sobre el área del editor
  (w=34 clamp, h=marco+items, clamps al alto) <!-- id: 2 -->
- [x] `handleEvent`: bloque `configActive` después del prompt y antes del menú
  (handleado → `configChanged()`: saveConfig + resizeEditors + explorer.Resize
  + tabBar.EnsureActive; no handleado → cierra); `Ctrl+,` (KeyRune ',' con
  ModCtrl, sin Shift) antes del guard de workspace vacío; mouse ignorado con
  `configActive` (como el menú) <!-- id: 3 -->
- [x] Persistencia: `configFilePath` var inyectable (home/.tcode/config.json,
  patrón de `themeFilePath`), `configFile{IndentUnit int, WordWrap *bool,
  ExplorerWidth int}` (puntero para distinguir ausencia de false),
  `loadConfig()` al arranque junto a `loadTheme` (JSON roto/ausente → defaults,
  jamás rompe), `saveConfig()` tras cada cambio con mensaje de error en la
  barra de estado <!-- id: 4 -->
- [x] Tests view (`config_menu_test.go`): defaults explícitos al inicio (las
  vars son globales) + cleanup; dibujo con marco y valores; navegación; clamps
  min/max; `(true,true)` solo cuando muta; tecla ajena `(false,false)`
  <!-- id: 5 -->
- [x] Tests controller (nuevo `app_config_test.go`): `configFilePath`
  redirigido a TempDir; Ctrl+, abre/cierra (y con workspace vacío); mutar un
  ajuste persiste el JSON; loadConfig al arranque aplica valores; cleanup de
  vars globales a defaults <!-- id: 6 -->
- [x] Verificación (gofmt/vet/go test, paquete view y controller) y commit de unidad
  del trabajo del writer: `e410608` <!-- id: 7 -->

## Design decisions
- **La config vive en `view` (vars más setters/getters exportados); el
  persistidor es el controlador.** El `ConfigMenu` es del paquete view y muta
  las vars en vivo; el controlador aplica (`loadConfig`) y persiste
  (`saveConfig`). `*bool` en el JSON separa "ausente → default" de "false".
- **Aplicación inmediata en vivo.** Cada cambio del menú re-encuadra las vistas
  (resizeEditors + explorer.Resize + tabBar) y redibuja, sin relanzar.
- **`Ctrl+,` llega como `KeyRune ','` con ModCtrl** en Windows Terminal (el
  patrón de Ctrl+Shift+Z/W); se acepta solo sin Shift. Riesgo documentado:
  en terminales Unix puede no llegar (como el form feed de Ctrl+L).
- **Overlay sin estado de sesión:** se dibuja sobre el editor con la misma
  Surface compuesta del menú de pestañas; mientras está abierto posee el
  teclado y el mouse se descarta, sin llegar nunca al documento.