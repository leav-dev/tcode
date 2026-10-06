package view

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// ExtensionEntry es una extensión del catálogo tal como la muestra el panel.
// El view no puede importar ext (ext ya importa view, por view.Diagnostic):
// el panel define su propio tipo y el controlador traduce entre los dos.
type ExtensionEntry struct {
	ID        string
	Name      string
	Version   string
	Subdir    string
	Installed bool
}

// ExtensionsPanel es el panel de extensiones del catálogo: una lista con
// cursor y scroll (la mecánica de TabMenu) con multiselección por Espacio y un
// item final que instala las marcadas. Lo abre la ventana de configuración
// (Ctrl+P → Extensions) y mientras está activo posee el teclado y el mouse:
// las teclas que no maneja (Escape incluido) lo cierran y devuelven a la
// ventana de configuración.
type ExtensionsPanel struct {
	items   []ExtensionEntry // el catálogo consultado
	marked  map[string]bool  // ids marcados para instalar
	cursor  int              // índice de la fila del cursor (incluye el item final)
	top     int              // primera fila visible
	width   int              // ancho del panel (Resize)
	height  int              // alto del panel (Resize, marco incluido)
	theme   Theme
	loading bool   // consultando el catálogo
	loadErr string // último error de consulta (vacío = sin error)
}

func NewExtensionsPanel() *ExtensionsPanel {
	return &ExtensionsPanel{marked: make(map[string]bool)}
}

// SetTheme reemplaza la paleta del componente.
func (p *ExtensionsPanel) SetTheme(th Theme) { p.theme = th }

// count es la cantidad de filas de la lista: las extensiones más el item final.
func (p *ExtensionsPanel) count() int { return len(p.items) + 1 }

// visibleRows es la cantidad de filas interiores: el alto menos el marco.
func (p *ExtensionsPanel) visibleRows() int { return p.height - 2 }

// clamp mantiene cursor y top dentro del rango de filas (y del alto).
func (p *ExtensionsPanel) clamp() {
	n := p.count()
	if n == 0 {
		p.cursor, p.top = 0, 0
		return
	}
	p.cursor = min(max(p.cursor, 0), n-1)
	maxTop := n - p.visibleRows()
	if maxTop < 0 {
		maxTop = 0
	}
	p.top = min(max(p.top, 0), maxTop)
}

// ensureCursorVisible corre top lo mínimo para que la fila del cursor quede
// dentro del alto del panel, como el explorador con su lista.
func (p *ExtensionsPanel) ensureCursorVisible() {
	n := p.count()
	if n == 0 || p.visibleRows() <= 0 {
		return
	}
	if p.cursor < p.top {
		p.top = p.cursor
	}
	if p.cursor >= p.top+p.visibleRows() {
		p.top = p.cursor - p.visibleRows() + 1
	}
	p.clamp()
}

// setCursor coloca el cursor en el índice i (clampeado) y mantiene la fila
// visible.
func (p *ExtensionsPanel) setCursor(i int) {
	p.cursor = i
	p.clamp()
	p.ensureCursorVisible()
}

// moveCursor desplaza el cursor en delta y devuelve si cambió de posición.
func (p *ExtensionsPanel) moveCursor(delta int) bool {
	n := p.count()
	if n == 0 {
		return false
	}
	target := p.cursor + delta
	if target < 0 {
		target = 0
	}
	if target >= n {
		target = n - 1
	}
	if target == p.cursor {
		return false
	}
	p.cursor = target
	p.ensureCursorVisible()
	return true
}

// page es el salto de página: lo que cabe en el alto del panel, mínimo 1.
func (p *ExtensionsPanel) page() int {
	if p.visibleRows() > 0 {
		return p.visibleRows()
	}
	return 1
}

// Resize actualiza las dimensiones del panel y reencuadra el scroll: al abrir
// el cursor y el scroll vuelven al principio (lista leída de arriba abajo).
func (p *ExtensionsPanel) Resize(width, height int) {
	if width > 0 {
		p.width = width
	}
	if height > 0 {
		p.height = height
	}
	p.cursor, p.top = 0, 0
	p.clamp()
	p.ensureCursorVisible()
}

// SetLoading marca el panel como "consultando el catálogo": muestra el aviso
// de carga hasta que lleguen las entradas o un error.
func (p *ExtensionsPanel) SetLoading() {
	p.loading = true
	p.loadErr = ""
}

