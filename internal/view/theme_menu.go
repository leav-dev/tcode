package view

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// ThemeMenu es la ventana flotante que lista TODOS los temas disponibles:
// paletas incluidas, temas de extensiones y el Custom al final. Una lista de
// una columna con cursor navegable y scroll mínimo (la mecánica del menú de
// pestañas): Up/Down mueven, PgUp/PgDn saltan, Home/End van a los extremos,
// Enter devuelve (true, true) para que el controlador aplique el tema del
// cursor, y Escape/Ctrl+C/toda tecla ajena devuelven (false, false) y el
// controlador la cierra descartando. Mientras está abierta posee el teclado.
type ThemeMenu struct {
	items  []ThemeOption // snapshot de AvailableThemes() al abrir
	cursor int
	top    int
	width  int
	height int
	theme  Theme
}

// NewThemeMenu crea la ventana de temas.
func NewThemeMenu() *ThemeMenu { return &ThemeMenu{} }

// SetTheme reemplaza la paleta del componente.
func (m *ThemeMenu) SetTheme(th Theme) { m.theme = th }

// Open captura los temas disponibles y coloca el cursor sobre el activo.
func (m *ThemeMenu) Open() {
	m.items = AvailableThemes()
	m.cursor = 0
	for i, it := range m.items {
		if it.ID == ActiveThemeID() {
			m.cursor = i
			break
		}
	}
	m.top = 0
	m.ensureCursorVisible()
}

// Items devuelve las filas capturadas al abrir (para tests y dibujo).
func (m *ThemeMenu) Items() []ThemeOption { return append([]ThemeOption(nil), m.items...) }

// Selected devuelve el tema del cursor y true, o false sin filas.
func (m *ThemeMenu) Selected() (ThemeOption, bool) {
	if len(m.items) == 0 {
		return ThemeOption{}, false
	}
	return m.items[m.cursor], true
}

// Cursor expone la fila del cursor (tests).
func (m *ThemeMenu) Cursor() int { return m.cursor }

func (m *ThemeMenu) clamp() {
	n := len(m.items)
	if n == 0 {
		m.cursor, m.top = 0, 0
		return
	}
	m.cursor = min(max(m.cursor, 0), n-1)
	rows := m.visibleRows()
	maxTop := n - rows
	if maxTop < 0 {
		maxTop = 0
	}
	m.top = min(max(m.top, 0), maxTop)
}

func (m *ThemeMenu) visibleRows() int {
	if rows := m.height - 2; rows > 0 {
		return rows
	}
	return 0
}

func (m *ThemeMenu) ensureCursorVisible() {
	n := len(m.items)
	rows := m.visibleRows()
	if n == 0 || rows <= 0 {
		return
	}
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+rows {
		m.top = m.cursor - rows + 1
	}
	m.clamp()
}

func (m *ThemeMenu) moveCursor(delta int) bool {
	if len(m.items) == 0 {
		return false
	}
	target := m.cursor + delta
	if target < 0 {
		target = 0
	}
	if target >= len(m.items) {
		target = len(m.items) - 1
	}
	if target == m.cursor {
		return false
	}
	m.cursor = target
	m.ensureCursorVisible()
	return true
}

func (m *ThemeMenu) page() int {
	if rows := m.visibleRows(); rows > 1 {
		return rows
	}
	return 1
}

// Resize actualiza las dimensiones y reencuadra el scroll.
func (m *ThemeMenu) Resize(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
	m.clamp()
	m.ensureCursorVisible()
}

// HandleEvent procesa el teclado: navegación devuelve (true, false), Enter
// sobre una fila devuelve (true, true), y lo ajeno (false, false).
func (m *ThemeMenu) HandleEvent(ev tcell.Event) (handled, selected bool) {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyUp:
			m.moveCursor(-1)
			return true, false
		case tcell.KeyDown:
			m.moveCursor(1)
			return true, false
		case tcell.KeyPgUp:
			if ev.Modifiers()&tcell.ModCtrl != 0 {
				return false, false
			}
			m.moveCursor(-m.page())
			return true, false
		case tcell.KeyPgDn:
			if ev.Modifiers()&tcell.ModCtrl != 0 {
				return false, false
			}
			m.moveCursor(m.page())
			return true, false
		case tcell.KeyHome:
			m.cursor = 0
			m.ensureCursorVisible()
			return true, false
		case tcell.KeyEnd:
			m.cursor = len(m.items) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.ensureCursorVisible()
			return true, false
		case tcell.KeyEnter, tcell.KeyLF:
			if len(m.items) == 0 {
				return true, false
			}
			return true, true
		}
	}
	return false, false
}

// Draw pinta la ventana en coordenadas propias desde (0,0): marco con título
// y filas visibles, la del cursor con la barra de selección. Cada fila es
// nombre + origen a la derecha; la activa lleva el marcador `*`.
func (m *ThemeMenu) Draw(s Surface) {
	if m.width < 2 || m.height < 2 {
		return
	}
	th := themeOr(m.theme)
	s.SetContent(0, 0, '┌', nil, th.Text)
	for x := 1; x < m.width-1; x++ {
		s.SetContent(x, 0, '─', nil, th.Text)
	}
	s.SetContent(m.width-1, 0, '┐', nil, th.Text)
	s.SetContent(0, m.height-1, '└', nil, th.Text)
	for x := 1; x < m.width-1; x++ {
		s.SetContent(x, m.height-1, '─', nil, th.Text)
	}
	s.SetContent(m.width-1, m.height-1, '┘', nil, th.Text)
	for y := 1; y < m.height-1; y++ {
		s.SetContent(0, y, '│', nil, th.Text)
		s.SetContent(m.width-1, y, '│', nil, th.Text)
	}
	title := "Temas"
	if start := (m.width - displayWidth(title)) / 2; start > 0 {
		writeString(s, start, 0, title, th.Text, m.width)
	} else {
		writeString(s, 0, 0, title, th.Text, m.width)
	}
	for row := 0; row < m.height-2; row++ {
		y := row + 1
		idx := m.top + row
		style := th.Text
		if idx == m.cursor && idx < len(m.items) {
			style = th.TreeCursor
		}
		for x := 1; x < m.width-1; x++ {
			s.SetContent(x, y, ' ', nil, style)
		}
		if idx >= len(m.items) {
			continue
		}
		it := m.items[idx]
		mark := "  "
		if it.ID == ActiveThemeID() {
			mark = "* "
		}
		text := mark + it.Name
		right := it.Source
		if gap := m.width - 2 - displayWidth(text) - displayWidth(right); gap > 0 {
			text += strings.Repeat(" ", gap)
		} else if gap := m.width - 2 - displayWidth(text); gap > 0 {
			text += strings.Repeat(" ", gap)
			right = ""
		} else {
			right = ""
		}
		text += right
		writeString(s, 1, y, text, style, m.width-2)
	}
}
