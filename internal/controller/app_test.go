package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func newTestApp(t *testing.T, content string) (*App, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "doc.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	s.SetSize(40, 10)

	app, err := NewAppWithScreen(s, path)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	t.Cleanup(func() {
		app.model.Close()
		s.Fini()
	})
	return app, path
}

func typeRune(app *App, r rune) {
	app.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
}

func press(app *App, key tcell.Key) bool {
	return app.handleEvent(tcell.NewEventKey(key, 0, tcell.ModNone))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", path, err)
	}
	return string(b)
}

func TestCtrlSSavesTheDocument(t *testing.T) {
	app, path := newTestApp(t, "uno")

	typeRune(app, 'X')
	if !app.model.Modified() {
		t.Fatal("tras escribir el documento debe quedar modificado")
	}

	if quit := press(app, tcell.KeyCtrlS); quit {
		t.Fatal("Ctrl+S no debe cerrar el editor")
	}

	if got := readFile(t, path); got != "Xuno" {
		t.Fatalf("archivo en disco = %q, se esperaba %q", got, "Xuno")
	}
	if app.model.Modified() {
		t.Fatal("tras Ctrl+S el documento no debe quedar modificado")
	}
}

func TestEscapeOnUnmodifiedDocumentQuits(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("Escape sin cambios debe cerrar el editor")
	}
}

func TestEscapeWithUnsavedChangesAsksFirst(t *testing.T) {
	app, path := newTestApp(t, "uno")

	typeRune(app, 'X')

	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("Escape con cambios sin guardar no debe cerrar: primero avisa")
	}
	if !app.confirmQuit {
		t.Fatal("debe quedar pendiente la confirmación de salida")
	}
	// El trabajo no se perdió y nada se escribió a disco.
	if got := app.model.GetContent(); got != "Xuno" {
		t.Fatalf("contenido en memoria = %q", got)
	}
	if got := readFile(t, path); got != "uno" {
		t.Fatalf("el archivo no debía escribirse todavía: %q", got)
	}
}

func TestSecondEscapeQuitsWithoutSaving(t *testing.T) {
	app, path := newTestApp(t, "uno")

	typeRune(app, 'X')
	press(app, tcell.KeyEscape)

	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("la segunda vez Escape debe cerrar sin guardar")
	}
	if got := readFile(t, path); got != "uno" {
		t.Fatalf("salir sin guardar no debe tocar el archivo: %q", got)
	}
}

func TestAnotherKeyCancelsThePendingQuitConfirmation(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	typeRune(app, 'X')
	press(app, tcell.KeyEscape)
	if !app.confirmQuit {
		t.Fatal("debe quedar pendiente la confirmación")
	}

	// Seguir trabajando cancela la salida pendiente: si no, la próxima Escape
	// cerraría sin avisar.
	press(app, tcell.KeyDown)
	if app.confirmQuit {
		t.Fatal("otra tecla debe cancelar la confirmación pendiente")
	}

	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("tras cancelar, Escape debe volver a avisar en vez de cerrar")
	}
}

func TestCtrlSSavesAndThenEscapeQuits(t *testing.T) {
	app, path := newTestApp(t, "uno")

	typeRune(app, 'X')
	press(app, tcell.KeyCtrlS)

	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("tras guardar, Escape debe cerrar sin preguntar")
	}
	if got := readFile(t, path); got != "Xuno" {
		t.Fatalf("archivo en disco = %q, se esperaba %q", got, "Xuno")
	}
}

func TestSaveErrorIsReportedAndKeepsTheDocumentDirty(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("corriendo como root: los permisos de solo lectura no aplican")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(path, []byte("uno"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla: %v", err)
	}
	s.SetSize(40, 10)
	app, err := NewAppWithScreen(s, path)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	defer func() { app.model.Close(); s.Fini() }()

	typeRune(app, 'X')

	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Chmod falló: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	press(app, tcell.KeyCtrlS)

	if !app.model.Modified() {
		t.Fatal("un guardado fallido no debe limpiar el estado modificado")
	}
	if got := readFile(t, path); got != "uno" {
		t.Fatalf("el archivo original cambió: %q", got)
	}
	// Y salir debe seguir pidiendo confirmación.
	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("con cambios sin guardar, Escape no debe cerrar")
	}
}

func TestResizeKeepsTheStatusRow(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	s, ok := app.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("el test espera una pantalla simulada")
	}
	s.SetSize(30, 5)
	app.handleEvent(tcell.NewEventResize(30, 5))

	// No debe entrar en pánico y la barra tiene que caber en la última fila.
	app.redraw()
}

func TestEditorHeightReservesTheStatusRow(t *testing.T) {
	for _, tc := range []struct {
		height, want int
	}{
		{10, 9},
		{2, 1},
		{1, 1},
		{0, 1},
	} {
		if got := editorHeight(tc.height); got != tc.want {
			t.Fatalf("editorHeight(%d) = %d, se esperaba %d", tc.height, got, tc.want)
		}
	}
}
