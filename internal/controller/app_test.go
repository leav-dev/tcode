package controller

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

// typeString tipea s con los eventos que entrega un terminal real para los saltos:
// Enter llega como KeyEnter y el tab como KeyTab, no como runas.
func typeString(app *App, s string) {
	for _, r := range s {
		switch r {
		case '\n':
			press(app, tcell.KeyEnter)
		case '\t':
			press(app, tcell.KeyTab)
		default:
			typeRune(app, r)
		}
	}
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

func TestCtrlZUndoesTheLastEdit(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	typeRune(app, 'X')
	if got := app.model.GetContent(); got != "Xuno" {
		t.Fatalf("contenido = %q", got)
	}

	if quit := press(app, tcell.KeyCtrlZ); quit {
		t.Fatal("Ctrl+Z no debe cerrar el editor")
	}

	if got := app.model.GetContent(); got != "uno" {
		t.Fatalf("tras deshacer, contenido = %q, se esperaba %q", got, "uno")
	}
	if app.model.Modified() {
		t.Fatal("deshacer hasta el estado inicial debe dejar el documento limpio")
	}
}

func TestCtrlYRedoesTheEdit(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	typeRune(app, 'X')
	press(app, tcell.KeyCtrlZ)
	press(app, tcell.KeyCtrlY)

	if got := app.model.GetContent(); got != "Xuno" {
		t.Fatalf("tras rehacer, contenido = %q, se esperaba %q", got, "Xuno")
	}
	if !app.model.Modified() {
		t.Fatal("tras rehacer el documento debe quedar modificado")
	}
}

func TestCtrlShiftZAlsoRedoes(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	typeRune(app, 'X')
	press(app, tcell.KeyCtrlZ)

	// tcell reporta Ctrl+Shift+Z como KeyRune con ambos modificadores, no como
	// KeyCtrlZ, así que es un camino distinto al de Ctrl+Y.
	app.handleEvent(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModCtrl|tcell.ModShift))

	if got := app.model.GetContent(); got != "Xuno" {
		t.Fatalf("tras rehacer, contenido = %q, se esperaba %q", got, "Xuno")
	}
}

func TestUndoWithNothingToUndoIsHarmless(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	if quit := press(app, tcell.KeyCtrlZ); quit {
		t.Fatal("Ctrl+Z sin historial no debe cerrar el editor")
	}
	if got := app.model.GetContent(); got != "uno" {
		t.Fatalf("contenido = %q, se esperaba sin cambios", got)
	}
}

// TestUndoPlacesTheCursorAtTheChange comprueba de punta a punta que el cursor
// quede donde ocurrió el cambio, mirando la celda que reporta la pantalla.
func TestUndoPlacesTheCursorAtTheChange(t *testing.T) {
	app, _ := newTestApp(t, "uno\ndos")

	app.handleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))
	typeRune(app, 'Z')
	if got := app.model.GetContent(); got != "uno\ndosZ" {
		t.Fatalf("contenido = %q", got)
	}

	press(app, tcell.KeyCtrlZ)

	if got := app.model.GetContent(); got != "uno\ndos" {
		t.Fatalf("contenido = %q", got)
	}

	sim, ok := app.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("el test espera una pantalla simulada")
	}
	x, y, visible := sim.GetCursor()
	if !visible {
		t.Fatal("el cursor debe quedar visible")
	}
	if x != 3 || y != 1 {
		t.Fatalf("cursor en (%d,%d), se esperaba (3,1): el final de \"dos\"", x, y)
	}
}

func TestUndoThenEscapeQuitsWithoutAsking(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	typeRune(app, 'X')
	press(app, tcell.KeyCtrlZ)

	// El documento volvió al estado inicial, así que ya no hay nada que perder.
	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("tras deshacer todo, Escape debe cerrar sin pedir confirmación")
	}
}

