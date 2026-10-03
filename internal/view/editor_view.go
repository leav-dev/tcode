package view

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/model"
)

// Viewport controla qué región del documento se proyecta en la pantalla.
type Viewport struct {
	TopLine    int // Primera línea lógica visible
	LeftColumn int // Primera columna lógica visible (scroll horizontal)
	Height     int // Alto en filas de la terminal
	Width      int // Ancho en columnas de la terminal
}

// EditorView es la Screen encargada de renderizar el documento.
type EditorView struct {
	model    *model.PieceTable
	viewport Viewport
}

func NewEditorView(m *model.PieceTable, height, width int) *EditorView {
	return &EditorView{
		model: m,
		viewport: Viewport{
			TopLine:    0,
			LeftColumn: 0,
			Height:     height,
			Width:      width,
		},
	}
}

// Resize actualiza las dimensiones del viewport cuando cambia la terminal.
func (v *EditorView) Resize(width, height int) {
	if width > 0 {
		v.viewport.Width = width
	}
	if height > 0 {
		v.viewport.Height = height
	}
	v.clamp()
}

// Draw solo recorre las líneas visibles y decodifica UTF-8 sin alocar.
func (v *EditorView) Draw(s tcell.Screen) {
	content := v.model.GetRange(v.viewport.TopLine, v.viewport.TopLine+v.viewport.Height)

	row := 0 // fila física en pantalla
	col := 0 // columna lógica dentro de la línea
	for i := 0; i < len(content) && row < v.viewport.Height; {
		r, size := utf8.DecodeRune(content[i:])
		i += size

		if r == '\n' {
			row++
			col = 0
			continue
		}

		if col >= v.viewport.LeftColumn {
			x := col - v.viewport.LeftColumn
			if x < v.viewport.Width {
				s.SetContent(x, row, r, nil, tcell.StyleDefault)
			}
		}
		col++
	}
}

// HandleEvent procesa teclado y mouse. Devuelve true si el viewport cambió
// y la pantalla necesita redibujarse.
func (v *EditorView) HandleEvent(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		return v.handleKey(ev)
	case *tcell.EventMouse:
		return v.handleMouse(ev)
	}
	return false
}

func (v *EditorView) handleKey(ev *tcell.EventKey) bool {
	page := v.viewport.Height
	if page < 1 {
		page = 1
	}

	switch ev.Key() {
	case tcell.KeyUp:
		return v.scrollBy(-1)
	case tcell.KeyDown:
		return v.scrollBy(1)
	case tcell.KeyPgUp:
		return v.scrollBy(-page)
	case tcell.KeyPgDn:
		return v.scrollBy(page)
	case tcell.KeyHome:
		return v.setTopLine(0)
	case tcell.KeyEnd:
		return v.setTopLine(v.maxTopLine())
	case tcell.KeyLeft:
		return v.scrollColumnBy(-1)
	case tcell.KeyRight:
		return v.scrollColumnBy(1)
	}

	switch ev.Rune() {
	case 'j':
		return v.scrollBy(1)
	case 'k':
		return v.scrollBy(-1)
	case 'h':
		return v.scrollColumnBy(-1)
	case 'l':
		return v.scrollColumnBy(1)
	case 'g':
		return v.setTopLine(0)
	case 'G':
		return v.setTopLine(v.maxTopLine())
	}
	return false
}

func (v *EditorView) handleMouse(ev *tcell.EventMouse) bool {
	switch {
	case ev.Buttons()&tcell.WheelUp != 0:
		return v.scrollBy(-3)
	case ev.Buttons()&tcell.WheelDown != 0:
		return v.scrollBy(3)
	case ev.Buttons()&tcell.WheelLeft != 0:
		return v.scrollColumnBy(-3)
	case ev.Buttons()&tcell.WheelRight != 0:
		return v.scrollColumnBy(3)
	}
	return false
}

// maxTopLine es la mayor primera línea visible que aún muestra contenido útil.
func (v *EditorView) maxTopLine() int {
	total := v.model.LineCount()
	if total <= v.viewport.Height {
		return 0
	}
	return total - v.viewport.Height
}

func (v *EditorView) setTopLine(line int) bool {
	if line < 0 {
		line = 0
	}
	if max := v.maxTopLine(); line > max {
		line = max
	}
	if line == v.viewport.TopLine {
		return false
	}
	v.viewport.TopLine = line
	return true
}

func (v *EditorView) scrollBy(delta int) bool {
	return v.setTopLine(v.viewport.TopLine + delta)
}

func (v *EditorView) scrollColumnBy(delta int) bool {
	col := v.viewport.LeftColumn + delta
	if col < 0 {
		col = 0
	}
	if col == v.viewport.LeftColumn {
		return false
	}
	v.viewport.LeftColumn = col
	return true
}

// clamp mantiene el viewport dentro del documento tras un resize.
func (v *EditorView) clamp() {
	v.setTopLine(v.viewport.TopLine)
}