// SetError fija el mensaje de error de la consulta: el panel muestra "Sin
// conexión" en lugar de la lista.
func (p *ExtensionsPanel) SetError(msg string) {
	p.loading = false
	p.loadErr = msg
}

// SetEntries reemplaza el catálogo consultado: limpia las marcas, el cursor y
// el scroll (lista nueva, lectura desde arriba) y apaga el aviso de carga.
func (p *ExtensionsPanel) SetEntries(entries []ExtensionEntry) {
	p.items = entries
	clear(p.marked)
	p.cursor, p.top = 0, 0
	p.loading = false
	p.loadErr = ""
	p.clamp()
	p.ensureCursorVisible()
}

// Entries devuelve el catálogo actualmente mostrado.
func (p *ExtensionsPanel) Entries() []ExtensionEntry { return p.items }

// Height es el alto que el panel necesita para mostrar TODAS sus filas: las
// extensiones, el item final y el marco de arriba y el de abajo. El
// controlador lo usa para dimensionar la región flotante.
func (p *ExtensionsPanel) Height() int { return len(p.items) + 1 + 2 }

// toggleMark alterna la marca de la extensión bajo el cursor: el item final
// (cursor == len(items)) no se marca.
func (p *ExtensionsPanel) toggleMark() {
	if p.cursor >= len(p.items) {
		return
	}
	id := p.items[p.cursor].ID
	p.marked[id] = !p.marked[id]
}

// installIDs resuelve los ids a instalar según la fila del cursor: sobre una
// extensión, esa; sobre el item final, las marcadas —o, sin marcas, la
// extensión de la fila de arriba (la última de la lista).
func (p *ExtensionsPanel) installIDs() []string {
	if p.cursor < len(p.items) {
		return []string{p.items[p.cursor].ID}
	}
	var ids []string
	for _, e := range p.items {
		if p.marked[e.ID] {
			ids = append(ids, e.ID)
		}
	}
	if len(ids) == 0 && len(p.items) > 0 {
		ids = append(ids, p.items[len(p.items)-1].ID)
	}
	return ids
}

// HandleEvent procesa el teclado del panel y devuelve (handled, installIDs,
// close): handled dice si la tecla era del panel, installIDs los ids que el
// panel pide instalar (Enter sobre una extensión o sobre el item final) y
// close si Escape pidió cerrar (el controlador vuelve a la ventana de
// configuración).
//
// Up/Down/PageUp/PageDown/Home/End mueven el cursor (con scroll en ambos
// sentidos), Espacio marca/desmarca la extensión del cursor (el item final no
// se marca), Enter instala —la del cursor, o las marcadas desde el item final—
// y Escape cierra. Ctrl+PageUp/PageDown cambian de pestaña y son del
// controlador: el panel los deja caer. El mouse no se maneja acá: con el panel
// abierto el controlador descarta el mouse entero.
func (p *ExtensionsPanel) HandleEvent(ev tcell.Event) (handled bool, installIDs []string, close bool) {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyUp:
			p.moveCursor(-1)
			return true, nil, false
		case tcell.KeyDown:
			p.moveCursor(1)
			return true, nil, false
		case tcell.KeyPgUp:
			// Ctrl+PageUp/PageDown cambian de pestaña y son del controlador,
			// no scroll de página: el panel los deja caer, igual que el
			// explorador y el editor con ModCtrl.
			if ev.Modifiers()&tcell.ModCtrl != 0 {
				return false, nil, false
			}
			p.moveCursor(-p.page())
			return true, nil, false
		case tcell.KeyPgDn:
			if ev.Modifiers()&tcell.ModCtrl != 0 {
				return false, nil, false
			}
			p.moveCursor(p.page())
			return true, nil, false
		case tcell.KeyHome:
			p.setCursor(0)
			return true, nil, false
		case tcell.KeyEnd:
			p.setCursor(p.count() - 1)
			return true, nil, false
		case tcell.KeyRune:
			if ev.Rune() == ' ' {
				p.toggleMark()
				return true, nil, false
			}
		case tcell.KeyEnter, tcell.KeyLF:
			return true, p.installIDs(), false
		case tcell.KeyEscape:
			return true, nil, true
		}
	}
	return false, nil, false
}

