package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/leav-dev/tcode/internal/ext"
	"github.com/leav-dev/tcode/internal/view"
)

// validExtSrc es un manifest sano con un comando declarado.
const validExtSrc = `{
	"id": "demo.sana",
	"name": "Sana",
	"version": "1.0.0",
	"activation": ["onStartup"],
	"contributes": {
		"commands": [{"id": "demo.sana.hola"}]
	}
}`

// newTestAppOnDir arranca el editor sobre un directorio (modo explorador), con
// la misma pantalla simulada y limpieza que newTestApp.
func newTestAppOnDir(t *testing.T, dir string) (*App, error) {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	s.SetSize(40, 10)
	app, err := NewAppWithScreen(s, dir)
	t.Cleanup(func() {
		if app != nil {
			app.ws.CloseAll()
		}
		s.Fini()
	})
	return app, err
}

// addExtension agrega un manifest a las extensiones del app y cierra el
// arranque como hace el constructor: activa las que declaran onStartup. En
// los tests la extensión llega después del arranque real, así que se re-dispara
// el evento de arranque (no reactiva a las ya activas).
func addExtension(t *testing.T, app *App, src string) {
	t.Helper()
	mf, err := ext.Load([]byte(src))
	if err != nil {
		t.Fatalf("Load del fixture falló: %v", err)
	}
	app.ext.AddExtensions([]ext.Extension{{Manifest: mf, Dir: "fixture"}})
	app.ext.ActivateEvent(ext.ActivateStartup)
}

// TestExtensionKeybindingRunsBuiltin es el camino feliz del wiring: un
// keybinding de extensión apuntando a un built-in tcode.* ejecuta la acción
// existente del controlador —acá, mostrar el panel del explorador—.
func TestExtensionKeybindingRunsBuiltin(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	addExtension(t, app, `{
		"id": "ext.muestra",
		"name": "Muestra",
		"version": "1.0.0",
		"activation": ["onStartup"],
		"contributes": {
			"keybindings": [{"key": "ctrl+m", "command": "tcode.toggleExplorer"}]
		}
	}`)

	if quit := press(app, tcell.KeyCtrlM); quit {
		t.Fatal("ctrl+m no debe cerrar el editor")
	}

	// El built-in corrió: el panel quedó visible. Con foco de teclado verificar
	// el estado del explorador es suficiente: la acción existente se ejecutó.
	tabW := app.tabBarWidth()
	width, _ := app.screen.Size()
	if app.explorerVisible && app.explorerColumn() == panelWidth(width) && tabW != width {
		return // toggleExplorer mostró el panel y el editor perdió ancho
	}
	t.Fatalf("el builtin no corrió: explorerVisible=%v tabW=%d width=%d", app.explorerVisible, tabW, width)
}

// TestExtensionKeybindingFallsThroughToDocument: una tecla que ninguna
// extensión reclama sigue insertándose en el documento: el resolver no
// intercepta el tecleo normal.
func TestExtensionKeybindingFallsThroughToDocument(t *testing.T) {
	app, _ := newTestApp(t, "")
	addExtension(t, app, `{
		"id": "ext.sorda",
		"name": "Sorda",
		"version": "1.0.0",
		"activation": ["*"],
		"contributes": {
			"keybindings": [{"key": "ctrl+alt+x", "command": "tcode.toggleExplorer"}]
		}
	}`)

	typeRune(app, 'x')
	buf := app.ws.Active()
	if got := string(buf.LineContent(0)); got != "x" {
		t.Fatalf("la tecla no cayó al documento: línea = %q", got)
	}
	if app.explorerVisible {
		t.Fatal("ctrl+alt+x no debería haberse disparado con una x suelta")
	}
}

// TestExtensionHookOnCloseRuns teje el bus de hooks al controlador: cerrar una
// pestaña con Ctrl+W emite onDidCloseBuffer y el comando del hook corre.
func TestExtensionHookOnCloseRuns(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	calls := 0
	if err := app.ext.Registry().Register("test.aldespedir", func() error { calls++; return nil }); err != nil {
		t.Fatalf("Register falló: %v", err)
	}
	addExtension(t, app, `{
		"id": "ext.despedida",
		"name": "Despedida",
		"version": "1.0.0",
		"activation": ["onStartup"],
		"contributes": {
			"hooks": [{"event": "onDidCloseBuffer", "command": "test.aldespedir"}]
		}
	}`)

	if quit := press(app, tcell.KeyCtrlW); quit {
		t.Fatal("Ctrl+W no debe cerrar el editor")
	}
	if calls != 1 {
		t.Fatalf("hook onDidCloseBuffer corrió %d veces, esperaba 1", calls)
	}
}

