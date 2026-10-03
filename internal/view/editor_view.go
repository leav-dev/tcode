package view

import (
	"unsafe"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
	"tcode/internal/model"
)

// tabWidth es la cantidad de columnas a la que se expande una tabulación.
const tabWidth = 4

// Viewport controla qué región del documento se proyecta en la pantalla.
type Viewport struct {
	TopLine    int // Primera línea lógica visible
	LeftColumn int // Primera columna lógica visible (scroll horizontal)
	Height     int // Alto en filas de la terminal
	Width      int // Ancho en columnas de la terminal
}

// Cursor es la posición de edición en coordenadas lógicas.
//
// Se guarda como (línea, byte dentro de la línea) y no como offset de documento
// porque el movimiento horizontal tiene que ser consciente de los *grapheme
// clusters* y de los caracteres anchos. La conversión es
// LineStart(Line) + ByteCol.
type Cursor struct {
	Line    int
	ByteCol int

	// desiredCol es la columna de pantalla que el usuario busca al moverse
	// verticalmente. Se conserva entre líneas para no "comerse" columnas cuando
	// el cursor pasa por líneas más cortas, y se recalcula en cuanto hay un
	// movimiento horizontal o una edición.
	desiredCol int
}

// EditorView es la Screen encargada de renderizar el documento.
type EditorView struct {
	model    *model.PieceTable
	viewport Viewport
	cursor   Cursor
}

