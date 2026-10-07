# Feature: Temas por extensión + ventana de temas

## Description
Permitir que las extensiones aporten temas (paletas) y listarlos todos en una
ventana propia: built-ins + Custom + temas de extensiones.

## Decisions (usuario)
- Aporte vía manifest: `contributes.themes[]` con archivos JSON (mismo formato
  que `~/.tcode/theme.json`, el que consume `view.LoadTheme`).
- Ventana nueva que lista TODOS los temas disponibles (no mezclar a mano en el
  enum de Ctrl+P): se abre con Enter sobre la fila Theme de la ventana de
  configuración.

## Design
- `ext`: `ThemeContribution{ID, Label, File}` + validación (id válido, label no
  vacío, file relativo `.json` sin escape, ids únicos). `LoadThemes(exts)` lee
  los archivos (tope ~256 KiB), acumula errores sin romper el arranque.
- `view`: registro dinámico de temas de extensiones
  (`RegisterExtensionThemes`, `AvailableThemes()` con `{ID, Name, Source}`),
  `ThemeByID` también resuelve extensiones. `SetActiveThemeID` los acepta.
  Nueva `ThemeMenu` (cursor/top, Up/Down/PgUp/PgDn/Home/End, Enter elige,
  Esc/ajena cierra) con marca del activo.
- `controller`: carga temas tras `Discover` y tras cada `Manager.Reload`
  (instalar/actualizar/borrar), `themeFor` resuelve extensiones, Enter en la
  fila Theme abre la ventana de temas, Enter en la ventana aplica + persiste.
- Docs: `extension-system.md` (manifest + ejemplo) y atajo/ventana donde liste
  atajos.

## Tasks
- [ ] Manifest `contributes.themes` + validación + tests <!-- id: 0 -->
- [ ] Carga de archivos de tema (`LoadThemes`) + tests <!-- id: 1 -->
- [ ] Registro dinámico en `view` + `AvailableThemes`/`ThemeByID` + tests <!-- id: 2 -->
- [ ] Ventana `ThemeMenu` + tests <!-- id: 3 -->
- [ ] Cableado controlador (carga, apertura, aplicar/persistir) + tests <!-- id: 4 -->
- [ ] Docs + verificación (`vet`, `gofmt`, `go test`) <!-- id: 5 -->
- [ ] Commit de unidad de trabajo <!-- id: 6 -->

## Evidence
- `ext/manifest.go`: `Contributions.Themes` + `ThemeContribution{ID, Label, File}`;
  validación (id/label/file `.json` relativo sin escape, ids únicos).
- `ext/themes.go`: `LoadThemes` (tope 256 KiB, exige objeto JSON, tolerante:
  acumula errores nombrando extensión+tema).
- `view/theme.go`: registro dinámico (`RegisterExtensionThemes`, sin pisar
  built-ins), `ThemeByID` resuelve extensiones, `AvailableThemes()`
  (incluidas + extensiones + Custom).
- `view/theme_menu.go`: ventana `ThemeMenu` (cursor/top, Enter elige, ajena
  cierra) con marca `*` del activo y columna de origen.
- `controller/app.go`: `themeMenu` + apertura con Enter en fila Theme,
  `applyThemeSelection` (aplica + persiste), `refreshExtensionThemes` tras
  `Discover` y cada `Reload`, dibujado overlay.
- `view/config_menu.go`: Enter en Theme deja pendiente `themes` (test
  actualizado).
- Tests: `ext/themes_test.go` (4), `view/theme_menu_test.go` (4),
  `controller/theme_menu_test.go` (2, punta a punta con extensión real).
- Verificación: `go vet ./...` limpio, `gofmt -l` limpio, `go test ./...` OK.
- Docs: `docs/extension-system.md` (manifest + sección de temas + roadmap).
