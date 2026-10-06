package controller

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/ext"
	"tcode/internal/model"
	"tcode/internal/view"
)

// TestMain aísla la suite del entorno REAL del usuario: ni ~/.tcode/config.json
// ni ~/.tcode/theme.json entran a los tests (son las dos rutas que el arranque
// lee) y la lectura de extensiones del arranque queda apuntando a un root
// inexistente —sin proveedores, sin red, sin HOME real—; las variables globales
// de view arrancan en sus defaults. Los tests que necesitan un config, tema o
// proveedor propio los remapean explícitamente (ver app_config_test.go,
// theme_test.go y pinStartupSources) — ese remapeo corre después de TestMain,
// con precedencia sobre los pines que quedan acá.
func TestMain(m *testing.M) {
	themeFilePath = func() string { return filepath.Join(os.TempDir(), "tcode-test-no-theme.json") }
	configFilePath = func() string { return filepath.Join(os.TempDir(), "tcode-test-no-config.json") }
	extensionUserSources = func() ([]ext.Provider, string, error) {
		return nil, filepath.Join(os.TempDir(), "tcode-test-no-extensions"), nil
	}
	view.SetIndentSize(4)
	view.SetWordWrapEnabled(true)
	view.SetExplorerWidth(24)
	view.SetActiveThemeID("")
	os.Exit(m.Run())
}

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
		app.ws.CloseAll()
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
	if !app.ws.Active().Modified() {
		t.Fatal("tras escribir el documento debe quedar modificado")
	}

	if quit := press(app, tcell.KeyCtrlS); quit {
		t.Fatal("Ctrl+S no debe cerrar el editor")
	}

	if got := readFile(t, path); got != "Xuno" {
		t.Fatalf("archivo en disco = %q, se esperaba %q", got, "Xuno")
	}
	if app.ws.Active().Modified() {
		t.Fatal("tras Ctrl+S el documento no debe quedar modificado")
	}
}

func TestSingleEscapeDoesNotQuit(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	// La ÚNICA salida es la doble presión rápida de Escape: un solo Escape
	// (con o sin cambios) nunca cierra.
	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("un solo Escape no debe cerrar el editor: se necesita doble rápido")
	}
	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("el segundo Escape rápido debe cerrar el editor")
	}
}

// TestDoubleEscapeMustBeQuick: la doble presión de Escape solo cierra dentro de
// la ventana (quitEscapeWindow). Con el reloj inyectado: un segundo Escape
// dentro de la ventana cierra; pasado el umbral no cierra y el conteo se
// reinicia (el siguiente rápido sí cierra). Sin cambios sin guardar también.
func TestDoubleEscapeMustBeQuick(t *testing.T) {
	oldClock := clockNow
	oldWin := quitEscapeWindow
	quitEscapeWindow = 500 * time.Millisecond
	now := time.Unix(0, 0).Add(time.Hour)
	clockNow = func() time.Time { return now }
	defer func() { clockNow, quitEscapeWindow = oldClock, oldWin }()

	app, _ := newTestApp(t, "uno")

	// Primer Escape en t, segundo a t+400ms: cierra.
	press(app, tcell.KeyEscape)
	now = now.Add(400 * time.Millisecond)
	clockNow = func() time.Time { return now }
	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("el segundo Escape dentro de la ventana debe cerrar")
	}

	// Reinicio: primer Escape en t2, segundo a t2+600ms (> ventana): NO cierra
	// y el tercero a t2+700ms (rápido respecto del segundo) sí.
	now = now.Add(time.Second)
	clockNow = func() time.Time { return now }
	press(app, tcell.KeyEscape) // primer press
	now = now.Add(600 * time.Millisecond)
	clockNow = func() time.Time { return now }
	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("un segundo Escape fuera de la ventana no debe cerrar")
	}
	now = now.Add(50 * time.Millisecond)
	clockNow = func() time.Time { return now }
	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("el Escape siguiente, rápido, debe cerrar (el conteo se reinició)")
	}
}

// TestCtrlCNoLongerQuits: la única salida es el doble Escape; Ctrl+C ya no
// cierra el editor (la tecla cae al flujo normal) y no toca el documento.
func TestCtrlCNoLongerQuits(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	if quit := press(app, tcell.KeyCtrlC); quit {
		t.Fatal("Ctrl+C no debe cerrar el editor")
	}
	if quit := press(app, tcell.KeyCtrlC); quit {
		t.Fatal("ni dos veces seguidas: la única salida es el doble Escape")
	}
	if got := app.ws.Active().GetContent(); got != "uno" {
		t.Fatalf("el documento no debe haberse tocado: %q", got)
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
	if got := app.ws.Active().GetContent(); got != "Xuno" {
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

	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("tras guardar, un solo Escape no cierra: doble rápido")
	}
	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("tras guardar, el segundo Escape rápido debe cerrar sin preguntar")
	}
	if got := readFile(t, path); got != "Xuno" {
		t.Fatalf("archivo en disco = %q, se esperaba %q", got, "Xuno")
	}
}

func TestSaveErrorIsReportedAndKeepsTheDocumentDirty(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("corriendo como root: los permisos de solo lectura no aplican")
	}
	if runtime.GOOS == "windows" {
		// Go no modela el chmod de solo lectura de un directorio en Windows
		// (ver TestSaveLeavesTheOriginalIntactOnFailure); este test pasaba acá
		// por el bug ya corregido del rename sobre archivo mapeado.
		t.Skip("chmod de solo lectura de directorios no aplica en Windows")
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
	defer func() { app.ws.CloseAll(); s.Fini() }()

	typeRune(app, 'X')

	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Chmod falló: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	press(app, tcell.KeyCtrlS)

	if !app.ws.Active().Modified() {
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
	if got := app.ws.Active().GetContent(); got != "Xuno" {
		t.Fatalf("contenido = %q", got)
	}

	if quit := press(app, tcell.KeyCtrlZ); quit {
		t.Fatal("Ctrl+Z no debe cerrar el editor")
	}

	if got := app.ws.Active().GetContent(); got != "uno" {
		t.Fatalf("tras deshacer, contenido = %q, se esperaba %q", got, "uno")
	}
	if app.ws.Active().Modified() {
		t.Fatal("deshacer hasta el estado inicial debe dejar el documento limpio")
	}
}

func TestCtrlYRedoesTheEdit(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	typeRune(app, 'X')
	press(app, tcell.KeyCtrlZ)
	press(app, tcell.KeyCtrlY)

	if got := app.ws.Active().GetContent(); got != "Xuno" {
		t.Fatalf("tras rehacer, contenido = %q, se esperaba %q", got, "Xuno")
	}
	if !app.ws.Active().Modified() {
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

	if got := app.ws.Active().GetContent(); got != "Xuno" {
		t.Fatalf("tras rehacer, contenido = %q, se esperaba %q", got, "Xuno")
	}
}

func TestUndoWithNothingToUndoIsHarmless(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	if quit := press(app, tcell.KeyCtrlZ); quit {
		t.Fatal("Ctrl+Z sin historial no debe cerrar el editor")
	}
	if got := app.ws.Active().GetContent(); got != "uno" {
		t.Fatalf("contenido = %q, se esperaba sin cambios", got)
	}
}

// TestUndoPlacesTheCursorAtTheChange comprueba de punta a punta que el cursor
// quede donde ocurrió el cambio, mirando la celda que reporta la pantalla.
func TestUndoPlacesTheCursorAtTheChange(t *testing.T) {
	app, _ := newTestApp(t, "uno\ndos")

	app.handleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))
	typeRune(app, 'Z')
	if got := app.ws.Active().GetContent(); got != "uno\ndosZ" {
		t.Fatalf("contenido = %q", got)
	}

	press(app, tcell.KeyCtrlZ)

	if got := app.ws.Active().GetContent(); got != "uno\ndos" {
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
	// La columna 3 de \ "dos\ " vive en la celda gutterWidth+3 (gutter de 2).
	if x != 5 || y != 2 {
		t.Fatalf("cursor en (%d,%d), se esperaba (5,2): el final de \"dos\" en la fila del editor", x, y)
	}
}

func TestUndoThenEscapeQuitsWithoutAsking(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	typeRune(app, 'X')
	press(app, tcell.KeyCtrlZ)

	// El documento volvió al estado inicial, así que ya no hay nada que perder.
	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("tras deshacer todo, un solo Escape no cierra: doble rápido")
	}
	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("tras deshacer todo, el segundo Escape rápido debe cerrar sin pedir confirmación")
	}
}

// TestTypingCoalescesIntoOneUndoInTheEditor verifica la agrupación de punta a
// punta: escribir una palabra y un solo Ctrl+Z la borra entera.
func TestTypingCoalescesIntoOneUndoInTheEditor(t *testing.T) {
	app, _ := newTestApp(t, "")

	typeString(app, "hola")
	if got := app.ws.Active().GetContent(); got != "hola" {
		t.Fatalf("contenido = %q", got)
	}

	press(app, tcell.KeyCtrlZ)

	if got := app.ws.Active().GetContent(); got != "" {
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

	if got := app.ws.Active().GetContent(); got != "abcdxy" {
		t.Fatalf("contenido = %q", got)
	}

	press(app, tcell.KeyCtrlZ)

	if got := app.ws.Active().GetContent(); got != "abxy" {
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
	if !app.activeForceSave() {
		t.Fatal("debe quedar habilitado el forzado para el próximo Ctrl+S")
	}
	if !app.ws.Active().Modified() {
		t.Fatal("el documento sigue teniendo cambios sin guardar")
	}

	// El segundo Ctrl+S sí pisa, que es lo que el usuario pidió.
	press(app, tcell.KeyCtrlS)

	if got := readFile(t, path); got != "Xuno" {
		t.Fatalf("tras el segundo Ctrl+S, archivo = %q, se esperaba %q", got, "Xuno")
	}
	if app.ws.Active().Modified() {
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
	if !app.activeForceSave() {
		t.Fatal("debe quedar habilitado el forzado")
	}

	press(app, tcell.KeyDown)
	if app.activeForceSave() {
		t.Fatal("otra tecla debe cancelar el permiso de pisar")
	}

	// Y el próximo Ctrl+S vuelve a avisar en vez de pisar.
	press(app, tcell.KeyCtrlS)
	if got := readFile(t, path); got != "ajeno" {
		t.Fatalf("archivo = %q, no debía pisarse", got)
	}
}

// --- Save As ---

// pressSaveAs dispara Ctrl+Shift+S, que tcell entrega como KeyRune con ModCtrl y
// ModShift en lugar del código KeyCtrlS.
func pressSaveAs(app *App) {
	app.handleEvent(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModCtrl|tcell.ModShift))
}

func TestSaveAsPromptOpensPrefilledWithTheCurrentPath(t *testing.T) {
	app, path := newTestApp(t, "uno")

	pressSaveAs(app)

	if !app.promptActive {
		t.Fatal("debe quedar abierto el pedido de ruta")
	}
	if app.promptBuf != path {
		t.Fatalf("buffer = %q, se esperaba la ruta actual %q", app.promptBuf, path)
	}
	if !strings.Contains(app.statusBar.Label(), "Guardar como:") {
		t.Fatalf("la barra = %q, se esperaba el pedido", app.statusBar.Label())
	}
}

// TestSaveAsPromptDoesNotEditTheDocument: con el pedido abierto el teclado es del
// pedido, no del documento.
func TestSaveAsPromptDoesNotEditTheDocument(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	pressSaveAs(app)
	typeRune(app, 'X')
	typeRune(app, 'Y')

	if got := app.ws.Active().GetContent(); got != "uno" {
		t.Fatalf("el documento no debía cambiar: %q", got)
	}
	if !strings.HasSuffix(app.promptBuf, "XY") {
		t.Fatalf("el texto debía ir al pedido: %q", app.promptBuf)
	}
}

func TestSaveAsPromptAcceptsATypedPath(t *testing.T) {
	app, original := newTestApp(t, "uno")

	dest := filepath.Join(t.TempDir(), "copia.txt")

	pressSaveAs(app)
	// Limpia la ruta prellenada y escribe la nueva.
	for range []rune(app.promptBuf) {
		press(app, tcell.KeyBackspace)
	}
	for _, r := range dest {
		typeRune(app, r)
	}
	press(app, tcell.KeyEnter)

	if app.promptActive {
		t.Fatal("el pedido debía cerrarse con Enter")
	}
	if got := readFile(t, dest); got != "uno" {
		t.Fatalf("destino = %q, se esperaba %q", got, "uno")
	}
	if got := app.ws.Active().Path(); got != dest {
		t.Fatalf("Path() = %q, se esperaba %q", got, dest)
	}
	if got := readFile(t, original); got != "uno" {
		t.Fatalf("el original no debía tocarse: %q", got)
	}
}

func TestSaveAsPromptCancelsWithEscape(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	dest := filepath.Join(t.TempDir(), "no-debe-existir.txt")

	pressSaveAs(app)
	for range []rune(app.promptBuf) {
		press(app, tcell.KeyBackspace)
	}
	for _, r := range dest {
		typeRune(app, r)
	}
	press(app, tcell.KeyEscape)

	if app.promptActive {
		t.Fatal("Escape debe cerrar el pedido")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("el archivo no debía crearse: %v", err)
	}
}

func TestSaveAsPromptWithEmptyPathDoesNothing(t *testing.T) {
	app, path := newTestApp(t, "uno")

	pressSaveAs(app)
	for range []rune(app.promptBuf) {
		press(app, tcell.KeyBackspace)
	}
	press(app, tcell.KeyEnter)

	if app.promptActive {
		t.Fatal("el pedido debe cerrarse")
	}
	if got := app.ws.Active().Path(); got != path {
		t.Fatalf("la ruta no debía cambiar: %q", got)
	}
	if got := readFile(t, path); got != "uno" {
		t.Fatalf("archivo = %q", got)
	}
}

// TestSaveAsForADocumentWithoutPath cubre el caso que motivó la unidad, de punta a
// punta: abrir el editor sin archivo, escribir y guardar en una ruta nueva.
func TestSaveAsForADocumentWithoutPath(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla: %v", err)
	}
	s.SetSize(40, 10)

	app, err := NewAppWithScreen(s, "")
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	defer func() { app.ws.CloseAll(); s.Fini() }()

	// El arranque sin argumentos ya no crea un documento (decisión A→B1): el
	// explorador es el camino de entrada y el workspace queda vacío. Este test
	// es sobre Save As, así que el buffer sin ruta se crea explícitamente,
	// como haría el usuario al abrirlo. Las aserciones de abajo no cambian.
	app.ws.NewUntitled()

	typeString(app, "contenido nuevo")

	dest := filepath.Join(t.TempDir(), "creado.txt")
	pressSaveAs(app)
	for range []rune(app.promptBuf) {
		press(app, tcell.KeyBackspace)
	}
	for _, r := range dest {
		typeRune(app, r)
	}
	press(app, tcell.KeyEnter)

	if got := readFile(t, dest); got != "contenido nuevo" {
		t.Fatalf("destino = %q, se esperaba %q", got, "contenido nuevo")
	}
	if app.ws.Active().Modified() {
		t.Fatal("tras guardar como, el documento queda limpio")
	}
}

// --- pruebas multi-buffer ---

// newTwoBufferApp abre dos archivos como dos pestañas. Devuelve la app, las
// rutas en orden de apertura y el workspace con el buffer 0 activo.
func newTwoBufferApp(t *testing.T, c0, c1 string) (*App, string, string) {
	t.Helper()

	dir := t.TempDir()
	path0 := filepath.Join(dir, "a.txt")
	path1 := filepath.Join(dir, "b.txt")
	for path, c := range map[string]string{path0: c0, path1: c1} {
		if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
			t.Fatalf("no se pudo crear el archivo: %v", err)
		}
	}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	s.SetSize(40, 10)

	app, err := NewAppWithScreen(s, path0)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	if _, err := app.ws.Open(path1); err != nil {
		t.Fatalf("Open falló: %v", err)
	}
	app.ws.SetActive(0)
	t.Cleanup(func() {
		app.ws.CloseAll()
		s.Fini()
	})
	return app, path0, path1
}

// bufferAt devuelve el buffer del workspace en el índice i.
func bufferAt(app *App, i int) *model.PieceTable {
	return app.ws.Buffers()[i]
}

// TestCtrlSSavesOnlyTheActiveBuffer: guardar no puede arrastrar los cambios de
// otras pestañas a sus archivos.
func TestCtrlSSavesOnlyTheActiveBuffer(t *testing.T) {
	app, path0, path1 := newTwoBufferApp(t, "uno", "dos")

	// Dejar sucio el buffer 1: si Ctrl+S guardara todos los buffers abiertos, su
	// archivo en disco tendría que cambiar y este test lo tiene que ver.
	if err := app.ws.SetActive(1); err != nil {
		t.Fatalf("SetActive falló: %v", err)
	}
	typeRune(app, 'Y')
	if !bufferAt(app, 1).Modified() {
		t.Fatal("el buffer 1 debía quedar sucio")
	}

	// Volver al buffer 0, editarlo y guardarlo.
	if err := app.ws.SetActive(0); err != nil {
		t.Fatalf("SetActive falló: %v", err)
	}
	typeRune(app, 'X')
	if quit := press(app, tcell.KeyCtrlS); quit {
		t.Fatal("Ctrl+S no debe cerrar el editor")
	}

	if got := readFile(t, path0); got != "Xuno" {
		t.Fatalf("buffer 0 en disco = %q, se esperaba %q", got, "Xuno")
	}
	// La prueba de que no se guardó todo: el archivo del buffer 1 sigue intacto y
	// su buffer sigue sucio.
	if got := readFile(t, path1); got != "dos" {
		t.Fatalf("Ctrl+S escribió el buffer 1, que no era el activo: %q", got)
	}
	if !bufferAt(app, 1).Modified() {
		t.Fatal("el buffer 1 no debía quedar limpio: no se guardó")
	}
}

// TestForceSavePermissionDoesNotCrossBuffers: el permiso de pisar cambios externos
// se da para UN archivo. Autorizar el buffer 0 no puede autorizar el 1, ni heredar
// la autorización por cambiar de pestaña.
func TestForceSavePermissionDoesNotCrossBuffers(t *testing.T) {
	app, path0, path1 := newTwoBufferApp(t, "uno", "dos")

	// Cambio externo en el buffer 0: el primer Ctrl+S avisa y arma el permiso.
	typeRune(app, 'X')
	writeExternally(t, path0, "ajeno")
	press(app, tcell.KeyCtrlS)

	if !app.activeForceSave() {
		t.Fatal("el buffer 0 debía quedar autorizado a pisar")
	}

	// Cambiar de pestaña no hereda la autorización.
	if err := app.ws.SetActive(1); err != nil {
		t.Fatalf("SetActive falló: %v", err)
	}
	if app.activeForceSave() {
		t.Fatal("el buffer 1 heredó el permiso de pisar del buffer 0")
	}

	// Y de punta a punta: con el buffer 1 también cambiado en disco, su Ctrl+S
	// tiene que volver a avisar en lugar de pisar.
	typeRune(app, 'Y')
	writeExternally(t, path1, "también ajeno")
	press(app, tcell.KeyCtrlS)

	if got := readFile(t, path1); got != "también ajeno" {
		t.Fatalf("el buffer 1 pisó cambios externos sin avisar: %q", got)
	}
}

