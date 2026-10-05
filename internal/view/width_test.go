package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// cell describe una celda renderizada: runa primaria, combinantes y ancho.
type cell struct {
	primary   rune
	combining []rune
	width     int
}

func cellAt(s tcell.SimulationScreen, x, y int) cell {
	primary, combining, _, width := s.GetContent(x, y)
	return cell{primary: primary, combining: combining, width: width}
}

// TestWideCJKCharacterAdvancesTwoColumns es el corazón del defecto: un carácter
// ancho debe consumir dos columnas, no una.
func TestWideCJKCharacterAdvancesTwoColumns(t *testing.T) {
	s := newTestScreen(t, 10, 1)
	v := newTestView(t, "日a", 10, 1)

	draw(v, s)

	got := cellAt(s, 2, 0)
	if got.primary != '日' || got.width != 2 {
		t.Fatalf("celda (2,0) = %q width=%d, se esperaba '日' width=2", got.primary, got.width)
	}
	if next := cellAt(s, 4, 0); next.primary != 'a' {
		t.Fatalf("celda (4,0) = %q, se esperaba 'a': el ancho debe avanzar 2 columnas", next.primary)
	}
}

// TestCombiningMarkSharesTheBaseCell cubre el caso de "e" + acento combinante:
// el acento no debe consumir una columna propia.
func TestCombiningMarkSharesTheBaseCell(t *testing.T) {
	s := newTestScreen(t, 10, 1)
	v := newTestView(t, "e\u0301x", 10, 1)

	draw(v, s)

	got := cellAt(s, 2, 0)
	if got.primary != 'e' || got.width != 1 {
		t.Fatalf("celda (2,0) = %q width=%d, se esperaba 'e' width=1", got.primary, got.width)
	}
	if len(got.combining) != 1 || got.combining[0] != '\u0301' {
		t.Fatalf("combinantes = %v, se esperaba [U+0301]", got.combining)
	}
	if next := cellAt(s, 3, 0); next.primary != 'x' {
		t.Fatalf("celda (3,0) = %q, se esperaba 'x': el acento no debe consumir columna", next.primary)
	}
}

// TestEmojiZWJSequenceCountsAsOneCluster verifica que una familia emoji unida por
// ZWJ avance como un único cluster y no como runa por runa.
func TestEmojiZWJSequenceCountsAsOneCluster(t *testing.T) {
	family := "\U0001F468\u200D\U0001F469\u200D\U0001F467" // 👨‍👩‍👧
	s := newTestScreen(t, 12, 1)
	v := newTestView(t, family+"x", 12, 1)

	draw(v, s)

	got := cellAt(s, 2, 0)
	if got.width != 2 {
		t.Fatalf("ancho del cluster emoji = %d, se esperaba 2", got.width)
	}
	if next := cellAt(s, 4, 0); next.primary != 'x' {
		t.Fatalf("celda (4,0) = %q, se esperaba 'x' tras un cluster de 2 columnas", next.primary)
	}
}

// TestTabExpandsToNextTabStop comprueba que la tabulación alinee al siguiente
// tab stop y no avance una sola columna.
func TestTabExpandsToNextTabStop(t *testing.T) {
	s := newTestScreen(t, 12, 1)
	v := newTestView(t, "a\tb\tc", 12, 1)

	draw(v, s)

	if c := cellAt(s, 2, 0); c.primary != 'a' {
		t.Fatalf("celda (2,0) = %q, se esperaba 'a'", c.primary)
	}
	if c := cellAt(s, 6, 0); c.primary != 'b' {
		t.Fatalf("celda (6,0) = %q, se esperaba 'b' en el primer tab stop", c.primary)
	}
	if c := cellAt(s, 10, 0); c.primary != 'c' {
		t.Fatalf("celda (10,0) = %q, se esperaba 'c' en el segundo tab stop", c.primary)
	}
}

// TestWideCharacterThatDoesNotFitIsNotDrawn evita que un carácter ancho pise la
// celda de continuación al escribirse contra el borde derecho.
func TestWideCharacterThatDoesNotFitIsNotDrawn(t *testing.T) {
	// 5 de pantalla = 2 del gutter + 3 del área de texto (la geometría previa).
	s := newTestScreen(t, 5, 1)
	v := newTestView(t, "ab日", 5, 1)

	draw(v, s)

	if c := cellAt(s, 4, 0); c.primary == '日' {
		t.Fatal("no se debe dibujar un carácter ancho que no entra completo")
	}
}

