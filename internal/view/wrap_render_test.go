package view

import (
	"testing"
)

// TestRenderWrapsLongLine: una línea que excede el ancho se pinta en varias
// filas físicas, cortando por palabra.
func TestRenderWrapsLongLine(t *testing.T) {
	// 12 de pantalla = 2 del gutter + 10 del área de texto: el corte por
	// palabra se mantiene idéntico al comportamiento previo con 10 columnas.
	s := newTestScreen(t, 12, 4)
	v := newTestView(t, "hola mundo ancho", 12, 4)
	draw(v, s)

	if got := screenLines(s)[0]; got != "1 hola mundo" {
		t.Fatalf("fila 0 = %q, esperaba %q (número + corte por palabra)", got, "1 hola mundo")
	}
	// La fila de continuación lleva el gutter en blanco y el texto desplazado.
	if got := screenLines(s)[1]; got != "  ancho" {
		t.Fatalf("fila 1 = %q, esperaba %q", got, "  ancho")
	}
	if got := screenLines(s)[2]; got != "" {
		t.Fatalf("fila 2 = %q, esperaba vacía", got)
	}
}

// TestCursorMovesToWrappedVisualRow: el clic en la fila física 1 de una línea
// envuelta cae en la línea lógica 0 con el byte de esa fila visual.
func TestCursorMovesToWrappedVisualRow(t *testing.T) {
	// 12 de pantalla = 2 del gutter + 10 del área de texto (el wrap previo).
	v := newTestView(t, "hola mundo ancho", 12, 4)

	// La columna 2 del texto vive en la celda gutterWidth+2.
	if !v.moveCursorToCell(2+v.gutterWidth(), 1) {
		t.Fatal("el clic en la fila envuelta no se aceptó")
	}
	if v.cursor.Line != 0 {
		t.Fatalf("cursor.Line = %d, esperaba 0 (la fila 1 es un corte de la línea 0)", v.cursor.Line)
	}
	// "ancho" en la fila visual 1: col 0='a'(11) col 1='n'(12) col 2='c'(13).
	if v.cursor.ByteCol != 13 {
		t.Fatalf("ByteCol = %d, esperaba 13 ('c' de ancho)", v.cursor.ByteCol)
	}
}

// TestCursorLineBackgroundOnWrappedRow: la línea lógica del cursor (envuelta en
// varias filas físicas) lleva el fondo en TODAS sus filas; la línea siguiente
// no.
func TestCursorLineBackgroundOnWrappedRow(t *testing.T) {
	s := newTestScreen(t, 12, 3)
	v := newTestView(t, "hola mundo ancho\notra línea\n", 12, 3)
	v.setCursorAt(11) // 'a' de "ancho" → fila visual 1 de la línea 0
	draw(v, s)

	cells, width, _ := s.GetContents()
	// El texto vive después del gutter (2 columnas): las celdas de texto son
	// las de la columna gutterWidth.
	g := v.gutterWidth()
	if bgOfCell(cells[0*width+g].Style) != DefaultTheme().CursorLineBg {
		t.Fatal("la fila física 0 (línea del cursor) debe llevar el fondo")
	}
	if bgOfCell(cells[1*width+g].Style) != DefaultTheme().CursorLineBg {
		t.Fatal("la fila física 1 (corte de la línea del cursor) debe llevar el fondo")
	}
	// La línea 1 ("otra línea") ocupa la fila física 2: sin el fondo.
	if bgOfCell(cells[2*width+g].Style) == DefaultTheme().CursorLineBg {
		t.Fatal("la fila de la línea siguiente no debe llevar el fondo del cursor")
	}
}

// TestWrapDisabledKeepsHorizontalCutting: con la configuración apagada, la
// línea larga pinta una sola fila y el clúster que no entra se corta en el
// borde (comportamiento previo).
func TestWrapDisabledKeepsHorizontalCutting(t *testing.T) {
	old := wordWrapEnabled
	wordWrapEnabled = false
	defer func() { wordWrapEnabled = old }()

	// 6 de pantalla = 2 del gutter + 4 del área de texto: el corte en el
	// borde recorta "holamundo" a 4 columnas, igual que antes con 4.
	s := newTestScreen(t, 6, 2)
	v := newTestView(t, "holamundo", 6, 2)
	draw(v, s)

	if got := screenLines(s)[0]; got != "1 hola" {
		t.Fatalf("sin wrap, fila 0 = %q, esperaba recortada en el borde", got)
	}
	if got := screenLines(s)[1]; got != "" {
		t.Fatalf("sin wrap no hay fila 1: %q", got)
	}
}

// TestToggleWordWrapFlipsAndReports: el toggle cambia el estado, lo reporta y
// dos toggles vuelven al original (independiente del valor inicial).
func TestToggleWordWrapFlipsAndReports(t *testing.T) {
	before := WordWrapEnabled()
	after := ToggleWordWrap()
	if after == before {
		t.Fatal("el toggle no cambió la configuración")
	}
	if WordWrapEnabled() != after {
		t.Fatal("el toggle no refleja el estado nuevo")
	}
	if again := ToggleWordWrap(); again != before {
		t.Fatal("dos toggles deben volver al estado original")
	}
}