// TestEscapeWarnsWhenAnyBufferIsDirty: la confirmación de salida es sobre TODO
// el workspace, no solo la pestaña visible.
func TestEscapeWarnsWhenAnyBufferIsDirty(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")

	// Editar el buffer 0 y cambiar a la pestaña del buffer 1.
	typeRune(app, 'X')
	if err := app.ws.SetActive(1); err != nil {
		t.Fatalf("SetActive falló: %v", err)
	}

	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("Escape con un buffer sucio en otra pestaña no debe cerrar: primero avisa")
	}
	// El buffer activo (el 1) está limpio: si la confirmación mirara solo el
	// activo, Escape habría cerrado acá.
	if bufferAt(app, 1).Modified() {
		t.Fatal("el buffer activo no debía estar sucio")
	}
	// El aviso tiene que quedar DIBUJADO en la barra, no solo existir como
	// mensaje: la regla vieja lo ocultaba si no entraba al lado de la etiqueta,
	// y un aviso invisible hace que el Escape parezca no validar nada.
	if got := statusRow(app); !strings.Contains(got, "Cambios sin guardar") {
		t.Fatalf("la fila de estado = %q, se esperaba el aviso de cambios sin guardar dibujado", got)
	}
}

// TestTheQuitWarningSurvivesANarrowTerminal: con la terminal angosta el aviso
// de la confirmación de salida se recorta con '…' pero NUNCA desaparece —la
// barra le da prioridad al mensaje por sobre la etiqueta—: es lo que hacía que
// un Escape pudiera parecer que no validaba nada.
func TestTheQuitWarningSurvivesANarrowTerminal(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	resizeApp(app, 30, 6)
	typeRune(app, 'X')

	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("Escape con cambios sin guardar no debe cerrar: primero avisa")
	}
	if got := statusRow(app); !strings.Contains(got, "Cambios sin guardar") {
		t.Fatalf("la fila de estado = %q, se esperaba el aviso visible aunque sea recortado", got)
	}
}

// TestSaveAsTargetsTheBufferCapturedAtPromptOpen: el destino es el buffer que
// estaba activo al abrir el pedido, no el que esté activo al apretar Enter.
func TestSaveAsTargetsTheBufferCapturedAtPromptOpen(t *testing.T) {
	app, _, path1 := newTwoBufferApp(t, "uno", "dos")

	dest := filepath.Join(t.TempDir(), "copia.txt")

	// El pedido se abre con el buffer 0 activo.
	pressSaveAs(app)
	if !app.promptActive || app.promptAction == nil {
		t.Fatal("el pedido debe abrirse armado con la acción que captura el buffer")
	}
	if app.promptBuf != bufferAt(app, 0).Path() {
		t.Fatalf("prefill = %q, se esperaba la ruta del buffer 0", app.promptBuf)
	}

	// Cambiar de pestaña MIENTRAS el pedido está abierto.
	if err := app.ws.SetActive(1); err != nil {
		t.Fatalf("SetActive falló: %v", err)
	}

	// Limpiar la ruta prellenada y escribir el destino.
	for range []rune(app.promptBuf) {
		press(app, tcell.KeyBackspace)
	}
	for _, r := range dest {
		typeRune(app, r)
	}
	press(app, tcell.KeyEnter)

	if got := readFile(t, dest); got != "uno" {
		t.Fatalf("destino = %q, se esperaba el contenido del buffer 0: %q", got, "uno")
	}
	if got := bufferAt(app, 0).Path(); got != dest {
		t.Fatalf("buffer 0 Path() = %q, se esperaba %q", got, dest)
	}
	if bufferAt(app, 0).Modified() {
		t.Fatal("el buffer 0 debe quedar limpio tras su Save As")
	}
	if got := bufferAt(app, 1).Path(); got != path1 {
		t.Fatalf("buffer 1 Path() = %q, no debía cambiar", got)
	}
}

// TestResizeUpdatesEveryEditor: el resize alcanza a todas las vistas, no solo a
// la del buffer activo. Una terminal que cambia de tamaño cambia el viewport de
// todos los buffers; si solo se redimensionara el activo, la vista de otro buffer
// dibujaría con el alto viejo al volver a esa pestaña.
func TestResizeUpdatesEveryEditor(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")

	// Forzar la creación de las dos vistas: cambiar de pestaña y dejar que el
	// dibujo de cada una las materialice.
	app.redraw()
	if err := app.ws.SetActive(1); err != nil {
		t.Fatalf("SetActive falló: %v", err)
	}
	app.redraw()
	if len(app.editors) != 2 {
		t.Fatalf("editors = %d, se esperaban 2", len(app.editors))
	}

	s, ok := app.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("el test espera una pantalla simulada")
	}
	s.SetSize(30, 5)
	app.handleEvent(tcell.NewEventResize(30, 5))

	for buf, ed := range app.editors {
		if ed == nil {
			t.Fatalf("la vista de %v es nil", buf)
		}
		// El área de texto descuenta el gutter (2 columnas para buffers de 1
		// línea): 30 de widget → 28 de texto.
		if w, h := ed.Size(); w != 28 || h != 3 {
			t.Fatalf("la vista de %v quedó con %dx%d, se esperaba 28x3", buf, w, h)
		}
	}
}

// TestResizeKeepsTheStatusRow comprueba que tras un resize el dibujo no entra
// en pánico y la barra tiene que caber en la última fila.
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

// TestEmptyWorkspaceDoesNotPanic: con el workspace vacío, Active() es nil. Ni
// el dibujo ni el teclado pueden entrar en pánico; salir sigue siendo posible.
func TestEmptyWorkspaceDoesNotPanic(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla: %v", err)
	}
	s.SetSize(40, 10)

	app, err := NewAppWithScreen(s, "")
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	defer func() { app.ws.CloseAll(); s.Fini() }()

	// Vaciar el workspace de todos los buffers.
	app.ws.CloseAll()

	app.redraw()
	if got := app.statusBar.Label(); got == "" {
		t.Fatal("la barra de estado debe poder dibujarse aunque no haya buffer")
	}

	// Los atajos que normalmente tocan el buffer activo son el camino con más
	// punteros sin dueño: sin buffer no pueden entrar en pánico ni hacer nada.
	for _, key := range []tcell.Key{tcell.KeyCtrlS, tcell.KeyCtrlZ, tcell.KeyCtrlY} {
		if quit := press(app, key); quit {
			t.Fatalf("la tecla %v no debe cerrar el editor con el workspace vacío", key)
		}
	}
	pressSaveAs(app)
	if app.promptActive {
		t.Fatal("Save As no debe abrirse sin un buffer de destino")
	}
	if quit := press(app, tcell.KeyDown); quit {
		t.Fatal("una tecla común no debe cerrar el editor")
	}
	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("un solo Escape sobre el workspace vacío no cierra: doble rápido")
	}
	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("el segundo Escape rápido sobre el workspace vacío debe cerrar el editor")
	}
	app.redraw()
}

func TestEditorHeightReservesTheStatusRow(t *testing.T) {
	for _, tc := range []struct {
		height, want int
	}{
		{10, 8},
		{2, 1},
		{1, 1},
		{0, 1},
	} {
		if got := editorHeight(tc.height); got != tc.want {
			t.Fatalf("editorHeight(%d) = %d, se esperaba %d", tc.height, got, tc.want)
		}
	}
}

// --- U2b: pestañas y navegación ---

// resizeApp fija el tamaño de la pantalla DESPUÉS de construir la app: Init()
// reinicia la pantalla simulada a 80x25 (simulation.go), así que un SetSize
// anterior es letra muerta. Disparar además el EventResize propaga la geometría
// a las vistas.
func resizeApp(app *App, w, h int) {
	sim := app.screen.(tcell.SimulationScreen)
	sim.SetSize(w, h)
	app.handleEvent(tcell.NewEventResize(w, h))
}

// cellRune devuelve la runa principal de la celda (x, y) de la pantalla
// simulada de la app, o 0 si la celda está vacía.
func cellRune(app *App, x, y int) rune {
	sim := app.screen.(tcell.SimulationScreen)
	cells, w, _ := sim.GetContents()
	c := cells[y*w+x]
	if len(c.Runes) == 0 {
		return 0
	}
	return c.Runes[0]
}

// statusRow devuelve el texto de la fila de estado de la pantalla simulada.
// La barra dibuja su mensaje a la derecha de la etiqueta, así que el texto
// completo de la fila es la forma de asertar sobre los avisos.
func statusRow(app *App) string {
	sim := app.screen.(tcell.SimulationScreen)
	cells, w, h := sim.GetContents()
	var sb strings.Builder
	for x := 0; x < w; x++ {
		c := cells[(h-1)*w+x]
		if len(c.Runes) == 0 {
			sb.WriteRune(' ')
			continue
		}
		sb.WriteRune(c.Runes[0])
	}
	return sb.String()
}

// newThreeBufferApp abre tres archivos como tres pestañas. Devuelve la app
// con el buffer 2 activo (el último abierto).
func newThreeBufferApp(t *testing.T, c0, c1, c2 string) *App {
	t.Helper()

	dir := t.TempDir()
	contents := []string{c0, c1, c2}
	var paths [3]string
	for i, name := range []string{"a.txt", "b.txt", "c.txt"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(contents[i]), 0o644); err != nil {
			t.Fatalf("no se pudo crear el archivo: %v", err)
		}
		paths[i] = p
	}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	s.SetSize(40, 10)

	app, err := NewAppWithScreen(s, paths[0])
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	for _, p := range paths[1:] {
		if _, err := app.ws.Open(p); err != nil {
			t.Fatalf("Open falló: %v", err)
		}
	}
	t.Cleanup(func() {
		app.ws.CloseAll()
		s.Fini()
	})
	return app
}

// TestCtrlPageDownSwitchesTabs: la navegación entre pestañas es circular y
// reencuadra la fila. Con el buffer 2 activo, Ctrl+PageDown vuelve al 0 y
// Ctrl+PageUp a la última.
func TestCtrlPageDownSwitchesTabs(t *testing.T) {
	app := newThreeBufferApp(t, "uno", "dos", "tres")
	if got := app.ws.ActiveIndex(); got != 2 {
		t.Fatalf("ActiveIndex() = %d, se esperaba 2", got)
	}

	app.handleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModCtrl))

	if got := app.ws.ActiveIndex(); got != 0 {
		t.Fatalf("tras Ctrl+PageDown, ActiveIndex() = %d, se esperaba 0 (wrap)", got)
	}

	app.handleEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModCtrl))

	if got := app.ws.ActiveIndex(); got != 2 {
		t.Fatalf("tras Ctrl+PageUp, ActiveIndex() = %d, se esperaba 2 (el último)", got)
	}
}

// TestCtrlKSwitchesTabs: Ctrl+K es el atajo de pestañas de la familia K
// (pedido del usuario; Ctrl+J no existe porque en la terminal es el byte LF,
// el Enter que ya activa/inserta salto de línea): pasa a la SIGUIENTE con
// wrap, como Ctrl+PageDown.
func TestCtrlKSwitchesTabs(t *testing.T) {
	app := newThreeBufferApp(t, "uno", "dos", "tres")
	if got := app.ws.ActiveIndex(); got != 2 {
		t.Fatalf("ActiveIndex() = %d, se esperaba 2", got)
	}

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 0 {
		t.Fatalf("tras Ctrl+K, ActiveIndex() = %d, se esperaba 0 (wrap)", got)
	}

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 1 {
		t.Fatalf("tras el segundo Ctrl+K, ActiveIndex() = %d, se esperaba 1", got)
	}
}

// TestCtrlLSwitchesTabsBackwards: Ctrl+L vuelve a la ANTERIOR con wrap —el
// par final es Ctrl+K (siguiente) / Ctrl+L (anterior); Ctrl+Shift+K quedó
// descartado por pedido del usuario. En Windows Terminal Ctrl+L llega limpio
// (el form feed de los terminales Unix, que limpian la pantalla, no aplica).
func TestCtrlLSwitchesTabsBackwards(t *testing.T) {
	app := newThreeBufferApp(t, "uno", "dos", "tres")
	if got := app.ws.ActiveIndex(); got != 2 {
		t.Fatalf("ActiveIndex() = %d, se esperaba 2", got)
	}

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 1 {
		t.Fatalf("tras Ctrl+L, ActiveIndex() = %d, se esperaba 1", got)
	}

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 0 {
		t.Fatalf("tras el segundo Ctrl+L, ActiveIndex() = %d, se esperaba 0", got)
	}

	// Wrap al inicio: desde la 0, cae en la última.
	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 2 {
		t.Fatalf("tras Ctrl+L en la primera, ActiveIndex() = %d, se esperaba 2 (wrap)", got)
	}
}

// TestCtrlWClosesACleanTab: una pestaña limpia se cierra sin confirmación y su
// vista sale del mapa de vistas.
func TestCtrlWClosesACleanTab(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")
	buf := bufferAt(app, 0)
	app.redraw() // materializar la vista del buffer 0

	if quit := press(app, tcell.KeyCtrlW); quit {
		t.Fatal("Ctrl+W no debe cerrar el editor")
	}
	if got := app.ws.Len(); got != 1 {
		t.Fatalf("Len() = %d tras cerrar una pestaña limpia, se esperaba 1", got)
	}
	if _, ok := app.editors[buf]; ok {
		t.Fatal("la vista del buffer cerrado debe eliminarse del mapa")
	}
	if got := app.ws.ActiveIndex(); got != 0 {
		t.Fatalf("ActiveIndex() = %d, se esperaba 0 (la pestaña que queda)", got)
	}
}

// TestCtrlWAsksBeforeClosingADirtyTab: la confirmación de cierre es no modal:
// la primera vez avisa, la segunda cierra sin guardar, y en el camino nada se
// escribe a disco.
func TestCtrlWAsksBeforeClosingADirtyTab(t *testing.T) {
	app, path := newTestApp(t, "uno")
	// El aviso mide más de 60 columnas, así que con la pantalla 80x25 la
	// regla de la barra ("solo entra si no pisa la etiqueta") lo ocultaría;
	// una terminal ancha es la condición real de dibujo.
	resizeApp(app, 110, 8)
	typeRune(app, 'X')

	press(app, tcell.KeyCtrlW)

	if got := app.ws.Len(); got != 1 {
		t.Fatalf("el primer Ctrl+W no debe cerrar: Len() = %d, se esperaba 1", got)
	}
	if !strings.Contains(statusRow(app), "Cambios sin guardar") {
		t.Fatalf("la barra debe avisar de los cambios sin guardar: %q", statusRow(app))
	}
	if got := readFile(t, path); got != "uno" {
		t.Fatalf("el archivo no debía tocarse: %q", got)
	}

	press(app, tcell.KeyCtrlW)

	if got := app.ws.Len(); got != 0 {
		t.Fatalf("el segundo Ctrl+W debe cerrar: Len() = %d, se esperaba 0", got)
	}
	if got := readFile(t, path); got != "uno" {
		t.Fatalf("cerrar sin guardar no debe escribir el archivo: %q", got)
	}
}

// TestAnotherKeyCancelsTheCloseConfirmation: seguir trabajando desarma la
// confirmación de cierre, como la de salida: el próximo Ctrl+W vuelve a avisar
// en lugar de cerrar.
func TestAnotherKeyCancelsTheCloseConfirmation(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	resizeApp(app, 110, 8)
	typeRune(app, 'X')

	press(app, tcell.KeyCtrlW)
	if !strings.Contains(statusRow(app), "Cambios sin guardar") {
		t.Fatalf("el primer Ctrl+W debe armar la confirmación: %q", statusRow(app))
	}

	press(app, tcell.KeyDown)

	press(app, tcell.KeyCtrlW)
	if got := app.ws.Len(); got != 1 {
		t.Fatalf("tras cancelar, Ctrl+W debe volver a avisar y no cerrar: Len() = %d", got)
	}
	if !strings.Contains(statusRow(app), "Cambios sin guardar") {
		t.Fatalf("tras cancelar, Ctrl+W debe volver a armar la confirmación: %q", statusRow(app))
	}
}

// TestClosingTheLastTabLeavesAnEmptyWorkspace: cerrar la última pestaña deja
// el workspace vacío sin pánico, y Escape sigue saliendo.
func TestClosingTheLastTabLeavesAnEmptyWorkspace(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	typeRune(app, 'X')

	press(app, tcell.KeyCtrlW)
	if quit := press(app, tcell.KeyCtrlW); quit {
		t.Fatal("Ctrl+W no debe cerrar el editor")
	}
	app.redraw() // no debe entrar en pánico con el workspace vacío

	if got := app.ws.Len(); got != 0 {
		t.Fatalf("Len() = %d, se esperaba 0", got)
	}
	if app.ws.Active() != nil || app.ws.ActiveIndex() != -1 {
		t.Fatalf("workspace vacío: Active()=%v ActiveIndex()=%d", app.ws.Active(), app.ws.ActiveIndex())
	}

	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("un solo Escape sobre el workspace vacío no cierra: doble rápido")
	}
	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("el segundo Escape rápido sobre el workspace vacío debe cerrar el editor")
	}
}

