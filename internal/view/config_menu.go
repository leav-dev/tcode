package view

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// ConfigKind distingue los tres tipos de ajuste de la ventana de
// configuración: un entero con rango y paso (Tab size, Panel width), un
// booleano (Word wrap) o un enum de opciones con nombre (Theme).
type ConfigKind int

const (
	ConfigBool ConfigKind = iota
	ConfigInt
	ConfigEnum
)

// configItem es una fila de la ventana de configuración: la etiqueta, el tipo
// de ajuste, el rango y paso (solo los enteros), las opciones con nombre
// (solo el enum) y las puertas get/set sobre la var del paquete. get devuelve
// el valor NORMALIZADO (el booleano como 0/1, el enum como índice) y set lo
// aplica. Las filas se construyen SIEMPRE con closures sobre las vars
// globales de view: la ventana edita la configuración viva del editor.
type configItem struct {
	label string
	kind  ConfigKind
	min   int
	max   int
	step  int
	names []string // opciones del enum, en orden (Theme: registry + "Custom")
	get   func() int
	set   func(int)
}

// configItems construye las cuatro filas fijas de la ventana de configuración,
// con sus rangos: Tab size de 1 a 8, Word wrap sin rango (booleano), Panel
// width de 16 a 48 en pasos de 2 y Theme con las paletas del registry más
// "Custom" al final (el tema del usuario o el default, id activo "").
func configItems() []configItem {
	return []configItem{
		{label: "Tab size", kind: ConfigInt, min: 1, max: 8, step: 1, get: IndentSize, set: SetIndentSize},
		{
			label: "Word wrap",
			kind:  ConfigBool,
			get: func() int {
				if WordWrapEnabled() {
					return 1
				}
				return 0
			},
			set: func(v int) { SetWordWrapEnabled(v == 1) },
		},
		{label: "Panel width", kind: ConfigInt, min: 16, max: 48, step: 2, get: ExplorerWidth, set: SetExplorerWidth},
		{
			label: "Theme",
			kind:  ConfigEnum,
			names: append(ThemeNames(), "Custom"),
			get: func() int {
				// El índice del id activo dentro del registry; "" (Custom, o un
				// id que ya no esté registrado) cae en la cola, "Custom".
				for i, id := range ThemeIDs() {
					if id == ActiveThemeID() {
						return i
					}
				}
				return len(ThemeNames())
			},
			set: func(i int) {
				if i >= len(ThemeNames()) {
					SetActiveThemeID("")
					return
				}
				SetActiveThemeID(ThemeIDs()[i])
			},
		},
	}
}

// ConfigMenuHeight es el alto que la ventana necesita para mostrar TODAS sus
// filas: el marco de arriba y el de abajo más las cuatro filas. El controlador
// lo usa para dimensionar la región flotante (configRegion).
func ConfigMenuHeight() int { return len(configItems()) + 2 }

// ConfigMenu es la ventana flotante de configuración (Ctrl+P): una lista de
// cuatro filas con cursor (la mecánica exacta del menú de pestañas —cursor/top
// y su scroll mínimo—) dentro de un marco centrado sobre el área del editor.
// Left/Right mutan la fila del cursor (y Enter alterna el booleano); Up/Down y
// el resto de la navegación mueven el cursor. En el enum, Left/Right circulan
// por las opciones (wrap por los extremos) y Enter no hace nada, como en los
// enteros. Escape, Ctrl+C y toda tecla ajena devuelven (false, false) y el
// controlador cierra la ventana descartando; mientras está abierta posee el
// teclado y el mouse, así que el documento no recibe nada por accidente.
type ConfigMenu struct {
	cursor int // índice de la fila del cursor
	top    int // primera fila visible
	width  int // ancho de la ventana (Resize)
	height int // alto de la ventana (Resize)
	theme  Theme
}

func NewConfigMenu() *ConfigMenu {
	return &ConfigMenu{}
}

// SetTheme reemplaza la paleta del componente.
func (m *ConfigMenu) SetTheme(th Theme) { m.theme = th }

// count es la cantidad de filas fijas de la ventana.
func (m *ConfigMenu) count() int { return len(configItems()) }

// clamp mantiene cursor y top dentro del rango de filas (y del alto).
func (m *ConfigMenu) clamp() {
	n := m.count()
	if n == 0 {
		m.cursor, m.top = 0, 0
		return
	}
	m.cursor = min(max(m.cursor, 0), n-1)
	maxTop := n - m.height
	if maxTop < 0 {
		maxTop = 0
	}
	m.top = min(max(m.top, 0), maxTop)
}

