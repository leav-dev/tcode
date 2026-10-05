package view

import (
	"strings"
	"unsafe"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
	"tcode/internal/model"
)

// tabWidth es la cantidad de columnas a la que se expande una tabulación.
const tabWidth = 4

// indentUnit es la unidad estándar de indentación del editor: la que inserta
// Tab y la que se suma como nivel extra tras abrir un bloque. Es una variable
// (la futura configuración del editor la podrá exponer); por defecto, "el
// tamaño del tab" del proyecto: 4 espacios.
var indentUnit = "    "

// Viewport controla qué región del documento se proyecta en la pantalla.
type Viewport struct {
	TopLine    int // Primera línea lógica visible
	LeftColumn int // Primera columna lógica visible (scroll horizontal)
	Height     int // Alto en filas de la terminal
	Width      int // Ancho del ÁREA DE TEXTO en columnas (sin el gutter)
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

	// theme es la paleta por rol del editor; el valor cero usa la default.
	// El controller la inyecta con SetTheme tras cargar ~/.tcode/theme.json.
	theme Theme

	// diagnostics anota las líneas del buffer (el proveedor es el backend de
	// scripting): el draw pinta el marcador de severidad en el gutter, el
	// subrayado de la línea anotada y el mensaje inline a la derecha.
	diagnostics []Diagnostic

	// diagBySource separa las anotaciones por proveedor (cada script de
	// extensión es un proveedor): SetDiagnostics(source, ...) reemplaza solo
	// lo propio, sin pisar a los demás; la lista mergeada se recomputa en
	// cada set y es la que renderiza el draw.
	diagBySource map[string][]Diagnostic

	// visCache es la última (línea lógica, fila visual global) computada por
	// visualRowOfLine: el movimiento secuencial del cursor evita re-sumar las
	// filas visuales desde el inicio en cada redibujo.
	visCache struct {
		line int
		row  int
	}

	// sel es el rango marcado ([Start, End) en offsets de documento) y
	// selAnchor el ancla de la extensión con Shift: el cursor se mueve y el
	// rango va del ancla al cursor. Cualquier movimiento sin Shift limpia.
	sel       Selection
	selAnchor *int

	// mouseDown y mouseAnchor sostienen el arrastre del mouse: presionar el
	// botón 1 ancla (sin seleccionar todavía); el movimiento con el botón
	// extiende desde el ancla; soltar termina y deja el rango.
	mouseDown   bool
	mouseAnchor *int
}

// visualRowOfLine devuelve la fila visual GLOBAL donde empieza la línea lógica
// line: la suma de las filas visuales de todas las líneas previas (con wrap
// activo, una línea envuelta ocupa una fila por corte). O(line) por llamada; un
// caché del último resultado hace el movimiento secuencial del cursor O(1).
func (v *EditorView) visualRowOfLine(line int) int {
	if v.visCache.line <= line {
		// caminar desde la última línea cacheada
		for last := v.visCache.line; last < line; last++ {
			if wordWrapEnabled {
				v.visCache.row += softLineCount(string(v.model.LineContent(last)), v.viewport.Width)
			} else {
				v.visCache.row++
			}
		}
	} else {
		// hacia atrás: recomputar desde 0 (raro: solo con saltos grandes)
		v.visCache.row = 0
		for i := 0; i < line && i < v.model.LineCount(); i++ {
			if wordWrapEnabled {
				v.visCache.row += softLineCount(string(v.model.LineContent(i)), v.viewport.Width)
			} else {
				v.visCache.row++
			}
		}
	}
	v.visCache.line = line
	return v.visCache.row
}

// cursorVisual devuelve la (fila visual global, columna visible) del cursor.
func (v *EditorView) cursorVisual() (row, col int) {
	row = v.visualRowOfLine(v.cursor.Line)
	content := v.model.LineContent(v.cursor.Line)
	if wordWrapEnabled && v.viewport.Width > 0 {
		r, c := softLineAt(string(content), v.viewport.Width, v.cursor.ByteCol)
		return row + r, c
	}
	return row, columnAt(content, v.cursor.ByteCol)
}

