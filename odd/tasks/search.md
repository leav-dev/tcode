# Feature: Search (Ctrl+F / Ctrl+Shift+F)

## Description
Búsqueda en archivo (`Ctrl+F`: pedido en barra, Enter salta al siguiente, Esc cierra)
y búsqueda en repo (`Ctrl+Shift+F`: pedido + ventana de resultados, Enter salta a archivo:línea).

## Tasks
- [x] Ctrl+F en archivo: prompt, matches literales, Enter siguiente, Esc cierra <!-- id: 0 -->
- [x] Ctrl+Shift+F en repo: walk del root, ventana de resultados, Enter abre y salta <!-- id: 1 -->
- [x] Tests del controlador + vista, README atajos, `go vet` + `go test` <!-- id: 2 -->
- [ ] Commit de unidad de trabajo <!-- id: 3 -->

## Design decisions
- El pedido de búsqueda es un modo propio (no reusa el prompt de SaveAs/crear),
  porque necesita edición continua + Enter repetido para "siguiente".
  Tiene prioridad en `handleEvent`, igual que el prompt existente.
- `Ctrl+F` llega como `KeyCtrlF`; `Ctrl+Shift+F` llega como `KeyRune 'F'/'f'`
  con `ModCtrl|ModShift` (mismo patrón que SaveAs/Wrap/Redo en tcell).
- Archivo: búsqueda literal case-sensitive por línea vía `LineContent`
  (no carga todo con `GetContent`, respeta mmap). Enter cicla matches.
- Repo: `filepath.WalkDir` desde `ws.Root()` (o cwd si no hay root),
  salta `.git`, `.tcode`, `node_modules`, `bin`; archivos >1MB o con NUL se saltan.
  Resultados `path:línea:col:texto`, overlay estilo `TabMenu`.