// TestClosingTheLastTabReturnsFocusToTheExplorer: cerrar la ÚLTIMA pestaña deja
// el workspace en el estado a propósito vacío, y el foco pasa al explorador
// —visible aunque estuviera oculto—, como en el arranque sobre un directorio.
// Sin buffers no hay documento que editar: el árbol es el destino natural del
// teclado para dirigirse a otro archivo; si el foco quedara en el editor
// vacío, el guard de workspace vacío dejaría las teclas muertas salvo salir.
func TestClosingTheLastTabReturnsFocusToTheExplorer(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(doc, []byte("uno"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	app, err := NewAppWithScreen(s, doc) // arranque de editor: panel oculto
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	t.Cleanup(func() { app.ws.CloseAll(); s.Fini() })

	if app.explorerVisible || app.explorerFocused {
		t.Fatal("el arranque con archivo debe dejar el panel oculto y sin foco")
	}

	press(app, tcell.KeyCtrlW) // una pestaña limpia cierra directo
	if got := app.ws.Len(); got != 0 {
		t.Fatalf("Len() = %d, se esperaba 0", got)
	}
	if !app.explorerVisible {
		t.Fatal("cerrar la última pestaña debe mostrar el explorador")
	}
	if !app.explorerFocused {
		t.Fatal("cerrar la última pestaña debe pasar el foco al explorador")
	}

	// El teclado queda vivo en el árbol: ↓ no sale ni abre nada (una sola
	// entrada se mantiene), y Enter abre el archivo de la raíz devolviendo el
	// foco al editor.
	press(app, tcell.KeyDown)
	if got := app.explorer.CursorPath(); got != doc {
		t.Fatalf("CursorPath() = %q, se esperaba %q", got, doc)
	}
	press(app, tcell.KeyEnter)
	if got := app.ws.Len(); got != 1 {
		t.Fatalf("Len() = %d, se esperaba 1 tras abrir desde el árbol", got)
	}
	if app.explorerFocused {
		t.Fatal("abrir un archivo desde el árbol devuelve el foco al editor")
	}
}

// TestClosingANonLastTabKeepsTheEditorFocus: el paso al explorador es SOLO del
// estado sin buffers —cerrar la última pestaña—, no un efecto lateral de
// cualquier cierre: cerrar una pestaña que no es la última deja el foco en el
// editor y el panel oculto queda oculto.
func TestClosingANonLastTabKeepsTheEditorFocus(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")
	if app.explorerVisible || app.explorerFocused {
		t.Fatal("el arranque con archivo debe dejar el panel oculto")
	}

	press(app, tcell.KeyCtrlW) // cierra el buffer activo (el 0); queda 1
	if got := app.ws.Len(); got != 1 {
		t.Fatalf("Len() = %d, se esperaba 1", got)
	}
	if app.explorerVisible {
		t.Fatal("cerrar una pestaña que no es la última no debe mostrar el panel")
	}
	if app.explorerFocused {
		t.Fatal("cerrar una pestaña que no es la última no debe mover el foco")
	}
}

// TestClosingATabRemovesItsEditorAndPermission: al cerrar, el controlador
// borra la vista del buffer (una vista vieja conservaría un puntero a un
// PieceTable ya desmapeado) y el permiso de pisar (autorizó a un archivo que
// ya no está abierto). Es la deuda (a) de U2a, que queda pagada acá.
func TestClosingATabRemovesItsEditorAndPermission(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")

	// Estado en el que la deuda se vuelve observable: el buffer 0 activo, con
	// vista materializada, sucio y autorizado a pisar cambios externos. El
	// permiso se arma con saveAsFor directamente: writeExternally renombra
	// sobre el archivo mapeado y en Windows eso falla (acceso denegado), y
	// este test no quiere depender del disco.
	typeRune(app, 'X')
	app.redraw()
	buf0 := bufferAt(app, 0)
	app.saveAsFor(buf0)
	if _, ok := app.editors[buf0]; !ok {
		t.Fatal("el test requiere una vista materializada del buffer 0")
	}
	if !app.forceSave[buf0] {
		t.Fatal("el test requiere el permiso de pisar del buffer 0")
	}

	// El buffer 0 está sucio: cerrarlo pide confirmación dos veces.
	press(app, tcell.KeyCtrlW)
	press(app, tcell.KeyCtrlW)

	if _, ok := app.editors[buf0]; ok {
		t.Fatal("la entrada del editor debe borrarse al cerrar el buffer")
	}
	if app.forceSave[buf0] {
		t.Fatal("el permiso de pisar debe borrarse al cerrar el buffer")
	}
}

// TestRedrawComposesTabsAboveTheEditor: la fila 0 es de las pestañas y el
// documento arranca en la fila 1; el cursor del documento también se traduce.
func TestRedrawComposesTabsAboveTheEditor(t *testing.T) {
	app, _ := newTestApp(t, "uno\ndos")
	resizeApp(app, 24, 6) // tamaño determinista, DESPUÉS de construir la app

	app.redraw()

	// La pestaña activa va entre corchetes: '[' en (0,0) y el primer
	// carácter de la etiqueta ("doc.txt" → 'd') en (1,0); el primer carácter
	// del documento vive tras el gutter de 2 columnas en (2,1), la primera
	// fila del editor.
	if got := cellRune(app, 0, 0); got != '[' {
		t.Fatalf("(0,0) = %q, se esperaba '[' (corchete de la pestaña activa)", got)
	}
	if got := cellRune(app, 1, 0); got != 'd' {
		t.Fatalf("(1,0) = %q, se esperaba 'd' (inicio de la pestaña)", got)
	}
	if got := cellRune(app, 2, 1); got != 'u' {
		t.Fatalf("(2,1) = %q, se esperaba 'u' (inicio del documento tras el gutter)", got)
	}

	sim := app.screen.(tcell.SimulationScreen)
	if x, y, vis := sim.GetCursor(); !vis || x != 2 || y != 1 {
		t.Fatalf("cursor = (%d,%d,vis=%v), se esperaba (2,1,true)", x, y, vis)
	}
}

// TestMouseClickIsTranslatedPastTheTabBar: la fila de pestañas no es parte del
// documento. Un clic en la fila 2 de pantalla cae en la línea 1 del documento
// (la fila 0 es la de pestañas), y eso se observa tipiando después del clic.
func TestMouseClickIsTranslatedPastTheTabBar(t *testing.T) {
	app, _ := newTestApp(t, "uno\ndos\ntres")

	// La columna 2 es la primera celda de texto (tras el gutter de la vista).
	app.handleEvent(tcell.NewEventMouse(2, 2, tcell.Button1, tcell.ModNone))
	typeRune(app, 'X')

	if got := app.ws.Active().GetContent(); got != "uno\nXdos\ntres" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "uno\nXdos\ntres")
	}
}

// --- U3: explorador de archivos y modos de arranque ---

// newExplorerApp arranca sobre un directorio (el modo "sin buffers"): la
// pantalla queda en el 80x25 que Init() deja tras construir la app; un test
// que necesite otra geometría usa resizeApp después, como en U2b.
func newExplorerApp(t *testing.T, dir string) *App {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	app, err := NewAppWithScreen(s, dir)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	t.Cleanup(func() {
		app.ws.CloseAll()
		s.Fini()
	})
	return app
}

// panelRow devuelve el texto visible de la fila y del panel del explorador en
// la pantalla simulada: de la columna 0 al borde derecho del panel. Es la
// forma de observar el listado que dibuja el controlador al componer.
func panelRow(app *App, y int) string {
	sim := app.screen.(tcell.SimulationScreen)
	cells, w, _ := sim.GetContents()
	panelW := 0
	if app.explorerVisible {
		panelW = panelWidth(w)
	}
	var sb strings.Builder
	for x := 0; x < panelW; x++ {
		c := cells[(y+tabBarHeight)*w+x]
		if len(c.Runes) == 0 {
			sb.WriteRune(' ')
			continue
		}
		sb.WriteRune(c.Runes[0])
	}
	return strings.TrimRight(sb.String(), " ")
}

// TestReadEntriesSkipsDotfiles: readEntries —la única puerta de datos del
// disco al árbol— NO lista los dotfiles: carpetas y archivos que arrancan
// con "." (.git/, .tcode/, .oculto) quedan fuera, en el nivel raíz y en
// cualquier subdirectorio. Es el contrato "el árbol muestra código, no el
// estado de la herramienta".
func TestReadEntriesSkipsDotfiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{".git", ".tcode", "docs", "visible.txt", ".oculto.txt"} {
		p := filepath.Join(dir, name)
		if name == ".git" || name == ".tcode" || name == "docs" {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatalf("no se pudo crear el dir %s: %v", name, err)
			}
			continue
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("no se pudo crear %s: %v", name, err)
		}
	}

	entries, err := readEntries(dir)
	if err != nil {
		t.Fatalf("readEntries falló: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	want := []string{"docs", "visible.txt"} // dirs primero, sin dotfiles
	if got := strings.Join(names, ","); got != strings.Join(want, ",") {
		t.Fatalf("readEntries = %v, se esperaba %v (los dotfiles no se listan)", names, want)
	}

	// Un subdirectorio con dotfiles tampoco los muestra.
	sub := filepath.Join(dir, "docs")
	if err := os.MkdirAll(filepath.Join(sub, ".escondido"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "nota.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err = readEntries(sub)
	if err != nil {
		t.Fatalf("readEntries del subdir falló: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "nota.md" {
		t.Fatalf("readEntries del subdir = %+v, se esperaba solo nota.md", entries)
	}
}

// TestPanelWidthKeepsTheEditorAtLeastSixteenColumns: la fórmula del ancho del
// panel es determinista para los tests y deja al editor al menos 16 columnas:
// 80 → 24, 30 → 14 (16 era la errata del contrato), 20 → 4, y el panel nunca
// desaparece (mínimo 1).
func TestPanelWidthKeepsTheEditorAtLeastSixteenColumns(t *testing.T) {
	for _, tc := range []struct {
		width, want int
	}{
		{80, 24},
		{30, 14},
		{20, 4},
		{16, 1},
	} {
		if got := panelWidth(tc.width); got != tc.want {
			t.Fatalf("panelWidth(%d) = %d, se esperaba %d", tc.width, got, tc.want)
		}
	}
}

// TestStartupWithoutArguments: sin argumento el explorador arranca sobre el
// directorio actual, visible y enfocado, y el workspace queda SIN buffers —el
// estado a propósito vacío que la feature declaró en U1—.
//
// El arranque sin argumentos corre sobre el DIRECTORIO DE TRABAJO, y el repo es
// un destino legítimo de pruebas que puede tener .tcode/session.json (U4): el
// test se aísla en un directorio temporal y restaura el cwd al terminar.
func TestStartupWithoutArguments(t *testing.T) {
	// El original se captura ANTES del Chdir: el cleanup restaura al directorio
	// real (re-chdir al temporal a punto de borrarse fallaría en Windows).
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("no se pudo leer el directorio de trabajo: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("no se pudo cambiar al directorio temporal: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("no se pudo restaurar el directorio de trabajo: %v", err)
		}
	})

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	app, err := NewAppWithScreen(s, "")
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	t.Cleanup(func() { app.ws.CloseAll(); s.Fini() })

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("no se pudo leer el directorio de trabajo: %v", err)
	}
	if got := app.ws.Root(); got != cwd {
		t.Fatalf("Root() = %q, se esperaba el directorio actual %q", got, cwd)
	}
	if !app.explorerVisible {
		t.Fatal("sin argumentos el explorador debe arrancar visible")
	}
	if !app.explorerFocused {
		t.Fatal("sin argumentos el foco debe estar en el explorador")
	}
	if got := app.ws.Len(); got != 0 {
		t.Fatalf("Len() = %d, se esperaba 0: sin argumentos no hay buffers", got)
	}
}

// TestStartupWithDirectoryArgument: un directorio como argumento arranca el
// explorador sobre él —visible y enfocado, sin buffers— y su listado ya está
// leído: el archivo del directorio figura entre las entradas.
func TestStartupWithDirectoryArgument(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(doc, []byte("contenido"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)

	if got := app.ws.Root(); got != dir {
		t.Fatalf("Root() = %q, se esperaba %q", got, dir)
	}
	if !app.explorerVisible || !app.explorerFocused {
		t.Fatal("arrancar sobre un directorio deja el explorador visible y enfocado")
	}
	if got := app.ws.Len(); got != 0 {
		t.Fatalf("Len() = %d, se esperaba 0", got)
	}

	// El archivo está en el primer nivel del árbol: es el único nodo, el cursor
	// ya está sobre él y el panel lo dibuja con el marcador de la fila activa.
	app.redraw()
	if got := app.explorer.CursorPath(); got != doc {
		t.Fatalf("CursorPath() = %q, se esperaba %q", got, doc)
	}
	if got := panelRow(app, 0); got != "> doc.txt" {
		t.Fatalf("fila 0 del panel = %q, se esperaba %q", got, "> doc.txt")
	}
}

// TestStartupWithFileArgumentKeepsTheExplorerHidden: el arranque clásico de
// editor. El panel queda oculto —no desplaza el documento—, el archivo se abre
// y la raíz de la sesión es su directorio padre.
func TestStartupWithFileArgumentKeepsTheExplorerHidden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(path, []byte("contenido"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	app, err := NewAppWithScreen(s, path)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	t.Cleanup(func() { app.ws.CloseAll(); s.Fini() })

	if got := app.ws.Root(); got != dir {
		t.Fatalf("Root() = %q, se esperaba %q (el directorio padre)", got, dir)
	}
	if got := app.ws.Len(); got != 1 {
		t.Fatalf("Len() = %d, se esperaba 1 (el archivo abierto)", got)
	}
	if app.explorerVisible {
		t.Fatal("el explorador debe arrancar oculto con un archivo como argumento")
	}

	// El documento arranca en (2,1): el panel oculto no desplaza nada salvo el
	// gutter de la vista del editor.
	resizeApp(app, 24, 6)
	app.redraw()
	if got := cellRune(app, 2, 1); got != 'c' {
		t.Fatalf("(2,1) = %q, se esperaba 'c' (el inicio del documento tras el gutter)", got)
	}
}

// TestTabsDoNotOverlapTheTree: la fila de pestañas se renderiza SOLO sobre el
// área del editor —arranca en la columna del panel—, nunca sobre el árbol.
// Con el panel oculto vuelve a la columna 0.
func TestTabsDoNotOverlapTheTree(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	resizeApp(app, 80, 8)

	// Panel oculto: la pestaña arranca en la columna 0 de la fila de pestañas.
	app.redraw()
	if got := cellRune(app, 0, 0); got != '[' {
		t.Fatalf("(0,0) = %q, se esperaba '[' (corchete en el borde izquierdo sin panel)", got)
	}

	// Panel visible (24 columnas): la columna 0 de la fila queda vacía —nada
	// de pestañas sobre el árbol— y la pestaña arranca en la columna 24.
	press(app, tcell.KeyCtrlB)
	app.redraw()
	// La fila limpia queda con el espacio del Clear: nada de pestañas sobre el
	// árbol —ni el primer carácter de la etiqueta ('d')— y la pestaña arranca
	// en la columna 24, el borde del editor.
	if got := cellRune(app, 0, 0); got != ' ' {
		t.Fatalf("(0,0) = %q, se esperaba el espacio del Clear: las pestañas no deben dibujarse sobre el árbol", got)
	}
	if got := cellRune(app, 23, 0); got != ' ' {
		t.Fatalf("(23,0) = %q, se esperaba vacío: la fila del árbol no tiene pestañas", got)
	}
	if got := cellRune(app, 24, 0); got != '[' {
		t.Fatalf("(24,0) = %q, se esperaba '[': la pestaña arranca en la columna del editor", got)
	}
	if got := cellRune(app, 25, 0); got != 'd' {
		t.Fatalf("(25,0) = %q, se esperaba 'd': inicio de la etiqueta tras el corchete", got)
	}

	// Ocultar de nuevo: la pestaña vuelve a la columna 0.
	press(app, tcell.KeyCtrlB)
	app.redraw()
	if got := cellRune(app, 0, 0); got != '[' {
		t.Fatalf("(0,0) = %q, se esperaba '[' tras ocultar el panel", got)
	}
}

// TestCtrlBTogglesTheExplorer: Ctrl+B muestra y oculta el panel. Mostrar
// redimensiona las vistas del editor al ancho restante y desplaza el documento
// a la columna del panel; ocultar lo devuelve a la columna 0 y desenfoca el
// panel.
func TestCtrlBTogglesTheExplorer(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	resizeApp(app, 80, 8)

	// Materializar la vista del editor: el toggle redimensiona las vistas
	// EXISTENTES, así que la vista tiene que existir primero.
	app.redraw()
	ed := app.activeEditor()
	// Size() es el área de texto: descuenta el gutter (2 para 1 línea).
	if w, _ := ed.Size(); w != 78 {
		t.Fatalf("la vista del editor = %d columnas de texto, se esperaba 78 (80 de widget - gutter, panel oculto)", w)
	}

	if quit := press(app, tcell.KeyCtrlB); quit {
		t.Fatal("Ctrl+B no debe cerrar el editor")
	}
	if !app.explorerVisible {
		t.Fatal("Ctrl+B debe mostrar el explorador")
	}
	if !app.explorerFocused {
		t.Fatal("al mostrar, el foco debe ir al explorador")
	}
	// El editor conserva 80-24 columnas de widget; el área de texto descuenta
	// el gutter y el documento arranca en la columna del panel + gutter.
	if w, h := ed.Size(); w != 54 || h != 6 {
		t.Fatalf("vista del editor = %dx%d, se esperaba 54x6", w, h)
	}
	if got := cellRune(app, 26, 1); got != 'u' {
		t.Fatalf("(26,1) = %q, se esperaba 'u': el documento desplazado por el panel y el gutter", got)
	}

	if quit := press(app, tcell.KeyCtrlB); quit {
		t.Fatal("Ctrl+B no debe cerrar el editor")
	}
	if app.explorerVisible {
		t.Fatal("el segundo Ctrl+B debe ocultar el explorador")
	}
	if app.explorerFocused {
		t.Fatal("al ocultar, el explorador debe desenfocarse")
	}
	if w, _ := ed.Size(); w != 78 {
		t.Fatalf("la vista del editor = %d columnas de texto tras ocultar, se esperaba 78", w)
	}
	if got := cellRune(app, 2, 1); got != 'u' {
		t.Fatalf("(2,1) = %q, se esperaba 'u': el documento de vuelta tras el gutter", got)
	}
}

// TestEnterOpensTheSelectedFile: Enter sobre un archivo lo abre en el
// workspace —con su contenido— y devuelve el foco al editor.
func TestEnterOpensTheSelectedFile(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "carpeta")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el directorio: %v", err)
	}
	doc := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(doc, []byte("contenido seteado"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)

	// Listado: [carpeta, doc.txt]. Bajar hasta el archivo y abrirlo.
	press(app, tcell.KeyDown)
	press(app, tcell.KeyEnter)

	if got := app.ws.Len(); got != 1 {
		t.Fatalf("Len() = %d, se esperaba 1", got)
	}
	if got := app.ws.Active().GetContent(); got != "contenido seteado" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "contenido seteado")
	}
	if app.explorerFocused {
		t.Fatal("abrir un archivo debe devolver el foco al editor")
	}
}