// ensureCursorVisible corre top lo mínimo para que la fila del cursor quede
// dentro del alto de la ventana, como el explorador con su lista.
func (m *ConfigMenu) ensureCursorVisible() {
	n := m.count()
	if n == 0 || m.height <= 0 {
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

// setCursor coloca el cursor en el índice i (clampeado) y mantiene la fila
// visible.
func (m *ConfigMenu) setCursor(i int) {
	m.cursor = i
	m.clamp()
	m.ensureCursorVisible()
}

// moveCursor desplaza el cursor en delta y devuelve si cambió de posición.
func (m *ConfigMenu) moveCursor(delta int) bool {
	n := m.count()
	if n == 0 {
		return false
	}
	target := m.cursor + delta
	if target < 0 {
		target = 0
	}
	if target >= n {
		target = n - 1
	}
	if target == m.cursor {
		return false
	}
	m.cursor = target
	m.ensureCursorVisible()
	return true
}

// page es el salto de página: lo que cabe en el alto de la ventana, mínimo 1.
func (m *ConfigMenu) page() int {
	if m.height > 1 {
		return m.height
	}
	return 1
}

// Resize actualiza las dimensiones de la ventana y reencuadra el scroll, como
// el resize del explorador: la fila del cursor queda visible y el top dentro
// del rango.
func (m *ConfigMenu) Resize(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
	m.clamp()
	m.ensureCursorVisible()
}

// mutate aplica un delta a la fila del cursor y devuelve si algo cambió. Un
// paso de -1/+1 viene de Left/Right y 0 de Enter: en un booleano cualquier
// paso (Enter incluido) alterna el valor; en un entero el nuevo valor se
// clampea a [min, max] con el paso y delta 0 no cambia nada (get()+0*step es
// el valor actual). El enum CIRCULA en vez de clampear: el paso sale por un
// extremo y entra por el otro (Right desde "Custom" vuelve a "light", Left
// desde "light" vuelve a "Custom"), con el ciclo reversible.
func (m *ConfigMenu) mutate(delta int) bool {
	it := configItems()[m.cursor]
	var nuevo int
	if it.kind == ConfigBool {
		nuevo = 1 - it.get()
	} else if it.kind == ConfigEnum {
		n := len(it.names)
		if n == 0 {
			return false
		}
		nuevo = (it.get() + delta) % n
		if nuevo < 0 {
			nuevo += n
		}
	} else {
		nuevo = min(max(it.get()+delta*it.step, it.min), it.max)
	}
	if nuevo == it.get() {
		return false
	}
	it.set(nuevo)
	return true
}

// HandleEvent procesa el teclado de la ventana y devuelve (handled, changed):
// handled dice si el evento era de la ventana y changed si una fila se mutó
// (para que el controlador persista y reencuadre).
//
// Up/Down/PageUp/PageDown/Home/End mueven el cursor y devuelven (true, false)
// aunque el cursor no se mueva: con la ventana abierta esas teclas son suyas y
// no del documento. Left/Right mutan la fila del cursor (delta -1/+1) y
// Enter/KeyLF alternan el booleano, devolviendo (true, changed). Escape,
// Ctrl+C y toda otra tecla devuelven (false, false) y caen al controlador,
// que cierra la ventana descartando —el mismo precedente del explorador—. El
// mouse no se maneja acá: con la ventana abierta el controlador descarta el
// mouse entero.
func (m *ConfigMenu) HandleEvent(ev tcell.Event) (handled, changed bool) {
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
			// (U2b), no scroll de página: la ventana los deja caer, igual que
			// el explorador y el editor con ModCtrl.
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
			m.setCursor(m.count() - 1)
			return true, false
		case tcell.KeyLeft:
			return true, m.mutate(-1)
		case tcell.KeyRight:
			return true, m.mutate(1)
		case tcell.KeyEnter, tcell.KeyLF:
			// Enter en un entero y en el enum no hace nada (delta 0); en un
			// booleano alterna. Siempre es de la ventana.
			return true, m.mutate(0)
		}
	}
	return false, false
}

// configValueText es el valor de la fila como texto: el entero con sus dígitos,
// "off"/"on" para el booleano o el NOMBRE de la opción para el enum.
func configValueText(it configItem) string {
	if it.kind == ConfigBool {
		if it.get() == 1 {
			return "on"
		}
		return "off"
	}
	if it.kind == ConfigEnum {
		if i := it.get(); i >= 0 && i < len(it.names) {
			return it.names[i]
		}
		return ""
	}
	return strconv.Itoa(it.get())
}

// Draw pinta la ventana en coordenadas propias desde (0,0): un marco con el
// título centrado sobre el borde superior y las filas visibles en el interior
// (desde la fila 1), la del cursor con la barra de selección a TODO el ancho
// interior y las demás con el estilo por defecto. Cada fila es etiqueta +
// separación + valor alineado a la derecha contra la pared. Sin alto (o sin
// ancho) no hay nada que dibujar; una fila fuera del alto se corta.
func (m *ConfigMenu) Draw(s Surface) {
	if m.width < 2 || m.height < 2 {
		return
	}
	th := themeOr(m.theme)
	items := configItems()

	// Marco: bordes superior e inferior y las paredes laterales.
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

	// Título centrado sobre el borde superior; writeString recorta si no entra.
	title := "Configuración"
	if start := (m.width - displayWidth(title)) / 2; start > 0 {
		writeString(s, start, 0, title, th.Text, m.width)
	} else {
		writeString(s, 0, 0, title, th.Text, m.width)
	}

	// Filas visibles: interior desde la fila 1, una fila por item a partir de
	// top. La del cursor se pinta entera con la barra de selección (TreeCursor)
	// sobre el ancho interior; las demás solo escriben su texto.
	for row := 0; row < m.height-2; row++ {
		idx := m.top + row
		if idx >= len(items) {
			break
		}
		it := items[idx]
		y := row + 1
		style := th.Text
		if idx == m.cursor {
			style = th.TreeCursor
			for x := 1; x < m.width-1; x++ {
				s.SetContent(x, y, ' ', nil, style)
			}
		}
		text := it.label
		if gap := m.width - 2 - displayWidth(text) - displayWidth(configValueText(it)); gap > 0 {
			text += strings.Repeat(" ", gap)
		}
		text += configValueText(it)
		writeString(s, 1, y, text, style, m.width-2)
	}
}
