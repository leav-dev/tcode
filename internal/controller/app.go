package controller

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/model"
	"tcode/internal/view"
)

// statusHeight es la cantidad de filas que ocupa la barra de estado.
const statusHeight = 1

type App struct {
	screen    tcell.Screen
	ws        *model.Workspace
	statusBar *view.StatusBar

	// editors guarda una vista por buffer. La clave es el puntero del buffer:
	// el workspace ya deduplica por ruta, así que el mapa no puede desincronizarse
	// de él como podría hacerlo un slice mantenido a mano.
	editors map[*model.PieceTable]*view.EditorView

	confirmQuit bool

	// forceSave es el permiso de pisar cambios externos, por buffer: autorizar
	// sobrescribir UN archivo no tiene que valer para otro.
	forceSave map[*model.PieceTable]bool

	// promptActive, promptBuf y promptTarget sostienen el pedido de texto de
	// Save As. Mientras está activo, el teclado alimenta el pedido y no el
	// documento. El buffer destino se captura al ABRIR el pedido: el texto que
	// alguien escribe pertenece al documento que estaba mirando cuando empezó,
	// no al que esté activo cuando aprieta Enter.
	promptActive bool
	promptBuf    string
	promptTarget *model.PieceTable
}

// NewApp inicializa la terminal y carga el archivo indicado (si path != "").
func NewApp(path string) (*App, error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	return NewAppWithScreen(s, path)
}

// NewAppWithScreen construye la aplicación sobre una pantalla ya provista. Es lo
// que permite testear el controlador con una pantalla simulada, sin terminal.
func NewAppWithScreen(s tcell.Screen, path string) (*App, error) {
	if err := s.Init(); err != nil {
		return nil, err
	}
	s.EnableMouse()

	ws := model.NewWorkspace()
	if path == "" {
		// Sin argumentos: un documento sin ruta, que es de donde Save As le da
		// una después.
		ws.NewUntitled()
	} else if _, err := ws.Open(path); err != nil {
		s.Fini()
		return nil, err
	}

	app := &App{
		screen:    s,
		ws:        ws,
		editors:   make(map[*model.PieceTable]*view.EditorView),
		forceSave: make(map[*model.PieceTable]bool),
		statusBar: view.NewStatusBar(),
	}
	app.syncStatus()
	return app, nil
}

// editorHeight es el alto disponible para el editor, descontando la barra.
func editorHeight(height int) int {
	if height <= statusHeight {
		return 1
	}
	return height - statusHeight
}

// activeBuffer devuelve el buffer activo, o nil si el workspace está vacío.
func (a *App) activeBuffer() *model.PieceTable { return a.ws.Active() }

// activeEditor resuelve la vista del buffer activo, creándola bajo demanda con
// el tamaño ACTUAL de la pantalla. Devuelve nil si no hay buffer activo.
func (a *App) activeEditor() *view.EditorView {
	buf := a.activeBuffer()
	if buf == nil {
		return nil
	}
	if ev, ok := a.editors[buf]; ok {
		return ev
	}
	width, height := a.screen.Size()
	ev := view.NewEditorView(buf, editorHeight(height), width)
	a.editors[buf] = ev
	return ev
}

func (a *App) syncStatus() {
	buf := a.activeBuffer()
	if buf == nil {
		a.statusBar.SetFile("", false)
		return
	}
	a.statusBar.SetFile(buf.Path(), buf.Modified())
}

// Run ejecuta el loop de eventos hasta que el usuario cierra el editor.
func (a *App) Run() error {
	defer a.screen.Fini()
	defer a.ws.CloseAll()

	a.redraw()

	for {
		ev := a.screen.PollEvent()
		if ev == nil {
			return nil
		}
		if a.handleEvent(ev) {
			return nil
		}
	}
}

