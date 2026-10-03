package view

import (
	"testing"
)

// TestRenderWrapsLongLine: una línea que excede el ancho se pinta en varias
// filas físicas, cortando por palabra.
func TestRenderWrapsLongLine(t *testing.T) {
	s := newTestScreen(t, 10, 4)
	v := newTestView(t, "hola mundo ancho", 10, 4)
	draw(v, s)

	if got := screenLines(s)[0]; got != "hola mundo" {
		t.Fatalf("fila 0 = %q, esperaba %q (corte por palabra)", got, "hola mundo")
	}
	if got := screenLines(s)[1]; got != "ancho" {
		t.Fatalf("fila 1 = %q, esperaba %q", got, "ancho")
	}
	if got := screenLines(s)[2]; got != "" {
		t.Fatalf("fila 2 = %q, esperaba vacía", got)
	}
}

// TestCursorMovesToWrappedVisualRow: el clic en la fila física 1 de una línea
// envuelta cae en la línea lógica 0 con el byte de esa fila visual.
func TestCursorMovesToWrappedVisualRow(t *testing.T) {
	v := newTestView(t, "hola mundo ancho", 10, 4)

	if !v.moveCursorToCell(2, 1) {
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
	s := newTestScreen(t, 10, 3)
	v := newTestView(t, "hola mundo ancho\notra línea\n", 10, 3)
	v.setCursorAt(11) // 'a' de "ancho" → fila visual 1 de la línea 0
	draw(v, s)

	cells, width, _ := s.GetContents()
	if bgOfCell(cells[0*width+0].Style) != DefaultTheme().CursorLineBg {
		t.Fatal("la fila física 0 (línea del cursor) debe llevar el fondo")
	}
	if bgOfCell(cells[1*width+0].Style) != DefaultTheme().CursorLineBg {
		t.Fatal("la fila física 1 (corte de la línea del cursor) debe llevar el fondo")
	}
	// La línea 1 ("otra línea") ocupa la fila física 2: sin el fondo.
	if bgOfCell(cells[2*width+0].Style) == DefaultTheme().CursorLineBg {
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

	s := newTestScreen(t, 4, 2)
	v := newTestView(t, "holamundo", 4, 2)
	draw(v, s)

	if got := screenLines(s)[0]; got != "hola" {
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
