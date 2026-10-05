package view

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// ExtTab es la sección activa de la ventana de extensiones: cada una tiene su
// propia lista, su propio cursor y su propio scroll, así que volver a una
// pestaña devuelve la vista exactamente donde estaba.
type ExtTab int

const (
	// ExtTabInstalled son las extensiones ya instaladas en la máquina.
	ExtTabInstalled ExtTab = iota
	// ExtTabUpdatable son las instaladas cuyo proveedor ofrece otra versión.
	ExtTabUpdatable
	// ExtTabAvailable son las que los proveedores ofrecen y no están
	// instaladas.
	ExtTabAvailable
	// ExtTabProviders son las fuentes registradas, con la de agregar al final.
	ExtTabProviders
	// extTabCount es cuántas pestañas hay: el tamaño de los arrays de cursor y
	// de scroll. No es una pestaña navegable.
	extTabCount
)

// extTabNames son los rótulos de las pestañas, en orden. El primero es el
// nombre de la extensión; "Proveedores" es donde vive la fuente.
var extTabNames = [extTabCount]string{
	"Instaladas",
	"Actualizables",
	"Disponibles",
	"Proveedores",
}

// extTabEmpty son los textos que se dibujan cuando una pestaña no tiene filas:
// una lista vacía sin explicación parece una ventana rota.
var extTabEmpty = [extTabCount]string{
	"sin extensiones instaladas",
	"sin actualizaciones pendientes",
	"sin extensiones disponibles",
}

// extTabHint es la pista de la acción propia de cada pestaña, escrita sobre el
// borde inferior para que la acción no viva solo en la documentación. Vacía =
// sin pista.
var extTabHint = [extTabCount]string{
	"espacio: activar/desactivar",
	"",
	"",
	"",
}

// ExtItemKind es lo que Enter hace con una fila. La ventana NO decide qué
// acción corresponde al tipo de pestaña: el controlador lo sabe por los datos
// que cargó, y la vista solo declara qué fila se tocó.
type ExtItemKind int

const (
	// ExtItemInfo es una fila informativa (un proveedor): Enter no actúa.
	ExtItemInfo ExtItemKind = iota
	// ExtItemInstall propone instalar la extensión de la fila.
	ExtItemInstall
	// ExtItemUpdate propone actualizarla a la versión del proveedor.
	ExtItemUpdate
	// ExtItemRemove propone borrarla del disco.
	ExtItemRemove
	// ExtItemAddProvider es la última fila de la pestaña de proveedores: abre
	// el pedido de texto para agregar una fuente.
	ExtItemAddProvider
)

// ExtItem es una fila de la ventana. Label es lo que se ve a la izquierda y
// Right el dato que se alinea a la derecha (la versión, el salto de versión, la
// marca de proveedor sin aprobar). ID, Provider y Ref son los datos con los que
// el controlador ejecuta la acción: la vista no lee el disco.
type ExtItem struct {
	Kind     ExtItemKind
	Label    string
	Right    string
	ID       string
	Provider string
	Ref      string
}

// ExtIntentKind es lo que la ventana pide al controlador: nada, actuar sobre la
// fila del cursor o agregar un proveedor.
type ExtIntentKind int

const (
	// ExtIntentNone es el resultado normal de una tecla que no propone nada.
	ExtIntentNone ExtIntentKind = iota
	// ExtIntentAction propone la acción de la fila del cursor: instalar,
	// actualizar o borrar.
	ExtIntentAction
	// ExtIntentAddProvider propone el pedido de texto para agregar una fuente.
	ExtIntentAddProvider
	// ExtIntentToggle propone alternar el estado (activa/desactivada) de la
	// extensión instalada del cursor.
	ExtIntentToggle
)

// ExtIntent es la intención que HandleEvent devuelve: qué pidió la ventana y
// sobre qué fila (de qué pestaña). El controlador la ejecuta —preguntando antes
// lo que corresponda— y recarga los datos.
type ExtIntent struct {
	Kind ExtIntentKind
	Tab  ExtTab
	Item ExtItem
}

// extManagerRows son las filas visibles que la ventana muestra: el alto lo
// decide este número (más el marco), no un literal en el controlador, para que
// la geometría sea la misma en el dibujo y en el resize.
const extManagerRows = 12

// ExtManagerHeight es el alto con el que la ventana se dimensiona: las filas
// visibles más el marco de arriba y el de abajo. El controlador lo recorta al
// alto real del editor si la terminal es chica.
func ExtManagerHeight() int { return extManagerRows + 2 }