// handleEvent procesa un evento y devuelve true si la aplicación debe terminar.
func (a *App) handleEvent(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		// Con un pedido activo el teclado es del pedido, no del documento.
		if a.promptActive {
			a.handlePromptKey(ev)
			return false
		}

		// Workspace vacío: no hay nada que editar, guardar ni deshacer. Salir
		// sigue funcionando, y sin buffers no hay nada que perder.
		buf := a.activeBuffer()
		if buf == nil {
			return ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC
		}

		switch {
		case ev.Key() == tcell.KeyCtrlS:
			a.confirmQuit = false
			a.save()
			return false

		case isSaveAsKey(ev):
			a.startPrompt()
			return false

		case isRedoKey(ev):
			a.confirmQuit = false
			a.applyHistory(buf.Redo, "Rehecho", "Nada que rehacer")
			return false

		case isUndoKey(ev):
			a.confirmQuit = false
			a.applyHistory(buf.Undo, "Deshecho", "Nada que deshacer")
			return false

		case ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC:
			// Salir con cambios sin guardar en CUALQUIER buffer pide
			// confirmación: la primera vez solo se avisa, así una tecla de más
			// no tira el trabajo de ninguna pestaña.
			if !a.ws.AnyModified() || a.confirmQuit {
				return true
			}
			a.confirmQuit = true
			a.statusBar.SetMessage("Cambios sin guardar: Ctrl+S guarda, Escape de nuevo sale igual")
			a.redraw()
			return false
		}

		// Cualquier otra tecla cancela la confirmación pendiente y el permiso de
		// pisar que se haya dado con un Ctrl+S previo.
		a.confirmQuit = false
		a.clearForceSave()
		a.statusBar.ClearMessage()
		if ed := a.activeEditor(); ed != nil && ed.HandleEvent(ev) {
			a.redraw()
		}

	case *tcell.EventMouse:
		if ed := a.activeEditor(); ed != nil && ed.HandleEvent(ev) {
			a.redraw()
		}

	case *tcell.EventResize:
		a.screen.Sync()
		width, height := a.screen.Size()
		// El resize cambia el viewport de TODOS los buffers abiertos, no solo
		// del activo: si no, la vista de otro buffer queda desactualizada y
		// dibuja mal al volver a esa pestaña.
		for _, ed := range a.editors {
			ed.Resize(width, editorHeight(height))
		}
		a.redraw()
	}
	return false
}

// isUndoKey reconoce Ctrl+Z sin modificadores.
func isUndoKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyCtrlZ && ev.Modifiers()&tcell.ModShift == 0
}

// isRedoKey reconoce Ctrl+Y y también Ctrl+Shift+Z.
//
// tcell solo reporta los códigos KeyCtrl* cuando no hay modificadores: con Shift
// presente, Ctrl+Shift+Z llega como KeyRune con ModCtrl y ModShift.
func isRedoKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyCtrlY {
		return true
	}
	return ev.Key() == tcell.KeyRune &&
		ev.Modifiers()&tcell.ModCtrl != 0 &&
		ev.Modifiers()&tcell.ModShift != 0 &&
		(ev.Rune() == 'z' || ev.Rune() == 'Z')
}

// applyHistory ejecuta deshacer o rehacer sobre el buffer activo y refleja el
// resultado en la barra.
func (a *App) applyHistory(op func() (model.Change, bool, error), done, empty string) {
	change, ok, err := op()
	switch {
	case err != nil:
		a.statusBar.SetMessage("Error: " + err.Error())
	case !ok:
		a.statusBar.SetMessage(empty)
	default:
		if ed := a.activeEditor(); ed != nil {
			ed.MoveCursorToOffset(change.Offset)
		}
		a.statusBar.SetMessage(done)
	}
	a.syncStatus()
	a.redraw()
}

// isSaveAsKey reconoce Ctrl+Shift+S. Como en Ctrl+Shift+Z, tcell entrega los
// control con Shift como KeyRune en lugar del código KeyCtrl*.
func isSaveAsKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyRune &&
		ev.Modifiers()&tcell.ModCtrl != 0 &&
		ev.Modifiers()&tcell.ModShift != 0 &&
		(ev.Rune() == 's' || ev.Rune() == 'S')
}

// startPrompt abre el pedido de Save As, prellenado con la ruta actual para poder
// editarla en lugar de reescribirla entera. El buffer destino queda capturado
// acá: no depende de qué pestaña esté activa cuando se termine de tipear.
func (a *App) startPrompt() {
	a.confirmQuit = false
	a.clearForceSave()
	a.promptActive = true
	a.promptBuf = a.activeBuffer().Path()
	a.promptTarget = a.activeBuffer()
	a.refreshPrompt()
	a.redraw()
}

