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
	maxTop := len(m.matches) - m.visibleRows()
	if maxTop < 0 {
		maxTop = 0
	}
	m.top = min(max(m.top, 0), maxTop)
}

// visibleRows son las filas de resultados que entran en la ventana: el alto
// total menos el marco (borde superior con título e inferior).
func (m *SearchResults) visibleRows() int {
	if m.height < 3 {
		return 0
	}
	return m.height - 2
}

func (m *SearchResults) ensureCursorVisible() {
	if len(m.matches) == 0 || m.visibleRows() <= 0 {
		return
	}
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+m.visibleRows() {
		m.top = m.cursor - m.visibleRows() + 1
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
	if m.visibleRows() > 1 {
		return m.visibleRows()
	}
	return 1
}

// repoLabel arma la etiqueta de una coincidencia: "ruta:línea:col: texto",
// con línea base 1 para el humano.
func repoLabel(rm RepoMatch) string {
	return fmt.Sprintf("%s:%d:%d: %s", rm.Path, rm.Line+1, rm.Col+1, rm.Text)
}

// DesiredWidth devuelve el ancho de la etiqueta más larga (para dimensionar
// la ventana flotante), mínimo 20 para que el marco y el título respiren.
func (m *SearchResults) DesiredWidth() int {
	w := 20
	for _, rm := range m.matches {
		if l := displayWidth(repoLabel(rm)); l > w {
			w = l
		}
	}
	return w
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

// Draw pinta la ventana flotante en coordenadas propias desde (0,0): marco
// con el título centrado sobre el borde superior y las filas visibles en el
// interior (desde la fila 1), la del cursor con la barra de selección a
// todo el ancho interior. Cada fila es "ruta:línea:col: texto", con línea
// base 1. Sin matches o sin tamaño mínimo no hay nada que dibujar.
func (m *SearchResults) Draw(s Surface, width int) {
	if len(m.matches) == 0 || m.width < 3 || m.height < 3 || width <= 0 {
		return
	}
	th := themeOr(m.theme)
	w := min(m.width, width)
	// Marco: bordes superior e inferior y paredes laterales.
	s.SetContent(0, 0, '┌', nil, th.Text)
	for x := 1; x < w-1; x++ {
		s.SetContent(x, 0, '─', nil, th.Text)
	}
	s.SetContent(w-1, 0, '┐', nil, th.Text)
	s.SetContent(0, m.height-1, '└', nil, th.Text)
	for x := 1; x < w-1; x++ {
		s.SetContent(x, m.height-1, '─', nil, th.Text)
	}
	s.SetContent(w-1, m.height-1, '┘', nil, th.Text)
	for y := 1; y < m.height-1; y++ {
		s.SetContent(0, y, '│', nil, th.Text)
		s.SetContent(w-1, y, '│', nil, th.Text)
	}
	// Título centrado sobre el borde superior: query + conteo.
	title := fmt.Sprintf("Buscar: %s (%d)", m.query, len(m.matches))
	if start := (w - displayWidth(title)) / 2; start > 0 {
		writeString(s, start, 0, title, th.Text, w)
	} else {
		writeString(s, 0, 0, title, th.Text, w)
	}
	// Interior: cada fila se pinta entera antes de escribir su texto para
	// que el documento no se transparente a través de la ventana.
	for row := 0; row < m.visibleRows(); row++ {
		y := row + 1
		idx := m.top + row
		if idx >= len(m.matches) {
			break
		}
		style := th.TabIdle
		if idx == m.cursor {
			style = th.TabActive
		}
		for x := 1; x < w-1; x++ {
			s.SetContent(x, y, ' ', nil, style)
		}
		// El texto arranca en el interior (x=1) y se recorta antes de la
		// pared derecha.
		writeString(s, 1, y, repoLabel(m.matches[idx]), style, w-2)
	}
}
