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
	screen      tcell.Screen
	model       *model.PieceTable
	editorView  *view.EditorView
	statusBar   *view.StatusBar
	confirmQuit bool

	// forceSave habilita el próximo Ctrl+S a pisar cambios externos, después de
	// haber avisado una vez.
	forceSave bool

	// promptActive y promptBuf sostienen el pedido de texto de Save As. Mientras
	// están activos, el teclado alimenta el pedido y no el documento.
	promptActive bool
	promptBuf    string
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

	m := model.NewPieceTable()
	if path != "" {
		if err := m.LoadFile(path); err != nil {
			s.Fini()
			return nil, err
		}
	}

	width, height := s.Size()
	app := &App{
		screen:     s,
		model:      m,
		editorView: view.NewEditorView(m, editorHeight(height), width),
		statusBar:  view.NewStatusBar(),
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

func (a *App) syncStatus() {
	a.statusBar.SetFile(a.model.Path(), a.model.Modified())
}

// Run ejecuta el loop de eventos hasta que el usuario cierra el editor.
func (a *App) Run() error {
	defer a.screen.Fini()
	defer a.model.Close()

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
			a.applyHistory(a.model.Redo, "Rehecho", "Nada que rehacer")
			return false

		case isUndoKey(ev):
			a.confirmQuit = false
			a.applyHistory(a.model.Undo, "Deshecho", "Nada que deshacer")
			return false

		case ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC:
			// Salir con cambios sin guardar pide confirmación: la primera vez
			// solo se avisa, así una tecla de más no tira el trabajo.
			if !a.model.Modified() || a.confirmQuit {
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
		a.forceSave = false
		a.statusBar.ClearMessage()
		if a.editorView.HandleEvent(ev) {
			a.redraw()
		}

	case *tcell.EventMouse:
		if a.editorView.HandleEvent(ev) {
			a.redraw()
		}

	case *tcell.EventResize:
		a.screen.Sync()
		width, height := a.screen.Size()
		a.editorView.Resize(width, editorHeight(height))
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

// applyHistory ejecuta deshacer o rehacer y refleja el resultado en la barra.
func (a *App) applyHistory(op func() (model.Change, bool, error), done, empty string) {
	change, ok, err := op()
	switch {
	case err != nil:
		a.statusBar.SetMessage("Error: " + err.Error())
	case !ok:
		a.statusBar.SetMessage(empty)
	default:
		a.editorView.MoveCursorToOffset(change.Offset)
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
// editarla en lugar de reescribirla entera.
func (a *App) startPrompt() {
	a.confirmQuit = false
	a.forceSave = false
	a.promptActive = true
	a.promptBuf = a.model.Path()
	a.refreshPrompt()
	a.redraw()
}

func (a *App) refreshPrompt() {
	a.statusBar.SetPrompt("Guardar como: " + a.promptBuf)
}

func (a *App) endPrompt() {
	a.promptActive = false
	a.promptBuf = ""
	a.statusBar.SetPrompt("")
}

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
		a.endPrompt()
		a.saveAs(path)
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

// saveAs guarda en la ruta elegida y pasa a trabajar sobre ella.
func (a *App) saveAs(path string) {
	if path == "" {
		a.statusBar.SetMessage("Save As cancelado")
		a.redraw()
		return
	}

	if err := a.model.SaveAs(path); err != nil {
		a.statusBar.SetMessage("Error al guardar como: " + err.Error())
	} else {
		a.confirmQuit = false
		a.forceSave = false
		a.statusBar.SetMessage("Guardado en " + filepath.Base(path))
	}

	a.syncStatus()
	a.redraw()
}

// save escribe el documento y refleja el resultado en la barra de estado.
//
// Si el archivo cambió en disco se avisa en lugar de pisarlo; un segundo Ctrl+S
// seguido fuerza la escritura. La decisión de perder esos cambios queda así en
// manos de quien usa el editor y no de un valor por defecto.
func (a *App) save() {
	var err error
	if a.forceSave {
		a.forceSave = false
		err = a.model.SaveForce()
	} else {
		err = a.model.Save()
	}

	switch {
	case errors.Is(err, model.ErrFileChangedExternally):
		a.forceSave = true
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

// redraw limpia y vuelve a dibujar todas las Screens activas.
func (a *App) redraw() {
	width, height := a.screen.Size()
	a.screen.Clear()
	a.editorView.Draw(a.screen)
	a.statusBar.Draw(a.screen, height-statusHeight, width)
	a.screen.Show()
}