func (a *App) refreshPrompt() {
	a.statusBar.SetPrompt("Guardar como: " + a.promptBuf)
}

func (a *App) endPrompt() {
	a.promptActive = false
	a.promptBuf = ""
	a.promptTarget = nil
	a.statusBar.SetPrompt("")
}

// clearForceSave revoca el permiso de pisar de todos los buffers. Reusa el mapa
// en lugar de asignar uno nuevo: esto corre en el camino caliente del tipeo
// (cualquier tecla que no sea un atajo lo llama) y alocar un mapa por tecla
// sería un costo por nada.
func (a *App) clearForceSave() { clear(a.forceSave) }

// activeForceSave devuelve el permiso de pisar del buffer activo.
func (a *App) activeForceSave() bool { return a.forceSave[a.activeBuffer()] }

// saveAsFor autoriza a un buffer concreto a pisar cambios externos.
func (a *App) saveAsFor(buf *model.PieceTable) { a.forceSave[buf] = true }

// handlePromptKey alimenta el pedido de texto. El borrado va por runa y no por
// grapheme cluster: alcanza para rutas, que son ASCII en la práctica.
func (a *App) handlePromptKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		a.endPrompt()
		a.statusBar.SetMessage("Save As cancelado")
		a.redraw()
		return

	case tcell.KeyEnter:
		path := strings.TrimSpace(a.promptBuf)
		target := a.promptTarget
		a.endPrompt()
		a.saveAs(target, path)
		return

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if r := []rune(a.promptBuf); len(r) > 0 {
			a.promptBuf = string(r[:len(r)-1])
		}

	case tcell.KeyRune:
		if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0 {
			if r := ev.Rune(); r != 0 && r != '\n' && r != '\t' {
				a.promptBuf += string(r)
			}
		}
	}

	a.refreshPrompt()
	a.redraw()
}

// saveAs guarda el buffer capturado al abrir el pedido en la ruta elegida y pasa
// a trabajar sobre ella.
func (a *App) saveAs(target *model.PieceTable, path string) {
	if path == "" {
		a.statusBar.SetMessage("Save As cancelado")
		a.redraw()
		return
	}

	// El buffer capturado puede haber salido del workspace entre medio: no hay
	// que escribir en un documento que ya no está abierto.
	inWorkspace := false
	for _, b := range a.ws.Buffers() {
		if b == target {
			inWorkspace = true
			break
		}
	}
	if !inWorkspace {
		a.statusBar.SetMessage("El buffer de destino ya no está abierto")
		a.redraw()
		return
	}

	if err := target.SaveAs(path); err != nil {
		a.statusBar.SetMessage("Error al guardar como: " + err.Error())
	} else {
		a.confirmQuit = false
		a.clearForceSave()
		a.statusBar.SetMessage("Guardado en " + filepath.Base(path))
	}

	a.syncStatus()
	a.redraw()
}

// save escribe el buffer activo y refleja el resultado en la barra de estado.
//
// Si el archivo cambió en disco se avisa en lugar de pisarlo; un segundo Ctrl+S
// seguido fuerza la escritura. La decisión de perder esos cambios queda así en
// manos de quien usa el editor y no de un valor por defecto.
func (a *App) save() {
	buf := a.activeBuffer()
	var err error
	if a.activeForceSave() {
		a.clearForceSave()
		err = buf.SaveForce()
	} else {
		err = buf.Save()
	}

	switch {
	case errors.Is(err, model.ErrFileChangedExternally):
		a.saveAsFor(buf)
		a.statusBar.SetMessage("El archivo cambió en disco: Ctrl+S de nuevo pisa esos cambios")

	case err != nil:
		a.statusBar.SetMessage("Error al guardar: " + err.Error())

	default:
		a.confirmQuit = false
		a.statusBar.SetMessage("Guardado")
	}

	a.syncStatus()
	a.redraw()
}

// redraw limpia y vuelve a dibujar todas las Screens activas. Con el workspace
// vacío solo dibuja la barra de estado: es un estado válido, no un error.
func (a *App) redraw() {
	width, height := a.screen.Size()
	a.screen.Clear()
	if ed := a.activeEditor(); ed != nil {
		ed.Draw(a.screen)
	}
	a.statusBar.Draw(a.screen, height-statusHeight, width)
	a.screen.Show()
}