// TestExtensionDeclaredCommandActivatesAndReports: ejecutar un comando
// declarado via keybinding activa la extensión (onCommand) y muestra el aviso
// honesto del roadmap en la barra de estado — la verdad observable del
// milestone declarativo.
func TestExtensionDeclaredCommandActivatesAndReports(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	addExtension(t, app, `{
		"id": "ext.stub",
		"name": "Stub",
		"version": "1.0.0",
		"activation": ["onCommand:ext.stub.saludar"],
		"contributes": {
			"commands": [{"id": "ext.stub.saludar", "title": "Saludar"}],
			"keybindings": [{"key": "ctrl+m", "command": "ext.stub.saludar"}]
		}
	}`)

	if app.ext.Active("ext.stub") {
		t.Fatal("la extensión no debería activarse al arrancar")
	}
	press(app, tcell.KeyCtrlM)
	if !app.ext.Active("ext.stub") {
		t.Fatal("ejecutar el comando no activó la extensión")
	}
	if msg := app.statusBar.Message(); !strings.Contains(msg, "sin implementación") {
		t.Errorf("mensaje = %q, esperaba el aviso del backend declarativo", msg)
	}
}

// TestExtensionUnknownCommandShowsStatus: un keybinding que apunta a un
// comando inexistente no crashea; la barra de estado muestra el comando
// desconocido.
func TestExtensionUnknownCommandShowsStatus(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	addExtension(t, app, `{
		"id": "ext.rota",
		"name": "Rota",
		"version": "1.0.0",
		"activation": ["*"],
		"contributes": {
			"keybindings": [{"key": "ctrl+m", "command": "tcode.noexiste"}]
		}
	}`)

	press(app, tcell.KeyCtrlM)
	msg := app.statusBar.Message()
	if !strings.Contains(msg, "desconocido") || !strings.Contains(msg, "tcode.noexiste") {
		t.Errorf("mensaje = %q, esperaba comando desconocido con su id", msg)
	}
}

// TestExtensionChordRunsInController recorre la máquina de estados del chord
// desde el controlador: la primera tecla arma el pendiente y la segunda
// ejecuta el built-in.
func TestExtensionChordRunsInController(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	addExtension(t, app, `{
		"id": "ext.chord",
		"name": "Chord",
		"version": "1.0.0",
		"activation": ["onStartup"],
		"contributes": {
			"keybindings": [{"key": "ctrl+m ctrl+g", "command": "tcode.toggleExplorer"}]
		}
	}`)

	press(app, tcell.KeyCtrlM)
	if app.explorerVisible {
		t.Fatal("la primera tecla del chord ya ejecutó el comando")
	}
	press(app, tcell.KeyCtrlG)
	if !app.explorerVisible {
		t.Fatal("la segunda tecla del chord no ejecutó tcode.toggleExplorer")
	}
}

// TestExtensionHookOnOpenLazyActivates arranca sobre un directorio (sin
// buffers), agrega una extensión perezosa con onDidOpenBuffer y verifica el
// camino completo: abrir un archivo desde el explorador activa la extensión y
// corre su hook del mismo evento.
func TestExtensionHookOnOpenLazyActivates(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "doc.txt"), []byte("hola\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, err := newTestAppOnDir(t, dir)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}

	calls := 0
	if err := app.ext.Registry().Register("ext.lazy.ver", func() error { calls++; return nil }); err != nil {
		t.Fatalf("Register falló: %v", err)
	}
	addExtension(t, app, `{
		"id": "ext.lazy",
		"name": "Perezosa",
		"version": "1.0.0",
		"activation": ["onDidOpenBuffer"],
		"contributes": {
			"hooks": [{"event": "onDidOpenBuffer", "command": "ext.lazy.ver"}]
		}
	}`)
	if app.ext.Active("ext.lazy") {
		t.Fatal("la extensión perezosa se activó sin abrir un buffer")
	}

	// Enter en el explorador abre el archivo del cursor: emit(open) -> activa
	// la extensión y corre su hook.
	press(app, tcell.KeyEnter)
	if !app.ext.Active("ext.lazy") {
		t.Fatal("abrir un buffer no activó la extensión perezosa")
	}
	if calls != 1 {
		t.Fatalf("hook onDidOpenBuffer corrió %d veces, esperaba 1", calls)
	}
}