// TestTypingCoalescesIntoOneUndoInTheEditor verifica la agrupación de punta a
// punta: escribir una palabra y un solo Ctrl+Z la borra entera.
func TestTypingCoalescesIntoOneUndoInTheEditor(t *testing.T) {
	app, _ := newTestApp(t, "")

	typeString(app, "hola")
	if got := app.model.GetContent(); got != "hola" {
		t.Fatalf("contenido = %q", got)
	}

	press(app, tcell.KeyCtrlZ)

	if got := app.model.GetContent(); got != "" {
		t.Fatalf("tras un Ctrl+Z, contenido = %q, se esperaba vacío", got)
	}
}

// TestMovingTheCursorEndsTheTypingGroup comprueba que el movimiento del cursor
// corte el grupo aunque se vuelva a la misma posición.
func TestMovingTheCursorEndsTheTypingGroup(t *testing.T) {
	app, _ := newTestApp(t, "xy")

	typeString(app, "ab")
	press(app, tcell.KeyLeft)
	press(app, tcell.KeyRight)
	typeString(app, "cd")

	if got := app.model.GetContent(); got != "abcdxy" {
		t.Fatalf("contenido = %q", got)
	}

	press(app, tcell.KeyCtrlZ)

	if got := app.model.GetContent(); got != "abxy" {
		t.Fatalf("tras un Ctrl+Z, contenido = %q, se esperaba %q", got, "abxy")
	}
}

// writeExternally reescribe el archivo como otro editor: temporal y renombre. El
// inodo cambia, así que nuestro mmap sigue viendo el contenido viejo.
func writeExternally(t *testing.T, path, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".externo-*")
	if err != nil {
		t.Fatalf("no se pudo crear el temporal: %v", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("no se pudo escribir: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("no se pudo cerrar: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(tmp.Name(), future, future); err != nil {
		t.Fatalf("no se pudo cambiar la fecha: %v", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		t.Fatalf("no se pudo renombrar: %v", err)
	}
}

// TestCtrlSWarnsBeforeOverwritingExternalChanges verifica la protección de punta a
// punta: un Ctrl+S no pisa el trabajo de otro proceso, avisa.
func TestCtrlSWarnsBeforeOverwritingExternalChanges(t *testing.T) {
	app, path := newTestApp(t, "uno")

	typeRune(app, 'X')

	const externo = "escrito por otro proceso"
	writeExternally(t, path, externo)

	press(app, tcell.KeyCtrlS)

	if got := readFile(t, path); got != externo {
		t.Fatalf("el archivo fue pisado con el primer Ctrl+S: %q", got)
	}
	if !app.forceSave {
		t.Fatal("debe quedar habilitado el forzado para el próximo Ctrl+S")
	}
	if !app.model.Modified() {
		t.Fatal("el documento sigue teniendo cambios sin guardar")
	}

	// El segundo Ctrl+S sí pisa, que es lo que el usuario pidió.
	press(app, tcell.KeyCtrlS)

	if got := readFile(t, path); got != "Xuno" {
		t.Fatalf("tras el segundo Ctrl+S, archivo = %q, se esperaba %q", got, "Xuno")
	}
	if app.model.Modified() {
		t.Fatal("tras forzar el guardado el documento queda limpio")
	}
}

// TestAnotherKeyCancelsTheForceSavePermission: seguir trabajando tiene que volver a
// pedir confirmación, para que un Ctrl+S lejano no pise sin avisar.
func TestAnotherKeyCancelsTheForceSavePermission(t *testing.T) {
	app, path := newTestApp(t, "uno")

	typeRune(app, 'X')
	writeExternally(t, path, "ajeno")

	press(app, tcell.KeyCtrlS)
	if !app.forceSave {
		t.Fatal("debe quedar habilitado el forzado")
	}

	press(app, tcell.KeyDown)
	if app.forceSave {
		t.Fatal("otra tecla debe cancelar el permiso de pisar")
	}

	// Y el próximo Ctrl+S vuelve a avisar en vez de pisar.
	press(app, tcell.KeyCtrlS)
	if got := readFile(t, path); got != "ajeno" {
		t.Fatalf("archivo = %q, no debía pisarse", got)
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
