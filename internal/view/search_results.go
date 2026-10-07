package view

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// RepoMatch es una coincidencia de la búsqueda en el repo: ruta, línea y
// columna (ambas base 0) y el texto de la línea recortado para mostrar.
type RepoMatch struct {
	Path string
	Line int
	Col  int
	Text string
}

// SearchResults es la ventana de resultados de Ctrl+Shift+F: lista de una
// columna con cursor navegable, misma mecánica que TabMenu. Enter devuelve
// (true, true) para que el controlador salte a la coincidencia; Escape y
// toda tecla ajena devuelven (false, false) y el controlador cierra
// descartando. Mientras está abierta posee el teclado.
type SearchResults struct {
	matches []RepoMatch
	query   string
	cursor  int
	top     int
	width   int
	height  int
	theme   Theme
}

// SetTheme reemplaza la paleta del componente.
func (m *SearchResults) SetTheme(th Theme) { m.theme = th }

// NewSearchResults crea la ventana vacía.
func NewSearchResults() *SearchResults { return &SearchResults{} }

// SetResults carga los resultados y la query que los produjo, con el cursor
// al inicio.
func (m *SearchResults) SetResults(matches []RepoMatch, query string) {
	m.matches = matches
	m.query = query
	m.cursor, m.top = 0, 0
	m.ensureCursorVisible()
}

// Len devuelve cuántos resultados hay.
func (m *SearchResults) Len() int { return len(m.matches) }

// Selected devuelve la coincidencia del cursor, o false si no hay ninguna.
func (m *SearchResults) Selected() (RepoMatch, bool) {
	if m.cursor < 0 || m.cursor >= len(m.matches) {
		return RepoMatch{}, false
	}
	return m.matches[m.cursor], true
}

func (m *SearchResults) clamp() {
	if len(m.matches) == 0 {
		m.cursor, m.top = 0, 0
		return
	}
	m.cursor = min(max(m.cursor, 0), len(m.matches)-1)
	maxTop := len(m.matches) - m.height
	if maxTop < 0 {
		maxTop = 0
	}
	m.top = min(max(m.top, 0), maxTop)
}

func (m *SearchResults) ensureCursorVisible() {
	if len(m.matches) == 0 || m.height <= 0 {
		return
	}
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+m.height {
		m.top = m.cursor - m.height + 1
	}
	m.clamp()
}

func (m *SearchResults) moveCursor(delta int) bool {
	if len(m.matches) == 0 {
		return false
	}
	target := m.cursor + delta
	if target < 0 {
		target = 0
	}
	if target >= len(m.matches) {
		target = len(m.matches) - 1
	}
	if target == m.cursor {
		return false
	}
	m.cursor = target
	m.ensureCursorVisible()
	return true
}

func (m *SearchResults) page() int {
	if m.height > 1 {
		return m.height
	}
	return 1
}

// Resize actualiza las dimensiones y reencuadra el scroll.
func (m *SearchResults) Resize(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
	m.clamp()
	m.ensureCursorVisible()
}

// HandleEvent procesa el teclado de la ventana y devuelve
// (handled, activate): Up/Down/PgUp/PgDn/Home/End navegan,
// Enter activa la coincidencia del cursor.
func (m *SearchResults) HandleEvent(ev tcell.Event) (handled, activate bool) {
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
			m.cursor = len(m.matches) - 1
			m.ensureCursorVisible()
			return true, false
		case tcell.KeyEnter, tcell.KeyLF:
			if len(m.matches) == 0 {
				return false, false
			}
			return true, true
		}
	}
	return false, false
}

// Draw pinta la ventana en coordenadas propias desde (0,0): la fila del
// cursor va resaltada a todo el ancho. Cada fila es
// "ruta:línea:col: texto", con línea base 1 para el humano.
func (m *SearchResults) Draw(s Surface, width int) {
	if len(m.matches) == 0 || m.height <= 0 || width <= 0 {
		return
	}
	for row := 0; row < m.height; row++ {
		idx := m.top + row
		if idx >= len(m.matches) {
			break
		}
		th := themeOr(m.theme)
		style := th.TabIdle
		if idx == m.cursor {
			style = th.TabActive
		}
		for x := 0; x < width; x++ {
			s.SetContent(x, row, ' ', nil, style)
		}
		rm := m.matches[idx]
		label := fmt.Sprintf("%s:%d:%d: %s", rm.Path, rm.Line+1, rm.Col+1, rm.Text)
		writeString(s, 0, row, label, style, width)
	}
}