// ExtManager es la ventana flotante de gestión de extensiones: cuatro pestañas
// (Instaladas / Actualizables / Disponibles / Proveedores) con cursor y scroll
// mínimo por pestaña —la mecánica del menú de pestañas y de la ventana de
// configuración— dentro de un marco centrado sobre el área del editor, con las
// pestañas dibujadas en el borde superior y la activa marcada.
//
// Left/Right cambian de pestaña y Up/Down mueven el cursor. Enter devuelve la
// INTENCIÓN de la fila del cursor (instalar, actualizar, borrar, agregar
// proveedor) para que el controlador la ejecute con su confirmación y sus
// datos: la vista no lee el disco ni muta nada. Escape, Ctrl+C y toda tecla ajena
// devuelven (false, ExtIntent{}) y el controlador cierra la ventana
// descartando, como en el resto de los overlays. Mientras está abierta posee el
// teclado (y el mouse se descarta entero en el controlador), así que el
// documento no recibe nada por accidente.
type ExtManager struct {
	items   [extTabCount][]ExtItem
	cursors [extTabCount]int
	tops    [extTabCount]int
	tab     ExtTab // pestaña activa
	width   int
	height  int
	theme   Theme
}

// NewExtManager devuelve una ventana vacía: sin filas hasta que el controlador
// cargue los datos con SetItems (al abrirla).
func NewExtManager() *ExtManager {
	return &ExtManager{}
}

// SetTheme reemplaza la paleta del componente.
func (m *ExtManager) SetTheme(th Theme) { m.theme = th }

// Reset vuelve la ventana a su estado inicial: primera pestaña, cursor y scroll
// en cero y listas vacías. openExtManager lo llama al abrir, para que la ventana
// nazca siempre en el mismo lugar: no es la pestaña donde quedó la sesión
// anterior la que debe decidir dónde empieza la próxima.
func (m *ExtManager) Reset() {
	m.items = [extTabCount][]ExtItem{}
	m.cursors = [extTabCount]int{}
	m.tops = [extTabCount]int{}
	m.tab = ExtTabInstalled
}

// SetItems deposita las filas de una pestaña. Reemplaza la lista entera (el
// controlador recarga la pestaña después de cada acción) y reencuadra el
// scroll: el cursor se queda donde estaba si la lista todavía lo tiene.
func (m *ExtManager) SetItems(tab ExtTab, items []ExtItem) {
	if tab < 0 || tab >= extTabCount {
		return
	}
	m.items[tab] = items
	m.clampTab(tab)
	m.ensureCursorVisible(tab)
}

// Items devuelve las filas de una pestaña (las que el controlador cargó).
func (m *ExtManager) Items(tab ExtTab) []ExtItem {
	if tab < 0 || tab >= extTabCount {
		return nil
	}
	return m.items[tab]
}

// Tab devuelve la pestaña activa.
func (m *ExtManager) Tab() ExtTab { return m.tab }

// Cursor devuelve la posición del cursor dentro de la pestaña activa.
func (m *ExtManager) Cursor() int { return m.cursors[m.tab] }

// Selected devuelve la fila del cursor de la pestaña activa, o la fila cero si
// la pestaña está vacía.
func (m *ExtManager) Selected() ExtItem {
	items := m.items[m.tab]
	if len(items) == 0 {
		return ExtItem{}
	}
	return items[min(max(m.cursors[m.tab], 0), len(items)-1)]
}

// topFor expone el scroll de una pestaña para los tests del reencuadre: con
// más filas que el alto, top dice cuál es la primera visible.
func (m *ExtManager) topFor(tab ExtTab) int { return m.tops[tab] }

// visibleRows es cuántas filas del interior muestra la ventana (el alto menos
// el marco), mínimo 0.
func (m *ExtManager) visibleRows() int {
	if rows := m.height - 2; rows > 0 {
		return rows
	}
	return 0
}

// clampTab mantiene cursor y top de una pestaña dentro del rango de filas (y
// del alto visible). Sobre una lista vacía ambos vuelven a 0.
func (m *ExtManager) clampTab(tab ExtTab) {
	n := len(m.items[tab])
	if n == 0 {
		m.cursors[tab], m.tops[tab] = 0, 0
		return
	}
	m.cursors[tab] = min(max(m.cursors[tab], 0), n-1)
	maxTop := n - m.visibleRows()
	if maxTop < 0 {
		maxTop = 0
	}
	m.tops[tab] = min(max(m.tops[tab], 0), maxTop)
}

