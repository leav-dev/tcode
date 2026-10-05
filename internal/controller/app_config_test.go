package controller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/view"
)

// resetConfigVars fija las vars globales de configuración de view a sus
// defaults al inicio de cada test y las restaura al final (t.Cleanup): las
// vars son GLOBALES y un test que las mute contagiaria al siguiente.
func resetConfigVars(t *testing.T) {
	t.Helper()
	view.SetIndentSize(4)
	view.SetWordWrapEnabled(true)
	view.SetExplorerWidth(24)
	view.SetActiveThemeID("")
	t.Cleanup(func() {
		view.SetIndentSize(4)
		view.SetWordWrapEnabled(true)
		view.SetExplorerWidth(24)
		view.SetActiveThemeID("")
	})
}

// tmpConfigFile remapea configFilePath a un archivo del TempDir del test y lo
// restaura al final, con el mismo patrón que themeFilePath en theme_test.go.
// Devuelve la ruta del archivo de configuración.
func tmpConfigFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := configFilePath
	configFilePath = func() string { return filepath.Join(dir, "config.json") }
	t.Cleanup(func() { configFilePath = old })
	return filepath.Join(dir, "config.json")
}

// TestCtrlPTogglesTheConfigWindow: Ctrl+P abre la ventana flotante de
// configuración (también con el workspace vacío, como el explorador), una
// tecla ajena la cierra descartando, y ninguna de las dos cierra el editor.
// La tecla original era Ctrl+, pero el terminal del usuario la intercepta.
func TestCtrlPTogglesTheConfigWindow(t *testing.T) {
	resetConfigVars(t)
	app, _ := newTestApp(t, "uno")

	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone)); quit {
		t.Fatal("Ctrl+P no debe cerrar el editor")
	}
	if !app.configActive {
		t.Fatal("Ctrl+P debe abrir la ventana de configuración")
	}

	// Una tecla ajena la cierra descartando, sin cerrar el editor.
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)); quit {
		t.Fatal("la tecla ajena con la ventana abierta no debe cerrar el editor")
	}
	if app.configActive {
		t.Fatal("una tecla ajena debe cerrar la ventana de configuración")
	}

	// Ctrl+P de nuevo la abre y Escape la cierra.
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone)); quit {
		t.Fatal("Ctrl+P no debe cerrar el editor")
	}
	if !app.configActive {
		t.Fatal("Ctrl+P debe volver a abrir la ventana de configuración")
	}
	pressed := app.handleEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if app.configActive {
		t.Fatal("Escape debe cerrar la ventana de configuración")
	}
	if pressed {
		t.Fatal("Escape consumido por la ventana no debe cerrar el editor")
	}

	// Con el workspace vacío (modo explorador) también abre: la configuración
	// existe sin buffers.
	app2 := newExplorerApp(t, t.TempDir())
	if quit := app2.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone)); quit {
		t.Fatal("Ctrl+P sin buffers no debe cerrar el editor")
	}
	if !app2.configActive {
		t.Fatal("Ctrl+P debe abrir la configuración también con el workspace vacío")
	}
}

// TestConfigChangePersists: mutar una fila de la ventana aplica el ajuste a la
// config viva del editor y escribe config.json en ~/.tcode; los demás ajustes
// quedan intactos.
func TestConfigChangePersists(t *testing.T) {
	resetConfigVars(t)
	path := tmpConfigFile(t)
	app, _ := newTestApp(t, "uno")

	// Abrir y mover: Right en "Tab size" → indent 5.
	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	if !app.configActive {
		t.Fatal("Ctrl+P debe abrir la ventana de configuración")
	}
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); quit {
		t.Fatal("Right dentro de la ventana no debe cerrar el editor")
	}
	if got := view.IndentSize(); got != 5 {
		t.Fatalf("IndentSize() = %d tras Right, se esperaba 5", got)
	}
	if !app.configActive {
		t.Fatal("mutar una fila no debe cerrar la ventana de configuración")
	}
	if !view.WordWrapEnabled() || view.ExplorerWidth() != 24 {
		t.Fatal("mutar Tab size no debe tocar los demás ajustes")
	}

	// El cambio se persiste en config.json.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config.json debe escribirse al mutar: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer config.json: %v", err)
	}
	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config.json no parsea: %v", err)
	}
	if cfg.IndentUnit != 5 {
		t.Fatalf("IndentUnit en disco = %d, se esperaba 5", cfg.IndentUnit)
	}
	if cfg.WordWrap == nil || *cfg.WordWrap != true {
		t.Fatal("WordWrap debe persistir como true (el valor actual)")
	}
	if cfg.ExplorerWidth != 24 {
		t.Fatalf("ExplorerWidth en disco = %d, se esperaba 24", cfg.ExplorerWidth)
	}
}