// TestEnterExpandsADirectoryAndItsChildrenAppear: Enter sobre un directorio
// del árbol lo EXPANDE (ActionExpand → el controlador lee sus hijos): el cursor
// queda en el dir expandido y los hijos aparecen a continuación, indentados con
// su profundidad. El árbol nunca cambia de carpeta: la base sigue siendo el
// root de la sesión.
func TestEnterExpandsADirectoryAndItsChildrenAppear(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "carpeta")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el directorio: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "fuente.txt"), []byte("adentro"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)

	press(app, tcell.KeyEnter) // expande carpeta

	app.redraw()
	// El cursor quedó en el dir expandido y sus hijos se leen al expandir.
	if got := app.explorer.CursorPath(); got != sub {
		t.Fatalf("CursorPath() = %q, se esperaba %q (el cursor en el dir)", got, sub)
	}
	if got := panelRow(app, 0); got != "▾ carpeta/" {
		t.Fatalf("fila 0 del panel = %q, se esperaba %q (dir expandido)", got, "▾ carpeta/")
	}
	if got := panelRow(app, 1); got != "  · fuente.txt" {
		t.Fatalf("fila 1 del panel = %q, se esperaba %q (el hijo, con la indentación de su nivel)", got, "  · fuente.txt")
	}
}

// TestLeftCollapsesADirectory: expandir, bajar a un hijo y volver: Left sobre
// el dir expandido lo colapsa (la vista sola, sin E/S). Los hijos desaparecen
// del aplanado y el cursor queda en el dir colapsado —el árbol volvió a su
// primer nivel, sin ".." que subir: la base es fija en el root de la sesión.
func TestLeftCollapsesADirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "carpeta")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el directorio: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "fuente.txt"), []byte("adentro"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)

	press(app, tcell.KeyEnter) // expande carpeta
	press(app, tcell.KeyDown)  // al hijo fuente.txt
	press(app, tcell.KeyUp)    // de vuelta al dir
	press(app, tcell.KeyLeft)  // colapsa el dir expandido del cursor

	app.redraw()
	// Los hijos desaparecieron y el cursor quedó en el dir colapsado (que sigue
	// visible): es el análogo de "volver arriba" —el root queda a la vista.
	if got := app.explorer.CursorPath(); got != sub {
		t.Fatalf("CursorPath() = %q, se esperaba %q (el cursor en el dir colapsado)", got, sub)
	}
	if got := panelRow(app, 0); got != "▸ carpeta/" {
		t.Fatalf("fila 0 del panel = %q, se esperaba %q (dir colapsado)", got, "▸ carpeta/")
	}
	if got := panelRow(app, 1); got != "" {
		t.Fatalf("fila 1 del panel = %q, se esperaba vacía: los hijos desaparecieron", got)
	}
}

// TestTabReturnsFocusToTheEditor: con el foco en el explorador, Tab lo
// devuelve al editor; al revés no aplica —con el foco en el editor Tab inserta
// tabulación (ver TestTabInsertsInTheDocumentWhileTheExplorerIsVisible)—:
// volver al panel es con clic o re-mostrándolo con Ctrl+B.
func TestTabReturnsFocusToTheEditor(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)
	if !app.explorerFocused {
		t.Fatal("arrancar sobre un directorio enfoca el explorador")
	}

	if quit := press(app, tcell.KeyTab); quit {
		t.Fatal("Tab no debe cerrar el editor")
	}
	if app.explorerFocused {
		t.Fatal("Tab con el foco en el explorador debe llevarlo al editor")
	}
}

// TestTabInsertsInTheDocumentWhileTheExplorerIsVisible: con el foco en el
// editor y el panel visible, Tab inserta tabulación en el documento: el foco
// del panel nunca le roba al editor su tecla de indentación.
func TestTabInsertsInTheDocumentWhileTheExplorerIsVisible(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(doc, []byte(""), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	app := newExplorerApp(t, dir)

	press(app, tcell.KeyEnter) // abre a.txt: el foco vuelve al editor
	if app.explorerFocused {
		t.Fatal("abrir un archivo devuelve el foco al editor")
	}

	press(app, tcell.KeyTab)
	// El tab del editor es la unidad estándar (view.indentUnit): 4 espacios por defecto.
	if got := app.ws.Active().GetContent(); got != "    " {
		t.Fatalf("contenido = %q, se esperaba %q: Tab inserta la unidad con el panel visible", got, "    ")
	}
	if app.explorerFocused {
		t.Fatal("Tab con el foco en el editor no debe cambiar el foco")
	}
}

// TestShiftTabReturnsFocusToTheExplorer: Shift+Tab (KeyBacktab) es el inverso
// de Tab: con el foco en el editor y el panel visible devuelve el foco al
// explorador SIN editar el documento (la tecla no llega a la edición) y SIN
// tocar la visibilidad del panel, que es decisión de Ctrl+B. Tab y Shift+Tab
// alternan entre panel y editor desde el teclado.
func TestShiftTabReturnsFocusToTheExplorer(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(doc, []byte(""), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	app := newExplorerApp(t, dir)

	press(app, tcell.KeyEnter) // abre a.txt: el foco vuelve al editor
	if app.explorerFocused {
		t.Fatal("abrir un archivo devuelve el foco al editor")
	}

	if quit := press(app, tcell.KeyBacktab); quit {
		t.Fatal("Shift+Tab no debe cerrar el editor")
	}
	if !app.explorerFocused {
		t.Fatal("Shift+Tab con el foco en el editor debe llevarlo al explorador")
	}
	if !app.explorerVisible {
		t.Fatal("Shift+Tab solo mueve el foco: no debe ocultar el panel")
	}
	// El foco cambió sin tocar el documento: Shift+Tab no es una tecla de
	// edición y el editor no llega a recibirla.
	if got := app.ws.Active().GetContent(); got != "" {
		t.Fatalf("contenido = %q, se esperaba \"\": Shift+Tab no edita el documento", got)
	}

	// El par alterna: Tab devuelve al editor y Shift+Tab vuelve al panel.
	press(app, tcell.KeyTab)
	if app.explorerFocused {
		t.Fatal("Tab debe devolver el foco al editor")
	}
	press(app, tcell.KeyBacktab)
	if !app.explorerFocused {
		t.Fatal("tras Tab+Shift+Tab el foco debe volver al explorador")
	}
}

// TestShiftTabWithHiddenExplorerDoesNothing: con el panel oculto Shift+Tab no
// lo muestra ni cambia el foco: mostrar el panel es decisión de Ctrl+B.
func TestShiftTabWithHiddenExplorerDoesNothing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	app := newExplorerApp(t, dir)

	press(app, tcell.KeyCtrlB) // ocultar
	if app.explorerVisible {
		t.Fatal("el test requiere el panel oculto")
	}
	press(app, tcell.KeyBacktab)
	if app.explorerVisible || app.explorerFocused {
		t.Fatal("Shift+Tab con el panel oculto no debe mostrarlo ni enfocarlo")
	}
}

// TestShiftTabFocusesTheExplorerWithAnEmptyWorkspace: mover el foco no toca
// ningún buffer, así que Shift+Tab funciona también sin pestañas abiertas —el
// arranque sobre un directorio—, igual que Tab.
func TestShiftTabFocusesTheExplorerWithAnEmptyWorkspace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	app := newExplorerApp(t, dir)
	if !app.explorerFocused {
		t.Fatal("el arranque sobre un directorio enfoca el explorador")
	}

	press(app, tcell.KeyTab) // desenfoca el panel sin abrir nada
	if app.explorerFocused {
		t.Fatal("el test requiere el foco en el editor")
	}
	press(app, tcell.KeyBacktab)
	if !app.explorerFocused {
		t.Fatal("Shift+Tab sin buffers debe devolver el foco al explorador")
	}
}

// TestShiftTabFromTheExplorerReturnsToTheEditor: Shift+Tab es SIMÉTRICO: con
// el foco ya en el explorador, Shift+Tab devuelve el foco al editor —el
// mismo comportamiento que Tab—. Quien llega al selector con Shift+Tab no
// queda atrapado: la misma tecla lo saca. El viaje de foco no edita el
// documento ni toca la visibilidad del panel.
func TestShiftTabFromTheExplorerReturnsToTheEditor(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(doc, []byte(""), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	app := newExplorerApp(t, dir)

	press(app, tcell.KeyEnter) // abre a.txt: foco al editor
	if app.explorerFocused {
		t.Fatal("abrir un archivo devuelve el foco al editor")
	}

	// Alternancia completa con la MISMA tecla: editor → Shift+Tab → selector
	// → Shift+Tab → editor.
	press(app, tcell.KeyBacktab)
	if !app.explorerFocused {
		t.Fatal("Shift+Tab desde el editor debe llevar el foco al selector")
	}
	press(app, tcell.KeyBacktab)
	if app.explorerFocused {
		t.Fatal("Shift+Tab desde el selector debe devolver el foco al editor (el par alterna)")
	}
	if !app.explorerVisible {
		t.Fatal("el viaje de foco no debe tocar la visibilidad del panel")
	}

	// Y Tab sigue saliendo del selector también (la vía clásica).
	press(app, tcell.KeyBacktab) // al selector
	press(app, tcell.KeyTab)
	if app.explorerFocused {
		t.Fatal("Tab desde el selector debe devolver el foco al editor")
	}

	if got := app.ws.Active().GetContent(); got != "" {
		t.Fatalf("contenido = %q, se esperaba \"\": el viaje de foco no edita el documento", got)
	}
}

// newSubtreeApp arma un workspace con directorios anidados y devuelve la app
// y la ruta del archivo hoja: el árbol arranca con el dir raíz colapsado y
// SIN hijos (nunca expandido), que es lo que fuerza al reveal a leer el disco.
func newSubtreeApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	sub := filepath.Join(dir, "internal", "view")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el subdirectorio: %v", err)
	}
	inner := filepath.Join(sub, "editor.go")
	if err := os.WriteFile(inner, []byte("package view"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	app := newExplorerApp(t, dir)
	if !app.explorerFocused {
		t.Fatal("el arranque sobre un directorio enfoca el explorador")
	}
	return app, inner
}

// TestShiftTabRevealsTheActiveFile: Shift+Tab no solo devuelve el foco al
// explorador: REVELA el buffer activo. El archivo se abrió por el modelo con
// el árbol todavía colapsado (internal sin hijos cargados); al volver con
// Shift+Tab el controlador expande el camino con E/S real y deja el cursor
// sobre editor.go.
func TestShiftTabRevealsTheActiveFile(t *testing.T) {
	app, inner := newSubtreeApp(t)

	if _, err := app.ws.Open(inner); err != nil {
		t.Fatalf("no se pudo abrir el buffer: %v", err)
	}
	app.explorerFocused = false // foco al editor

	press(app, tcell.KeyBacktab)

	if !app.explorerFocused {
		t.Fatal("Shift+Tab debe devolver el foco al explorador")
	}
	if got := app.explorer.CursorPath(); got != inner {
		t.Fatalf("CursorPath() = %q, se esperaba el archivo activo %q (el árbol debe revelarlo)", got, inner)
	}

	// El panel dibuja el archivo en alguna fila (la activa, con el marcador).
	found := false
	for y := 0; y < 5; y++ {
		if strings.Contains(panelRow(app, y), "editor.go") {
			found = true
		}
	}
	if !found {
		t.Fatal("el panel debe dibujar el archivo revelado en alguna fila")
	}
}

// TestCtrlBShowRevealsTheActiveFile: mostrar el panel con Ctrl+B es otra
// puerta de ENTRADA del foco y también revela el buffer activo; ocultarlo no
// toca el árbol.
func TestCtrlBShowRevealsTheActiveFile(t *testing.T) {
	app, inner := newSubtreeApp(t)

	if _, err := app.ws.Open(inner); err != nil {
		t.Fatalf("no se pudo abrir el buffer: %v", err)
	}
	app.explorerFocused = false // foco al editor

	press(app, tcell.KeyCtrlB) // ocultar
	if app.explorerVisible {
		t.Fatal("el test requiere el panel oculto")
	}
	press(app, tcell.KeyCtrlB) // mostrar: enfoca y revela

	if !app.explorerVisible || !app.explorerFocused {
		t.Fatal("el segundo Ctrl+B debe volver a mostrar y enfocar el panel")
	}
	if got := app.explorer.CursorPath(); got != inner {
		t.Fatalf("CursorPath() = %q tras mostrar con Ctrl+B, se esperaba el archivo activo %q", got, inner)
	}
}

// TestClickOnTheTreeDoesNotReveal: un clic en el árbol es intención del
// usuario: selecciona la fila del clic y NO se pisa con el reveal del buffer
// activo, que solo corre al ENTRAR el foco por teclado o al mostrar el panel.
func TestClickOnTheTreeDoesNotReveal(t *testing.T) {
	app, inner := newSubtreeApp(t)

	if _, err := app.ws.Open(inner); err != nil {
		t.Fatalf("no se pudo abrir el buffer: %v", err)
	}
	app.explorerFocused = false
	press(app, tcell.KeyBacktab)
	if got := app.explorer.CursorPath(); got != inner {
		t.Fatalf("el test requiere el reveal previo: CursorPath() = %q", got)
	}

	// Clic sobre el NOMBRE del dir del primer nivel (fuera de su caret): solo
	// selecciona la fila 0 (internal) y no se re-revela el buffer activo.
	app.handleEvent(tcell.NewEventMouse(5, tabBarHeight, tcell.Button1, tcell.ModNone))
	want := filepath.Dir(filepath.Dir(inner)) // el dir internal del nivel raíz
	if got := app.explorer.CursorPath(); got != want {
		t.Fatalf("CursorPath() = %q tras el clic, se esperaba el dir clickeado %q (el clic manda)", got, want)
	}
}

// TestCtrlBTogglesWithAnEmptyWorkspace: el toggle del panel funciona también
// sin ningún buffer abierto —el camino de entrada de U3 es el arranque sobre
// un directorio con el workspace vacío, y ocultar el panel para ganar ancho
// no puede exigir abrir primero un archivo.
func TestCtrlBTogglesWithAnEmptyWorkspace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	app := newExplorerApp(t, dir)
	if !app.explorerVisible || !app.explorerFocused {
		t.Fatal("el arranque sobre un directorio muestra y enfoca el panel")
	}

	press(app, tcell.KeyCtrlB)
	if app.explorerVisible {
		t.Fatal("Ctrl+B debe ocultar el panel también con el workspace vacío")
	}

	press(app, tcell.KeyCtrlB)
	if !app.explorerVisible || !app.explorerFocused {
		t.Fatal("el segundo Ctrl+B debe volver a mostrar y enfocar el panel")
	}
}

// TestExplorerNavigationCancelsThePendingConfirmations: navegar el panel es
// "seguir trabajando", como cualquier otra tecla del documento: una
// confirmación de salida armada con Escape se desarma, y sin eso el próximo
// Escape cerraría sin avisar.
func TestExplorerNavigationCancelsThePendingConfirmations(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(doc, []byte("uno"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	app := newExplorerApp(t, dir)

	press(app, tcell.KeyEnter) // abre a.txt: foco al editor
	typeRune(app, 'X')         // ensuciar
	press(app, tcell.KeyEscape)
	if !app.confirmQuit {
		t.Fatal("el test requiere la confirmación de salida armada")
	}

	// Foco al panel con un clic y navegar la lista: es otro "trabajo".
	app.handleEvent(tcell.NewEventMouse(1, tabBarHeight, tcell.Button1, tcell.ModNone))
	press(app, tcell.KeyDown)

	if app.confirmQuit {
		t.Fatal("navegar el explorador debe desarmar la confirmación de salida")
	}

	// La prueba de punta a punta: Escape vuelve a avisar en vez de cerrar.
	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("tras navegar el panel, Escape debe volver a avisar y no cerrar")
	}
	if !app.confirmQuit {
		t.Fatal("Escape debe haber vuelto a armar la confirmación, no cerrar")
	}
}

// TestEnterOpensAFileFromDepth: un archivo dentro de un subdir se abre desde
// el árbol: expandir el dir, bajar al archivo y Enter lo abre con su contenido,
// devolviendo el foco al editor.
func TestEnterOpensAFileFromDepth(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "carpeta")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el directorio: %v", err)
	}
	doc := filepath.Join(sub, "fuente.txt")
	if err := os.WriteFile(doc, []byte("adentro"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)

	press(app, tcell.KeyEnter) // expande carpeta
	press(app, tcell.KeyDown)  // al hijo fuente.txt
	press(app, tcell.KeyEnter) // lo abre

	if got := app.ws.Len(); got != 1 {
		t.Fatalf("Len() = %d, se esperaba 1", got)
	}
	if got := app.ws.Active().Path(); got != doc {
		t.Fatalf("Path() = %q, se esperaba %q (el archivo del subdir)", got, doc)
	}
	if got := app.ws.Active().GetContent(); got != "adentro" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "adentro")
	}
	if app.explorerFocused {
		t.Fatal("abrir un archivo debe devolver el foco al editor")
	}
}

// TestSubdirsAreNotReadUntilExpanded: el arranque lee SOLO el primer nivel del
// árbol. Un archivo dentro de un subdir no aparece en el panel hasta que el
// subdir se expande (ActionExpand) —la única E/S por nivel—: se observa por el
// dibujo del panel antes y después de la expansión.
func TestSubdirsAreNotReadUntilExpanded(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "carpeta")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el directorio: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "fuente.txt"), []byte("adentro"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)
	app.redraw()

	// Al arrancar el árbol tiene UN solo nivel: el subdir se dibuja colapsado
	// y su contenido NO aparece —el controlador aún no lo leyó—.
	if got := panelRow(app, 0); got != "▸ carpeta/" {
		t.Fatalf("fila 0 del panel = %q, se esperaba %q (subdir colapsado)", got, "▸ carpeta/")
	}
	if got := panelRow(app, 1); got != "" {
		t.Fatalf("fila 1 del panel = %q, se esperaba vacía: el subdir no se leyó al arrancar", got)
	}

	// La expansión es la lectura: tras Enter, el controlador lee el subdir y
	// sus hijos aparecen.
	press(app, tcell.KeyEnter)
	app.redraw()
	if got := panelRow(app, 1); got != "  · fuente.txt" {
		t.Fatalf("fila 1 del panel = %q tras expandir, se esperaba %q", got, "  · fuente.txt")
	}
}