// TestExtensionBrokenInDiskDoesNotBreakStartup arranca con .tcode/extensions
// del proyecto que contiene una extensión rota: el editor arranca igual, la
// sana se carga y el error se avisa en la barra de estado.
func TestExtensionBrokenInDiskDoesNotBreakStartup(t *testing.T) {
	dir := t.TempDir()
	writeExtensionDir(t, filepath.Join(dir, ".tcode", "extensions"), map[string]string{
		"rota": `{ json roto`,
		"sana": validExtSrc,
	})
	app, err := newTestAppOnDir(t, dir)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló con una extensión rota en disco: %v", err)
	}
	if !app.ext.Registry().Has("demo.sana.hola") {
		t.Fatal("la extensión sana de disco no se registró")
	}
	if msg := app.statusBar.Message(); !strings.Contains(msg, "rota") {
		t.Errorf("mensaje = %q, esperaba el aviso de la extensión rota", msg)
	}
}

// writeExtensionDir materializa root/ con las entradas dadas {carpeta:
// contenido-de-extension.json} y devuelve root. Si root es vacío usa un
// TempDir.
func writeExtensionDir(t *testing.T, root string, entries map[string]string) string {
	t.Helper()
	if root == "" {
		root = t.TempDir()
	}
	for name, src := range entries {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(src), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return root
}

// TestAppCursorTracksEditorMovement: el puente ScriptAPI.Cursor refleja la
// posición del editor activo (0-indexada) y sigue su movimiento; sin buffer
// abierto, ok=false.
func TestAppCursorTracksEditorMovement(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	line, col, ok := app.Cursor()
	if !ok || line != 0 || col != 0 {
		t.Fatalf("Cursor inicial = (%d, %d, %v), esperaba (0, 0, true)", line, col, ok)
	}
	press(app, tcell.KeyRight)
	line, col, ok = app.Cursor()
	if !ok || line != 0 || col != 1 {
		t.Fatalf("Cursor tras Right = (%d, %d, %v), esperaba (0, 1, true)", line, col, ok)
	}
}

// TestExtensionScriptInsertsAtCursor: integración del backend de scripting —
// una extensión de disco con main.lua y un comando que declara (script, fn)
// se dispara desde su keybinding; la función Lua inserta texto en el cursor
// vía tcode.insert y el buffer queda editado.
func TestExtensionScriptInsertsAtCursor(t *testing.T) {
	resetConfigVars(t)
	app, _ := newTestApp(t, "uno")

	root := writeExtensionDir(t, "", map[string]string{
		"escritor": `{
			"id": "escritor",
			"name": "Escritor",
			"version": "1.0.0",
			"activation": ["onStartup"],
			"contributes": {
				"commands": [
					{ "id": "escritor.hola", "title": "Hola", "script": "main.lua", "fn": "hola" }
				],
				"keybindings": [
					{ "key": "ctrl+shift+h", "command": "escritor.hola" }
				]
			}
		}`,
	})
	luaSrc := `function hola()
  tcode.insert("HOLA")
end
`
	if err := os.WriteFile(filepath.Join(root, "escritor", "main.lua"), []byte(luaSrc), 0o644); err != nil {
		t.Fatalf("main.lua: %v", err)
	}
	app.extensionRoots = []string{root}
	app.loadExtensions()
	app.ext.ActivateEvent(ext.ActivateStartup)

	// La tecla del keybinding resuelve el comando declarado con script.
	app.handleEvent(tcell.NewEventKey(tcell.KeyRune, 'H', tcell.ModCtrl|tcell.ModShift))
	if got := app.ws.Active().GetContent(); got != "HOLAuno" {
		t.Fatalf("contenido = %q, se esperaba %q (la fn Lua insertó en el cursor 0)", got, "HOLAuno")
	}
}

// TestLoadExtensionsFromDisk: el cargador descubre las extensiones de las
// raíces configuradas, registra sus stubs, deja sus keybindings activos y
// avisa (sin romper) por cada extensión rota.
func TestLoadExtensionsFromDisk(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	root := writeExtensionDir(t, "", map[string]string{
		"buena": `{
			"id": "demo.buena",
			"name": "Buena",
			"version": "1.0.0",
			"activation": ["onStartup"],
			"contributes": {
				"commands": [{"id": "demo.buena.hola"}],
				"keybindings": [{"key": "ctrl+k", "command": "tcode.toggleExplorer"}]
			}
		}`,
		"rota": `{ json roto`,
	})
	app.extensionRoots = []string{root}
	app.loadExtensions()

	if !app.ext.Registry().Has("demo.buena.hola") {
		t.Fatal("el stub del comando declarado no se registró desde disco")
	}
	// El keybinding del disco está activo: resuelve el built-in.
	// Se activa el startup recién agregado como hace el arranque.
	app.ext.ActivateEvent(ext.ActivateStartup)
	if got := app.ext.Resolve(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModCtrl)); got != "tcode.toggleExplorer" {
		t.Fatalf("Resolve desde disco = %q, esperaba tcode.toggleExplorer", got)
	}
	if msg := app.statusBar.Message(); !strings.Contains(msg, "rota") {
		t.Errorf("mensaje = %q, esperaba el aviso de la extensión rota", msg)
	}
}