// TestHorizontalScrollSkipsByDisplayWidth verifica que el scroll horizontal
// descuente columnas de terminal, no runas.
func TestHorizontalScrollSkipsByDisplayWidth(t *testing.T) {
	// Estos tests fijan el scroll HORIZONTAL: sin wrap (el wrap es la
	// alternativa y envuelve la línea, anulando el desplazamiento).
	oldWrap := wordWrapEnabled
	wordWrapEnabled = false
	defer func() { wordWrapEnabled = oldWrap }()
	// 6 de pantalla = 2 del gutter + 4 del área de texto (la geometría previa).
	s := newTestScreen(t, 6, 1)
	v := newTestView(t, "日日ab", 6, 1)
	v.viewport.LeftColumn = 2

	draw(v, s)

	if c := cellAt(s, 2, 0); c.primary != '日' {
		t.Fatalf("celda (2,0) = %q, se esperaba el segundo '日' (tras el gutter)", c.primary)
	}
	if c := cellAt(s, 4, 0); c.primary != 'a' {
		t.Fatalf("celda (4,0) = %q, se esperaba 'a'", c.primary)
	}
	if c := cellAt(s, 5, 0); c.primary != 'b' {
		t.Fatalf("celda (5,0) = %q, se esperaba 'b'", c.primary)
	}
}

// TestCRLFIsASingleLineBreak cubre archivos con finales de línea Windows: CRLF
// debe ser un salto de línea y el CR no debe dibujarse.
func TestCRLFIsASingleLineBreak(t *testing.T) {
	s := newTestScreen(t, 10, 2)
	v := newTestView(t, "uno\r\ndos", 10, 2)

	draw(v, s)

	if c := cellAt(s, 2, 0); c.primary != 'u' {
		t.Fatalf("celda (2,0) = %q, se esperaba 'u'", c.primary)
	}
	if c := cellAt(s, 2, 1); c.primary != 'd' {
		t.Fatalf("celda (2,1) = %q, se esperaba 'd': CRLF debe cortar la línea", c.primary)
	}

	cells, w, h := s.GetContents()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for _, r := range cells[y*w+x].Runes {
				if r == '\r' {
					t.Fatalf("CR quedó dibujado en (%d,%d)", x, y)
				}
			}
		}
	}
}

// TestLoneCarriageReturnResetsColumnOnSameRow documenta la semántica de CR
// aislado: vuelve al inicio de la fila sin saltar de línea.
func TestLoneCarriageReturnResetsColumnOnSameRow(t *testing.T) {
	s := newTestScreen(t, 10, 1)
	v := newTestView(t, "ab\rxy", 10, 1)

	draw(v, s)

	if c := cellAt(s, 2, 0); c.primary != 'x' {
		t.Fatalf("celda (2,0) = %q, se esperaba 'x' tras el CR", c.primary)
	}
	if c := cellAt(s, 3, 0); c.primary != 'y' {
		t.Fatalf("celda (3,0) = %q, se esperaba 'y'", c.primary)
	}
}

// TestStandaloneCombiningMarkDoesNotAdvanceColumn cubre texto inválido: un
// acento sin runa base no tiene dónde anclarse y no debe consumir columna.
func TestStandaloneCombiningMarkDoesNotAdvanceColumn(t *testing.T) {
	s := newTestScreen(t, 10, 1)
	v := newTestView(t, "\u0301x", 10, 1)

	draw(v, s)

	if c := cellAt(s, 2, 0); c.primary != 'x' {
		t.Fatalf("celda (2,0) = %q, se esperaba 'x'", c.primary)
	}
}

// TestMixedWidthLineKeepsColumnsAligned es la prueba de integración: una línea
// con ASCII, CJK y emoji debe dejar cada carácter en la columna correcta.
func TestMixedWidthLineKeepsColumnsAligned(t *testing.T) {
	s := newTestScreen(t, 20, 1)
	// "a" (1) + "日" (2) + "b" (1) + "☕" (2) + "c" (1)
	v := newTestView(t, "a日b☕c", 20, 1)

	draw(v, s)

	for _, tc := range []struct {
		x    int
		want rune
	}{
		{2, 'a'},
		{3, '日'},
		{5, 'b'},
		{6, '☕'},
		{8, 'c'},
	} {
		if c := cellAt(s, tc.x, 0); c.primary != tc.want {
			t.Fatalf("celda (%d,0) = %q, se esperaba %q", tc.x, c.primary, tc.want)
		}
	}
}
