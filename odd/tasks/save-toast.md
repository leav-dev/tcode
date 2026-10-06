# Feature: Save Toast

## Description
La confirmación de guardado («Guardado») y los errores de guardado viven hoy
en la barra de estado, al fondo. Se mueven a una notificación tipo toast en la
**parte superior derecha de la pantalla**, con desaparición automática a los
pocos segundos. La notificación es **modular**: las extensiones (scripts Lua)
pueden mostrar la misma notificación vía `tcode.notify(msg, kind)`.

Decisiones de la persona usuaria (2026-10-05):
- **Mover**: el mensaje desaparece de la barra inferior; vive solo en el toast.
- **Desaparece sola**: se oculta a los pocos segundos (toast típico).
- **También errores**: el error de guardado aparece en el mismo toast, en rojo.

## Tasks
- [x] `view/toast.go`: componente `Toast` (kinds success/error, `Show` con
  expiración, `Clear`, `Visible`, `Draw` arriba a la derecha con marco y
  fondo) + tests <!-- id: 1 -->
- [x] Theme: roles `ToastSuccess`/`ToastError` en las 7 paletas + `LoadTheme`
  (`toastSuccess`/`toastError`) <!-- id: 2 -->
- [x] Controller: campo `toast`, `showToast(msg, kind)` con timer
  (`time.AfterFunc` + `toastEvent` + seq, patrón `extSnapshotEvent`), wiring
  en `save()` y `saveAs()` (éxito y error → toast), dispatch en `handleEvent`,
  tests <!-- id: 3 -->
- [x] API de extensiones: `Notify(msg, kind)` en `ScriptAPI` +
  `tcode.notify(msg [, kind])` en el bridge Lua + `fakeAPI`/`forwardAPI` +
  tests <!-- id: 4 -->
- [x] Docs: `docs/extension-system.md` (fila `tcode.notify`) <!-- id: 5 -->
- [x] Verificación (gofmt/vet/test -race) y commit de unidad del trabajo <!-- id: 6 -->

## Design decisions
- **Toast en fila 0, alineado a la derecha**: es la esquina superior derecha de
  la pantalla, literal. Se dibuja como overlay DESPUÉS de pestañas, editor y
  barra (con fondo propio tipo píldora) y desaparece sola: tapar una pestaña
  o la primera línea por ~2.5 s es el comportamiento esperado de un toast.
- **Timer con secuencia**: cada `Show` incrementa `toastSeq`; el evento
  llevado por `time.AfterFunc` trae ese seq y el manejador solo limpia si
  coincide —un toast nuevo no puede ser borrado por el timer del anterior
  (mismo patrón que `extPrefetchSeq`).
- **Dos kinds**: `success` (verde) y `error` (rojo). El aviso de «el archivo
  cambió en disco» es un guardado rechazado → kind error.
- **`tcode.message` queda intacto**: sigue siendo el mensaje de la barra para
  todo lo que no es guardado (recargar, wrap, extensiones…). Solo las
  confirmaciones y errores de guardado migran al toast.
- **Notificación desde extensiones**: `Notify(msg, kind string) error` en
  `ScriptAPI` (kind vacío → success; kind desconocido → error). Lua:
  `tcode.notify("lista")` / `tcode.notify("falló", "error")`. Pasa por el
  mismo `showToast`: mismo timer, mismo dibujo.

## Verificación
- `gofmt -l` limpio en los archivos tocados (script.go y script_test.go/
  manager_test.go ya venían sin formatear de HEAD; no se reformatearon para
  no inflar el diff).
- `go vet ./...` limpio.
- `go test -race ./...` ok (controller, ext, model, view).
- Tests: 8 de view (toast), 6 de controller (wiring + timer + seq + Notify),
  3 de ext (puente tcode.notify).

## Commit
`7791669` (rama `feat/save-toast`).
