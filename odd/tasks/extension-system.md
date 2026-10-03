# Feature: Sistema de extensiones — manifest, comandos, keybindings y hooks

## Description
tcode no tiene forma de que un usuario añada comportamiento sin tocar el núcleo.
Esta unidad crea la **base arquitectónica tipo VSCode pero nativa**: extensiones
declarativas (`extension.json`) que el editor descubre, valida y activa bajo
demanda, y que contribuyen **comandos, keybindings y hooks**. Sin compatibilidad
con `.vsix` y sin ejecutar código todavía: el backend de scripting (WASM/Lua)
queda documentado como evolución futura, y las costuras —registro de comandos,
bus de hooks, activación— son exactamente donde se enchufaría.

Lo que una extensión puede hacer en esta unidad:

- Contribuir comandos declarados (títulos visibles; en esta unidad son stubs que
  activan la extensión, con el mensaje honesto de que falta el backend).
- Re-mapear teclado: `"key": "ctrl+k ctrl+g"` → comando (incluye chords de dos
  tiempos y mods `ctrl`/`shift`/`alt`). Puede apuntar a built-ins `tcode.*`.
- Correr un comando ante eventos de buffer: `onDidOpenBuffer`,
  `onDidSaveBuffer`, `onDidCloseBuffer` (hooks declarativos evento→comando).
- Activar bajo demanda: `onCommand:<id>`, `onDidOpenBuffer`, `onStartup`, `*`.

Reglas de prioridad y límites:

- **El núcleo gana**: los atajos hardcodeados del editor se revisan primero; una
  extensión no puede pisarlos. Determinista y documentado.
- **Lazy**: descubrir manifests al arrancar es barato (solo JSON); nada corre
  hasta que un evento de activación lo dispare.
- **Descubrimiento tolerante**: una extensión rota no rompe el editor; el error
  se acumula y se avisa una vez por la barra de estado.
- **Resolver puro**: el parseo y la resolución de keybindings viven en
  `internal/ext` como funciones puras más una máquina de estados de chords,
  testeables sin el controlador.
- **Hooks de eventos del controlador**: el controlador emite open/save/close vía
  el manager; los hooks son despachos evento→comando. Queda fuera
  `onDidChangeText` (el tecleo es el camino caliente; sin diseño de debounce no
  se toca).

Sin command palette, sin explorador de extensiones, sin scripts: eso es la
evolución siguiente (palette, y el backend de scripting).

## Tasks
- [x] `internal/ext` manifest: tipos (`Manifest`, `Command`, `Keybinding`, `Hook`) + validación: id/versión obligatorios, ids de comando únicos, evento de hook en el set permitido, keybinding parseable <!-- id: 0 -->
- [x] `internal/ext` `Discover(dir)`: recorre subdirectorios, lee `extension.json`, salta inválidas acumulando errores sin fallar <!-- id: 1 -->
- [ ] `internal/ext` `Registry`: `Register`/`Has`/`Run` con resultado manejable; comando desconocido → error reportable <!-- id: 2 -->
- [ ] `internal/ext` keybindings: parser `"ctrl+k"`, mods combinados, F-keys, teclas nombradas, chords `"ctrl+k ctrl+g"`; `Resolve` sobre `tcell.EventKey` con la máquina de estados de chord <!-- id: 3 -->
- [ ] `internal/ext` hooks + activación: bus `onDidOpenBuffer`/`onDidSaveBuffer`/`onDidCloseBuffer`; activación por `onCommand:<id>`/`onDidOpenBuffer`/`onStartup`/`*` <!-- id: 4 -->
- [ ] Controller: `App` crea el `Manager` con `NewAppWithScreen`; built-ins `tcode.*` (save, saveAs, closeTab, toggleExplorer, undo, redo, switchTabNext/Prev) registrados contra acciones existentes; resolución de keybindings en `handleEvent` después del switch del núcleo y antes del guard de workspace vacío; emisión de hooks en open/save/close; mensajes de estado (activación, comando desconocido, error) <!-- id: 5 -->
- [ ] Rutas reales de descubrimiento (usuario `~/.tcode/extensions` + proyecto `.tcode/extensions`) con override para tests; `docs/extension-system.md` (spec del manifest, ejemplo completo, referencia de keybindings, roadmap a backend WASM/Lua y palette) <!-- id: 6 -->
- [ ] Tests de integración del controlador (`SimulationScreen`): built-in por keybinding de extensión, chord de dos tiempos, hook en save/open/close, activación diferida, extensión rota en disco no rompe el arranque <!-- id: 7 -->
- [ ] Verificación final: `gofmt -l .`, `go vet ./...`, `go build ./...`, `go test ./...` completo en verde <!-- id: 8 -->

## Design decisions