// TestEnterTogglesAnExpandedDirectory: Enter alterna expandido ↔ colapsado
// sobre un directorio (la misma tecla abre y cierra, sin releer ni duplicar
// hijos); la selección queda siempre en el dir.
func TestEnterTogglesAnExpandedDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "carpeta")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el directorio: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "fuente.txt"), []byte("adentro"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "otra.txt"), []byte("dos"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)

	press(app, tcell.KeyEnter) // expande el dir
	app.redraw()
	if got := panelRow(app, 0); got != "▾ carpeta/" {
		t.Fatalf("fila 0 del panel tras el primer Enter = %q, se esperaba %q", got, "▾ carpeta/")
	}
	if got := panelRow(app, 1); got != "  · fuente.txt" {
		t.Fatalf("fila 1 del panel = %q, se esperaba %q", got, "  · fuente.txt")
	}

	press(app, tcell.KeyEnter) // lo colapsa (toggle)
	app.redraw()
	if got := panelRow(app, 0); got != "▸ carpeta/" {
		t.Fatalf("fila 0 del panel tras el segundo Enter = %q, se esperaba %q (dir colapsado)", got, "▸ carpeta/")
	}
	if got := panelRow(app, 1); got != "" {
		t.Fatalf("fila 1 del panel = %q, se esperaba vacía: los hijos se ocultaron al colapsar", got)
	}
	if got := app.explorer.CursorPath(); got != sub {
		t.Fatalf("CursorPath() = %q tras colapsar, se esperaba %q (la selección queda en el dir)", got, sub)
	}

	press(app, tcell.KeyEnter) // y un tercer Enter lo vuelve a expandir, sin releer
	app.redraw()
	if got := panelRow(app, 0); got != "▾ carpeta/" {
		t.Fatalf("fila 0 del panel tras el tercer Enter = %q, se esperaba %q", got, "▾ carpeta/")
	}
	if got := panelRow(app, 2); got != "  · otra.txt" {
		t.Fatalf("fila 2 del panel = %q, se esperaba %q (los hijos no se duplican en el toggle)", got, "  · otra.txt")
	}
}

// TestClickSelectsAnEntryAtAnyDepth: el clic selecciona la fila del panel
// —incluido un nodo que esté a profundidad, dentro de un dir expandido—, y
// Enter lo abre desde esa profundidad.
func TestClickSelectsAnEntryAtAnyDepth(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "carpeta")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el directorio: %v", err)
	}
	subsub := filepath.Join(sub, "interior")
	if err := os.Mkdir(subsub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el directorio: %v", err)
	}
	doc := filepath.Join(subsub, "archivo.txt")
	if err := os.WriteFile(doc, []byte("profundo"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)

	press(app, tcell.KeyEnter) // expande carpeta → [carpeta, interior]
	press(app, tcell.KeyDown)  // al interior
	press(app, tcell.KeyEnter) // expande interior → [carpeta, interior, archivo.txt]

	// Quitar el foco: el clic tiene que devolverlo y seleccionar la fila 2 del
	// panel, un nodo de profundidad 2.
	press(app, tcell.KeyTab)
	if app.explorerFocused {
		t.Fatal("el test requiere el foco en el editor primero")
	}
	app.handleEvent(tcell.NewEventMouse(1, tabBarHeight+2, tcell.Button1, tcell.ModNone))

	if !app.explorerFocused {
		t.Fatal("un clic dentro del panel debe enfocarlo")
	}
	if got := app.explorer.CursorPath(); got != doc {
		t.Fatalf("CursorPath() = %q, se esperaba %q: el clic selecciona el nodo de profundidad", got, doc)
	}

	// Y Enter lo abre desde esa profundidad.
	press(app, tcell.KeyEnter)
	if got := app.ws.Active().GetContent(); got != "profundo" {
		t.Fatalf("contenido = %q tras abrir desde profundidad, se esperaba %q", got, "profundo")
	}
}

// TestClickInThePanelFocusesIt: un clic dentro del panel lo enfoca y
// selecciona la fila clicada.
func TestClickInThePanelFocusesIt(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(doc, []byte("a"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)

	// Quitar el foco del explorador para que el clic tenga que devolverlo.
	press(app, tcell.KeyTab)
	if app.explorerFocused {
		t.Fatal("el test requiere el foco en el editor primero")
	}

	// Clic en la fila 0 del panel: (1, tabBarHeight) de pantalla.
	app.handleEvent(tcell.NewEventMouse(1, tabBarHeight, tcell.Button1, tcell.ModNone))
	if !app.explorerFocused {
		t.Fatal("un clic dentro del panel debe enfocarlo")
	}
	if got := app.explorer.CursorPath(); got != doc {
		t.Fatalf("CursorPath() = %q, se esperaba %q: el clic selecciona la fila", got, doc)
	}
}

// TestCtrlPageSwitchingWorksWithTheExplorerFocused: cambiar de pestaña con
// Ctrl+PageUp/PageDown sigue funcionando con el foco en el panel: el
// explorador no consume esos Pg con ModCtrl (son del controlador, de U2b), o
// la navegación de pestañas moriría mientras la lista está enfocada.
func TestCtrlPageSwitchingWorksWithTheExplorerFocused(t *testing.T) {
	dir := t.TempDir()
	nameA := filepath.Join(dir, "a.txt")
	nameB := filepath.Join(dir, "b.txt")
	for path, c := range map[string]string{nameA: "uno", nameB: "dos"} {
		if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
			t.Fatalf("no se pudo crear el archivo: %v", err)
		}
	}
	app := newExplorerApp(t, dir)

	// Abrir a.txt (Enter) y volver el foco al panel con un clic; abrir b.txt
	// y volver a clicar: dos pestañas abiertas con el explorador enfocado.
	// Volver al panel es con clic —Tab con el foco en el editor inserta
	// tabulación y no cambia de pane—.
	press(app, tcell.KeyEnter)
	app.handleEvent(tcell.NewEventMouse(1, tabBarHeight, tcell.Button1, tcell.ModNone))
	press(app, tcell.KeyDown)
	press(app, tcell.KeyEnter)
	app.handleEvent(tcell.NewEventMouse(1, tabBarHeight, tcell.Button1, tcell.ModNone))
	if !app.explorerFocused {
		t.Fatal("el test requiere el foco en el explorador")
	}
	if got := app.ws.Len(); got != 2 {
		t.Fatalf("Len() = %d, se esperaban 2 pestañas", got)
	}

	// Ctrl+PageUp con el panel enfocado cambia a la pestaña anterior.
	app.handleEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModCtrl))
	if got := app.ws.Active().Path(); got != nameA {
		t.Fatalf("tras Ctrl+PageUp, la activa = %q, se esperaba %q", got, nameA)
	}
}

// TestMouseClickInTheEditorIsTranslatedPastThePanel: con el panel visible, el
// clic de pantalla en (24+1, 2) cae en la columna 1 de la línea 1 del
// documento; sin la traducción por la columna del panel habría caído en otra
// columna y eso se observa tipeando después del clic.
func TestMouseClickInTheEditorIsTranslatedPastThePanel(t *testing.T) {
	app, _ := newTestApp(t, "uno\ndos\ntres")
	resizeApp(app, 80, 8)
	press(app, tcell.KeyCtrlB) // mostrar el panel: el editor arranca en x=24

	// Pantalla (27, 2): el editor recibe (3, 1) — línea 1, columna 1 de "dos"
	// (3 = gutter de 2 + columna 1 del texto).
	app.handleEvent(tcell.NewEventMouse(27, 2, tcell.Button1, tcell.ModNone))
	typeRune(app, 'X')

	if got := app.ws.Active().GetContent(); got != "uno\ndXos\ntres" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "uno\ndXos\ntres")
	}
}

// --- U4: menú de pestañas (Ctrl+T) y sesión JSON ---

// TestCtrlTOpensTheTabMenu: Ctrl+T abre el menú transitorio y su cursor queda
// sobre la pestaña activa.
func TestCtrlTOpensTheTabMenu(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")

	if quit := press(app, tcell.KeyCtrlT); quit {
		t.Fatal("Ctrl+T no debe cerrar el editor")
	}
	if !app.menuActive {
		t.Fatal("Ctrl+T debe abrir el menú de pestañas")
	}
	if got := app.menu.Selected(); got != app.ws.ActiveIndex() {
		t.Fatalf("el cursor del menú = %d, se esperaba sobre la activa %d", got, app.ws.ActiveIndex())
	}
}

// TestTabMenuEnterSwitchesToTheSelectedTab: Down mueve el cursor del menú y
// Enter cambia a la pestaña elegida y cierra el menú.
func TestTabMenuEnterSwitchesToTheSelectedTab(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")

	press(app, tcell.KeyCtrlT)
	press(app, tcell.KeyDown)
	if quit := press(app, tcell.KeyEnter); quit {
		t.Fatal("Enter del menú no debe cerrar el editor")
	}

	if app.menuActive {
		t.Fatal("activar una pestaña debe cerrar el menú")
	}
	if got := app.ws.ActiveIndex(); got != 1 {
		t.Fatalf("ActiveIndex() = %d tras Enter sobre la segunda, se esperaba 1", got)
	}
}

// TestTabMenuEscapeDismissesWithoutSwitching: Escape cierra el menú sin
// cambiar de pestaña: nada de lo navegado se ejecuta.
func TestTabMenuEscapeDismissesWithoutSwitching(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")

	press(app, tcell.KeyCtrlT)
	press(app, tcell.KeyDown) // mover el cursor: Escape tiene que descartar esto
	press(app, tcell.KeyEscape)

	if app.menuActive {
		t.Fatal("Escape debe cerrar el menú")
	}
	if got := app.ws.ActiveIndex(); got != 0 {
		t.Fatalf("ActiveIndex() = %d, se esperaba 0: Escape no debe cambiar la pestaña", got)
	}
}

// TestTabMenuAnotherKeyDismisses: cualquier otra tecla —una letra— también
// cierra el menú descartando: el teclado es del menú mientras está abierto y
// nada cae al documento.
func TestTabMenuAnotherKeyDismisses(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	press(app, tcell.KeyCtrlT)
	press(app, tcell.KeyDown)
	typeRune(app, 'x')

	if app.menuActive {
		t.Fatal("una tecla ajena debe cerrar el menú")
	}
	if got := app.ws.ActiveIndex(); got != 0 {
		t.Fatalf("ActiveIndex() = %d, la tecla ajena no debe cambiar la pestaña", got)
	}
	if got := app.ws.Active().GetContent(); got != "uno" {
		t.Fatalf("contenido = %q, la tecla ajena no debe llegar al documento", got)
	}
}

// TestCtrlTOnEmptyWorkspaceDoesNothing: sin pestañas, Ctrl+T no abre nada
// (el atajo vive después del guard de workspace vacío, como los otros).
func TestCtrlTOnEmptyWorkspaceDoesNothing(t *testing.T) {
	app := newExplorerApp(t, t.TempDir())

	if quit := press(app, tcell.KeyCtrlT); quit {
		t.Fatal("Ctrl+T no debe cerrar el editor")
	}
	if app.menuActive {
		t.Fatal("Ctrl+T con el workspace vacío no debe abrir el menú")
	}
}

// TestRunSavesTheSession: al salir con Run, la sesión queda en
// <root>/.tcode/session.json con el root, las pestañas abiertas y la activa.
func TestRunSavesTheSession(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(doc, []byte("contenido"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)
	press(app, tcell.KeyEnter) // abre doc.txt desde el explorador

	app.screen.(tcell.SimulationScreen).InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	app.screen.(tcell.SimulationScreen).InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	if err := app.Run(); err != nil {
		t.Fatalf("Run falló: %v", err)
	}

	path := filepath.Join(dir, ".tcode", "session.json")
	s, err := model.LoadSession(path)
	if err != nil {
		t.Fatalf("la sesión debe guardarse al salir: %v", err)
	}
	if s.Root != dir {
		t.Fatalf("Root = %q, se esperaba %q", s.Root, dir)
	}
	if len(s.Tabs) != 1 || s.Tabs[0] != doc {
		t.Fatalf("Tabs = %v, se esperaba [%q]", s.Tabs, doc)
	}
	if s.Active != doc {
		t.Fatalf("Active = %q, se esperaba %q", s.Active, doc)
	}
}

// TestStartupRestoresTheSession: arrancar sobre un directorio con session.json
// restaura las pestañas en orden de la sesión con la activa por ruta.
func TestStartupRestoresTheSession(t *testing.T) {
	dir := t.TempDir()
	nameA := filepath.Join(dir, "a.txt")
	nameB := filepath.Join(dir, "b.txt")
	for path, c := range map[string]string{nameA: "uno", nameB: "dos"} {
		if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
			t.Fatalf("no se pudo crear el archivo: %v", err)
		}
	}
	if err := model.SaveSession(filepath.Join(dir, ".tcode", "session.json"), model.Session{
		Version: 1,
		Root:    dir,
		Tabs:    []string{nameA, nameB},
		Active:  nameB,
	}); err != nil {
		t.Fatalf("SaveSession falló: %v", err)
	}

	app := newExplorerApp(t, dir)

	if got := app.ws.Len(); got != 2 {
		t.Fatalf("Len() = %d, se esperaban 2 pestañas restauradas", got)
	}
	if got := app.ws.Buffers()[0].Path(); got != nameA {
		t.Fatalf("pestaña 0 = %q, se esperaba %q (el orden de la sesión)", got, nameA)
	}
	if got := app.ws.Buffers()[1].Path(); got != nameB {
		t.Fatalf("pestaña 1 = %q, se esperaba %q", got, nameB)
	}
	if got := app.ws.Active().Path(); got != nameB {
		t.Fatalf("activa = %q, se esperaba %q (la activa de la sesión)", got, nameB)
	}
}

// TestFileStartupIgnoresTheSession: abrir un ARCHIVO por línea de comandos es
// una acción puntual de edición, no una sesión: ni restaura ni guarda.
func TestFileStartupIgnoresTheSession(t *testing.T) {
	dir := t.TempDir()
	nameA := filepath.Join(dir, "a.txt")
	nameB := filepath.Join(dir, "b.txt")
	for path, c := range map[string]string{nameA: "uno", nameB: "dos"} {
		if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
			t.Fatalf("no se pudo crear el archivo: %v", err)
		}
	}
	// La sesión que habría restaurado a y b con b activa.
	if err := model.SaveSession(filepath.Join(dir, ".tcode", "session.json"), model.Session{
		Version: 1,
		Root:    dir,
		Tabs:    []string{nameA, nameB},
		Active:  nameB,
	}); err != nil {
		t.Fatalf("SaveSession falló: %v", err)
	}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	app, err := NewAppWithScreen(s, nameA)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	t.Cleanup(func() { app.ws.CloseAll(); s.Fini() })

	if got := app.ws.Len(); got != 1 {
		t.Fatalf("Len() = %d, el modo archivo no debe restaurar la sesión", got)
	}
	if got := app.ws.Active().Path(); got != nameA {
		t.Fatalf("activa = %q, se esperaba %q (el archivo de la línea de comandos)", got, nameA)
	}
}

