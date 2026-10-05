# Feature: Status Bar Sections

## Description
Las extensiones que escriben en la barra de estado usan `tcode.message()`,
que llama a `statusBar.SetMessage()`: **reemplaza** el texto. Con varias
extensiones escribiendo (unused-imports, error-detector, undefined-vars,
git-changes), solo se ve la última. El usuario quiere un "panorama completo"
y aclaró el alcance: **solo `git-changes` debe escribir en la barra**.

Solución: API de **secciones** — `tcode.statusBar.setSection(id, text)`— donde
cada extensión escribe su propia sección identificada. El editor las muestra
lado a lado entre el nombre del archivo y el mensaje transitorio, sin que se
pisen entre sí ni con los mensajes propios del editor.

## Decisiones de diseño
- **Secciones por buffer**: el controlador guarda `map[buffer]map[id]texto`.
  Al cambiar de pestaña, `syncStatus` empuja las secciones del buffer activo
  (sin datos stale). Al cerrar el buffer, sus secciones se borran.
- **Layout**: `archivo  sección1  sección2  <mensaje>`. El mensaje transitorio
  conserva su prioridad (derecha, como hoy); la etiqueta del archivo trunca
  primero que las secciones cuando no hay lugar.
- **Tema**: rol `Section` (gris subordinado, como Comment) en las 7 paletas +
  `LoadTheme` (`section`).
- **Contrato**: `setSection(id, text)` — texto vacío **remueve** la sección;
  id vacío es error de Lua. El id es el manifest id de la extensión
  (p. ej. `tcode.gitchanges`).
- **`tcode.message` queda intacto** para los mensajes transitorios del editor
  (recargar, wrap, confirmaciones). Las secciones y el mensaje coexisten.

## Verificación
- `gofmt -l` limpio en los archivos tocados (script.go ya venía sin formatear
  de HEAD; no se reformateó).
- `go vet ./...` limpio.
- `go test -race ./...` ok (controller, ext, model, view).
- Tests: 5 de status_bar (secciones), 2 de ext (puente), 5 de controller
  (SetSection, syncStatus, closeTab).

## Commits
- tcode: `103b0a7` (rama `feat/status-bar-sections`).
- tcode-extention: `2475575` (rama `feat/git-changes-status-section`).

## Tasks
- [x] `view/status_bar.go`: `sections` + `SetSections`, render entre etiqueta
  y mensaje, truncado con prioridad mensaje > etiqueta > secciones + tests <!-- id: 1 -->
- [x] `view/theme.go`: rol `Section` en 7 paletas + `LoadTheme` <!-- id: 2 -->
- [x] `internal/ext/script.go`: `SetSection(id, text)` en `ScriptAPI` +
  `tcode.statusBar.setSection` en el bridge Lua + tests <!-- id: 3 -->
- [x] `internal/controller/app.go`: storage por buffer, `SetSection` impl,
  push en `syncStatus`, borrado al cerrar (closeTab/closeBuffersUnder) + tests <!-- id: 4 -->
- [x] `docs/extension-system.md`: fila `tcode.statusBar.setSection` <!-- id: 5 -->
- [x] Verificación (gofmt/vet/test -race) y commit de unidad del trabajo <!-- id: 6 -->
- [x] Repo `tcode-extention`: migrar `git-changes` de `tcode.message()` a
  `tcode.statusBar.setSection("tcode.gitchanges", ...)` + doc + commit <!-- id: 7 -->