// logicalAtVisualRow traduce una fila física (relativa al TopLine) a la línea
// lógica y a la fila visual dentro de ella.
func (v *EditorView) logicalAtVisualRow(y int) (line, rowInLine int) {
	rest := y
	for l := v.viewport.TopLine; l < v.model.LineCount(); l++ {
		n := 1
		if wordWrapEnabled && v.viewport.Width > 0 {
			n = softLineCount(string(v.model.LineContent(l)), v.viewport.Width)
		}
		if rest < n {
			return l, rest
		}
		rest -= n
	}
	if v.model.LineCount() == 0 {
		return 0, 0
	}
	return v.model.LineCount() - 1, 0
}

// SetTheme reemplaza la paleta del editor.
func (v *EditorView) SetTheme(th Theme) { v.theme = th }

// themeOrDefault devuelve la paleta activa: la inyectada o la default.
func (v *EditorView) themeOrDefault() Theme {
	if v.theme == (Theme{}) {
		return DefaultTheme()
	}
	return v.theme
}

func NewEditorView(m *model.PieceTable, height, width int) *EditorView {
	v := &EditorView{
		model: m,
		viewport: Viewport{
			Height: height,
			Width:  width,
		},
	}
	// width es el ancho total del widget; el área de texto pierde el gutter.
	v.viewport.Width = v.textWidth(width)
	return v
}

// textWidth descuenta el gutter de un ancho total de widget, con un mínimo de
// 1 columna de texto: un widget demasiado angosto nunca deja 0 columnas.
func (v *EditorView) textWidth(width int) int {
	if w := width - v.gutterWidth(); w >= 1 {
		return w
	}
	return 1
}

// Resize actualiza las dimensiones del viewport cuando cambia la terminal. El
// ancho recibido es el TOTAL del widget: el área de texto (lo que ve el wrap
// y el clip) descuenta el gutter.
func (v *EditorView) Resize(width, height int) {
	if width > 0 {
		v.viewport.Width = v.textWidth(width)
	}
	if height > 0 {
		v.viewport.Height = height
	}
	v.clamp()
	v.ensureCursorVisible()
}

