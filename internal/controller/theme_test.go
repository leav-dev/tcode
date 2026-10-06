package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/leav-dev/tcode/internal/view"
)

// TestAppLoadsThemeFromUserFile: el arranque lee ~/.tcode/theme.json y la
// paleta queda aplicada al App (y a las vistas vía SetTheme).
func TestAppLoadsThemeFromUserFile(t *testing.T) {
	resetConfigVars(t) // el id de tema es una global: otros tests de config pueden dejarla en otra paleta
	dir := t.TempDir()
	old := themeFilePath
	themeFilePath = func() string { return filepath.Join(dir, "theme.json") }
	defer func() { themeFilePath = old }()
	// El arranque también lee ~/.tcode/config.json, y el config REAL del
	// usuario puede llevar una clave Theme que pisaría el theme.json del test:
	// se pin a un path inexistente para que no se cuele nada.
	oldCfg := configFilePath
	configFilePath = func() string { return filepath.Join(dir, "no-config.json") }
	defer func() { configFilePath = oldCfg }()

	if err := os.WriteFile(filepath.Join(dir, "theme.json"), []byte(`{"keyword": "#ff0000"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	app, _ := newTestApp(t, "uno")
	if fg, _, _ := app.theme.Keyword.Decompose(); fg != tcell.NewHexColor(0xff0000) {
		t.Fatalf("keyword = %v, esperaba #ff0000", fg)
	}
	if app.theme == view.DefaultTheme() {
		t.Fatal("el tema cargado no puede ser idéntico al default")
	}
}

// TestAppFallsBackToDefaultTheme: un theme.json roto no rompe el arranque y la
// paleta queda en la default.
func TestAppFallsBackToDefaultTheme(t *testing.T) {
	resetConfigVars(t)
	dir := t.TempDir()
	old := themeFilePath
	themeFilePath = func() string { return filepath.Join(dir, "theme.json") }
	defer func() { themeFilePath = old }()
	// Pin de configFilePath: el config real del usuario no debe aplicar su
	// Theme en un test que verifica el fallback del theme.json.
	oldCfg := configFilePath
	configFilePath = func() string { return filepath.Join(dir, "no-config.json") }
	defer func() { configFilePath = oldCfg }()

	if err := os.WriteFile(filepath.Join(dir, "theme.json"), []byte("{ roto"), 0o644); err != nil {
		t.Fatal(err)
	}

	app, _ := newTestApp(t, "uno")
	if app.theme.Keyword != view.DefaultTheme().Keyword {
		t.Fatalf("con JSON roto debe quedar el default de keyword")
	}
}

// TestAppNoThemeFileUsesDefault: sin archivo de tema, el arranque es el default.
func TestAppNoThemeFileUsesDefault(t *testing.T) {
	resetConfigVars(t)
	dir := t.TempDir()
	old := themeFilePath
	themeFilePath = func() string { return filepath.Join(dir, "no-existe.json") }
	defer func() { themeFilePath = old }()
	// Pin de configFilePath: el Theme del config real del usuario (si existe)
	// no debe pisar el default que verifica este test.
	oldCfg := configFilePath
	configFilePath = func() string { return filepath.Join(dir, "no-config.json") }
	defer func() { configFilePath = oldCfg }()

	app, _ := newTestApp(t, "uno")
	if app.theme != view.DefaultTheme() {
		t.Fatal("sin archivo de tema debe quedar el default")
	}
}
