package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/view"
)

// TestAppLoadsThemeFromUserFile: el arranque lee ~/.tcode/theme.json y la
// paleta queda aplicada al App (y a las vistas vía SetTheme).
func TestAppLoadsThemeFromUserFile(t *testing.T) {
	dir := t.TempDir()
	old := themeFilePath
	themeFilePath = func() string { return filepath.Join(dir, "theme.json") }
	defer func() { themeFilePath = old }()

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
	dir := t.TempDir()
	old := themeFilePath
	themeFilePath = func() string { return filepath.Join(dir, "theme.json") }
	defer func() { themeFilePath = old }()

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
	dir := t.TempDir()
	old := themeFilePath
	themeFilePath = func() string { return filepath.Join(dir, "no-existe.json") }
	defer func() { themeFilePath = old }()

	app, _ := newTestApp(t, "uno")
	if app.theme != view.DefaultTheme() {
		t.Fatal("sin archivo de tema debe quedar el default")
	}
}
