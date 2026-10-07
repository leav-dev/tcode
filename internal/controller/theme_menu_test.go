package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/leav-dev/tcode/internal/ext"
	"github.com/leav-dev/tcode/internal/view"
)

// TestThemeWindowAppliesFromConfig: Enter sobre la fila Theme abre la ventana
// de temas, y Enter en la ventana aplica el tema del cursor y lo persiste.
func TestThemeWindowAppliesFromConfig(t *testing.T) {
	resetConfigVars(t)
	path := tmpConfigFile(t)
	view.RegisterExtensionThemes(nil)
	t.Cleanup(func() { view.RegisterExtensionThemes(nil) })
	app, _ := newTestApp(t, "uno")

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	if !app.configActive {
		t.Fatal("Ctrl+P debe abrir la configuración")
	}
	for i := 0; i < 3; i++ {
		app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.themeMenuActive {
		t.Fatal("Enter en Theme debe abrir la ventana de temas")
	}
	if app.configActive {
		t.Fatal("al abrir temas la configuración debe cerrarse")
	}
	// El cursor arranca en el activo ("light" tras ciclar? acá Custom=último):
	// subir al primer built-in y elegir.
	app.handleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.themeMenuActive {
		t.Fatal("Enter en la ventana debe cerrarla")
	}
	if got := view.ActiveThemeID(); got != "light" {
		t.Fatalf("ActiveThemeID() = %q, esperaba light", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer config.json: %v", err)
	}
	if !containsTheme(data, "light") {
		t.Fatalf("config.json no persiste light: %s", data)
	}
}

// TestExtensionThemesLoadIntoWindow: un tema aportado por una extensión
// habilitada aparece en la ventana de temas y se puede aplicar.
func TestExtensionThemesLoadIntoWindow(t *testing.T) {
	resetConfigVars(t)
	tmpConfigFile(t)
	view.RegisterExtensionThemes(nil)
	t.Cleanup(func() { view.RegisterExtensionThemes(nil) })

	dir := t.TempDir()
	extDir := filepath.Join(dir, "demo.temas")
	thDir := filepath.Join(extDir, "themes")
	if err := os.MkdirAll(thDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extension.json"), []byte(`{"id":"demo.temas","version":"1.0.0","contributes":{"themes":[{"id":"demo.rosa","label":"Rosa","file":"themes/rosa.json"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(thDir, "rosa.json"), []byte(`{"keyword":"#ff79c6"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	found, errs := ext.Discover(dir)
	if len(errs) != 0 || len(found) != 1 {
		t.Fatalf("discover = %d exts %+v errs", len(found), errs)
	}
	app, _ := newTestApp(t, "uno")
	app.refreshExtensionThemes(found)
	// La ventana lista el tema de la extensión.
	app.openThemeMenu()
	defer func() { app.themeMenuActive = false }()
	foundTheme := false
	for _, it := range app.themeMenu.Items() {
		if it.ID == "demo.rosa" {
			foundTheme = true
		}
	}
	if !foundTheme {
		t.Fatalf("la ventana no lista demo.rosa: %+v", app.themeMenu.Items())
	}
	// themeFor resuelve la paleta de la extensión.
	view.SetActiveThemeID("demo.rosa")
	if app.themeFor() != view.LoadTheme([]byte(`{"keyword":"#ff79c6"}`)) {
		t.Fatal("themeFor debe resolver la paleta de la extensión")
	}
	view.SetActiveThemeID("")
}

func containsTheme(data []byte, id string) bool {
	return len(data) > 0 && stringContains(string(data), `"`+id+`"`)
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
