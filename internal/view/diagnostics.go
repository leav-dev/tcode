package view

import (
	"sort"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// Severity es el nivel de un diagnóstico: lo que decide el color del marcador
// del gutter (y, en el hito siguiente, el del subrayado de la línea anotada).
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

// Diagnostic anota una línea del buffer con un mensaje y una severidad. Las
// anotaciones solo decoran el dibujo y alimentan el mensaje de la barra de
// estado; nunca editan el documento.
type Diagnostic struct {
	Line     int // línea lógica (0-indexada)
	Message  string
	Severity Severity
}

// SetDiagnostics reemplaza las anotaciones del buffer. El proveedor es el
// backend de scripting: un analizador deposita acá el diagnóstico de cada
// línea y el editor lo pinta en el próximo redibujo (el draw consulta siempre
// el buffer activo, así que el reemplazo es inmediato).
// Diagnostics devuelve la lista de anotaciones actuales del editor (la misma
// del último SetDiagnostics). Es el acceso de lectura de la integración: el
// backend de scripting escribe con SetDiagnostics y este getter le permite al
// controlador (y a sus tests) verificar lo depositado.
func (v *EditorView) Diagnostics() []Diagnostic {
	return v.diagnostics
}

// SetDiagnostics reemplaza las anotaciones del buffer y las normaliza: se
// ordenan por severidad (Error > Warning > Info, con orden de llegada dentro
// del mismo nivel) para que el marcador del gutter y el mensaje del cursor
// usen SIEMPRE el más grave de la línea y la barra pueda listar el resto.
// Los mensajes son meramente informativos: nunca forman parte del archivo.
func (v *EditorView) SetDiagnostics(d []Diagnostic) {
	sort.SliceStable(d, func(i, j int) bool { return d[i].Severity > d[j].Severity })
	v.diagnostics = d
}

// diagAt devuelve el diagnóstico MÁS GRAVE de la línea (el primero de la lista,
// ya ordenada por severidad): es el que marca el gutter y subraya la línea.
func (v *EditorView) diagAt(line int) (Diagnostic, bool) {
	for _, d := range v.diagnostics {
		if d.Line == line {
			return d, true
		}
	}
	return Diagnostic{}, false
}

// diagsAt devuelve TODOS los diagnósticos de la línea, en orden de severidad
// (Error → Warning → Info): la barra de estado los muestra completos y solo el
// primero decide el marcador del gutter.
func (v *EditorView) diagsAt(line int) []Diagnostic {
	var out []Diagnostic
	for _, d := range v.diagnostics {
		if d.Line == line {
			out = append(out, d)
		}
	}
	return out
}

// DiagAtCursor devuelve el diagnóstico principal (más grave) de la línea del
// cursor; DiagsAtCursor devuelve todos. El controlador usa la lista completa
// para el mensaje de la barra de estado.
func (v *EditorView) DiagAtCursor() (Diagnostic, bool) {
	return v.diagAt(v.cursor.Line)
}

// DiagsAtCursor devuelve todos los diagnósticos de la línea del cursor.
func (v *EditorView) DiagsAtCursor() []Diagnostic {
	return v.diagsAt(v.cursor.Line)
}

// digitsOf devuelve la cantidad de dígitos decimales de n, con mínimo 1: 0 y 9
// devuelven 1, 10 y 99 devuelven 2, 100 devuelve 3.
func digitsOf(n int) int {
	d := 1
	for n >= 10 {
		n /= 10
		d++
	}
	return d
}

// gutterWidth devuelve el ancho de la columna de números de línea: los dígitos
// del total de líneas del buffer (mínimo 1) más 1 columna de separador/
// marcador. Es estable por buffer: solo cambia cuando el LineCount crece de
// dígitos (1→9, 10→99, ...), nunca por editar el contenido.
func (v *EditorView) gutterWidth() int {
	return digitsOf(v.model.LineCount()) + 1
}

// drawGutter pinta la columna de números (0..gutterWidth-1) de la fila física
// row. El número de línea solo va en la primera fila visual de la línea lógica
// (firstRow); las filas de continuación (wrap) dejan la posición en blanco y
// conservan el fondo del gutter que ya pintó el relleno de Draw. La última
// celda del gutter es la del separador y, cuando la línea está anotada, lleva
// el marcador de severidad: '!' Error, '?' Warning, 'i' Info, siempre con el
// color de su rol del tema sobre el fondo del documento.
func (v *EditorView) drawGutter(s Surface, th Theme, line, row int, firstRow bool) {
	if !firstRow {
		return
	}
	gutter := v.gutterWidth()
	digits := gutter - 1
	st := th.Gutter.Background(th.docBg())

	// Número (line+1) alineado a la derecha en sus columnas de dígitos; las
	// celdas de alineación ya vienen pintadas por el relleno de Draw.
	num := strconv.Itoa(line + 1)
	x := digits - len(num)
	if x < 0 {
		x = 0 // fuera de rango imposible: line+1 nunca excede al LineCount
	}
	for _, r := range num {
		s.SetContent(x, row, r, nil, st)
		x++
	}

	if d, ok := v.diagAt(line); ok {
		var mark rune
		var ms tcell.Style
		switch d.Severity {
		case SeverityError:
			mark, ms = '!', th.DiagError
		case SeverityWarning:
			mark, ms = '?', th.DiagWarning
		default:
			mark, ms = 'i', th.DiagInfo
		}
		s.SetContent(gutter-1, row, mark, nil, ms.Background(th.docBg()))
		return
	}
	s.SetContent(gutter-1, row, ' ', nil, st)
}

// diagInlineStyle devuelve el estilo del mensaje inline de una línea anotada:
// el color de su severidad (el mismo rol que el marcador del gutter) sobre el
// fondo de la línea (doc o línea del cursor).
func (v *EditorView) diagInlineStyle(th Theme, line int, cursorLine bool) (tcell.Style, bool) {
	d, ok := v.diagAt(line)
	if !ok {
		return tcell.Style{}, false
	}
	st := th.DiagError
	switch d.Severity {
	case SeverityWarning:
		st = th.DiagWarning
	case SeverityInfo:
		st = th.DiagInfo
	}
	bg := th.docBg()
	if cursorLine {
		bg = th.CursorLineBg
	}
	return st.Background(bg), true
}

// drawInlineDiag pinta el mensaje del diagnóstico a la derecha del texto de la
// línea (endX = columna donde terminó el texto), con un separador de dos
// espacios, truncado con "…" contra el borde derecho del área de texto. Se
// omite si no quedan al menos dos celdas libres (el texto ya llena la línea).
func (v *EditorView) drawInlineDiag(s Surface, th Theme, line, row, endX int, cursorLine bool) {
	st, ok := v.diagInlineStyle(th, line, cursorLine)
	if !ok {
		return
	}
	d, _ := v.diagAt(line)
	if d.Message == "" {
		return
	}
	right := v.gutterWidth() + v.viewport.Width
	// Separador de dos espacios; sin al menos dos celdas libres, se omite.
	if endX+2 >= right {
		return
	}
	x := endX
	s.Put(x, row, "  ", st)
	x += 2
	g := uniseg.NewGraphemes(d.Message)
	truncated := false
	for g.Next() {
		cl := g.Str()
		w := clusterWidth(g, x)
		if x+w > right-1 {
			truncated = true
			break
		}
		s.Put(x, row, cl, st)
		x += w
	}
	if truncated && x < right {
		s.Put(x, row, "…", st)
	}
}
