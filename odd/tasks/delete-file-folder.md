# Feature: Borrar archivos y carpetas desde el explorador

## Description
El explorador permite crear archivos/carpetas pero no borrarlos. El usuario quiere
borrar el nodo del cursor (archivo o carpeta) con la tecla Delete, con confirmación.

## Decisiones de diseño (confirmadas con el usuario)

| # | Decisión |
| --- | --- |
| 1 | Disparador: la tecla **Delete** (y Backspace) con el foco en el explorador; borra el nodo del cursor |
| 2 | Borra **archivo o carpeta** (una carpeta, recursiva con todo su contenido) |
| 3 | **Confirmación sí/no**: "¿Borrar <nombre>? [s/N]" (default NO) |
| 4 | **Borrado permanente** (`os.RemoveAll`), sin papelera |

## Decisiones de arquitectura (del agente)

- `FileBrowser`: nueva `ActionDelete` (el explorador avisa, el controller decide) y
  `RemoveNode(path)` — el reverso de `AddChild`, que ya existe.
- El controller maneja `ActionDelete`: lee `CursorPath()`, muestra el prompt sí/no
  (reusa `openPrompt`) y al aceptar llama a `deletePath`.
- **Buffers abiertos**: si el archivo borrado (o alguno dentro de la carpeta borrada)
  está abierto, se cierra la pestaña con `CloseForce` — un buffer apuntando a un
  archivo que ya no existe no debe quedar vivo (guardarlo lo recrearía).
- Comando `tcode.deleteFile` para extensiones, por simetría con `tcode.createFile`.
- `KeyDelete`/`KeyBackspace` no están usados por el explorador: están libres.

## Tasks
- [ ] `internal/view/file_browser.go`: `ActionDelete` + manejo de `KeyDelete`/`KeyBackspace` + `RemoveNode(path)` <!-- id: 0 -->
- [ ] `internal/controller/app.go`: manejar `ActionDelete` con prompt sí/no; `deletePath` (RemoveAll + cerrar buffers + RemoveNode + mensaje); `tcode.deleteFile` <!-- id: 1 -->
- [ ] Tests: RemoveNode, ActionDelete, confirmar borra, denegar no, buffer abierto se cierra <!-- id: 2 -->
- [ ] Docs: README (keybinding Delete) <!-- id: 3 -->

## Evidence
- No existe `os.Remove` para archivos del usuario: solo la sesión (`app.go`) y los
  temporales del guardado atómico (`piece_table.go`).
- No existe `RemoveNode` en el FileBrowser (solo `AddChild`, `SetRootEntries`, `SetChildren`).
- `KeyDelete` no está usado por el explorador (solo `KeyBackspace` en el prompt).
- `Workspace.CloseForce(i)` existe para cerrar una pestaña sin la guarda de modificado.
