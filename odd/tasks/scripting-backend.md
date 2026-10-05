# Feature: Backend de scripting Lua para extensiones (hito 1)

## Description
El usuario eligió el milestone grande: las extensiones declarativas pueden
tener **lógica propia** corriendo **Lua** (gopher-lua, sin cgo). Hito 1: un
comando declarado puede delegar su implementación a una función del script de
la extensión; el script habla con el editor vía la tabla `tcode` (ejecutar
comandos registrados, leer el buffer activo, insertar texto en el cursor,
mensajes a la barra). Errores del script → mensaje en la barra, nunca rompen
el editor. Sin `onDidChangeText` (deuda documentada de rendimiento).

La dependencia `github.com/yuin/gopher-lua v1.1.2` ya está en `go.mod`/`go.sum`
(traída por el parent, `go build` verificado).

## Tasks
- [ ] `Manifest.Command` gana `Script`/`Fn` (`json:"script"/"fn"`); validación:
  juntos o ninguno; el comando declarado con ambos tiene IMPLEMENTACIÓN en el
  script (keybindings/hooks lo referencian igual) <!-- id: 0 -->
- [ ] `internal/ext/script.go`: `ScriptAPI` (RunCommand, ActiveBuffer,
  InsertAtCursor, StatusMessage), `ScriptHost` (LState con solo base/table/
  string/math, tabla `tcode`, `Call(fn)` con recover → error del script,
  `Close`) <!-- id: 1 -->
- [ ] `Manager`: `SetEditor(ScriptAPI)`; el stub de un comando con script+fn
  delega en `runScriptCommand` (host cacheado por extensión+script, leído de
  `e.Dir`); guard de profundidad de reentrada de scripts (máx 8) <!-- id: 2 -->
- [ ] Controller: `App` implementa `ScriptAPI` (RunCommand vía `ext.RunCommand`
  directo sin re-stub, `ActiveBuffer` = buffer activo + contenido, `InsertAtCursor`
  con `CursorOffset()` nuevo del EditorView, `StatusMessage`); wiring
  `app.ext.SetEditor(app)` en el constructor <!-- id: 3 -->
- [ ] `EditorView.CursorOffset() int`: offset del cursor en el documento
  (LineStart(line)+ByteCol) <!-- id: 4 -->
- [ ] Tests: host con api fake (call, errores, panics, tabla tcode);
  manager delega a script con editor fake; validación script/fn; integración
  controller: extensión de disco con main.lua + keybinding → tecla → comando →
  script inserta — buffer verificado <!-- id: 5 -->
- [ ] Verificación y commit; docs (sección Lua en extension-system.md + README)