// ensureCursorVisible corre top lo MÍNIMO para que la fila del cursor quede
// dentro del alto visible, como el explorador con su lista y la ventana de
// configuración con las suyas.
func (m *ExtManager) ensureCursorVisible(tab ExtTab) {
	rows := m.visibleRows()
	if len(m.items[tab]) == 0 || rows <= 0 {
		return
	}
	if m.cursors[tab] < m.tops[tab] {
		m.tops[tab] = m.cursors[tab]
	}
	if m.cursors[tab] >= m.tops[tab]+rows {
		m.tops[tab] = m.cursors[tab] - rows + 1
	}
	m.clampTab(tab)
}

// setCursor coloca el cursor de la pestaña en i (clampeado) y mantiene la fila
// visible.
func (m *ExtManager) setCursor(tab ExtTab, i int) {
	m.cursors[tab] = i
	m.clampTab(tab)
	m.ensureCursorVisible(tab)
}

// moveCursor desplaza el cursor de la pestaña en delta y devuelve si cambió.
func (m *ExtManager) moveCursor(delta int) bool {
	tab := m.tab
	n := len(m.items[tab])
	if n == 0 {
		return false
	}
	target := min(max(m.cursors[tab]+delta, 0), n-1)
	if target == m.cursors[tab] {
		return false
	}
	m.setCursor(tab, target)
	return true
}

// page es el salto de página: lo que cabe en el alto visible, mínimo 1.
func (m *ExtManager) page() int {
	if rows := m.visibleRows(); rows > 1 {
		return rows
	}
	return 1
}

// switchTab cambia de pestaña con wrap (Izquierda desde la primera va a la
// última y viceversa) y deja la nueva con su cursor donde estaba.
func (m *ExtManager) switchTab(delta int) {
	m.tab = ExtTab(((int(m.tab)+delta)%int(extTabCount) + int(extTabCount)) % int(extTabCount))
}

// Resize actualiza las dimensiones de la ventana y reencuadra el scroll de
// TODAS las pestañas, como el resize del explorador: la fila del cursor de cada
// una queda visible y su top dentro del rango.
func (m *ExtManager) Resize(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
	for tab := ExtTab(0); tab < extTabCount; tab++ {
		m.clampTab(tab)
		m.ensureCursorVisible(tab)
	}
}

// HandleEvent procesa el teclado de la ventana y devuelve (handled, intent):
// handled dice si el evento era de la ventana e intent es lo que la ventana
// propone al controlador.
//
// Up/Down/PageUp/PageDown/Home/End mueven el cursor de la pestaña activa y
// devuelven (true, ExtIntentNone) aunque no se mueva: con la ventana abierta esas
// teclas son suyas. Left/Right cambian de pestaña. Enter/KeyLF devuelven
// (true, ExtIntentAction) con la fila del cursor si la fila actúa (instalar,
// actualizar, borrar), (true, ExtIntentAddProvider) si es la de agregar
// proveedor y (true, ExtIntentNone) si la fila es informativa o la pestaña está
// vacía. Escape, Ctrl+C y toda otra tecla devuelven (false, ExtIntent{}) y caen
// al controlador, que cierra la ventana descartando. El mouse no se maneja acá:
// con la ventana abierta el controlador descarta el mouse entero.
func (m *ExtManager) HandleEvent(ev tcell.Event) (handled bool, intent ExtIntent) {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyUp:
			m.moveCursor(-1)
			return true, ExtIntent{}
		case tcell.KeyDown:
			m.moveCursor(1)
			return true, ExtIntent{}
		case tcell.KeyPgUp:
			// Ctrl+PageUp/PageDown cambian de pestaña del EDITOR y son del
			// controlador, no scroll de página: la ventana los deja caer, igual
			// que el resto de los overlays.
			if ev.Modifiers()&tcell.ModCtrl != 0 {
				return false, ExtIntent{}
			}
			m.moveCursor(-m.page())
			return true, ExtIntent{}
		case tcell.KeyPgDn:
			if ev.Modifiers()&tcell.ModCtrl != 0 {
				return false, ExtIntent{}
			}
			m.moveCursor(m.page())
			return true, ExtIntent{}
		case tcell.KeyHome:
			m.setCursor(m.tab, 0)
			return true, ExtIntent{}
		case tcell.KeyEnd:
			m.setCursor(m.tab, len(m.items[m.tab])-1)
			return true, ExtIntent{}
		case tcell.KeyLeft:
			m.switchTab(-1)
			return true, ExtIntent{}
		case tcell.KeyRight:
			m.switchTab(1)
			return true, ExtIntent{}
		case tcell.KeyEnter, tcell.KeyLF:
			return true, m.activate()
		case tcell.KeyRune:
			// Space alterna el estado de la extensión instalada del cursor. Es
			// una tecla DE LA VENTANA (no la cierra): en otra pestaña no hace
			// nada, pero sigue siendo suya.
			if ev.Rune() == ' ' {
				return true, m.toggleIntent()
			}
		}
	}
	return false, ExtIntent{}
}

