package controller

import (
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
		switch {
		case ev.Key() == tcell.KeyCtrlS:
			a.confirmQuit = false
			a.save()
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

		// Cualquier otra tecla cancela la confirmación pendiente.
		a.confirmQuit = false
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

// save escribe el documento y refleja el resultado en la barra de estado.
func (a *App) save() {
	if err := a.model.Save(); err != nil {
		a.statusBar.SetMessage("Error al guardar: " + err.Error())
	} else {
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