func NewEditorView(m *model.PieceTable, height, width int) *EditorView {
	return &EditorView{
		model: m,
		viewport: Viewport{
			Height: height,
			Width:  width,
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
	v.ensureCursorVisible()
}

// --- helpers de grapheme cluster ---

// graphemes itera los clusters de b sin copiarlos: la vista de string comparte la
// memoria de b, que es de solo lectura para uniseg.
func graphemes(b []byte) *uniseg.Graphemes {
	if len(b) == 0 {
		return uniseg.NewGraphemes("")
	}
	return uniseg.NewGraphemes(unsafe.String(&b[0], len(b)))
}

// clusterWidth devuelve el ancho en celdas del cluster actual, resolviendo las
// tabulaciones contra la columna en la que caen.
func clusterWidth(g *uniseg.Graphemes, col int) int {
	if g.Str() == "\t" {
		return tabWidth - col%tabWidth
	}
	if w := g.Width(); w > 0 {
		return w
	}
	return 0
}

// columnAt devuelve la columna de pantalla del offset de byte indicado dentro de b.
func columnAt(b []byte, byteOffset int) int {
	col := 0
	g := graphemes(b)
	for g.Next() {
		from, _ := g.Positions()
		if from >= byteOffset {
			break
		}
		col += clusterWidth(g, col)
	}
	return col
}

// offsetAtColumn es la inversa de columnAt: devuelve el offset de byte del cluster
// que contiene la columna col. Si col cae en la mitad de un carácter ancho,
// devuelve el inicio de ese carácter, que es lo que la gente espera al clickear.
// Es la primitiva del hit testing del mouse.
func offsetAtColumn(b []byte, col int) int {
	if col <= 0 {
		return 0
	}

	at := 0
	g := graphemes(b)
	for g.Next() {
		from, _ := g.Positions()
		width := clusterWidth(g, at)
		if width == 0 {
			continue
		}
		if col < at+width {
			return from
		}
		at += width
	}
	return len(b)
}

// nextCluster devuelve el offset del cluster siguiente a offset.
func nextCluster(b []byte, offset int) int {
	if offset >= len(b) {
		return len(b)
	}
	g := graphemes(b)
	for g.Next() {
		from, to := g.Positions()
		if from == offset {
			return to
		}
		if from > offset {
			return from
		}
	}
	return len(b)
}

// prevCluster devuelve el offset del cluster anterior a offset.
func prevCluster(b []byte, offset int) int {
	if offset <= 0 {
		return 0
	}
	g := graphemes(b)
	for g.Next() {
		from, to := g.Positions()
		if to >= offset {
			return from
		}
	}
	return 0
}

// --- cursor ---

func (v *EditorView) lineCount() int { return v.model.LineCount() }

// cursorOffset traduce el cursor a un offset de documento.
func (v *EditorView) cursorOffset() int {
	return v.model.LineStart(v.cursor.Line) + v.cursor.ByteCol
}

// setCursorAt coloca el cursor en un offset de documento, resolviendo la línea.
func (v *EditorView) setCursorAt(docOffset int) {
	line := v.model.LineAt(docOffset)
	content := v.model.LineContent(line)

	col := docOffset - v.model.LineStart(line)
	if col < 0 {
		col = 0
	}
	if col > len(content) {
		col = len(content)
	}

	v.cursor.Line = line
	v.cursor.ByteCol = col
	v.cursor.desiredCol = columnAt(content, col)
}

// moveCursorToCell mueve el cursor a la celda de pantalla indicada. Es el hit
// testing del mouse: convierte (x, y) en (línea, byte) con el ancho real.
func (v *EditorView) moveCursorToCell(x, y int) bool {
	line := v.viewport.TopLine + y
	if lines := v.lineCount(); lines == 0 || line < 0 || line >= lines {
		return false
	}

	content := v.model.LineContent(line)
	col := offsetAtColumn(content, x+v.viewport.LeftColumn)
	if v.cursor.Line == line && v.cursor.ByteCol == col {
		return false
	}

	v.cursor.Line = line
	v.cursor.ByteCol = col
	v.cursor.desiredCol = columnAt(content, col)
	return true
}

// moveHorizontal avanza o retrocede un grapheme cluster, cruzando de línea en los
// extremos.
func (v *EditorView) moveHorizontal(delta int) bool {
	if delta == 0 {
		return false
	}
	content := v.model.LineContent(v.cursor.Line)

	if delta > 0 {
		switch {
		case v.cursor.ByteCol < len(content):
			v.cursor.ByteCol = nextCluster(content, v.cursor.ByteCol)
		case v.cursor.Line+1 < v.lineCount():
			v.cursor.Line++
			v.cursor.ByteCol = 0
		default:
			return false
		}
	} else {
		switch {
		case v.cursor.ByteCol > 0:
			v.cursor.ByteCol = prevCluster(content, v.cursor.ByteCol)
		case v.cursor.Line > 0:
			v.cursor.Line--
			v.cursor.ByteCol = len(v.model.LineContent(v.cursor.Line))
		default:
			return false
		}
	}

	v.cursor.desiredCol = columnAt(v.model.LineContent(v.cursor.Line), v.cursor.ByteCol)
	return true
}

// moveVertical mueve el cursor de línea conservando la columna deseada, que es lo
// que evita que el cursor se pegue al final de las líneas cortas.
func (v *EditorView) moveVertical(delta int) bool {
	lines := v.lineCount()
	if lines == 0 || delta == 0 {
		return false
	}

	target := v.cursor.Line + delta
	if target < 0 {
		target = 0
	}
	if target >= lines {
		target = lines - 1
	}
	if target == v.cursor.Line {
		return false
	}

	content := v.model.LineContent(target)
	v.cursor.Line = target
	v.cursor.ByteCol = offsetAtColumn(content, v.cursor.desiredCol)
	return true
}

func (v *EditorView) moveLineStart() bool {
	if v.cursor.ByteCol == 0 {
		return false
	}
	v.cursor.ByteCol = 0
	v.cursor.desiredCol = 0
	return true
}

func (v *EditorView) moveLineEnd() bool {
	content := v.model.LineContent(v.cursor.Line)
	if v.cursor.ByteCol == len(content) {
		return false
	}
	v.cursor.ByteCol = len(content)
	v.cursor.desiredCol = columnAt(content, len(content))
	return true
}

func (v *EditorView) moveDocStart() bool {
	if v.cursor.Line == 0 && v.cursor.ByteCol == 0 {
		return false
	}
	v.cursor.Line = 0
	v.cursor.ByteCol = 0
	v.cursor.desiredCol = 0
	return true
}

func (v *EditorView) moveDocEnd() bool {
	lines := v.lineCount()
	if lines == 0 {
		return false
	}
	last := lines - 1
	content := v.model.LineContent(last)
	if v.cursor.Line == last && v.cursor.ByteCol == len(content) {
		return false
	}
	v.cursor.Line = last
	v.cursor.ByteCol = len(content)
	v.cursor.desiredCol = columnAt(content, len(content))
	return true
}

// --- Draw ---

// Draw renderiza solo las líneas visibles avanzando por *grapheme cluster* con su
// ancho monoespaciado real.
//
// La columna lógica se cuenta en celdas de terminal, no en runas: un cluster
// puede ocupar 0 celdas (combinante huérfano), 1 (ASCII), 2 (CJK, emoji) o más.
// Contar runas desalinea las columnas y rompe el hit testing del mouse.
func (v *EditorView) Draw(s tcell.Screen) {
	content := v.model.GetRange(v.viewport.TopLine, v.viewport.TopLine+v.viewport.Height)
	if len(content) > 0 {
		// Vista de string sin copia sobre los bytes del mmap. Es segura porque el
		// mapeo es de solo lectura y tanto uniseg como tcell únicamente leen a
		// través de ella; ninguna de las dos la retiene más allá de este método.
		text := unsafe.String(&content[0], len(content))

		row := 0 // fila física en pantalla
		col := 0 // columna lógica en celdas de terminal
		g := uniseg.NewGraphemes(text)
		for g.Next() && row < v.viewport.Height {
			cluster := g.Str()

			switch cluster {
			case "\n":
				row++
				col = 0
				continue
			case "\r\n":
				// GB3 de UAX #29 une CR y LF en un solo cluster.
				row++
				col = 0
				continue
			case "\r":
				// Retorno de carro aislado: vuelve al inicio de la misma fila.
				col = 0
				continue
			}

			width := clusterWidth(g, col)
			if width <= 0 {
				// Cluster sin celda propia (combinante huérfano): no hay dónde anclarlo.
				continue
			}

			x := col - v.viewport.LeftColumn
			// Solo se dibuja un cluster que entre completo: escribir uno ancho en la
			// última celda pisaría la celda de continuación que marca tcell.
			// Las tabulaciones no se dibujan; la pantalla ya viene limpia.
			if x >= 0 && x+width <= v.viewport.Width && cluster != "\t" {
				s.Put(x, row, cluster, tcell.StyleDefault)
			}
			col += width
		}
	}

	v.drawCursor(s)
}

// drawCursor ubica el cursor del terminal en la celda que le corresponde.
func (v *EditorView) drawCursor(s tcell.Screen) {
	row := v.cursor.Line - v.viewport.TopLine
	if row < 0 || row >= v.viewport.Height {
		s.HideCursor()
		return
	}

	col := columnAt(v.model.LineContent(v.cursor.Line), v.cursor.ByteCol) - v.viewport.LeftColumn
	if col < 0 || col >= v.viewport.Width {
		s.HideCursor()
		return
	}

	s.ShowCursor(col, row)
}

// --- eventos ---

// HandleEvent procesa teclado y mouse. Devuelve true si algo cambió y la pantalla
// necesita redibujarse.
//
// Las teclas mueven el cursor y el viewport lo acompaña; la rueda del mouse
// scrollea libremente sin arrastrar el cursor, que es el comportamiento esperado.
func (v *EditorView) HandleEvent(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		if !v.handleKey(ev) {
			return false
		}
		v.ensureCursorVisible()
		return true

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
		return v.moveVertical(-1)
	case tcell.KeyDown:
		return v.moveVertical(1)
	case tcell.KeyLeft:
		return v.moveHorizontal(-1)
	case tcell.KeyRight:
		return v.moveHorizontal(1)
	case tcell.KeyPgUp:
		return v.moveVertical(-page)
	case tcell.KeyPgDn:
		return v.moveVertical(page)
	case tcell.KeyHome:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return v.moveDocStart()
		}
		return v.moveLineStart()
	case tcell.KeyEnd:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return v.moveDocEnd()
		}
		return v.moveLineEnd()
	}
	return false
}

