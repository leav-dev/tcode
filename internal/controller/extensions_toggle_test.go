package controller

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/leav-dev/tcode/internal/ext"
	"github.com/leav-dev/tcode/internal/view"
)

// installedExtSrc es una extensión instalada con un comando con script (para
// poder limpiar sus diagnósticos por source) y un keybinding.
const installedExtSrc = `{
	"id": "demo.gitchanges",
	"name": "Git Changes",
	"version": "1.0.0",
	"activation": ["onStartup"],
	"contributes": {
		"commands": [{"id": "demo.gitchanges.status", "script": "main.lua", "fn": "status"}],
		"keybindings": [{"key": "alt+g", "command": "demo.gitchanges.status"}]
	}
}`

// newToggleApp arma una app con una extensión instalada en un root propio y la
// config remapeada a un TempDir. El root se usa también como root de usuario
// para que la ventana (que lista desde ahí) vea la extensión. Devuelve la app y
// la ruta de la config.
func newToggleApp(t *testing.T) (*App, string) {
	t.Helper()
	cfgPath := tmpConfigFile(t)
	app, _ := newTestApp(t, "uno")
	root := writeExtensionDir(t, "", map[string]string{"gitchanges": installedExtSrc})
	app.extensionRoots = []string{root}
	app.extUserRoot = root
	app.reloadExtensions()
	return app, cfgPath
}

// TestToggleExtensionDisablesAndPersists: desactivar saca la extensión de la
// sesión —sin comando registrado— y persiste el estado en la config.
func TestToggleExtensionDisablesAndPersists(t *testing.T) {
	app, cfgPath := newToggleApp(t)

	if !app.ext.Active("demo.gitchanges") {
		t.Fatal("la extensión debe arrancar activa")
	}
	if !slices.Contains(app.ext.RegisteredCommands(), "demo.gitchanges.status") {
		t.Fatalf("comandos = %v, se esperaba el de la extensión", app.ext.RegisteredCommands())
	}

	app.toggleExtension("demo.gitchanges", "Git Changes")

	if app.ext.Active("demo.gitchanges") {
		t.Fatal("tras desactivar no debe estar activa")
	}
	if !app.disabledExts["demo.gitchanges"] {
		t.Fatal("el estado debe quedar en memoria")
	}
	if slices.Contains(app.ext.RegisteredCommands(), "demo.gitchanges.status") {
		t.Fatalf("comandos = %v, el de la desactivada no debe estar", app.ext.RegisteredCommands())
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("no se leyó la config: %v", err)
	}
	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config ilegible: %v", err)
	}
	if !slices.Contains(cfg.DisabledExtensions, "demo.gitchanges") {
		t.Fatalf("DisabledExtensions = %v, esperaba el id", cfg.DisabledExtensions)
	}
}

// TestToggleExtensionReEnables: el toggle es reversible —reactivar registra el
// comando de nuevo y limpia el estado persistido—.
func TestToggleExtensionReEnables(t *testing.T) {
	app, _ := newToggleApp(t)

	app.toggleExtension("demo.gitchanges", "Git Changes")
	app.toggleExtension("demo.gitchanges", "Git Changes")

	if !app.ext.Active("demo.gitchanges") {
		t.Fatal("tras reactivar debe estar activa")
	}
	if app.disabledExts["demo.gitchanges"] {
		t.Fatal("el estado desactivada debe desaparecer")
	}
	if !slices.Contains(app.ext.RegisteredCommands(), "demo.gitchanges.status") {
		t.Fatalf("comandos = %v, la reactivada debe volver a registrar", app.ext.RegisteredCommands())
	}
}

// TestLoadConfigReadsDisabledExtensions: la lista del config repuebla el estado
// de la sesión al arrancar.
func TestLoadConfigReadsDisabledExtensions(t *testing.T) {
	cfgPath := tmpConfigFile(t)
	if err := os.WriteFile(cfgPath, []byte(`{"DisabledExtensions":["demo.otra"]}`), 0o644); err != nil {
		t.Fatalf("no se escribió la config: %v", err)
	}

	app, _ := newTestApp(t, "uno")

	if !app.disabledExts["demo.otra"] {
		t.Fatalf("disabledExts = %v, se esperaba el id del config", app.disabledExts)
	}
}

