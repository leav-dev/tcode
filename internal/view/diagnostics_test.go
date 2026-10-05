package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// --- HITO A de "diagnostics": gutter de números + marcador + underline ---

// TestGutterShowsLineNumbers: cada línea lógica muestra su número en el gutter
// (alineado a la derecha) y el texto del documento arranca después de él. El
// gutter de un buffer corto mide dígitos(LineCount)+1 columnas: 2 para menos
// de 10 líneas (número + separador/marcador).
func TestGutterShowsLineNumbers(t *testing.T) {
	s := newTestScreen(t, 20, 3)
	v := newTestView(t, "uno\ndos\ntres", 20, 3)
	draw(v, s)

	if got := v.gutterWidth(); got != 2 {
		t.Fatalf("gutterWidth = %d, se esperaba 2 (dígitos(3)+1)", got)
	}
	if got := cellRuneAt(s, 0, 0); got != '1' {
		t.Fatalf("gutter (0,0) = %q, se esperaba '1'", got)
	}
	if got := cellRuneAt(s, 0, 1); got != '2' {
		t.Fatalf("gutter (0,1) = %q, se esperaba '2'", got)
	}
	if got := cellRuneAt(s, 0, 2); got != '3' {
		t.Fatalf("gutter (0,2) = %q, se esperaba '3'", got)
	}
	// La celda del separador queda en blanco sin diagnóstico.
	if got := cellRuneAt(s, 1, 1); got != ' ' {
		t.Fatalf("separador (1,1) = %q, se esperaba un espacio", got)
	}
	// La fila visible arranca con número + separador + contenido.
	if got := screenLines(s)[0]; got != "1 uno" {
		t.Fatalf("fila 0 = %q, se esperaba %q (número + separador + contenido)", got, "1 uno")
	}
}

// TestUnderlineAndGutterMarkDiagnostics: la línea anotada lleva su marcador de
// severidad en la última celda del gutter ('!' = error) y sus clusters pintan
// con underline; la línea sin diagnóstico no tiene marcador ni subrayado.
func TestUnderlineAndGutterMarkDiagnostics(t *testing.T) {
	s := newTestScreen(t, 20, 3)
	v := newTestView(t, "uno\ndos\ntres", 20, 3)
	v.SetDiagnostics([]Diagnostic{{Line: 1, Message: "x", Severity: SeverityError}})
	draw(v, s)

	if got := cellRuneAt(s, 1, 1); got != '!' {
		t.Fatalf("marcador (1,1) = %q, se esperaba '!' (Error)", got)
	}
	if got := cellRuneAt(s, 1, 0); got != ' ' {
		t.Fatalf("celda del marcador de la línea 0 = %q, se esperaba el separador en blanco", got)
	}

	cells, width, _ := s.GetContents()
	// 'd' de "dos" (the text lives after the gutter: columna gutterWidth).
	_, _, attr := cells[1*width+2].Style.Decompose()
	if attr&tcell.AttrUnderline == 0 {
		t.Fatal("la línea con diagnóstico debe pintarse subrayada")
	}
	// 'u' de "uno": sin diagnóstico, sin subrayado.
	_, _, plain := cells[0*width+2].Style.Decompose()
	if plain&tcell.AttrUnderline != 0 {
		t.Fatal("la línea sin diagnóstico no debe pintarse subrayada")
	}
}

// TestGutterContinuationRowsBlank: las filas de continuación de una línea
// envuelta no repiten el número (solo la primera fila visual lo lleva) y
// conservan el fondo del gutter.
func TestGutterContinuationRowsBlank(t *testing.T) {
	old := wordWrapEnabled
	wordWrapEnabled = true
	defer func() { wordWrapEnabled = old }()

	s := newTestScreen(t, 10, 4)
	v := newTestView(t, "hola mundo ancho", 10, 4)
	draw(v, s)

	if got := cellRuneAt(s, 0, 0); got != '1' {
		t.Fatalf("fila 0: número (0,0) = %q, se esperaba '1'", got)
	}
	if got := cellRuneAt(s, 0, 1); got != ' ' {
		t.Fatalf("fila 1 (continuación): (0,1) = %q, se esperaba en blanco (sin número)", got)
	}
	if bg := cellBg(s, 0, 1); bg != DefaultTheme().docBg() {
		t.Fatalf("fondo del gutter en la continuación = %v, se esperaba %v", bg, DefaultTheme().docBg())
	}
}

// TestDiagSeverityMappingAndPriority: diagAt devuelve el diagnóstico de la
// línea priorizando la severidad (Error > Warning > Info) y, con empate, el
// primero de la lista; las líneas sin anotar devuelven false.
func TestDiagSeverityMappingAndPriority(t *testing.T) {
	v := newTestView(t, "uno\ndos", 20, 2)
	v.SetDiagnostics([]Diagnostic{
		{Line: 0, Message: "bajo", Severity: SeverityInfo},
		{Line: 0, Message: "medio", Severity: SeverityWarning},
		{Line: 0, Message: "alto", Severity: SeverityError},
		{Line: 0, Message: "otro alto", Severity: SeverityError},
	})

	d, ok := v.diagAt(0)
	if !ok {
		t.Fatal("la línea 0 tiene diagnósticos")
	}
	if d.Severity != SeverityError {
		t.Fatalf("severidad = %v, se esperaba Error (prioridad sobre Warning/Info)", d.Severity)
	}
	if d.Message != "alto" {
		t.Fatalf("mensaje = %q, se esperaba %q (el primero de la severidad máxima)", d.Message, "alto")
	}
	if _, ok := v.diagAt(1); ok {
		t.Fatal("la línea 1 no tiene diagnósticos")
	}
}

// TestMouseClickBeyondGutterTranslates: el clic en el área de texto se
// traduce a la columna lógica restando el gutter; el clic sobre el gutter no
// mueve el cursor.
func TestMouseClickBeyondGutterTranslates(t *testing.T) {
	v := newTestView(t, "uno\ndos\ntres", 20, 3)
	const gutter = 2

	if !v.HandleEvent(tcell.NewEventMouse(gutter+2, 1, tcell.Button1, tcell.ModNone)) {
		t.Fatal("el clic fuera del gutter debería mover el cursor")
	}
	if v.cursor.Line != 1 || v.cursor.ByteCol != 2 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,2)", v.cursor.Line, v.cursor.ByteCol)
	}

	if v.HandleEvent(tcell.NewEventMouse(1, 1, tcell.Button1, tcell.ModNone)) {
		t.Fatal("un clic en el gutter no debería mover el cursor")
	}
	if v.cursor.Line != 1 || v.cursor.ByteCol != 2 {
		t.Fatalf("el cursor no debe moverse con un clic en el gutter: (%d,%d)", v.cursor.Line, v.cursor.ByteCol)
	}
}

// TestGutterResizeKeepsTextAreaWide: Resize recibe el ancho total del widget y
// descuenta el gutter para el área de texto, con un mínimo de 1 columna.
func TestGutterResizeKeepsTextAreaWide(t *testing.T) {
	v := newTestView(t, "uno\ndos", 20, 2)
	if w, _ := v.Size(); w != 18 {
		t.Fatalf("área de texto = %d, se esperaba 18 (20 menos el gutter de 2)", w)
	}
	v.Resize(20, 2)
	if w, _ := v.Size(); w != 18 {
		t.Fatalf("área de texto = %d, se esperaba 18 tras el resize", w)
	}
	v.Resize(1, 2)
	if w, _ := v.Size(); w < 1 {
		t.Fatalf("área de texto = %d, se esperaba al menos 1 (guard del widget angosto)", w)
	}
}
