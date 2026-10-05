# Feature: Crear archivos y carpetas desde el explorador

## Description
Hoy el explorador de archivos no permite crear archivos ni carpetas: solo
navegar, expandir y abrir. El usuario quiere crear nuevos archivos y carpetas
desde el explorador, con un prompt de nombre, validación, y apertura
automática del archivo nuevo.

## Decisiones de diseño (confirmadas con el usuario)

| # | Decisión |
| --- | --- |
| 1 | Disparador: `Ctrl+N` (archivo) + `Ctrl+Shift+N` (carpeta) |
| 2 | Destino: contextual — la carpeta del cursor (si es carpeta), la carpeta del archivo (si es archivo), o la raíz (si no hay cursor) |
| 3 | Después de crear el archivo: se abre automáticamente en el editor |
| 4 | Validación: rechazar nombre vacío, que ya exista, y caracteres inválidos (`/`, `\`, `..`) |

## Decisiones de arquitectura (del agente)

- **Prompt generalizado**: el prompt de Save As (`promptActive`/`promptBuf`) se
  generaliza con `promptLabel` y `promptAction func(path) error`, para que
  Create lo reutilice. `startPrompt` ya no desreferencia `activeBuffer()` sin
  chequear (hoy panica con workspace vacío).
- **Comandos**: `tcode.createFile` y `tcode.createFolder` en `registerBuiltins`.
- **Keybindings**: `Ctrl+N` / `Ctrl+Shift+N` van ANTES del guard de workspace
  vacío (como `Ctrl+B`/`Ctrl+P`), porque crear debe funcionar sin buffers.
- **Árbol**: `FileBrowser.AddChild(parentPath, Entry)` inserta un nodo hijo en
  un padre por ruta (hoy solo hay `SetRootEntries` y `SetChildren`
  cursor-scoped). `CursorDir()` devuelve el directorio destino.
- **Creación**: archivo = `os.Create` + insertar en árbol + abrir; carpeta =
  `os.Mkdir` + insertar en árbol.

## Tasks
- [ ] Generalizar el prompt (app.go): `promptLabel` + `promptAction` + `promptPrefill`; `startPrompt` sin panic con workspace vacío <!-- id: 0 -->
- [ ] `registerBuiltins`: `tcode.createFile` y `tcode.createFolder` <!-- id: 1 -->
- [ ] Keybindings `Ctrl+N` / `Ctrl+Shift+N` antes del guard de workspace vacío <!-- id: 2 -->
- [ ] `FileBrowser.AddChild(parentPath, Entry)` + `CursorDir()` (file_browser.go) <!-- id: 3 -->
- [ ] Lógica de creación en el controller: validar, crear, insertar en árbol, abrir archivo <!-- id: 4 -->
- [ ] Tests: prompt generalizado, createFile/createFolder, AddChild, keybindings, validación <!-- id: 5 -->
- [ ] Docs: README (keybindings) <!-- id: 6 -->

## Evidence
- El prompt de Save As ya existe (app.go: `promptActive`/`promptBuf`/`promptTarget`, `startPrompt`, `handlePromptKey`, `endPrompt`).
- `ws.NewUntitled()` crea un buffer sin ruta (existe, testeado, sin caller en producción).
- `buffer.SaveAs(path)` escribe un archivo con temp + rename.
- `Ctrl+N` está libre (no lo usa ningún keybinding hoy).
- El árbol no tiene API de inserción: solo `SetRootEntries` y `SetChildren` (cursor-scoped).