// handleMouse solo altera el viewport: la rueda no debe arrastrar el cursor.
// El clic sí lo mueve, porque cae dentro de la pantalla por definición.
func (v *EditorView) handleMouse(ev *tcell.EventMouse) bool {
	btns := ev.Buttons()

	switch {
	case btns&tcell.WheelUp != 0:
		return v.scrollBy(-3)
	case btns&tcell.WheelDown != 0:
		return v.scrollBy(3)
	case btns&tcell.WheelLeft != 0:
		return v.scrollColumnBy(-3)
	case btns&tcell.WheelRight != 0:
		return v.scrollColumnBy(3)
	case btns&tcell.Button1 != 0:
		x, y := ev.Position()
		return v.moveCursorToCell(x, y)
	}
	return false
}

// --- viewport ---

// ensureCursorVisible desplaza el viewport lo mínimo necesario para que el cursor
// quede dentro de la pantalla.
func (v *EditorView) ensureCursorVisible() {
	// Vertical
	if v.cursor.Line < v.viewport.TopLine {
		v.viewport.TopLine = v.cursor.Line
	}
	if bottom := v.viewport.TopLine + v.viewport.Height; v.cursor.Line >= bottom && v.viewport.Height > 0 {
		v.viewport.TopLine = v.cursor.Line - v.viewport.Height + 1
	}
	v.clamp()

	// Horizontal
	col := columnAt(v.model.LineContent(v.cursor.Line), v.cursor.ByteCol)
	if col < v.viewport.LeftColumn {
		v.viewport.LeftColumn = col
	}
	if v.viewport.Width > 0 && col >= v.viewport.LeftColumn+v.viewport.Width {
		v.viewport.LeftColumn = col - v.viewport.Width + 1
	}
	if v.viewport.LeftColumn < 0 {
		v.viewport.LeftColumn = 0
	}
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