### Declarativo primero; el código llega por una costura, no por reescritura
Una extensión de VSCode es JS corriendo en un host. tcode es Go puro y de RAM
mínima; embeder Node es inviable (constitución: eficiencia extrema). El primer
milestone es 100% declarativo: manifest → comandos/keybindings/hooks sin código.
El `Registry` acepta handlers por id (hoy: built-ins `tcode.*` y stubs de
activación; mañana: un backend WASM vía wazero o Lua vía gopher-lua registra los
suyos), el bus de hooks despacha evento→comando y la activación es un callback
(hoy: marca estado y valida; mañana: carga el módulo). El costo de diseño extra
es cero y el modelo queda fiel al de VSCode sin prometer lo que no ejecuta.

### El núcleo gana; la extensión resuelve después del switch del editor
Los atajos hardcodeados (`Ctrl+B`, `Ctrl+S`, `Ctrl+T`, `Ctrl+W`, undo/redo,
PgUp/PgDn con Ctrl, Save As, salida) se revisan primero en `handleEvent`. La
resolución de extensiones va después del switch del núcleo y antes del guard de
workspace vacío: así un keybinding de extensión funciona sin buffers abiertos
(comandos `tcode.*` tipo toggleExplorer no necesitan documento), pero jamás pisa
un atajo del editor. Un chord pendiente se cancela con cualquier otra tecla.

### Descubrimiento tolerante por diseño
`Discover` no falla ante una extensión rota: acumula el error y sigue. El editor
arranca siempre; la barra de estado avisa una vez por extensión inválida. Esto no
es laxitud: un editor de terminal no puede volverse inutilizable porque una
carpeta `extensions/` tenga un JSON malformado.

### Eventos de buffer como hooks, no como suscripciones de código
Los hooks son `evento → comando` declarados en el manifest. El controlador ya
tiene los puntos de emisión exactos: abrir (`activateExplorerEntry`/carga),
guardar (`save`/`saveAs`) y cerrar (`closeTab`/salida). `onDidChangeText` queda
fuera a propósito: cada tecla es un evento y sin debounce diseñado es un riesgo
de rendimiento que contradice la constitución.

### Keybinding mínimo pero real
Gramática v1: mods `ctrl+`/`shift+`/`alt+` combinables, tecla letra/dígito/F-keys
o nombrada (`enter`, `tab`, `escape`, `space`, `backspace`, `delete`, `insert`,
`home`, `end`, `pageup`, `pagedown`, `up`, `down`, `left`, `right`), y chords de
dos tiempos separados por espacio. El `Resolve` recibe el `tcell.EventKey` y
devuelve el id de comando o nada. Es lo mínimo que hace útil el sistema y lo
suficiente para que el parser sea honestamente testeable.

## Falsificación (tests que escriben primero contra el código roto)
1. Sin validación de manifest → `TestManifestValidation*` fallan (id vacío,
   versión ausente, ids de comando duplicados, hook desconocido, keybinding inválido).
2. Sin `Discover` → `TestDiscoverSkipsBrokenExtensions` no compila / no existe.
3. Sin `Registry` → `TestRegistryRunReportsUnknownCommand` no compila.
4. Sin parser de keybindings → `TestParseKeybinding*` no compilan (mods, chord error).
5. Sin `Resolve` con estado de chord → `TestResolveChordFiresOnSecondKey` y
   `TestResolvePendingChordCancelledByOtherKey` no compilan.
6. Sin dispatcher de hooks → `TestHookRunsCommandOnEvent` no compila.
7. Sin activación diferida → `TestExtensionActivatesOnCommandAndNotBefore` falla
   (la extensión ya está activa antes de su evento).
8. Sin resolución en el controlador → `TestExtensionKeybindingRunsBuiltin` falla
   (la tecla cae al documento o no hace nada).

## Evidence

### U0 — manifest y validación · `6d5f2d9`
- **RED:** los tests no compilan (no existe `Load`/`Manifest`).
- **GREEN:** `go test ./internal/ext/` en verde; `go vet` limpio; `gofmt` aplicado.
- **Ajuste:** el mensaje de id inválido duplicaba el prefijo de campo
  (`id: id: id inválido`); `validateID` devuelve solo el problema y el llamador
  agrega el campo.

### U1 — descubrimiento · *hash en el próximo update*
- **RED:** no compila (no existe `Discover`).
- **Ajuste del test:** la aserción exigía el error exacto `"rota"`; ahora
  verifica que el error contenga el nombre de la extensión.
- **GREEN:** los 4 tests de Discover en verde (válidas, rotas acumuladas,
  carpetas comunes ignoradas, root inexistente sin error).

### Nota de fondo
- `gofmt -l .` del repo entero no es vacío bajo Go 1.26 (formato previo del
  repo con otra versión); la verificación de formato cubre los archivos de
  esta feature, que quedan limpios.
- `TestWorkspaceNextPrevSingleBufferStaysPut` falla en main (Windows, mmap
  retiene el archivo durante el cleanup de `TempDir`); pre-existente y ajeno a
  esta feature (rama con cero diff contra main al inicio).