// Size devuelve el ancho y el alto del ÁREA DE TEXTO (sin el gutter), en
// celdas de terminal. Es la geometría con la que la vista envuelve y recorta
// el texto ahora mismo; el widget total mide Size() + gutterWidth().
func (v *EditorView) Size() (int, int) {
	return v.viewport.Width, v.viewport.Height
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

// CursorOffset devuelve el offset de documento del cursor. Lo usa el backend
// de scripting (ScriptAPI.InsertAtCursor) para insertar texto en la posición
// exacta del cursor activo.
func (v *EditorView) CursorOffset() int {
	return v.model.LineStart(v.cursor.Line) + v.cursor.ByteCol
}

// ClampCursor recorta el cursor y el viewport al documento después de una
// recarga: el archivo pudo quedarse más corto y un cursor fuera de rango
// paniquearía en el próximo dibujo (LineContent fuera). Un documento vacío cae
// al origen sin leer líneas; si el cursor sigue existiendo, se recorta al
// final de su línea y el viewport lo acompaña.
func (v *EditorView) ClampCursor() {
	if v.lineCount() == 0 {
		v.cursor.Line = 0
		v.cursor.ByteCol = 0
		v.cursor.desiredCol = 0
		v.viewport.TopLine = 0
		v.viewport.LeftColumn = 0
		return
	}
	// Si la línea dejó de existir, se conserva la columna y el paso siguiente la
	// recorta al final de la nueva línea: el cursor no vuelve al inicio salvo
	// que el documento entero sea más corto que su línea.
	if v.cursor.Line >= v.lineCount() {
		v.cursor.Line = v.lineCount() - 1
	}
	content := v.model.LineContent(v.cursor.Line)
	if v.cursor.ByteCol > len(content) {
		v.cursor.ByteCol = len(content)
	}
	v.cursor.desiredCol = columnAt(content, v.cursor.ByteCol)
	v.ensureCursorVisible()
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

// breakTypingGroup corta el grupo de tipeo acumulado. Mover el cursor separa lo
// que se escribió antes de lo que se escriba después: son dos pasos de deshacer.
func (v *EditorView) breakTypingGroup() { v.model.BreakTypingGroup() }

// moveCursorToCell mueve el cursor a la celda de pantalla indicada. Es el hit
// testing del mouse: convierte (x, y) en (línea, byte) con el ancho real.
func (v *EditorView) moveCursorToCell(x, y int) bool {
	// El clic sobre el gutter (números y marcadores) no selecciona: no toca ni
	// el cursor ni el grupo de tipeo.
	if x < v.gutterWidth() {
		return false
	}

	v.breakTypingGroup()
	// Con wrap, la fila física puede ser un corte de una línea envuelta: se
	// traduce a (línea lógica, fila dentro de la línea) y de ahí al byte.
	line, rowInLine := v.logicalAtVisualRow(y)
	if lines := v.lineCount(); lines == 0 || line < 0 || line >= lines {
		return false
	}

	content := v.model.LineContent(line)
	// x viene en coordenadas de pantalla: la columna del texto resta el gutter.
	colVis := x - v.gutterWidth() + v.viewport.LeftColumn
	var col int
	if wordWrapEnabled && v.viewport.Width > 0 {
		col = softLineToByte(string(content), v.viewport.Width, rowInLine, colVis)
	} else {
		col = offsetAtColumn(content, colVis)
	}
	if v.cursor.Line == line && v.cursor.ByteCol == col {
		return false
	}

	v.cursor.Line = line
	v.cursor.ByteCol = col
	v.cursor.desiredCol = columnAt(content, col)
	return true
}

// deleteSelection borra el rango marcado y deja el cursor en su inicio.
func (v *EditorView) deleteSelection() bool {
	if !v.SelectionActive() {
		return false
	}
	v.breakTypingGroup()
	if _, err := v.model.Delete(v.sel.Start, v.sel.End); err != nil {
		return false
	}
	v.setCursorAt(v.sel.Start)
	v.clearSelection()
	return true
}

// moveHorizontal avanza o retrocede un grapheme cluster, cruzando de línea en los
// extremos.
func (v *EditorView) moveHorizontal(delta int) bool {
	v.breakTypingGroup()
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
	v.breakTypingGroup()
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
	v.breakTypingGroup()
	if v.cursor.ByteCol == 0 {
		return false
	}
	v.cursor.ByteCol = 0
	v.cursor.desiredCol = 0
	return true
}

func (v *EditorView) moveLineEnd() bool {
	v.breakTypingGroup()
	content := v.model.LineContent(v.cursor.Line)
	if v.cursor.ByteCol == len(content) {
		return false
	}
	v.cursor.ByteCol = len(content)
	v.cursor.desiredCol = columnAt(content, len(content))
	return true
}

func (v *EditorView) moveDocStart() bool {
	v.breakTypingGroup()
	if v.cursor.Line == 0 && v.cursor.ByteCol == 0 {
		return false
	}
	v.cursor.Line = 0
	v.cursor.ByteCol = 0
	v.cursor.desiredCol = 0
	return true
}

func (v *EditorView) moveDocEnd() bool {
	v.breakTypingGroup()
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

// MoveCursorToOffset coloca el cursor en un offset de documento. Es lo que
// necesitan deshacer y rehacer para dejar el cursor donde ocurrió el cambio.
func (v *EditorView) MoveCursorToOffset(offset int) {
	v.setCursorAt(offset)
	v.ensureCursorVisible()
}

// --- edición ---

// insertText inserta s en la posición del cursor y lo deja después del texto.
// autoIndent calcula la indentación de la línea nueva tras Enter: hereda el
// prefijo de whitespace de la línea de origen (donde está el cursor) y, si esa
// línea termina —ignorando el whitespace de cola— en {, [ o :, suma indentUnit
// (un nivel del estándar del editor). Es una regla mecánica, sin análisis
// sintáctico: un ':' en un slice o un '{' en una cadena también indenta; se
// corrige con Backspace y queda anotado como límite de esta versión.
func (v *EditorView) autoIndent() string {
	content := v.model.LineContent(v.cursor.Line)
	i := 0
	for i < len(content) && (content[i] == ' ' || content[i] == '\t') {
		i++
	}
	prefix := string(content[:i])
	body := strings.TrimRight(string(content[i:]), " \t")
	if body == "" {
		return prefix // línea de origen sin contenido: solo se hereda el prefijo
	}
	if last := body[len(body)-1]; last == '{' || last == '[' || last == ':' {
		return prefix + indentUnit
	}
	return prefix
}

func (v *EditorView) insertText(s string) bool {
	if s == "" {
		return false
	}
	if v.SelectionActive() {
		// Escribir reemplaza la selección (VSCode-like) y corta el grupo de
		// tipeo (borrar+rango es una edición distinta): borra el rango e
		// inserta desde su inicio.
		v.breakTypingGroup()
		if _, err := v.model.Delete(v.sel.Start, v.sel.End); err != nil {
			return false
		}
		v.setCursorAt(v.sel.Start)
		v.clearSelection()
	}
	off := v.cursorOffset()
	if err := v.model.Insert(off, s); err != nil {
		return false
	}
	v.setCursorAt(off + len(s))
	return true
}

// backspace borra el grapheme cluster anterior al cursor. Al inicio de una línea
// borra el salto anterior, que es lo que fusiona las dos líneas.
func (v *EditorView) backspace() bool {
	if v.SelectionActive() {
		return v.deleteSelection()
	}
	line := v.cursor.Line

	if v.cursor.ByteCol > 0 {
		start := v.model.LineStart(line)
		content := v.model.LineContent(line)
		from := start + prevCluster(content, v.cursor.ByteCol)
		if _, err := v.model.Delete(from, start+v.cursor.ByteCol); err != nil {
			return false
		}
		v.setCursorAt(from)
		return true
	}

	if line == 0 {
		return false
	}

	// Inicio de línea: se borra el salto completo de la línea anterior.
	breakLen := v.model.LineBreakLen(line - 1)
	if breakLen == 0 {
		return false
	}
	at := v.model.LineStart(line)
	if _, err := v.model.Delete(at-breakLen, at); err != nil {
		return false
	}
	v.setCursorAt(at - breakLen)
	return true
}

// deleteForward borra el grapheme cluster que está en el cursor. Al final de una
// línea borra el salto, fusionandola con la siguiente.
func (v *EditorView) deleteForward() bool {
	if v.SelectionActive() {
		return v.deleteSelection()
	}
	line := v.cursor.Line
	start := v.model.LineStart(line)
	content := v.model.LineContent(line)

	if v.cursor.ByteCol < len(content) {
		from := start + v.cursor.ByteCol
		to := start + nextCluster(content, v.cursor.ByteCol)
		if _, err := v.model.Delete(from, to); err != nil {
			return false
		}
		v.setCursorAt(from)
		return true
	}

	breakLen := v.model.LineBreakLen(line)
	if breakLen == 0 {
		return false
	}
	at := start + len(content)
	if _, err := v.model.Delete(at, at+breakLen); err != nil {
		return false
	}
	v.setCursorAt(at)
	return true
}

// --- Draw ---

// Draw renderiza solo las líneas visibles avanzando por *grapheme cluster* con su
// ancho monoespaciado real.
//
// La columna lógica se cuenta en celdas de terminal, no en runas: un cluster
// puede ocupar 0 celdas (combinante huérfano), 1 (ASCII), 2 (CJK, emoji) o más.
// Contar runas desalinea las columnas y rompe el hit testing del mouse.
func (v *EditorView) Draw(s Surface) {
	// El resaltado de sintaxis: por extensión del buffer y por línea visible.
	hl := newHighlighter(v.model.Path())
	th := v.themeOrDefault()
	cursorLine := v.cursor.Line

	// El fondo del documento es el del tema (Text lleva su propio fondo): se
	// limpia el viewport completo para que la terminal no asome en las celdas
	// sin texto. Mismo costo que ya paga el explorador por su panel.
	if v.viewport.Width > 0 && v.viewport.Height > 0 {
		for y := 0; y < v.viewport.Height; y++ {
			for x := 0; x < v.viewport.Width; x++ {
				s.SetContent(x, y, ' ', nil, th.Text)
			}
		}
	}

	row := 0 // fila física en pantalla
	selStart, selEnd, selActive := 0, 0, false
	if v.SelectionActive() {
		selStart, selEnd, selActive = v.sel.Start, v.sel.End, true
	}
	for line := v.viewport.TopLine; line < v.model.LineCount() && row < v.viewport.Height; line++ {
		// COPIA obligatoria, no vista de mmap: tcell retiene el string que le
		// pasamos en su buffer de celdas (currStr/lastStr) hasta el próximo
		// redibujo, y cerrar la pestaña desmapea el archivo mientras el buffer
		// sigue apuntando a él —ese uso-después-de-desmapear segfaulta en el
		// primer redraw posterior (acceso a memoria liberada en Dirty). La copia
		// cubre solo la región visible (el viewport), el costo correcto si el
		// contenido va a llegar a pantalla.
		content := v.model.LineContent(line)
		text := string(content)
		isCursorLine := line == cursorLine
		lineStart := v.model.LineStart(line)

		if wordWrapEnabled && v.viewport.Width > 0 {
			// Las filas visuales de la línea: cada una en su propia fila física;
			// solo la primera fila lleva el número de línea en el gutter.
			for i, sl := range softLines(text, v.viewport.Width) {
				if row >= v.viewport.Height {
					break
				}
				v.drawSoftLine(s, th, hl, line, text, content, sl, row, isCursorLine, i == 0, lineStart, selStart, selEnd, selActive)
				row++
			}
			continue
		}

		v.drawLineUnwrapped(s, th, hl, line, text, content, row, isCursorLine, lineStart, selStart, selEnd, selActive)
		row++
	}

	v.drawCursor(s)
}

// drawSoftLine pinta una fila visual (un corte de la línea lógica) en la fila
// física row. text es la copia de la línea (los clusters son substrings suyos,
// seguros para tcell); content es la vista del modelo solo para calcular roles
// (nunca se retiene).
func (v *EditorView) drawSoftLine(s Surface, th Theme, hl *highlighter, line int, text string, content []byte, sl softLine, row int, cursorLine bool, firstRow bool, lineStart, selStart, selEnd int, selActive bool) {
	gutter := v.gutterWidth()
	v.drawGutter(s, th, line, row, firstRow)
	_, hasDiag := v.diagAt(line)
	col := 0
	g := uniseg.NewGraphemes(sl.text)
	for g.Next() {
		cl := g.Str()
		from, _ := g.Positions()
		if cl == "\r" {
			col = 0 // retorno de carro aislado: vuelve al inicio de la misma fila
			continue
		}
		w := wrapClusterWidth(cl, col)
		if w <= 0 {
			continue // combinante huérfano: sin celda propia
		}
		x := col - v.viewport.LeftColumn + gutter
		// Solo se dibuja un cluster que entre completo EN EL ÁREA DE TEXTO (a
		// la derecha del gutter); las tabulaciones no se dibujan (el fondo ya
		// viene pintado).
		if x >= gutter && x+w <= v.viewport.Width+gutter && cl != "\t" {
			st := th.StyleForRole(hl.styleAt(content, sl.in+from))
			goff := lineStart + sl.in + from // offset global del cluster
			switch {
			case selActive && goff >= selStart && goff < selEnd:
				st = th.Selection
			case cursorLine:
				st = st.Background(th.CursorLineBg) // la línea del cursor: toda su fila
			}
			if hasDiag {
				st = st.Underline(true) // la línea anotada: subrayada
			}
			s.Put(x, row, cl, st)
		}
		col += w
	}

	// El mensaje del diagnóstico se pinta a la derecha de la línea lógica:
	// solo en su ÚLTIMA fila visual (con wrap, el texto termina ahí).
	if sl.in+len(sl.text) == len(text) {
		v.drawInlineDiag(s, th, line, row, col-v.viewport.LeftColumn+v.gutterWidth(), cursorLine)
	}
}

// drawLineUnwrapped pinta la línea lógica completa en una fila física (sin
// wrap): el cluster que no entra se corta contra el borde, como siempre.
func (v *EditorView) drawLineUnwrapped(s Surface, th Theme, hl *highlighter, line int, text string, content []byte, row int, cursorLine bool, lineStart, selStart, selEnd int, selActive bool) {
	gutter := v.gutterWidth()
	v.drawGutter(s, th, line, row, true)
	_, hasDiag := v.diagAt(line)
	col := 0
	g := uniseg.NewGraphemes(text)
	for g.Next() {
		cl := g.Str()
		from, _ := g.Positions()
		if cl == "\r" {
			col = 0 // retorno de carro aislado: vuelve al inicio de la misma fila
			continue
		}
		w := clusterWidth(g, col)
		if w <= 0 {
			continue
		}
		x := col - v.viewport.LeftColumn + gutter
		// Solo se dibuja un cluster que entre completo EN EL ÁREA DE TEXTO (a
		// la derecha del gutter); las tabulaciones no se dibujan (el fondo ya
		// viene pintado).
		if x >= gutter && x+w <= v.viewport.Width+gutter && cl != "\t" {
			st := th.StyleForRole(hl.styleAt(content, from))
			goff := lineStart + from // offset global del cluster
			switch {
			case selActive && goff >= selStart && goff < selEnd:
				st = th.Selection
			case cursorLine:
				st = st.Background(th.CursorLineBg)
			}
			if hasDiag {
				st = st.Underline(true) // la línea anotada: subrayada
			}
			s.Put(x, row, cl, st)
		}
		col += w
	}

	// El mensaje del diagnóstico, a la derecha del texto de la línea.
	v.drawInlineDiag(s, th, line, row, col-v.viewport.LeftColumn+v.gutterWidth(), cursorLine)
}

// drawCursor ubica el cursor del terminal en la celda que le corresponde.
func (v *EditorView) drawCursor(s Surface) {
	// Con wrap, el cursor vive en la fila visual de su línea: se proyecta a la
	// fila física restando las filas visuales del TopLine.
	cr, ccol := v.cursorVisual()
	row := cr - v.visualRowOfLine(v.viewport.TopLine)
	if row < 0 || row >= v.viewport.Height {
		s.HideCursor()
		return
	}

	col := ccol - v.viewport.LeftColumn
	if col < 0 || col >= v.viewport.Width {
		s.HideCursor()
		return
	}

	// La columna de pantalla del cursor suma el gutter: el texto vive después
	// de la columna de números.
	s.ShowCursor(col+v.gutterWidth(), row)
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
	shift := ev.Modifiers()&tcell.ModShift != 0

	switch ev.Key() {
	case tcell.KeyUp:
		return v.moveWithShift(shift, func() bool { return v.moveVertical(-1) })
	case tcell.KeyDown:
		return v.moveWithShift(shift, func() bool { return v.moveVertical(1) })
	case tcell.KeyLeft:
		return v.moveWithShift(shift, func() bool { return v.moveHorizontal(-1) })
	case tcell.KeyRight:
		return v.moveWithShift(shift, func() bool { return v.moveHorizontal(1) })
	case tcell.KeyPgUp:
		// Ctrl+PageUp/PageDown cambian de pestaña y son del controlador, no
		// scroll de página: la vista los ignora con ModCtrl para que ninguna
		// variante de terminal escrolle por accidente mientras se cambia de
		// pestaña.
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return false
		}
		return v.moveWithShift(shift, func() bool { return v.moveVertical(-page) })
	case tcell.KeyPgDn:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return false
		}
		return v.moveWithShift(shift, func() bool { return v.moveVertical(page) })
	case tcell.KeyHome:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return v.moveWithShift(shift, v.moveDocStart)
		}
		return v.moveWithShift(shift, v.moveLineStart)
	case tcell.KeyEnd:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return v.moveWithShift(shift, v.moveDocEnd)
		}
		return v.moveWithShift(shift, v.moveLineEnd)
	case tcell.KeyCtrlA:
		return v.selectAll()
	case tcell.KeyCtrlV:
		return v.PasteClipboard()

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return v.backspace()
	case tcell.KeyDelete:
		return v.deleteForward()
	case tcell.KeyEnter, tcell.KeyLF:
		// KeyEnter es el camino normal (CR). Algunos terminales y modos de línea
		// mandan LF, así que se acepta también para no perder el salto de línea.
		// La línea nueva hereda la indentación de la línea de origen (autoIndent).
		return v.insertText("\n" + v.autoIndent())
	case tcell.KeyTab:
		// El tab del editor es la unidad de indentación estándar (4 espacios por
		// defecto), no un tab crudo: el nivel extra y la tecla coinciden.
		return v.insertText(indentUnit)
	}

	// Texto: solo runas sin modificadores. Ctrl y Alt quedan libres para atajos,
	// así que una combinación nunca inserta por accidente.
	if ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0 {
		if r := ev.Rune(); r != 0 {
			return v.insertText(string(r))
		}
	}
	return false
}