// TestSessionLoadSkipsMissingTabs: una pestaña de la sesión cuyo archivo ya no
// existe se salta sin abortar el arranque; la activa se resuelve por ruta entre
// las que sobrevivieron.
func TestSessionLoadSkipsMissingTabs(t *testing.T) {
	dir := t.TempDir()
	alive := filepath.Join(dir, "viva.txt")
	dead := filepath.Join(dir, "muerta.txt")
	if err := os.WriteFile(alive, []byte("viva"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	// muerta.txt NO se crea: la pestaña de la sesión apunta a un archivo ido.
	if err := model.SaveSession(filepath.Join(dir, ".tcode", "session.json"), model.Session{
		Version: 1,
		Root:    dir,
		Tabs:    []string{dead, alive},
		Active:  alive,
	}); err != nil {
		t.Fatalf("SaveSession falló: %v", err)
	}

	app := newExplorerApp(t, dir)

	if got := app.ws.Len(); got != 1 {
		t.Fatalf("Len() = %d, la pestaña muerta debe saltarse, se esperaba 1", got)
	}
	if got := app.ws.Active().Path(); got != alive {
		t.Fatalf("activa = %q, se esperaba %q", got, alive)
	}
}

// TestQuitRemovesTheSessionFileWhenNoTabs: sin pestañas persistibles al salir,
// la sesión se BORRA: el estado por defecto del directorio vuelve a ser "sin
// sesión" en lugar de un archivo que miente con tabs vacíos.
func TestQuitRemovesTheSessionFileWhenNoTabs(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(doc, []byte("contenido"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)
	if _, err := app.ws.Open(doc); err != nil {
		t.Fatalf("Open falló: %v", err)
	}
	// Una sesión PREVIA dejó el archivo (sin él, el assert pasarí aun sin
	// saveSession): el cierre tiene que borrarlo.
	if err := model.SaveSession(sessionPath(dir), model.Session{
		Version: 1,
		Root:    dir,
		Tabs:    []string{doc},
		Active:  doc,
	}); err != nil {
		t.Fatalf("SaveSession falló: %v", err)
	}
	if err := app.ws.Close(0); err != nil {
		t.Fatalf("Close falló: %v", err)
	}

	app.screen.(tcell.SimulationScreen).InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	app.screen.(tcell.SimulationScreen).InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	if err := app.Run(); err != nil {
		t.Fatalf("Run falló: %v", err)
	}

	if _, err := os.Stat(sessionPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("sin pestañas persistibles la sesión debe borrarse: %v", err)
	}
}

// --- pestañas con el mouse ---

// TestClickOnATabSwitchesToIt: el clic sobre la fila de pestañas activa la
// pestaña clickeada, como en cualquier editor. A 80 columnas con el panel
// oculto: "[a.txt]" en 0..6, separador en 7, "b.txt" en 8..12.
func TestClickOnATabSwitchesToIt(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")
	resizeApp(app, 80, 8)
	app.redraw()
	if got := app.ws.ActiveIndex(); got != 0 {
		t.Fatalf("ActiveIndex() inicial = %d, se esperaba 0", got)
	}

	app.handleEvent(tcell.NewEventMouse(8, 0, tcell.Button1, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 1 {
		t.Fatalf("ActiveIndex() = %d tras el clic en la segunda pestaña, se esperaba 1", got)
	}
	if got := app.ws.Active().Path(); got == "" {
		t.Fatal("la pestaña activa debe estar abierta")
	}

	app.handleEvent(tcell.NewEventMouse(1, 0, tcell.Button1, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 0 {
		t.Fatalf("ActiveIndex() = %d tras el clic en la primera pestaña, se esperaba 0", got)
	}
}

// TestClickOnATabWithThePanelVisible: con el panel a la izquierda la fila de
// pestañas arranca en la columna del editor; el clic se traduce igual y un clic
// sobre la fila del árbol no activa ninguna pestaña.
func TestClickOnATabWithThePanelVisible(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")
	resizeApp(app, 80, 8)
	press(app, tcell.KeyCtrlB) // panel visible: la barra arranca en x=24
	app.redraw()

	// "[a.txt]" ocupa 24..30, separador en 31, "b.txt" en 32..36.
	app.handleEvent(tcell.NewEventMouse(24+9, 0, tcell.Button1, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 1 {
		t.Fatalf("ActiveIndex() = %d, se esperaba 1 (clic traducido por la columna del panel)", got)
	}

	// Un clic sobre la fila del árbol (x < panelW) no activa pestañas: la fila
	// de pestañas no vive sobre el panel.
	app.handleEvent(tcell.NewEventMouse(1, 0, tcell.Button1, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 1 {
		t.Fatalf("ActiveIndex() = %d tras un clic sobre el árbol en la fila 0, no debía cambiar", got)
	}
}

// TestWheelOverTheTabBarSwitchesTabs: la rueda sobre la fila de pestañas cambia
// de pestaña (abajo = siguiente, arriba = anterior), como en un navegador.
func TestWheelOverTheTabBarSwitchesTabs(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")
	resizeApp(app, 80, 8)
	app.redraw()

	app.handleEvent(tcell.NewEventMouse(2, 0, tcell.WheelDown, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 1 {
		t.Fatalf("ActiveIndex() = %d tras la rueda abajo, se esperaba 1", got)
	}
	app.handleEvent(tcell.NewEventMouse(2, 0, tcell.WheelUp, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 0 {
		t.Fatalf("ActiveIndex() = %d tras la rueda arriba, se esperaba 0", got)
	}
}

// TestSaveAsPromptOwnsTheMouse: mientras el pedido de Save As está abierto, el
// mouse es del pedido: un clic sobre la fila de pestañas no cambia de pestaña
// por debajo de lo que el usuario está escribiendo.
func TestSaveAsPromptOwnsTheMouse(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")
	resizeApp(app, 80, 8)
	app.redraw()

	pressSaveAs(app)
	if !app.promptActive {
		t.Fatal("el pedido de Save As debía quedar abierto")
	}

	app.handleEvent(tcell.NewEventMouse(7, 0, tcell.Button1, tcell.ModNone))
	if got := app.ws.ActiveIndex(); got != 0 {
		t.Fatalf("ActiveIndex() = %d: el pedido debe ser dueño del mouse y no cambiar de pestaña por debajo", got)
	}
	if !app.promptActive {
		t.Fatal("el clic no debe cerrar el pedido")
	}
}

// TestCtrlCCopiesSelectionInsteadOfQuitting: Ctrl+C con una selección activa
// copia (mensaje "Copiado") y el editor NO sale; sin selección, Ctrl+C dejó
// de ser una forma de salir: la única salida es la doble presión de Escape.
func TestCtrlCCopiesSelectionInsteadOfQuitting(t *testing.T) {
	app, _ := newTestApp(t, "uno\ndos")

	// Seleccionar la primera palabra con Shift+derechas.
	app.handleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	app.handleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	app.handleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))

	if quit := press(app, tcell.KeyCtrlC); quit {
		t.Fatal("Ctrl+C con selección no debe cerrar el editor")
	}
	if msg := app.toast.Message(); msg != "Copiado" && !strings.Contains(app.statusBar.Message(), "Error al copiar") {
		t.Errorf("toast = %q, esperaba el aviso del copiado", msg)
	}
	if app.ws.Active() == nil {
		t.Fatal("el editor no debe haber salido")
	}
}

// TestDiagMessageStaysOffStatusBar: el diagnóstico ya no vive en la barra de
// estado (decisión de producto: se muestra inline, a la derecha de cada línea
// anotada, para ver varias a la vez). Mover el cursor por una línea anotada
// NO debe depositar el mensaje del diagnóstico en la barra.
func TestDiagMessageStaysOffStatusBar(t *testing.T) {
	app, _ := newTestApp(t, "uno\ndos\ntres")
	ed := app.activeEditor()
	ed.SetDiagnostics("", []view.Diagnostic{
		{Line: 0, Message: "mal", Severity: view.SeverityError},
		{Line: 0, Message: "aviso", Severity: view.SeverityWarning},
	})

	// Una tecla que mueve el cursor (derecha) pasa por el path de teclas del
	// editor: la línea 0 del cursor tiene diagnóstico, pero la barra no lo
	// muestra.
	press(app, tcell.KeyRight)
	if got := app.statusBar.Message(); got != "" {
		t.Fatalf("mensaje = %q, se esperaba la barra sin diagnóstico (el inline vive en el view)", got)
	}

	press(app, tcell.KeyDown)
	if got := app.statusBar.Message(); got != "" {
		t.Fatalf("mensaje = %q, la barra debe seguir sin diagnóstico", got)
	}
}

// --- crear archivos y carpetas ---

// newCreateApp arranca el editor sobre un directorio temporal como ARGUMENTO (el
// modo "sin buffers"): el workspace queda vacío, la raíz de la sesión es ese
// directorio y el explorador visible y enfocado —el estado real donde se crea un
// archivo por primera vez—. Devuelve la app y la raíz: crear SIEMPRE apunta a un
// temporal, nunca al directorio de trabajo del proceso de test.
func newCreateApp(t *testing.T) (*App, string) {
	t.Helper()

	dir := t.TempDir()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	s.SetSize(40, 10)

	app, err := NewAppWithScreen(s, dir)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	t.Cleanup(func() {
		app.ws.CloseAll()
		s.Fini()
	})
	if got := app.ws.Root(); got != dir {
		t.Fatalf("Root() = %q, se esperaba el temporal %q", got, dir)
	}
	return app, dir
}

// pressCtrlN dispara Ctrl+N (código KeyCtrlN sin modificadores).
func pressCtrlN(app *App) {
	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlN, 0, tcell.ModNone))
}

// pressCtrlShiftN dispara Ctrl+Shift+N, que tcell entrega como KeyRune con
// ModCtrl y ModShift.
func pressCtrlShiftN(app *App) {
	app.handleEvent(tcell.NewEventKey(tcell.KeyRune, 'N', tcell.ModCtrl|tcell.ModShift))
}

// TestCtrlNCreatesFileInRootAndOpensIt: con el workspace vacío y sin nodos en el
// árbol, Ctrl+N abre el pedido con el destino contextual (la raíz de la sesión),
// el archivo creado existe en disco, entra al árbol y queda ABIERTO y activo en
// el editor —que es el punto de la feature: crear es la primera acción de una
// sesión sin buffers.
func TestCtrlNCreatesFileInRootAndOpensIt(t *testing.T) {
	app, root := newCreateApp(t)

	pressCtrlN(app)
	if !app.promptActive {
		t.Fatal("Ctrl+N debe abrir el pedido incluso con el workspace vacío")
	}
	if !strings.Contains(app.statusBar.Label(), "Nuevo archivo") {
		t.Fatalf("etiqueta del pedido = %q, se esperaba el rótulo de creación", app.statusBar.Label())
	}
	typeString(app, "nuevo.txt")
	press(app, tcell.KeyEnter)

	path := filepath.Join(root, "nuevo.txt")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("el archivo no se creó en %q: %v", path, err)
	}
	buf := app.ws.Active()
	if buf == nil {
		t.Fatal("el archivo creado debe quedar abierto en el editor")
	}
	if buf.Path() != path {
		t.Fatalf("buffer activo = %q, se esperaba %q", buf.Path(), path)
	}
	if !treeHasPath(app, path) {
		t.Fatalf("el nodo nuevo no entró al árbol del explorador")
	}
	if !strings.Contains(app.toast.Message(), "nuevo.txt") {
		t.Fatalf("toast = %q, se esperaba el nombre del archivo creado", app.toast.Message())
	}
	if app.toast.Kind() != view.ToastSuccess {
		t.Fatalf("el toast es de tipo %v, se esperaba éxito", app.toast.Kind())
	}
}

// treeHasPath recorre el árbol visible del explorador con la tecla de abajo y
// dice si el nodo de esa ruta está en él: el test necesita saber si el nodo
// entró al árbol, no en qué fila quedó.
func treeHasPath(app *App, path string) bool {
	for range 50 {
		if app.explorer.CursorPath() == path {
			return true
		}
		app.explorer.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	return false
}

// TestCtrlShiftNCreatesFolderInsideTheCursorDirectory: el destino es contextual
// —con el cursor sobre una carpeta, el nuevo nodo va DENTRO de ella— y una
// carpeta no se abre: el workspace sigue como estaba.
func TestCtrlShiftNCreatesFolderInsideTheCursorDirectory(t *testing.T) {
	app, root := newCreateApp(t)
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("no se pudo crear el dir de prueba: %v", err)
	}
	app.explorer.SetRootEntries([]view.Entry{
		{Name: "docs", Path: filepath.Join(root, "docs"), IsDir: true},
	})

	pressCtrlShiftN(app)
	if !app.promptActive || !strings.Contains(app.statusBar.Label(), "Nueva carpeta") {
		t.Fatalf("Ctrl+Shift+N debe abrir el pedido de carpeta (etiqueta %q)", app.statusBar.Label())
	}
	typeString(app, "borradores")
	press(app, tcell.KeyEnter)

	info, err := os.Stat(filepath.Join(root, "docs", "borradores"))
	if err != nil {
		t.Fatalf("la carpeta no se creó dentro de docs: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("lo creado debe ser un directorio")
	}
	if app.ws.Len() != 0 {
		t.Fatalf("Len() = %d, se esperaba 0: una carpeta no abre buffer", app.ws.Len())
	}
}

// TestCreateDestinationFollowsTheCursorFile: con el cursor sobre un archivo, el
// destino es el directorio que lo contiene, no la raíz.
func TestCreateDestinationFollowsTheCursorFile(t *testing.T) {
	app, root := newCreateApp(t)
	sub := filepath.Join(root, "docs")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el dir de prueba: %v", err)
	}
	inside := filepath.Join(sub, "notas.txt")
	if err := os.WriteFile(inside, []byte("x"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo de prueba: %v", err)
	}
	app.explorer.SetRootEntries([]view.Entry{
		{Name: "docs", Path: sub, IsDir: true},
		{Name: "notas.txt", Path: filepath.Join(root, "notas.txt")},
	})
	app.explorer.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) // cursor al archivo
	if got := app.explorer.CursorDir(); got != root {
		t.Fatalf("CursorDir() = %q, se esperaba la raíz %q", got, root)
	}

	pressCtrlN(app)
	typeString(app, "hermano.txt")
	press(app, tcell.KeyEnter)

	if _, err := os.Stat(filepath.Join(root, "hermano.txt")); err != nil {
		t.Fatalf("el archivo no se creó junto al del cursor: %v", err)
	}
}

// TestCreateRejectsInvalidNames: nombre vacío, separadores de ruta, ".." y un
// nombre que ya existe se rechazan con un mensaje legible, sin crear nada en
// disco ni tocar el árbol.
func TestCreateRejectsInvalidNames(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"vacío", ""},
		{"solo espacios", "   "},
		{"separador unix", "a/b.txt"},
		{"separador windows", `a\b.txt`},
		{"salida del árbol", "../fuera.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, root := newCreateApp(t)
			pressCtrlN(app)
			typeString(app, tc.input)
			press(app, tcell.KeyEnter)

			if app.ws.Len() != 0 {
				t.Fatalf("un nombre inválido no puede abrir un buffer: Len() = %d", app.ws.Len())
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatalf("no se pudo leer la raíz: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("la raíz quedó con %d entradas, se esperaba ninguna", len(entries))
			}
			if !strings.Contains(app.statusBar.Message(), "no puede") {
				t.Fatalf("mensaje = %q, se esperaba el aviso de nombre inválido", app.statusBar.Message())
			}
		})
	}
}

// TestCreateRejectsAnExistingName: crear sobre un nombre que ya existe no pisa el
// archivo ni duplica el nodo —el error es legible y el contenido sigue siendo el
// de antes.
func TestCreateRejectsAnExistingName(t *testing.T) {
	app, root := newCreateApp(t)
	existente := filepath.Join(root, "notas.txt")
	if err := os.WriteFile(existente, []byte("contenido viejo"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo de prueba: %v", err)
	}
	app.explorer.SetRootEntries([]view.Entry{{Name: "notas.txt", Path: existente}})

	pressCtrlN(app)
	typeString(app, "notas.txt")
	press(app, tcell.KeyEnter)

	if got := readFile(t, existente); got != "contenido viejo" {
		t.Fatalf("el archivo existente cambió a %q: crear no pisa nada", got)
	}
	if app.ws.Len() != 0 {
		t.Fatalf("Len() = %d, se esperaba 0: no se abrió nada", app.ws.Len())
	}
	if !strings.Contains(app.statusBar.Message(), "ya existe") {
		t.Fatalf("mensaje = %q, se esperaba el aviso de nombre repetido", app.statusBar.Message())
	}
}

// TestCreateCommandsAreRegistered: los comandos tcode.createFile y
// tcode.createFolder existen en el registro —una extensión puede invocarlos— y
// abren el pedido con el rótulo que corresponde.
func TestCreateCommandsAreRegistered(t *testing.T) {
	app, _ := newCreateApp(t)

	for _, tc := range []struct{ id, label string }{
		{"tcode.createFile", "Nuevo archivo"},
		{"tcode.createFolder", "Nueva carpeta"},
	} {
		if err := app.ext.RunCommand(tc.id); err != nil {
			t.Fatalf("%s falló: %v", tc.id, err)
		}
		if !app.promptActive {
			t.Fatalf("%s debe abrir el pedido", tc.id)
		}
		if !strings.Contains(app.statusBar.Label(), tc.label) {
			t.Fatalf("%s abrió el pedido %q, se esperaba el rótulo %q", tc.id, app.statusBar.Label(), tc.label)
		}
		press(app, tcell.KeyEscape)
	}
}

// TestCreateWorksWithTheExplorerFocused: el atajo funciona con el foco en el
// panel (donde cae al entrar) y también con el foco en el editor.
func TestCreateWorksWithTheExplorerFocused(t *testing.T) {
	app, root := newCreateApp(t)

	if !app.explorerFocused {
		t.Fatal("el arranque sin argumentos enfoca el explorador")
	}
	pressCtrlN(app)
	typeString(app, "uno.txt")
	press(app, tcell.KeyEnter)

	app.explorerFocused = false
	pressCtrlN(app)
	typeString(app, "dos.txt")
	press(app, tcell.KeyEnter)

	for _, name := range []string{"uno.txt", "dos.txt"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("%s no se creó: %v", name, err)
		}
	}
}

// TestPromptCancelClearsTheRequest: Escape cierra el pedido Generalized sin
// ejecutar la acción: el prompt queda apagado, la barra vuelve al estado normal
// y no se crea nada.
func TestPromptCancelClearsTheRequest(t *testing.T) {
	app, root := newCreateApp(t)

	pressCtrlN(app)
	typeString(app, "nuevo.txt")
	press(app, tcell.KeyEscape)

	if app.promptActive || app.promptBuf != "" || app.promptAction != nil {
		t.Fatalf("Escape debe apagar el pedido (buf %q, acción nil: %v)", app.promptBuf, app.promptAction == nil)
	}
	if !strings.Contains(app.statusBar.Label(), "") || app.toast.Message() != "Cancelado" {
		t.Fatalf("toast = %q, se esperaba el aviso de cancelación", app.toast.Message())
	}
	if app.toast.Kind() != view.ToastInfo {
		t.Fatalf("el toast es de tipo %v, se esperaba neutro", app.toast.Kind())
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("no se pudo leer la raíz: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("cancelar no puede crear nada: la raíz tiene %d entradas", len(entries))
	}
}

// TestSaveAsPromptOpensWithoutABuffer: el prompt Generalized no desreferencia el
// buffer activo —crear con el workspace vacío es justamente el caso de uso—: sin
// buffers, abrir el pedido de Save As no entra en pánico, muestra el rótulo con
// el prefill vacío y, al confirmar sin documento, responde con el error legible
// en vez de escribir sobre la nada.
func TestSaveAsPromptOpensWithoutABuffer(t *testing.T) {
	app, _ := newCreateApp(t)

	app.startPrompt()
	if !app.promptActive {
		t.Fatal("el pedido de Save As debe abrirse aunque no haya buffer activo")
	}
	if app.promptBuf != "" {
		t.Fatalf("prefill = %q, se esperaba vacío sin buffer activo", app.promptBuf)
	}
	if !strings.Contains(app.statusBar.Label(), "Guardar como") {
		t.Fatalf("etiqueta = %q, se esperaba el rótulo de Save As", app.statusBar.Label())
	}

	// Confirmar sin buffer no puede inventarse un documento: error legible.
	typeString(app, filepath.Join(t.TempDir(), "x.txt"))
	press(app, tcell.KeyEnter)
	if app.ws.Len() != 0 {
		t.Fatalf("Len() = %d, se esperaba 0: nada que guardar", app.ws.Len())
	}
	if !strings.Contains(app.statusBar.Message(), "sin buffer abierto") {
		t.Fatalf("mensaje = %q, se esperaba el error de buffer faltante", app.statusBar.Message())
	}
}

// TestCreateEntryValidatesTheNameWithoutTheKeyboard: la acción es pública
// (command tcode.createFile), así que valida el nombre aunque no venga del
// prompt.
func TestCreateEntryValidatesTheNameWithoutTheKeyboard(t *testing.T) {
	app, root := newCreateApp(t)

	if err := app.createEntry(root, "", false); err == nil {
		t.Fatal("un nombre vacío debe rechazarse")
	}
	if err := app.createEntry(root, "a/b", false); err == nil {
		t.Fatal("un nombre con separador debe rechazarse")
	}
	if err := app.createEntry("", "x.txt", false); err == nil {
		t.Fatal("sin directorio destino no hay dónde crear")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("no se pudo leer la raíz: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("la raíz quedó con %d entradas, se esperaba ninguna", len(entries))
	}
}

// TestBracketedPasteInsertsTheBlockAsOneStep: el paste del TERMINAL con modo
// bracketed (Ctrl+V en Windows Terminal) llega como EventPaste de inicio, el
// contenido como teclas y uno de cierre. Debe entrar como un bloque —un solo
// paso de undo— y no carácter por carácter.
func TestBracketedPasteInsertsTheBlockAsOneStep(t *testing.T) {
	app, _ := newTestApp(t, "fin")

	// Inicio del bloque.
	app.handleEvent(tcell.NewEventPaste(true))
	// Contenido: "hola", salto de línea, "mundo".
	for _, r := range "hola" {
		app.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	for _, r := range "mundo" {
		app.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	// Cierre: recién acá el documento cambia.
	if got := string(app.ws.Active().GetContent()); got != "fin" {
		t.Fatalf("contenido antes del cierre = %q, el paste no debe insertar tecla por tecla", got)
	}
	app.handleEvent(tcell.NewEventPaste(false))

	if got := string(app.ws.Active().GetContent()); got != "hola\nmundofin" {
		t.Fatalf("contenido tras el paste = %q, se esperaba el bloque de una vez", got)
	}
	// Un solo Ctrl+Z saca el bloque entero.
	press(app, tcell.KeyCtrlZ)
	if got := string(app.ws.Active().GetContent()); got != "fin" {
		t.Fatalf("contenido tras un undo = %q, se esperaba el documento original", got)
	}
}

// --- prompt de actualizaciones y novedades al arrancar ---

// extFetchFake emula el sparse checkout de git sobre un "monorepo" en disco:
// "*/extension.json" deja solo los manifests del proveedor y el nombre de una
// subcarpeta deja esa extensión entera. Es el mismo fake que usa el paquete
// ext, replicado acá porque los tests del controller no alcanzan las variables
// internas de ese paquete.
func extFetchFake(src string) ext.FetchFunc {
	return func(_ string, dest, pattern string) error {
		return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(src, p)
			if err != nil {
				return err
			}
			if rel == "." {
				return os.MkdirAll(dest, 0o755)
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			if strings.Contains(pattern, "/") {
				matched, err := path.Match(pattern, rel)
				if err != nil {
					return err
				}
				if !matched {
					return nil
				}
			} else if rel != pattern && !strings.HasPrefix(rel, pattern+"/") {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			to := filepath.Join(dest, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			return os.WriteFile(to, data, 0o644)
		})
	}
}

// startupProviderFixture crea un "proveedor" en disco: una subcarpeta por
// extensión con su extension.json.
func startupProviderFixture(t *testing.T, exts map[string][3]string) string {
	t.Helper()
	root := t.TempDir()
	for dir, spec := range exts {
		full := filepath.Join(root, dir)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
		writeStartupManifest(t, full, spec[0], spec[1], spec[2])
	}
	return root
}

// writeStartupManifest escribe el extension.json de una extensión del
// "proveedor": id, nombre y versión dados.
func writeStartupManifest(t *testing.T, dir, id, name, version string) {
	t.Helper()
	data, err := json.Marshal(ext.Manifest{ID: id, Name: name, Version: version})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile extension.json: %v", err)
	}
}

// pinStartupSources apunta la LECTURA DE FONDO del arranque a un proveedor en
// disco y a una raíz de extensiones temporal, y devuelve nada: todo lo que usa
// el prefetch y la ventana queda aislado del HOME real y de la red.
func pinStartupSources(t *testing.T, p ext.Provider, userRoot string) {
	t.Helper()
	pinStartupFetch(t, p, userRoot, extFetchFake(p.Source))
}

// pinStartupFetch es pinStartupSources con un lector propio: es lo que permite
// probar que el arranque NO espera a la lectura (un lector bloqueado).
func pinStartupFetch(t *testing.T, p ext.Provider, userRoot string, fetch ext.FetchFunc) {
	t.Helper()
	oldSources, oldFetch := extensionUserSources, startupExtFetch
	extensionUserSources = func() ([]ext.Provider, string, error) { return []ext.Provider{p}, userRoot, nil }
	startupExtFetch = fetch
	t.Cleanup(func() { extensionUserSources, startupExtFetch = oldSources, oldFetch })
}

// awaitExtSnapshot espera la entrega de la goroutine de arranque y la aplica
// como lo haría el bucle de eventos (handleEvent). Sin el constructor ya
// devuelto, el test no puede afirmar nada del bucle, así que el evento se
// consume a mano.
func awaitExtSnapshot(t *testing.T, app *App) {
	t.Helper()
	events := make(chan tcell.Event, 1)
	go func() { events <- app.screen.PollEvent() }()
	select {
	case ev := <-events:
		app.handleEvent(ev)
	case <-time.After(10 * time.Second):
		t.Fatal("la lectura de segundo plano no entregó el snapshot")
	}
	if app.extSnapshot == nil {
		t.Fatal("el evento llegó pero el snapshot no quedó cacheado")
	}
}

// prefetchSetup arma el escenario de la lectura en segundo plano: un proveedor
// "remoto" con dos extensiones (una instalada en la versión vieja, que da la
// actualización, y otra sin instalar, que es la novedad) y el editor arrancado
// SIN esperar la entrega —que es lo que pasa en la vida real—.
func prefetchSetup(t *testing.T) (*App, string) {
	t.Helper()
	src := startupProviderFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.0.0"},
		"tema":   {"tcode.tema", "Tema", "2.0.0"},
	})
	userRoot := t.TempDir()
	p := ext.Provider{Name: "remoto", Source: src, Approved: true}
	if _, err := ext.InstallByID("tcode.linter", []ext.Provider{p}, userRoot, extFetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}
	// El autor publica una versión nueva del linter: eso es la actualización.
	writeStartupManifest(t, filepath.Join(src, "linter"), "tcode.linter", "Linter", "1.1.0")
	pinStartupSources(t, p, userRoot)

	app, _ := newTestApp(t, "uno")
	return app, userRoot
}

// TestStartupDoesNotWaitForTheProviderRead: la lectura de los proveedores es
// un partial clone de segundos y el arranque NO la espera: con el lector
// bloqueado a propósito, el constructor volvió igual y el editor sigue
// respondiendo (decisión 1: la E/S cara va en segundo plano).
func TestStartupDoesNotWaitForTheProviderRead(t *testing.T) {
	src := startupProviderFixture(t, map[string][3]string{"tema": {"tcode.tema", "Tema", "2.0.0"}})
	bloqueado := make(chan struct{})
	t.Cleanup(func() { close(bloqueado) })
	pinStartupFetch(t,
		ext.Provider{Name: "remoto", Source: "https://example.com/remoto.git", Approved: true},
		t.TempDir(),
		func(url, dest, pattern string) error {
			<-bloqueado
			return extFetchFake(src)(url, dest, pattern)
		},
	)

	app, _ := newTestApp(t, "uno") // si esperara la lectura, el test colgaría acá

	if app.extSnapshot != nil {
		t.Fatal("el snapshot no puede estar listo mientras la lectura sigue bloqueada")
	}
	// El editor responde mientras la lectura corre.
	typeRune(app, 'x')
	if got := string(app.ws.Active().LineContent(0)); got != "xuno" {
		t.Fatalf("contenido = %q, el editor no responden con la lectura en curso", got)
	}
}

// TestPrefetchSnapshotIsCachedAndAnnounced: el evento que entrega la goroutine
// lo atiende el bucle de eventos, que cachea el snapshot, deriva las dos listas
// y avisa en la barra de estado cuántas actualizaciones y novedades hay y por
// dónde se gestionan (decisión 2: aviso en la barra, sin prompt).
func TestPrefetchSnapshotIsCachedAndAnnounced(t *testing.T) {
	app, _ := prefetchSetup(t)

	awaitExtSnapshot(t, app)

	if len(app.extUpdates) != 1 || app.extUpdates[0].Ref != "remoto/tcode.linter" {
		t.Fatalf("actualizaciones = %+v, esperaba la del linter", app.extUpdates)
	}
	if len(app.extAvailable) != 1 || app.extAvailable[0].ID != "tcode.tema" {
		t.Fatalf("novedades = %+v, esperaba la del tema", app.extAvailable)
	}
	msg := app.statusBar.Message()
	if !strings.Contains(msg, "1 actualización") || !strings.Contains(msg, "1 novedad") ||
		!strings.Contains(msg, "Ctrl+P") {
		t.Errorf("mensaje = %q, esperaba el aviso con los conteos y dónde gestionarlos", msg)
	}
	// Detectar NO toca el disco: la versión instalada sigue siendo la vieja.
	userRoot := app.extUserRoot
	if got := readFile(t, filepath.Join(userRoot, "remoto", "tcode.linter", "extension.json")); !strings.Contains(got, "1.0.0") {
		t.Errorf("la lectura de fondo aplicó algo: el manifest dice %s", got)
	}
}

// TestExtPendingNoticeCountsAndPlurals: el aviso dice cuántos cambios hay
// (plural correcto, incluido el singular) y siempre por dónde se gestionan.
func TestExtPendingNoticeCountsAndPlurals(t *testing.T) {
	cases := []struct {
		updates, available int
		want               string
	}{
		{1, 0, "1 actualización — Ctrl+P → Extensiones"},
		{0, 1, "1 novedad — Ctrl+P → Extensiones"},
		{2, 3, "2 actualizaciones, 3 novedades — Ctrl+P → Extensiones"},
	}
	for _, c := range cases {
		if got := extPendingNotice(c.updates, c.available); got != c.want {
			t.Errorf("extPendingNotice(%d, %d) = %q, esperaba %q", c.updates, c.available, got, c.want)
		}
	}
}

// TestExtManagerOpensWithLoadingState: abierta antes de que llegue la lectura,
// la ventana NO espera: muestra "cargando…" (decisión 3) y conserva la pestaña
// de proveedores, que es disco local y ya se puede mostrar.
func TestExtManagerOpensWithLoadingState(t *testing.T) {
	app, _ := prefetchSetup(t)
	if app.extSnapshot != nil {
		t.Fatal("el test necesita el snapshot sin llegar")
	}

	items := extTabItems(t, app, view.ExtTabInstalled)

	if len(items) != 1 || items[0].Label != "cargando…" {
		t.Fatalf("las instaladas = %+v, se esperaba la fila de cargando", items)
	}
	if got := app.extManager.Items(view.ExtTabProviders); len(got) == 0 {
		t.Error("la pestaña de proveedores no puede depender de la red: se lee de disco")
	}
}

// TestExtWindowFillsItselfWhenTheSnapshotArrives: la ventana abierta mientras
// carga se repinta sola cuando llega el evento, sin que el usuario tenga que
// cerrarla y abrirla de nuevo.
func TestExtWindowFillsItselfWhenTheSnapshotArrives(t *testing.T) {
	app, _ := prefetchSetup(t)
	app.openExtManager()

	awaitExtSnapshot(t, app)

	items := app.extManager.Items(view.ExtTabInstalled)
	if len(items) != 1 || items[0].Ref != "remoto/tcode.linter" {
		t.Fatalf("las instaladas = %+v, se esperaba el linter del fixture", items)
	}
}

// TestExtManagerIgnoresStaleSnapshot: una lectura vieja que llega después de
// una más nueva se descarta: el número de secuencia impide que el datoDelayed
// pise el fresco.
func TestExtManagerIgnoresStaleSnapshot(t *testing.T) {
	app, _ := prefetchSetup(t)
	app.handleExtSnapshot(extSnapshotEvent{
		Seq:      app.extPrefetchSeq,
		Snapshot: ext.Snapshot{Providers: []ext.Provider{{Name: "nuevo", Approved: true}}},
	})
	app.handleExtSnapshot(extSnapshotEvent{
		Seq:      app.extPrefetchSeq - 1,
		Snapshot: ext.Snapshot{Providers: []ext.Provider{{Name: "viejo", Approved: true}}},
	})

	if len(app.extSnapshot.Providers) != 1 || app.extSnapshot.Providers[0].Name != "nuevo" {
		t.Fatalf("el snapshot cacheado = %+v, una entrega vieja lo pisó", app.extSnapshot.Providers)
	}
}

// --- borrar archivo o carpeta desde el explorador ---

// newDeleteApp arranca una sesión sobre un directorio temporal con archivos ya
// escritos, para que el explorador tenga nodos desde el arranque (el primer
// nivel se lee siempre) y el cursor caiga en el primero.
func newDeleteApp(t *testing.T, files map[string]string) (*App, string) {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("no se pudo crear %s: %v", name, err)
		}
	}
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	s.SetSize(40, 10)

	app, err := NewAppWithScreen(s, dir)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	t.Cleanup(func() {
		app.ws.CloseAll()
		s.Fini()
	})
	if !app.explorerVisible || !app.explorerFocused {
		t.Fatal("una sesión sobre un directorio arranca con el explorador visible y enfocado")
	}
	return app, dir
}

// TestDeleteKeyConfirmsAndRemovesTheFile: Delete sobre el nodo del cursor abre
// el pedido sí/no con el nombre del archivo, y al confirmar con "s" el archivo
// desaparece de disco y del árbol, con un mensaje en la barra.
func TestDeleteKeyConfirmsAndRemovesTheFile(t *testing.T) {
	app, dir := newDeleteApp(t, map[string]string{"a.txt": "uno", "b.txt": "dos"})
	path := filepath.Join(dir, "a.txt")

	press(app, tcell.KeyDelete)
	if !app.promptActive {
		t.Fatal("Delete debe abrir el pedido de confirmación")
	}
	label := app.statusBar.Label()
	if !strings.Contains(label, "¿Borrar a.txt?") || !strings.Contains(label, "[s/N]") {
		t.Fatalf("rótulo = %q, se esperaba \"¿Borrar a.txt? [s/N]\"", label)
	}

	typeString(app, "s")
	press(app, tcell.KeyEnter)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("el archivo sigue en disco (err=%v), se esperaba borrado permanente", err)
	}
	if got := app.explorer.CursorPath(); got == path {
		t.Fatalf("CursorPath() = %q, el nodo borrado no puede seguir en el árbol", got)
	}
	if !strings.Contains(app.toast.Message(), "Borrado: a.txt") {
		t.Fatalf("toast = %q, se esperaba el aviso de borrado", app.toast.Message())
	}
	// El hermano que no estaba bajo el cursor sobrevive.
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatalf("b.txt no debía tocarse: %v", err)
	}
}

// TestDeleteDefaultsToNo: el pedido es [s/N] —Enter sin escribir nada NO borra—,
// y tampoco lo borra una respuesta que no sea "s". El archivo y su nodo quedan.
func TestDeleteDefaultsToNo(t *testing.T) {
	for _, answer := range []string{"", "n", "no", "x"} {
		app, dir := newDeleteApp(t, map[string]string{"a.txt": "uno"})
		path := filepath.Join(dir, "a.txt")

		press(app, tcell.KeyDelete)
		typeString(app, answer)
		press(app, tcell.KeyEnter)

		if app.promptActive {
			t.Fatalf("respuesta %q: el pedido debió cerrarse al confirmar", answer)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("respuesta %q: el archivo no debía borrarse: %v", answer, err)
		}
		if got := app.explorer.CursorPath(); got != path {
			t.Fatalf("respuesta %q: CursorPath() = %q, el nodo debía seguir", answer, got)
		}
		if app.toast.Message() != "Cancelado" || app.toast.Kind() != view.ToastInfo {
			t.Fatalf("respuesta %q: toast = %q, se esperaba el aviso neutro de cancelación", answer, app.toast.Message())
		}
	}
}

// TestDeleteFolderRemovesItRecursively: borrar un directorio se lleva todo su
// contenido, y el nodo desaparece del árbol junto con sus descendientes
// expandidos.
func TestDeleteFolderRemovesItRecursively(t *testing.T) {
	app, dir := newDeleteApp(t, map[string]string{"notas.txt": "notas"})
	sub := filepath.Join(dir, "docs")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear la carpeta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "dentro.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo interno: %v", err)
	}
	// El árbol arrancó sin la carpeta (se creó después): se recarga el primer
	// nivel por la misma puerta que usa el arranque.
	entries, err := readEntries(dir)
	if err != nil {
		t.Fatalf("no se pudo leer la raíz: %v", err)
	}
	app.explorer.SetRootEntries(entries)
	// El cursor cae en docs (los directorios se listan primero) y se expande.
	if got := app.explorer.CursorPath(); got != sub {
		t.Fatalf("CursorPath() = %q, se esperaba la carpeta", got)
	}
	app.explorerExpand()

	press(app, tcell.KeyDelete)
	typeString(app, "s")
	press(app, tcell.KeyEnter)

	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Fatalf("la carpeta sigue en disco (err=%v), se esperaba el borrado recursivo", err)
	}
	if got := app.explorer.CursorPath(); got == sub {
		t.Fatal("el nodo del directorio borrado no puede seguir en el árbol")
	}
	if !strings.Contains(app.toast.Message(), "Borrado: docs") {
		t.Fatalf("toast = %q, se esperaba el aviso de borrado", app.toast.Message())
	}
}

// TestDeleteClosesTheOpenBufferOfTheDeletedPath: un buffer abierto sobre el
// archivo borrado (o sobre uno dentro de la carpeta borrada) se cierra con
// CloseForce —sin él, guardar recrearía el archivo que acaba de desaparecer—
// y las pestañas que no estaban bajo la ruta sobreviven.
func TestDeleteClosesTheOpenBufferOfTheDeletedPath(t *testing.T) {
	app, dir := newDeleteApp(t, map[string]string{"a.txt": "uno", "b.txt": "dos"})

	if _, err := app.ws.Open(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatalf("no se pudo abrir a.txt: %v", err)
	}
	if _, err := app.ws.Open(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatalf("no se pudo abrir b.txt: %v", err)
	}
	if app.ws.Len() != 2 {
		t.Fatalf("Len() = %d, se esperaban 2 buffers", app.ws.Len())
	}
	if got := app.explorer.CursorPath(); got != filepath.Join(dir, "a.txt") {
		t.Fatalf("CursorPath() = %q, se esperaba a.txt", got)
	}

	press(app, tcell.KeyDelete)
	typeString(app, "s")
	press(app, tcell.KeyEnter)

	if app.ws.Len() != 1 {
		t.Fatalf("Len() = %d, se esperaba 1: el buffer del archivo borrado se cierra", app.ws.Len())
	}
	if got := app.ws.Active().Path(); got != filepath.Join(dir, "b.txt") {
		t.Fatalf("buffer activo = %q, se esperaba b.txt", got)
	}
}

// TestDeletePathClosesBuffersUnderTheDeletedFolder: borrar una carpeta cierra
// los buffers de los archivos que estaban DENTRO de ella, no solo uno con la
// ruta exacta.
func TestDeletePathClosesBuffersUnderTheDeletedFolder(t *testing.T) {
	app, dir := newDeleteApp(t, map[string]string{"a.txt": "uno"})
	sub := filepath.Join(dir, "docs")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear la carpeta: %v", err)
	}
	inside := filepath.Join(sub, "dentro.txt")
	if err := os.WriteFile(inside, []byte("x"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo interno: %v", err)
	}
	if _, err := app.ws.Open(inside); err != nil {
		t.Fatalf("no se pudo abrir el archivo interno: %v", err)
	}
	if _, err := app.ws.Open(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatalf("no se pudo abrir a.txt: %v", err)
	}

	if err := app.deletePath(sub); err != nil {
		t.Fatalf("deletePath falló: %v", err)
	}
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Fatalf("la carpeta sigue en disco (err=%v)", err)
	}
	if app.ws.Len() != 1 {
		t.Fatalf("Len() = %d, se esperaba 1: el buffer interno se cerró", app.ws.Len())
	}
	if got := app.ws.Active().Path(); got != filepath.Join(dir, "a.txt") {
		t.Fatalf("buffer activo = %q, se esperaba a.txt", got)
	}
}

// TestDeletePathRefusesAnEmptyPath: la acción es pública (command
// tcode.deleteFile), así que una ruta vacía no borra nada.
func TestDeletePathRefusesAnEmptyPath(t *testing.T) {
	app, dir := newDeleteApp(t, map[string]string{"a.txt": "uno"})
	if err := app.deletePath(""); err == nil {
		t.Fatal("una ruta vacía no se puede borrar")
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatalf("a.txt no debía tocarse: %v", err)
	}
}

// TestDeleteFileCommandIsRegistered: el command tcode.deleteFile existe (por
// simetría con tcode.createFile) y borra el nodo del cursor pasando por la
// misma confirmación.
func TestDeleteFileCommandIsRegistered(t *testing.T) {
	app, dir := newDeleteApp(t, map[string]string{"a.txt": "uno"})
	path := filepath.Join(dir, "a.txt")

	if err := app.ext.Registry().Run("tcode.deleteFile"); err != nil {
		t.Fatalf("tcode.deleteFile falló: %v", err)
	}
	if !app.promptActive {
		t.Fatal("el command debe abrir la confirmación")
	}
	typeString(app, "s")
	press(app, tcell.KeyEnter)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("el archivo sigue en disco (err=%v), se esperaba borrado", err)
	}
}

// pinExtSources apunta la resolución de extensiones (proveedores + raíz del
// usuario) a un proveedor en disco y a una raíz temporal, y deja el lector
// inyectado contra ese proveedor. El chequeo de ARRANQUE queda en el no-op de
// TestMain, así que abrir el editor no abre el prompt de actualizaciones: lo que
// se prueba acá es la ventana, no el arranque.
func pinExtSources(t *testing.T, providers []ext.Provider, userRoot string) {
	t.Helper()
	oldSources, oldFetch := extensionUserSources, startupExtFetch
	extensionUserSources = func() ([]ext.Provider, string, error) { return providers, userRoot, nil }
	if len(providers) > 0 {
		startupExtFetch = extFetchFake(providers[0].Source)
	}
	t.Cleanup(func() { extensionUserSources, startupExtFetch = oldSources, oldFetch })
}

// extWindowFixture arma el escenario común de la ventana de extensiones: un
// proveedor en disco con dos extensiones —una instalada en la versión vieja
// (que da la actualización) y otra sin instalar (la novedad)—, una raíz de
// extensiones del usuario temporal y el editor arrancado sobre un archivo.
func extWindowFixture(t *testing.T, approved bool) (*App, string, ext.Provider) {
	t.Helper()
	src := startupProviderFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.0.0"},
		"tema":   {"tcode.tema", "Tema", "2.0.0"},
	})
	userRoot := t.TempDir()
	p := ext.Provider{Name: "remoto", Source: src, Approved: approved}
	// El fixture instala con una confirmación que aprueba: lo que el test
	// ejercita es cómo la VENTANA trata a un proveedor sin aprobar, no la
	// guarda de instalación de InstallByID (que se cubre aparte). Sin esto, el
	// caso approved=false no llegaría nunca a tener la extensión instalada.
	approve := func(ext.Provider) bool { return true }
	if _, err := ext.InstallByID("tcode.linter", []ext.Provider{p}, userRoot, extFetchFake(src), approve); err != nil {
		t.Fatalf("InstallByID del linter: %v", err)
	}
	// El autor publica una versión nueva del linter: eso es la actualización.
	writeStartupManifest(t, filepath.Join(src, "linter"), "tcode.linter", "Linter", "1.1.0")

	pinExtSources(t, []ext.Provider{p}, userRoot)
	app, _ := newTestApp(t, "uno")
	// La ventana abre con lo que hay en caché; el prefetch del arranque todavía
	// está en vuelo, así que se espera su entrega antes de abrirla (en la vida
	// real la ventana abre al instante con lo que haya).
	awaitExtSnapshot(t, app)
	return app, userRoot, p
}

