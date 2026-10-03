package controller

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/ext"
)

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
			"keybindings": [{"key": "ctrl+k", "command": "tcode.toggleExplorer"}]
		}
	}`)

	if quit := press(app, tcell.KeyCtrlK); quit {
		t.Fatal("ctrl+k no debe cerrar el editor")
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
			"keybindings": [{"key": "ctrl+k", "command": "ext.stub.saludar"}]
		}
	}`)

	if app.ext.Active("ext.stub") {
		t.Fatal("la extensión no debería activarse al arrancar")
	}
	press(app, tcell.KeyCtrlK)
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
			"keybindings": [{"key": "ctrl+k", "command": "tcode.noexiste"}]
		}
	}`)

	press(app, tcell.KeyCtrlK)
	msg := app.statusBar.Message()
	if !strings.Contains(msg, "desconocido") || !strings.Contains(msg, "tcode.noexiste") {
		t.Errorf("mensaje = %q, esperaba comando desconocido con su id", msg)
	}
}
