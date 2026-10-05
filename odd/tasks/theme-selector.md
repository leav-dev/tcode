# Feature: Selector de temas en la ventana de configuración

## Description
Agregar a la ventana flotante de configuración (Ctrl+P) una fila **Theme**
que cicla entre seis paletas: **Light** (claro), **Dark** (el actual Dark+),
**Light HC** (claro alto contraste), **Dark HC** (oscuro alto contraste),
**Tokyo Night** y **Dracula**. El cambio aplica **en vivo** a todas las vistas
—incluidos los editores abiertos, que hoy no reciben el tema en runtime— y
persiste en `~/.tcode/config.json` (clave `Theme`). El `theme.json` del
usuario sigue siendo el fallback: se muestra como la opción **Custom** (id "") y
se aplica cuando `Theme` es "".

## Tasks
- [x] `view/theme.go`: seis fábricas de paleta, helpers `fg`/`on`/`active`
  promovidos a nivel de paquete, registry ordenado (id/name/build) con
  `ThemeIDs()`, `ThemeNames()`, `ThemeByID()`; `DefaultTheme()` = `DarkTheme()`
  (LoadTheme y themeOr intactos) <!-- id: 0 -->
- [x] `view/settings.go`: `activeThemeID` ("" = custom) + `ActiveThemeID()` /
  `SetActiveThemeID(id)` validando contra el registry <!-- id: 1 -->
- [x] `view/config_menu.go`: `ConfigKind` nuevo `ConfigEnum` (names), ítem
  `Theme` (opciones = nombres de los 6 + "Custom"), get = índice del id actual
  ("" → Custom), set = id o ""; `ConfigMenuHeight()` exportado (items+2) para
  la geometría del controller <!-- id: 2 -->
- [x] Controller: `loadTheme` carga `a.customTheme`; `applyTheme()` central
  (5 vistas + `for ed := range a.editors`); `themeFor()` = registry si el id
  existe, si no custom; `loadConfig` aplica `Theme` y re-aplica; `saveConfig`
  persiste el id; `configChanged` re-aplica el tema; `configRegion` usa
  `ConfigMenuHeight()` <!-- id: 3 -->
- [x] Tests view: paletas válidas y distintas entre sí, registry estable y
  `ThemeByID` con id desconocido → no; ConfigMenu: fila Theme visible, ciclar
  cambia `ActiveThemeID`, Custom → "" <!-- id: 4 -->
- [x] Tests controller: `Theme` en config carga y aplica (a.theme == paleta);
  ciclar en el menú persiste el id; custom (sin id) sigue usando theme.json
  <!-- id: 5 -->
- [x] Verificación y commit de unidad: `448b879` <!-- id: 6 -->

## Design decisions
- **El tema deja de aplicarse solo al arranque.** `applyTheme` central recorre
  también `a.editors` (map puntero→view): hoy los editores ya abiertos NO
  reciben el cambio de tema en runtime (solo los que se crean después); el
  selector en vivo lo cierra.
- **`theme.json` = Custom.** El archivo del usuario no desaparece: la opción
  Custom (id "") lo aplica. Sin `Theme` en config.json, el comportamiento es
  idéntico al actual.
- **Custom como valor "" en vez de un id del registry:** el registry queda
  puro (solo las 6 paletas); Custom es el estado default del selector, como el
  tema cargado de disco.
- **Paletas con hex reales** (fidelidad Tokyo Night / Dracula) o índices ANSI
  (Dark actual, HC como Windows High Contrast). La barra de selección
  (TreeCursor) y el status conservan el acento del sistema donde no hay un
  color de marca claro.