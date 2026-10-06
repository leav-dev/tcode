package view

import (
	"github.com/gdamore/tcell/v2"
	"github.com/leav-dev/tcode/internal/model"
)

// TabMenu es el superpuesto transitorio de pestañas (Ctrl+T): una lista de una
// columna con cursor navegable y scroll mínimo, con la misma mecánica que el
// explorador. Enter devuelve (true, true) para que el controlador active la
// pestaña elegida; Escape y toda tecla ajena devuelven (false, false) y el
// controlador cierra el menú descartando. Mientras está abierto posee el
// teclado (el controlador también descarta el mouse entero), así que el
// documento no recibe nada por accidente.
//
// El conteo de pestañas se captura en Open: HandleEvent no recibe el
// workspace, y el conteo no cambia mientras el menú está abierto porque el
// teclado es del menú (o lo cierra). El controlador lo usa para el clamp y
// para que Enter solo active si hay pestañas.
type TabMenu struct {
	cursor int // índice de la pestaña del cursor
	count  int // cuántas pestañas había al abrir (capturado en Open)
	top    int // primera pestaña visible
	width  int // ancho del panel (Resize)
	height int // alto del panel (Resize)
	theme  Theme
}

// SetTheme reemplaza la paleta del componente.
func (m *TabMenu) SetTheme(th Theme) { m.theme = th }

func NewTabMenu() *TabMenu {
	return &TabMenu{}
}

// Open prepara el menú para el workspace actual: captura el conteo y coloca el
// cursor sobre la pestaña activa (ws.ActiveIndex() clampeado a [0, count-1]),
// con el scroll corrido lo mínimo para que sea visible.
func (m *TabMenu) Open(ws *model.Workspace) {
	m.count = ws.Len()
	if m.count == 0 {
		m.cursor, m.top = 0, 0
		return
	}
	m.cursor = min(max(ws.ActiveIndex(), 0), m.count-1)
	m.ensureCursorVisible()
}

// clamp mantiene cursor y top dentro del rango de pestañas (y del alto).
// Sobre un conteo cero ambos vuelven a 0.
func (m *TabMenu) clamp() {
	if m.count == 0 {
		m.cursor, m.top = 0, 0
		return
	}
	m.cursor = min(max(m.cursor, 0), m.count-1)
	maxTop := m.count - m.height
	if maxTop < 0 {
		maxTop = 0
	}
	m.top = min(max(m.top, 0), maxTop)
}

// ensureCursorVisible corre top lo mínimo para que la pestaña del cursor quede
// dentro del alto del menú, como hace el explorador con su lista.
func (m *TabMenu) ensureCursorVisible() {
	if m.count == 0 || m.height <= 0 {
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

// setCursor coloca el cursor en el índice i (clampeado) y mantiene la pestaña
// visible.
func (m *TabMenu) setCursor(i int) {
	m.cursor = i
	m.clamp()
	m.ensureCursorVisible()
}

// moveCursor desplaza el cursor en delta y devuelve si cambió de posición.
func (m *TabMenu) moveCursor(delta int) bool {
	if m.count == 0 {
		return false
	}
	target := m.cursor + delta
	if target < 0 {
		target = 0
	}
	if target >= m.count {
		target = m.count - 1
	}
	if target == m.cursor {
		return false
	}
	m.cursor = target
	m.ensureCursorVisible()
	return true
}

// page es el salto de página: lo que cabe en el alto del menú, mínimo 1.
func (m *TabMenu) page() int {
	if m.height > 1 {
		return m.height
	}
	return 1
}

// Resize actualiza las dimensiones del menú y reencuadra el scroll, como el
// resize del explorador: la pestaña del cursor queda visible y el top dentro
// del rango.
func (m *TabMenu) Resize(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
	m.clamp()
	m.ensureCursorVisible()
}

// Selected devuelve el índice de la pestaña del cursor.
func (m *TabMenu) Selected() int { return m.cursor }

// HandleEvent procesa el teclado del menú y devuelve (handled, activate):
// handled dice si el evento era del menú y activate si Enter pidió activar la
// pestaña del cursor (para que el controlador la active y cierre).
//
// Up/Down/PageUp/PageDown/Home/End mueven el cursor y devuelven (true, false)
// aunque el cursor no se mueva: con el menú abierto esas teclas son del menú y
// no del documento. Enter/KeyLF devuelven (true, true) solo si hay pestañas.
// Escape/KeyCtrlC y toda otra tecla devuelven (false, false) y caen al
// controlador, que cierra el menú descartando —el mismo precedente del
// explorador, que tampoco maneja Escape—. El mouse no se maneja acá: con el
// menú abierto el controlador descarta el mouse entero.
func (m *TabMenu) HandleEvent(ev tcell.Event) (handled, activate bool) {
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
			// Ctrl+PageUp/PageDown cambian de pestaña y son del controlador
			// (U2b), no scroll de página: el menú los deja caer, igual que el
			// explorador y el editor con ModCtrl.
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
			m.setCursor(0)
			return true, false
		case tcell.KeyEnd:
			m.setCursor(m.count - 1)
			return true, false
		case tcell.KeyEnter, tcell.KeyLF:
			if m.count == 0 {
				return false, false
			}
			return true, true
		}
	}
	return false, false
}

// Draw pinta el menú en coordenadas propias desde (0,0): la fila del cursor va
// resaltada a todo el ancho y las demás con el estilo por defecto; las
// etiquetas reusan tabLabel (nombre base, "[+]" si está sucia, "(sin nombre)"
// sin ruta), recortadas contra el ancho con writeString. Con count cero o sin
// alto no hay nada que dibujar.
func (m *TabMenu) Draw(s Surface, ws *model.Workspace, width int) {
	if m.count == 0 || m.height <= 0 || width <= 0 {
		return
	}
	for row := 0; row < m.height; row++ {
		idx := m.top + row
		if idx >= m.count {
			break
		}
		// La fila se pinta entera con el estilo de la pestaña: la barra de
		// resaltado de ancho completo hace legible la del cursor aunque la
		// etiqueta sea corta, tapa el editor que queda debajo (es un overlay)
		// y limpia el resaltado viejo del redibujo anterior.
		th := themeOr(m.theme)
		style := th.TabIdle
		if idx == m.cursor {
			style = th.TabActive
		}
		for x := 0; x < width; x++ {
			s.SetContent(x, row, ' ', nil, style)
		}
		// La pestaña ACTIVA lleva su marcador («> ») aunque el cursor esté
		// en otra fila: al abrir, cursor y activa coinciden, pero al navegar
		// el menú conviene seguir viendo cuál es la abierta.
		prefix := "  "
		if idx == ws.ActiveIndex() {
			prefix = "> "
		}
		writeString(s, 0, row, prefix+tabLabel(ws.BufferAt(idx)), style, width)
	}
}