// TestDisabledExtensionIsFilteredOut: una desactivada no llega al Manager —su
// comando y su keybinding desaparecen— pero sigue descubierta para poder
// reactivarla desde la ventana.
func TestDisabledExtensionIsFilteredOut(t *testing.T) {
	app, _ := newToggleApp(t)

	ev := tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModAlt)
	if cmd := app.ext.Resolve(ev); cmd != "demo.gitchanges.status" {
		t.Fatalf("antes de desactivar, Resolve = %q, se esperaba el comando de la extensión", cmd)
	}

	app.toggleExtension("demo.gitchanges", "Git Changes")

	if cmd := app.ext.Resolve(ev); cmd != "" {
		t.Fatalf("Resolve = %q, el keybinding de la desactivada no debe resolver", cmd)
	}
	if !slices.ContainsFunc(app.extDiscovered, func(e ext.Extension) bool {
		return e.Manifest.ID == "demo.gitchanges"
	}) {
		t.Fatal("la desactivada debe seguir en el conjunto descubierto")
	}
}

// TestDisabledExtensionRowIsMarked: la fila de Instaladas marca el estado para
// que la ventana lo muestre sin abrir nada.
func TestDisabledExtensionRowIsMarked(t *testing.T) {
	app, _ := newToggleApp(t)

	app.handleExtSnapshot(extSnapshotEvent{
		Seq:      app.extPrefetchSeq,
		Snapshot: ext.Snapshot{Installed: []ext.Info{{ID: "demo.gitchanges", Name: "Git Changes", Version: "1.0.0"}}},
	})
	app.toggleExtension("demo.gitchanges", "Git Changes")

	items := app.extManager.Items(view.ExtTabInstalled)
	if len(items) != 1 {
		t.Fatalf("filas = %+v, se esperaba la instalada", items)
	}
	if !strings.Contains(items[0].Right, "(desactivada)") {
		t.Fatalf("Right = %q, debe marcar la extensión desactivada", items[0].Right)
	}
}

// TestDisableClearsSectionsAndDiagnostics: al desactivar, la extensión ya no
// puede mantener lo que escribió: sus secciones de la barra y sus diagnósticos
// se retiran.
func TestDisableClearsSectionsAndDiagnostics(t *testing.T) {
	app, _ := newToggleApp(t)

	if err := app.SetSection("demo.gitchanges", "Git: 3 files"); err != nil {
		t.Fatalf("SetSection: %v", err)
	}
	ed := app.activeEditor()
	if ed == nil {
		t.Fatal("setup: se esperaba un editor activo")
	}
	source := app.extDiscovered[0].Dir + "::main.lua"
	ed.SetDiagnostics(source, []view.Diagnostic{{Line: 0, Message: "x", Severity: view.SeverityError}})
	if len(app.statusBar.Sections()) == 0 || len(ed.Diagnostics()) == 0 {
		t.Fatal("setup: se esperaban sección y diagnóstico")
	}

	app.toggleExtension("demo.gitchanges", "Git Changes")

	if got := app.statusBar.Sections(); len(got) != 0 {
		t.Fatalf("secciones = %v, deben limpiarse al desactivar", got)
	}
	if got := ed.Diagnostics(); len(got) != 0 {
		t.Fatalf("diagnósticos = %v, deben limpiarse al desactivar", got)
	}
}

// TestExtManagerToggleIntentToggles: la intención de la ventana llega al toggle
// del controlador.
func TestExtManagerToggleIntentToggles(t *testing.T) {
	app, _ := newToggleApp(t)

	app.handleExtIntent(view.ExtIntent{
		Kind: view.ExtIntentToggle,
		Tab:  view.ExtTabInstalled,
		Item: view.ExtItem{ID: "demo.gitchanges", Label: "Git Changes"},
	})

	if !app.disabledExts["demo.gitchanges"] {
		t.Fatal("la intención de toggle debe desactivar la extensión")
	}
}
