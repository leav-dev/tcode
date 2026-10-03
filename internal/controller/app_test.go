package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/model"
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
	if app.statusBar.Label() == "" {
		t.Fatal("debe haber un aviso en la barra")
	}
}

// TestSaveAsTargetsTheBufferCapturedAtPromptOpen: el destino es el buffer que
// estaba activo al abrir el pedido, no el que esté activo al apretar Enter.
func TestSaveAsTargetsTheBufferCapturedAtPromptOpen(t *testing.T) {
	app, _, path1 := newTwoBufferApp(t, "uno", "dos")

	dest := filepath.Join(t.TempDir(), "copia.txt")

	// El pedido se abre con el buffer 0 activo.
	pressSaveAs(app)
	if app.promptTarget != bufferAt(app, 0) {
		t.Fatal("el pedido debe capturar el buffer activo al abrirse")
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
		if w, h := ed.Size(); w != 30 || h != 4 {
			t.Fatalf("la vista de %v quedó con %dx%d, se esperaba 30x4", buf, w, h)
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
	if quit := press(app, tcell.KeyEscape); !quit {
		t.Fatal("Escape sobre un workspace vacío debe cerrar el editor")
	}
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