// TestLoadConfigAppliesAtStartup: un config.json existente se aplica al
// arrancar (antes de que se use cualquier vista); un JSON roto no rompe el
// arranque y todo queda en los defaults.
func TestLoadConfigAppliesAtStartup(t *testing.T) {
	resetConfigVars(t)
	path := tmpConfigFile(t)

	if err := os.WriteFile(path, []byte(`{"IndentUnit": 8, "WordWrap": false, "ExplorerWidth": 32}`), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _ := newTestApp(t, "uno")
	if got := view.IndentSize(); got != 8 {
		t.Fatalf("IndentSize() = %d al arrancar con config, se esperaba 8", got)
	}
	if view.WordWrapEnabled() {
		t.Fatal("WordWrap debe arrancar en false (config aplicada)")
	}
	if got := view.ExplorerWidth(); got != 32 {
		t.Fatalf("ExplorerWidth() = %d al arrancar con config, se esperaba 32", got)
	}
	_ = app

	// JSON roto: el arranque sigue y quedan los defaults (los globals se
	// restauran a mano porque el JSON corrupto no los toca).
	if err := os.WriteFile(path, []byte("{ roto"), 0o644); err != nil {
		t.Fatal(err)
	}
	view.SetIndentSize(4)
	view.SetWordWrapEnabled(true)
	view.SetExplorerWidth(24)
	app2, _ := newTestApp(t, "uno")
	if view.IndentSize() != 4 || !view.WordWrapEnabled() || view.ExplorerWidth() != 24 {
		t.Fatal("JSON roto: el arranque debe quedar con los defaults")
	}
	_ = app2
}

// TestConfigThemeAppliesAtStartup: un config.json con "Theme" se aplica al
// arrancar: el id queda en la var del selector y el tema aplicado al App es
// exactamente la paleta del registry (comparar con == vale: Theme solo lleva
// estilos y colores).
func TestConfigThemeAppliesAtStartup(t *testing.T) {
	resetConfigVars(t)
	path := tmpConfigFile(t)
	if err := os.WriteFile(path, []byte(`{"IndentUnit":4,"WordWrap":true,"ExplorerWidth":24,"Theme":"dracula"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _ := newTestApp(t, "uno")
	if got := view.ActiveThemeID(); got != "dracula" {
		t.Fatalf("ActiveThemeID() = %q al arrancar con config, se esperaba \"dracula\"", got)
	}
	if app.theme != view.DraculaTheme() {
		t.Fatal("el tema aplicado debe ser exactamente la paleta Dracula")
	}
}

// TestConfigThemePersistsWhenCycled: ciclar la fila Theme aplica la paleta en
// vivo y persiste el id en config.json (solo con "Theme": "light" porque el
// resto de las filas no se tocó).
func TestConfigThemePersistsWhenCycled(t *testing.T) {
	resetConfigVars(t)
	path := tmpConfigFile(t)
	app, _ := newTestApp(t, "uno")

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	if !app.configActive {
		t.Fatal("Ctrl+P debe abrir la ventana de configuración")
	}
	// Navegar hasta la fila Theme (Down×3) y ciclar un paso.
	app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); quit {
		t.Fatal("Right dentro de la ventana no debe cerrar el editor")
	}
	if got := view.ActiveThemeID(); got != "light" {
		t.Fatalf("ActiveThemeID() = %q tras Right en Theme, se esperaba \"light\"", got)
	}
	if !app.configActive {
		t.Fatal("mutar la fila Theme no debe cerrar la ventana de configuración")
	}

	// El cambio se persiste en config.json, con el resto de los ajustes
	// intactos (los defaults de resetConfigVars).
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer config.json: %v", err)
	}
	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config.json no parsea: %v", err)
	}
	if cfg.Theme != "light" {
		t.Fatalf("Theme en disco = %q, se esperaba \"light\"", cfg.Theme)
	}
	if cfg.IndentUnit != 4 || cfg.ExplorerWidth != 24 {
		t.Fatalf("ciclar el tema no debe tocar los demás ajustes: indent=%d width=%d", cfg.IndentUnit, cfg.ExplorerWidth)
	}
}

// TestConfigWithoutThemeKeepsCustom: un config.json sin "Theme" no toca el
// selector (queda "", Custom) y el tema aplicado sigue siendo el Custom de
// theme.json —el id del config solo decide si el selector elige una paleta del
// registry, y el fallback Custom sobrevive.
func TestConfigWithoutThemeKeepsCustom(t *testing.T) {
	resetConfigVars(t)
	cpath := tmpConfigFile(t)
	if err := os.WriteFile(cpath, []byte(`{"IndentUnit":4}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Un theme.json válido para el Custom, con el mismo patrón de remapeo que
	// los tests de tema.
	dir := t.TempDir()
	old := themeFilePath
	themeFilePath = func() string { return filepath.Join(dir, "theme.json") }
	defer func() { themeFilePath = old }()
	content := []byte(`{"keyword": "#123456"}`)
	if err := os.WriteFile(filepath.Join(dir, "theme.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	app, _ := newTestApp(t, "uno")
	if got := view.ActiveThemeID(); got != "" {
		t.Fatalf("ActiveThemeID() = %q, se esperaba \"\" (Custom sin config Theme)", got)
	}
	if app.theme != view.LoadTheme(content) {
		t.Fatal("sin Theme en el config, el tema aplicado debe ser el Custom de theme.json")
	}
}
