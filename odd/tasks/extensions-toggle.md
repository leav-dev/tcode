# Feature: Enable/Disable Extensions

## Description
La ventana de extensiones (Ctrl+P → Extensiones) permite instalar, actualizar y
borrar, pero no **activar/desactivar** una extensión instalada. Se agrega:
`Space` sobre la pestaña **Instaladas** alterna el estado; la fila muestra
`(desactivada)` y el estado **persiste** en `~/.tcode/config.json`.

## Decisiones de diseño
- **Una extensión desactivada no se carga**: `loadExtensions` la saltea, así que
  no llega al Manager —sin comandos, sin keybindings, sin hooks—. La ventana la
  sigue listando (sale de `ext.List`, que lee disco) para poder reactivarla.
- **Persistencia**: `configFile.DisabledExtensions []string` en
  `~/.tcode/config.json`, leída en `loadConfig` y reescrita al alternar.
- **`Space` alterna, `Enter` sigue borrando**: aditivo, no rompe el contrato
  documentado de la ventana. Pista `espacio: activar/desactivar` en el borde
  inferior de la pestaña Instaladas (una acción de teclado invisible es una
  acción perdida).
- **Limpieza al desactivar**: la extensión no puede volver a escribir, así que
  sus artefactos se retiran —secciones de la barra (clave = manifest id) y
  diagnósticos en todos los buffers (clave = `dir::script`)—.
- **Sin cambios en el Manager**: el filtro vive en el controlador (capa que ya
  conoce el estado de la sesión); el Manager recibe solo las activas.

## Tasks
- [x] Controller: `DisabledExtensions` en `configFile`, `a.disabledExts`,
  filtro en `loadExtensions`, `toggleExtension` + tests <!-- id: 1 -->
- [x] Controller: limpieza de secciones y diagnósticos al desactivar + tests <!-- id: 2 -->
- [x] View: `Space` → `ExtIntentToggle`, marca `(desactivada)`, pista en el
  borde inferior + tests <!-- id: 3 -->
- [x] Controller: wiring de la intención toggle y marca en `buildExtItems` + tests <!-- id: 4 -->
- [x] Docs: `docs/extension-system.md` (ventana + teclas) y `docs/config.md`
  (clave nueva) <!-- id: 5 -->
- [x] Verificación (gofmt/vet/test -race) y commit de unidad del trabajo <!-- id: 6 -->

## Verificación
- `gofmt -l` limpio en los archivos tocados.
- `go vet ./...` limpio.
- `go test -race ./...` ok (controller, ext, model, view).
- Tests: 4 de la ventana (Space), 7 del controlador (toggle, config, filtro,
  marca de fila, limpieza de artefactos).

## Commit
`ca249e7` (rama `feat/extensions-toggle`).