// toggleIntent construye la intención de alternar el estado de la extensión
// instalada del cursor. En otra pestaña (o sin filas) no hay nada que
// alternar: se consume la tecla sin proponer nada.
func (m *ExtManager) toggleIntent() ExtIntent {
	if m.tab != ExtTabInstalled {
		return ExtIntent{Tab: m.tab}
	}
	it := m.Selected()
	if it.Kind != ExtItemRemove {
		return ExtIntent{Tab: m.tab}
	}
	return ExtIntent{Kind: ExtIntentToggle, Tab: m.tab, Item: it}
}

// activate construye la intención de la fila del cursor: la de agregar
// proveedor si es esa, la acción de la fila si actúa, y nada en los demás casos
// (una fila informativa o una pestaña vacía).
func (m *ExtManager) activate() ExtIntent {
	it := m.Selected()
	if it.Kind == ExtItemAddProvider {
		return ExtIntent{Kind: ExtIntentAddProvider, Tab: m.tab, Item: it}
	}
	switch it.Kind {
	case ExtItemInstall, ExtItemUpdate, ExtItemRemove:
		return ExtIntent{Kind: ExtIntentAction, Tab: m.tab, Item: it}
	}
	return ExtIntent{Tab: m.tab}
}

// Draw pinta la ventana en coordenadas propias desde (0,0): un marco con las
// PESTAÑAS en el borde superior —la activa entre corchetes y con el estilo de
// pestaña activa, las demás con el del texto— y las filas visibles de la
// pestaña activa en el interior (desde la fila 1), la del cursor con la barra de
// selección a TODO el ancho interior. Cada fila es etiqueta + separación + dato
// alineado a la derecha contra la pared; una pestaña sin filas muestra su texto
// de estado. Sin alto (o sin ancho) no hay nada que dibujar.
func (m *ExtManager) Draw(s Surface) {
	if m.width < 2 || m.height < 2 {
		return
	}
	th := themeOr(m.theme)

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

	// Pestañas sobre el borde superior: cada una con su rótulo y un espacio de
	// separación; la activa va entre corchetes para que se vea cuál es sin
	// depender del color. writeString recorta si no entran todas.
	x := 1
	for tab := ExtTab(0); tab < extTabCount && x < m.width-1; tab++ {
		label := extTabNames[tab]
		style := th.Text
		if tab == m.tab {
			label = "[" + label + "]"
			style = th.TabActive
		}
		advance := writeString(s, x, 0, label, style, m.width-1-x) + 1
		x += advance
	}

	// Pista de la acción de la pestaña activa, escrita sobre el borde inferior:
	// una acción de teclado invisible es una acción perdida.
	if hint := extTabHint[m.tab]; hint != "" && displayWidth(hint)+4 < m.width {
		writeString(s, 2, m.height-1, " "+hint+" ", th.Text, m.width-4)
	}

	// Filas visibles de la pestaña activa: interior desde la fila 1, una fila
	// por item a partir de su top. TODA fila se pinta entera ANTES de su texto
	// —la del cursor con la barra de selección, las demás con el estilo base—:
	// sin eso, una fila sin item deja ver el documento de atrás y la ventana
	// flotante parece transparente, que es justo lo que confunde.
	items := m.items[m.tab]
	top := m.tops[m.tab]
	for row := range m.visibleRows() {
		idx := top + row
		y := row + 1
		style := th.Text
		if idx == m.cursors[m.tab] && idx < len(items) {
			style = th.TreeCursor
		}
		for col := 1; col < m.width-1; col++ {
			s.SetContent(col, y, ' ', nil, style)
		}

		var label, right string
		switch {
		case idx < len(items):
			label, right = items[idx].Label, items[idx].Right
		case idx == len(items) && len(items) == 0:
			// Pestaña vacía: se explica una vez, en la primera fila, para que
			// el espacio en blanco no parezca una ventana rota.
			label = "(" + extTabEmpty[m.tab] + ")"
		default:
			break
		}
		if label == "" && right == "" {
			continue
		}
		if gap := m.width - 2 - displayWidth(label) - displayWidth(right); gap > 0 {
			label += strings.Repeat(" ", gap)
		}
		writeString(s, 1, y, label+right, style, m.width-2)
	}
}