// extTabItems abre la ventana y devuelve las filas de la pestaña pedida.
func extTabItems(t *testing.T, app *App, tab view.ExtTab) []view.ExtItem {
	t.Helper()
	app.openExtManager()
	for range tab {
		app.handleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	if got := app.extManager.Tab(); got != tab {
		t.Fatalf("la ventana quedó en la pestaña %v, se esperaba %v", got, tab)
	}
	return app.extManager.Items(tab)
}

// answerPrompt escribe answer y confirma con Enter, como lo haría el teclado.
func answerPrompt(app *App, answer string) {
	for _, r := range answer {
		app.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
}

// TestExtensionWindowOpensFromTheConfigRow: la fila Extensiones de la ventana
// de configuración dispara la acción (Enter) y el controlador abre la ventana
// de extensiones — cerrando la de configuración — con los datos ya cargados: la
// pestaña de instaladas trae la extensión del fixture.
func TestExtensionWindowOpensFromTheConfigRow(t *testing.T) {
	resetConfigVars(t)
	app, _, _ := extWindowFixture(t, true)

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	if !app.configActive {
		t.Fatal("Ctrl+P debe abrir la ventana de configuración")
	}
	// Bajar hasta la última fila (Extensiones): el alto de la ventana menos el
	// marco son sus filas.
	for range view.ConfigMenuHeight() - 3 {
		app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter en la fila de acción no debe cerrar el editor")
	}

	if app.configActive {
		t.Fatal("abrir la ventana de extensiones debe cerrar la de configuración")
	}
	if !app.extActive {
		t.Fatal("Enter en la fila Extensiones debe abrir la ventana de extensiones")
	}
	if got := app.extManager.Tab(); got != view.ExtTabInstalled {
		t.Fatalf("la ventana abre en la pestaña %v, se esperaba la de instaladas", got)
	}
	items := app.extManager.Items(view.ExtTabInstalled)
	if len(items) != 1 || items[0].Ref != "remoto/tcode.linter" {
		t.Fatalf("las instaladas = %+v, se esperaba el linter del fixture", items)
	}
	if items[0].Kind != view.ExtItemRemove {
		t.Fatalf("la fila instalada es de tipo %v, se esperaba ExtItemRemove", items[0].Kind)
	}
}

// TestExtensionWindowTabsCarryTheirData: cada pestaña trae lo que le toca: la
// actualización con su salto de versión, la novedad con la marca del
// proveedor, y los proveedores con la de agregar al final.
func TestExtensionWindowTabsCarryTheirData(t *testing.T) {
	resetConfigVars(t)
	app, _, _ := extWindowFixture(t, false)

	updates := extTabItems(t, app, view.ExtTabUpdatable)
	if len(updates) != 1 || updates[0].Right != "1.0.0 → 1.1.0" {
		t.Fatalf("las actualizables = %+v, se esperaba el salto 1.0.0 → 1.1.0", updates)
	}
	if updates[0].Kind != view.ExtItemUpdate {
		t.Fatalf("la fila actualizable es de tipo %v, se esperaba ExtItemUpdate", updates[0].Kind)
	}

	available := extTabItems(t, app, view.ExtTabAvailable)
	if len(available) != 1 || available[0].ID != "tcode.tema" {
		t.Fatalf("las disponibles = %+v, se esperaba la novedad tcode.tema", available)
	}
	if !strings.Contains(available[0].Right, "sin aprobar") {
		t.Fatalf("la novedad de un proveedor sin aprobar no está marcada: %q", available[0].Right)
	}

	providers := extTabItems(t, app, view.ExtTabProviders)
	if len(providers) != 2 {
		t.Fatalf("la pestaña de proveedores tiene %d filas, se esperaban 2 (el proveedor y la de agregar)", len(providers))
	}
	if providers[0].Label != "remoto" || providers[0].Kind != view.ExtItemInfo {
		t.Fatalf("la primera fila de proveedores = %+v, se esperaba el proveedor informativo", providers[0])
	}
	last := providers[len(providers)-1]
	if last.Kind != view.ExtItemAddProvider {
		t.Fatalf("la última fila de proveedores es de tipo %v, se esperaba ExtItemAddProvider", last.Kind)
	}
}

// TestExtensionWindowInstallsAfterConfirmation: Enter sobre una novedad abre el
// pedido de confirmación; solo con un "s" explícito se instala. La ventana sigue
// abierta con los datos recargados y la extensión nueva queda en disco.
func TestExtensionWindowInstallsAfterConfirmation(t *testing.T) {
	resetConfigVars(t)
	app, userRoot, _ := extWindowFixture(t, true)
	extTabItems(t, app, view.ExtTabAvailable)

	// Enter abre la confirmación: NO instala todavía.
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.promptActive {
		t.Fatal("Enter sobre una novedad debe pedir confirmación")
	}
	if !strings.Contains(app.promptLabel, "Instalar") || !strings.Contains(app.promptLabel, "remoto/tcode.tema") {
		t.Fatalf("el rótulo del pedido es %q, debe nombrar la extensión a instalar", app.promptLabel)
	}
	if !app.extActive {
		t.Fatal("la ventana de extensiones sigue abierta detrás del pedido")
	}
	if _, err := os.Stat(filepath.Join(userRoot, "remoto", "tcode.tema")); !os.IsNotExist(err) {
		t.Fatal("no se puede instalar antes de confirmar")
	}

	// El sí instala.
	answerPrompt(app, "s")
	if app.promptActive {
		t.Fatal("el pedido debe cerrarse al confirmar")
	}
	if _, err := os.Stat(filepath.Join(userRoot, "remoto", "tcode.tema", "extension.json")); err != nil {
		t.Fatalf("la extensión confirmada debe quedar instalada: %v", err)
	}
	// El progreso se vio en la barra ("Instalando…") y el éxito en el toast.
	if got := app.toast.Message(); !strings.Contains(got, "Instalada") {
		t.Fatalf("el toast es %q, debe confirmar la instalación", got)
	}
	if app.toast.Kind() != view.ToastSuccess {
		t.Fatalf("el toast es de tipo %v, se esperaba éxito", app.toast.Kind())
	}
	if !app.extActive {
		t.Fatal("la ventana debe seguir abierta después de instalar")
	}
	// Los datos se recargan: la novedad desapareció y hay dos instaladas.
	if got := app.extManager.Items(view.ExtTabAvailable); len(got) != 0 {
		t.Fatalf("las disponibles = %+v, después de instalar no debe quedar la novedad", got)
	}
	installed := app.extManager.Items(view.ExtTabInstalled)
	if len(installed) != 2 {
		t.Fatalf("las instaladas = %d filas, se esperaban 2 tras instalar", len(installed))
	}
}

// TestExtensionWindowCancelsInstallWithoutConfirmation: Enter vacío (el
// default es NO) cancela: la extensión no se instala y el mensaje lo dice.
func TestExtensionWindowCancelsInstallWithoutConfirmation(t *testing.T) {
	resetConfigVars(t)
	app, userRoot, _ := extWindowFixture(t, true)
	extTabItems(t, app, view.ExtTabAvailable)

	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.promptActive {
		t.Fatal("Enter sobre una novedad debe pedir confirmación")
	}
	answerPrompt(app, "") // Enter a ciegas: NO
	if _, err := os.Stat(filepath.Join(userRoot, "remoto", "tcode.tema")); !os.IsNotExist(err) {
		t.Fatal("un Enter vacío no puede instalar (el default es NO)")
	}
	if app.toast.Message() != "Cancelado" || app.toast.Kind() != view.ToastInfo {
		t.Fatalf("el toast es %q, debe decir que se canceló en neutro", app.toast.Message())
	}
	if !app.extActive {
		t.Fatal("cancelar debe dejar la ventana abierta")
	}
}

// TestExtensionWindowRefusesUnapprovedProvider: instalar desde un proveedor sin
// aprobar no se ofrece: Enter avisa cómo aprobarlo y NO abre ni el pedido ni la
// instalación.
func TestExtensionWindowRefusesUnapprovedProvider(t *testing.T) {
	resetConfigVars(t)
	app, userRoot, _ := extWindowFixture(t, false)
	extTabItems(t, app, view.ExtTabAvailable)

	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.promptActive {
		t.Fatal("una novedad sin aprobar no debe abrir el pedido de instalación")
	}
	if !strings.Contains(app.statusBar.Message(), "approve-provider") {
		t.Fatalf("el mensaje es %q, debe explicar cómo aprobar el proveedor", app.statusBar.Message())
	}
	if _, err := os.Stat(filepath.Join(userRoot, "remoto", "tcode.tema")); !os.IsNotExist(err) {
		t.Fatal("una novedad de un proveedor sin aprobar no puede quedar instalada")
	}
}

// TestExtensionWindowUpdatesAfterConfirmation: Enter sobre una actualización
// pide confirmación y el sí la aplica: la versión nueva queda en disco y la
// pestaña de actualizables queda vacía tras la recarga.
func TestExtensionWindowUpdatesAfterConfirmation(t *testing.T) {
	resetConfigVars(t)
	app, userRoot, _ := extWindowFixture(t, true)
	extTabItems(t, app, view.ExtTabUpdatable)

	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.promptActive || !strings.Contains(app.promptLabel, "Actualizar") {
		t.Fatalf("Enter sobre una actualización debe pedir confirmación; rótulo %q, activo=%v", app.promptLabel, app.promptActive)
	}
	answerPrompt(app, "s")

	data, err := os.ReadFile(filepath.Join(userRoot, "remoto", "tcode.linter", "extension.json"))
	if err != nil {
		t.Fatalf("no se pudo leer el manifest instalado: %v", err)
	}
	if !strings.Contains(string(data), "1.1.0") {
		t.Fatalf("la extensión actualizada sigue con la versión vieja: %s", data)
	}
	if got := app.extManager.Items(view.ExtTabUpdatable); len(got) != 0 {
		t.Fatalf("las actualizables = %+v, después de actualizar no debe quedar ninguna", got)
	}
}

// TestExtensionWindowRemovesAfterConfirmation: Enter sobre una instalada pide
// confirmación y el sí la borra del disco; sin confirmación no se borra nada.
func TestExtensionWindowRemovesAfterConfirmation(t *testing.T) {
	resetConfigVars(t)
	app, userRoot, _ := extWindowFixture(t, true)
	extTabItems(t, app, view.ExtTabInstalled)

	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.promptActive || !strings.Contains(app.promptLabel, "Eliminar") {
		t.Fatalf("Enter sobre una instalada debe pedir confirmación; rótulo %q, activo=%v", app.promptLabel, app.promptActive)
	}
	if _, err := os.Stat(filepath.Join(userRoot, "remoto", "tcode.linter")); err != nil {
		t.Fatalf("no se puede borrar antes de confirmar: %v", err)
	}
	answerPrompt(app, "s")

	if _, err := os.Stat(filepath.Join(userRoot, "remoto", "tcode.linter")); !os.IsNotExist(err) {
		t.Fatal("la extensión confirmada debe quedar borrada del disco")
	}
	if got := app.extManager.Items(view.ExtTabInstalled); len(got) != 0 {
		t.Fatalf("las instaladas = %+v, después de borrar no debe quedar ninguna", got)
	}
}

// TestExtensionWindowAddsProviderFromThePrompt: la última fila de la pestaña de
// proveedores abre el pedido de texto; la fuente escrita se registra SIN
// aprobar y aparece en la lista con la ventana todavía abierta.
func TestExtensionWindowAddsProviderFromThePrompt(t *testing.T) {
	resetConfigVars(t)
	app, userRoot, remoto := extWindowFixture(t, true)

	// El archivo de proveedores del HOME del test, aislado como el config. El
	// loader de la ventana se re-pinnea para LEER ESE ARCHIVO (más el proveedor
	// del fixture): el camino real resuelve el mismo archivo que escribe
	// addProviderSource, así que pinnear una lista fija haría que el agregado se
	// guarde pero la lista no lo vea, y el test no probaría lo que dice probar.
	dir := t.TempDir()
	providersFile := filepath.Join(dir, "providers.json")
	oldPath, oldSources := providersConfigPath, extensionUserSources
	providersConfigPath = func() (string, error) { return providersFile, nil }
	extensionUserSources = func() ([]ext.Provider, string, error) {
		stored, err := ext.LoadProviders(providersFile)
		if err != nil {
			return nil, "", err
		}
		return append([]ext.Provider{remoto}, stored...), userRoot, nil
	}
	t.Cleanup(func() { providersConfigPath, extensionUserSources = oldPath, oldSources })

	extTabItems(t, app, view.ExtTabProviders)
	// Ir a la última fila (la de agregar) con End.
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.promptActive || !strings.Contains(app.promptLabel, "Agregar proveedor") {
		t.Fatalf("la fila de agregar debe abrir su pedido; rótulo %q, activo=%v", app.promptLabel, app.promptActive)
	}

	nuevo := startupProviderFixture(t, map[string][3]string{"otro": {"tcode.otro", "Otro", "1.0.0"}})
	answerPrompt(app, nuevo)

	stored, err := ext.LoadProviders(providersFile)
	if err != nil {
		t.Fatalf("no se pudo leer providers.json: %v", err)
	}
	if len(stored) != 1 || stored[0].Source != filepath.Clean(nuevo) {
		t.Fatalf("los proveedores guardados = %+v, se esperaba la carpeta %q", stored, nuevo)
	}
	if stored[0].Name != filepath.Base(nuevo) {
		t.Fatalf("el nombre guardado es %q, se esperaba el derivado %q", stored[0].Name, filepath.Base(nuevo))
	}
	if stored[0].Approved {
		t.Fatal("un proveedor agregado queda SIN aprobar: agregar no es confiar")
	}
	if !app.extActive {
		t.Fatal("agregar un proveedor deja la ventana abierta")
	}
	// Aparece en la lista, marcada como sin aprobar.
	providers := app.extManager.Items(view.ExtTabProviders)
	found := false
	for _, p := range providers {
		if p.Label == stored[0].Name {
			found = true
			if !strings.Contains(p.Right, "sin aprobar") {
				t.Fatalf("el proveedor nuevo no está marcado como sin aprobar: %+v", p)
			}
		}
	}
	if !found {
		t.Fatalf("el proveedor agregado no aparece en la lista: %+v", providers)
	}
}

// TestExtensionWindowClosesOnForeignKeys: Escape y cualquier tecla ajena cierran
// la ventana descartando, como las demás overlays, sin cerrar el editor.
func TestExtensionWindowClosesOnForeignKeys(t *testing.T) {
	resetConfigVars(t)
	app, _, _ := extWindowFixture(t, true)
	app.openExtManager()
	if !app.extActive {
		t.Fatal("openExtManager debe abrir la ventana")
	}
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); quit {
		t.Fatal("Escape consumido por la ventana no debe cerrar el editor")
	}
	if app.extActive {
		t.Fatal("Escape debe cerrar la ventana de extensiones")
	}
}