// extensionRow arma la fila de una extensión: la marca de selección ("[x] " o
// "[ ] "), el nombre, el padding que alinea la versión a la derecha contra la
// pared y "(installed)" si ya está instalada. interior es el ancho interior
// del panel (para el padding).
func extensionRow(e ExtensionEntry, marked bool, interior int) string {
	mark := "[ ] "
	if marked {
		mark = "[x] "
	}
	ver := "v" + e.Version
	if e.Installed {
		ver += " (installed)"
	}
	text := mark + e.Name
	if gap := interior - displayWidth(text) - displayWidth(ver); gap > 0 {
		text += strings.Repeat(" ", gap)
	}
	return text + ver
}

// finalRow arma el item final: "Install N selected" con la cantidad de
// marcadas, o "Install (under cursor)" si no hay ninguna.
func (p *ExtensionsPanel) finalRow() string {
	if len(p.marked) == 0 {
		return "Install (under cursor)"
	}
	return fmt.Sprintf("Install %d selected", len(p.marked))
}

// Draw pinta el panel en coordenadas propias desde (0,0): un marco con el
// título "Extensions" centrado sobre el borde superior y, en el interior, la
// lista de extensiones con su marca, nombre y versión (más "(installed)" si
// ya está), el item final de instalación alineado como las demás filas y la
// fila del cursor con la barra de selección a TODO el ancho interior. Mientras
// consulta el catálogo muestra "Consultando el catálogo…" (o "Sin conexión" si
// hubo error); sin entradas y sin carga, "Sin extensiones disponibles". Sin
// alto (o sin ancho) no hay nada que dibujar; una fila fuera del alto se corta.
func (p *ExtensionsPanel) Draw(s Surface) {
	if p.width < 2 || p.height < 2 {
		return
	}
	th := themeOr(p.theme)

	// Marco: bordes superior e inferior y las paredes laterales.
	s.SetContent(0, 0, '┌', nil, th.Text)
	for x := 1; x < p.width-1; x++ {
		s.SetContent(x, 0, '─', nil, th.Text)
	}
	s.SetContent(p.width-1, 0, '┐', nil, th.Text)
	s.SetContent(0, p.height-1, '└', nil, th.Text)
	for x := 1; x < p.width-1; x++ {
		s.SetContent(x, p.height-1, '─', nil, th.Text)
	}
	s.SetContent(p.width-1, p.height-1, '┘', nil, th.Text)
	for y := 1; y < p.height-1; y++ {
		s.SetContent(0, y, '│', nil, th.Text)
		s.SetContent(p.width-1, y, '│', nil, th.Text)
	}

	// Título centrado sobre el borde superior; writeString recorta si no entra.
	title := "Extensions"
	if start := (p.width - displayWidth(title)) / 2; start > 0 {
		writeString(s, start, 0, title, th.Text, p.width)
	} else {
		writeString(s, 0, 0, title, th.Text, p.width)
	}

	// Fila única de estado: cargando, error o vacío (sin catálogo consultado).
	if p.loading {
		writeString(s, 1, 1, "Consultando el catálogo…", th.Text, p.width-2)
		return
	}
	if p.loadErr != "" {
		writeString(s, 1, 1, "Sin conexión", th.Text, p.width-2)
		return
	}
	if len(p.items) == 0 {
		writeString(s, 1, 1, "Sin extensiones disponibles", th.Text, p.width-2)
		return
	}

	// Lista: las extensiones y el item final, desde top. La fila del cursor se
	// pinta entera con la barra de selección (TreeCursor) sobre el ancho
	// interior; las demás solo escriben su texto.
	interior := p.width - 2
	for row := 0; row < p.visibleRows(); row++ {
		idx := p.top + row
		if idx >= p.count() {
			break
		}
		y := row + 1
		style := th.Text
		if idx == p.cursor {
			style = th.TreeCursor
			for x := 1; x < p.width-1; x++ {
				s.SetContent(x, y, ' ', nil, style)
			}
		}
		var text string
		if idx < len(p.items) {
			text = extensionRow(p.items[idx], p.marked[p.items[idx].ID], interior)
		} else {
			text = p.finalRow()
		}
		writeString(s, 1, y, text, style, interior)
	}
}