// handleMouse traduce el mouse del editor: la rueda scrollea sin arrastrar el
// cursor, el clic mueve el cursor y limpia la selección, el arrastre con el
// Button1 presionado extiende una selección VIVA desde el ancla del press, y
// Shift+clic extiende desde el ancla previo (VSCode-like). La columna del
// gutter es de solo lectura visual: el clic O el arrastre que arranca ahí no
// seleccionan (se ignoran, como el clic que ya no movía el cursor).
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
		// El clic/arrastre sobre el gutter (números y marcadores de severidad)
		// no arma ni extiende selección: se ignora entero.
		if x < v.gutterWidth() {
			return false
		}
		if v.mouseDown {
			// tcell no distingue el movimiento con botón: un Button1 repetido
			// con el flag de arrastre ya activo es un drag (extiende desde el
			// ancla del press); el primero es el press que ancla.
			if v.mouseAnchor == nil {
				return false
			}
			v.selAnchor = v.mouseAnchor
			changed := v.moveCursorToCell(x, y)
			v.updateSelection()
			return changed || v.SelectionActive()
		}
		// Press (sin arrastre previo): mueve el cursor y ancla el punto para
		// un posible drag. Sin Shift limpia la selección (clic simple); con
		// Shift extiende desde el ancla (o la arranca acá) — VSCode-like, el
		// mismo patrón del teclado (selAnchor + updateSelection).
		shift := ev.Modifiers()&tcell.ModShift != 0
		if shift {
			v.beginExtend()
		}
		changed := v.moveCursorToCell(x, y)
		v.mouseDown = true
		a := v.cursorOffset()
		v.mouseAnchor = &a
		if shift {
			v.updateSelection()
			return changed || v.SelectionActive()
		}
		v.clearSelection()
		return changed
	}
	// Release (ButtonNone) o cualquier otro estado sin el botón: fin del
	// arrastre; la selección queda marcada como quedó.
	v.mouseDown = false
	v.mouseAnchor = nil
	return false
}

