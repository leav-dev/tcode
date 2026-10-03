package controller

import (
	"github.com/gdamore/tcell/v2"
	"tcode/internal/model"
	"tcode/internal/view"
)

type App struct {
	screen     tcell.Screen
	model      *model.PieceTable
	editorView *view.EditorView
}

// NewApp inicializa la terminal y carga el archivo indicado (si path != "").
func NewApp(path string) (*App, error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
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
	return &App{
		screen:     s,
		model:      m,
		editorView: view.NewEditorView(m, height, width),
	}, nil
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

		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return nil
			}
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
			a.editorView.Resize(width, height)
			a.redraw()
		}
	}
}

// redraw limpia y vuelve a dibujar todas las Screens activas.
func (a *App) redraw() {
	a.screen.Clear()
	a.editorView.Draw(a.screen)
	a.screen.Show()
}