// TestLoadExtensionsPrefersFirstRoot: ante ids duplicados entre raíces, el
// primer root (usuario) gana: su keybinding prevalece sobre el del proyecto.
func TestLoadExtensionsPrefersFirstRoot(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	userRoot := writeExtensionDir(t, "", map[string]string{
		"usuario": `{
			"id": "demo.x",
			"name": "Usuario",
			"version": "1.0.0",
			"activation": ["onStartup"],
			"contributes": {
				"keybindings": [{"key": "ctrl+k", "command": "cmd.usuario"}]
			}
		}`,
	})
	projRoot := writeExtensionDir(t, "", map[string]string{
		"proyecto": `{
			"id": "demo.x",
			"name": "Proyecto",
			"version": "1.0.0",
			"activation": ["onStartup"],
			"contributes": {
				"keybindings": [{"key": "ctrl+k", "command": "cmd.proyecto"}]
			}
		}`,
	})
	app.extensionRoots = []string{userRoot, projRoot}
	app.loadExtensions()

	if got := app.ext.Resolve(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModCtrl)); got != "cmd.usuario" {
		t.Fatalf("Resolve = %q, esperaba el keybinding del primer root (usuario)", got)
	}
}

// TestExtensionScriptFillsDiagnostics: integración del backend de
// diagnostics — una extensión de disco recorre el buffer con lineCount/line y
// deposita las anotaciones (tcode.diagnostics.set); el editor las expone por
// el getter con línea 0-indexada, tal como el HITO A las renderiza. El camino
// es el mismo que TestExtensionScriptInsertsAtCursor (extensión de disco con
// main.lua, keybinding y Activación de arranque).
func TestExtensionScriptFillsDiagnostics(t *testing.T) {
	resetConfigVars(t)
	app, _ := newTestApp(t, "uno\nTODO aqui\nfin")

	root := writeExtensionDir(t, "", map[string]string{
		"marcador": `{
			"id": "marcador",
			"name": "Marcador TODO",
			"version": "1.0.0",
			"activation": ["onStartup"],
			"contributes": {
				"commands": [
					{ "id": "marcador.todo", "title": "Marcar TODOs", "script": "main.lua", "fn": "marcar" }
				],
				"keybindings": [
					{ "key": "ctrl+shift+t", "command": "marcador.todo" }
				]
			}
		}`,
	})
	luaSrc := `function marcar()
  local n = tcode.lineCount()
  local diags = {}
  for i = 1, n do
    local l = tcode.line(i)
    if string.find(l, "TODO") then
      table.insert(diags, { line = i, message = "todo pendiente", severity = "warning" })
    end
  end
  tcode.diagnostics.set(diags)
end
`
	if err := os.WriteFile(filepath.Join(root, "marcador", "main.lua"), []byte(luaSrc), 0o644); err != nil {
		t.Fatalf("main.lua: %v", err)
	}
	app.extensionRoots = []string{root}
	app.loadExtensions()
	app.ext.ActivateEvent(ext.ActivateStartup)

	// La tecla del keybinding corre la función Lua que marca las TODOs.
	app.handleEvent(tcell.NewEventKey(tcell.KeyRune, 'T', tcell.ModCtrl|tcell.ModShift))

	ed := app.activeEditor()
	d := ed.Diagnostics()
	if len(d) != 1 {
		t.Fatalf("diagnósticos = %+v, esperaba 1 (solo la línea con TODO)", d)
	}
	if d[0].Line != 1 || d[0].Severity != view.SeverityWarning || d[0].Message != "todo pendiente" {
		t.Errorf("diag = %+v, esperaba Line=1 (0-indexada), Warning, todo pendiente", d[0])
	}
}