// --- viewport ---

// ensureCursorVisible desplaza el viewport para que el cursor quede dentro de la
// pantalla. Verticalmente el scroll es CENTRADO (la línea del cursor tiende al
// medio del viewport), con una red de seguridad por filas visuales para no
// perder el cursor cuando el wrap desajusta las filas.
func (v *EditorView) ensureCursorVisible() {
	cr, ccol := v.cursorVisual()

	// Vertical: centrado. La línea del cursor tiende al centro del viewport,
	// SOLO si el documento tiene suficiente contenido arriba y abajo; si no, el
	// clamp la deja pegada al borde (1ª línea → top 0; última → top máximo;
	// documento más corto que la pantalla → top 0).
	h := v.viewport.Height
	if h > 0 && v.lineCount() > h {
		target := v.cursor.Line - h/2
		if target < 0 {
			target = 0
		}
		if maxTop := v.lineCount() - h; target > maxTop {
			target = maxTop
		}
		v.viewport.TopLine = target
	}

	// Red de seguridad por FILAS visuales (wrap): el centrado trabaja por líneas
	// lógicas y una línea envuelta ocupa varias filas, así que la fila del
	// cursor podría quedar fuera del alto. Si pasa, se corre el desplazamiento
	// mínimo viejo SOLO en esa dirección, moviendo TopLine de a líneas lógicas
	// (modelo nano v1) hasta cubrir la fila visual del cursor; nunca se pierde.
	if v.viewport.TopLine > v.cursor.Line {
		v.viewport.TopLine = v.cursor.Line
	}
	topRow := v.visualRowOfLine(v.viewport.TopLine)
	if v.viewport.Height > 0 && cr >= topRow+v.viewport.Height {
		for v.viewport.TopLine < v.cursor.Line &&
			cr >= v.visualRowOfLine(v.viewport.TopLine)+v.viewport.Height {
			v.viewport.TopLine++
		}
		if v.viewport.TopLine > v.cursor.Line {
			v.viewport.TopLine = v.cursor.Line
		}
	}

	v.clamp()

	// Horizontal, en la columna visible del cursor (dentro de su fila visual).
	if ccol < v.viewport.LeftColumn {
		v.viewport.LeftColumn = ccol
	}
	if v.viewport.Width > 0 && ccol >= v.viewport.LeftColumn+v.viewport.Width {
		v.viewport.LeftColumn = ccol - v.viewport.Width + 1
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
