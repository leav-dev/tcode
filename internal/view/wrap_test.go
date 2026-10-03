package view

import (
	"strings"
	"testing"
)

// joinSoft une las filas visuales con '|' para aserciones legibles.
func joinSoft(ls []softLine) string {
	var b strings.Builder
	for i, l := range ls {
		if i > 0 {
			b.WriteString("|")
		}
		b.WriteString(string(l.text))
	}
	return b.String()
}

// TestSoftLinesBreaksAtWordBoundaries: la fila corta en el último espacio que
// entra; la palabra que no cabe va entera a la fila siguiente.
func TestSoftLinesBreaksAtWordBoundaries(t *testing.T) {
	// "hola mundo ancho" con width 8: "hola mun" no; el corte por palabra:
	// "hola mun" → no: los bytes: h(1)o(2)l(3)a(4) esp(5) m(6)u(7)n(8) → entra
	// "hola mun" (8); "do" no entra → "do" va sola...
	// Con width=10: "hola mundo" (10) entra; "ancho" va a la fila 2.
	ls := softLines("hola mundo ancho", 10)
	// El espacio del corte queda al final de la fila anterior (invisible al
	// pintar en el borde) y se conserva en la concatenación.
	if got := joinSoft(ls); got != "hola mundo |ancho" {
		t.Fatalf("softLines = %q, esperaba %q", got, "hola mundo |ancho")
	}
}

// TestSoftLinesBreaksAtTheLastFittingSpace: si dos espacios alcanzaban antes
// del desborde, el corte usa el último que entra.
func TestSoftLinesBreaksAtTheLastFittingSpace(t *testing.T) {
	// width 12: "uno dos tres" → "uno dos" (7) + esp(8) "tres" (12) entra…
	// hasta "tres" con el espacio del 8: 8+4=12 ✓ → "uno dos tres" entra entero.
	ls := softLines("uno dos tres", 10)
	if got := joinSoft(ls); got != "uno dos |tres" {
		t.Fatalf("softLines = %q, esperaba %q", got, "uno dos |tres")
	}
}

// TestSoftLinesSplitsAGiantWord: una palabra más larga que el ancho se parte
// por carácter (VSCode-like).
func TestSoftLinesSplitsAGiantWord(t *testing.T) {
	ls := softLines("abcdefghij", 4)
	if got := joinSoft(ls); got != "abcd|efgh|ij" {
		t.Fatalf("softLines = %q, esperaba %q", got, "abcd|efgh|ij")
	}
}

// TestSoftLinesExactFitNoBreak: la línea que entra exacta no se envuelve.
func TestSoftLinesExactFitNoBreak(t *testing.T) {
	if got := joinSoft(softLines("hola", 4)); got != "hola" {
		t.Fatalf("ancho justo = %q, esperaba sin corte", got)
	}
	if got := joinSoft(softLines("", 4)); got != "" {
		t.Fatalf("línea vacía = %q, esperaba una fila vacía", got)
	}
}

// TestSoftLinesPreservesTextTogether: la concatenación de las filas es la
// línea original (slice, sin pérdida ni duplicado).
func TestSoftLinesPreservesTextTogether(t *testing.T) {
	src := "café con 日本語 y palabras largas ininterrumpidas"
	ls := softLines(src, 8)
	var b strings.Builder
	for _, l := range ls {
		b.WriteString(l.text)
	}
	if b.String() != src {
		t.Fatalf("concatenación = %q, esperaba %q", b.String(), src)
	}
}

// TestSoftLineCountMatches: las filas visuales contadas son las producidas.
func TestSoftLineCountMatches(t *testing.T) {
	src := "hola mundo ancho y algo más"
	want := len(softLines(src, 7))
	if got := softLineCount(src, 7); got != want {
		t.Fatalf("count = %d, esperaba %d", got, want)
	}
}

// TestSoftLineAtFindsTheByte: la fila y columna visibles del byte pedido.
func TestSoftLineAtFindsTheByte(t *testing.T) {
	// "hola mundo ancho" width 10: filas "hola mundo " (0) y "ancho" (1).
	// bytes: h0..o9 sp10 a11 n12.. (fila 0 = [0,11) con el espacio).
	// 'h' byte 0 → fila 0, col 0.
	if row, col := softLineAt("hola mundo ancho", 10, 0); row != 0 || col != 0 {
		t.Fatalf("softLineAt(0) = (%d,%d), esperaba (0,0)", row, col)
	}
	// 'o' de "mundo" byte 9 → fila 0, col 9.
	if row, col := softLineAt("hola mundo ancho", 10, 9); row != 0 || col != 9 {
		t.Fatalf("softLineAt(9) = (%d,%d), esperaba (0,9)", row, col)
	}
	// 'a' de "ancho" es el byte 11 (frontera exacta) → fila 1, col 0.
	if row, col := softLineAt("hola mundo ancho", 10, 11); row != 1 || col != 0 {
		t.Fatalf("softLineAt(11) = (%d,%d), esperaba (1,0)", row, col)
	}
	// 'n' de "ancho" es el byte 12 → fila 1, col 1.
	if row, col := softLineAt("hola mundo ancho", 10, 12); row != 1 || col != 1 {
		t.Fatalf("softLineAt(12) = (%d,%d), esperaba (1,1)", row, col)
	}
	// El FINAL de la línea (cursor al final del documento) cae en la última
	// fila con su columna completa, no en la 0: el bug que destapó el undo.
	if row, col := softLineAt("dos", 40, 3); row != 0 || col != 3 {
		t.Fatalf("softLineAt(3) (fin de línea) = (%d,%d), esperaba (0,3)", row, col)
	}
}

// TestSoftLineToByteRoundTrip: (fila, col) → byte → at devuelve lo mismo.
func TestSoftLineToByteRoundTrip(t *testing.T) {
	src := "hola mundo ancho con 日本語"
	width := 6
	rows := softLines(src, width)
	// El inicio de cada fila se proyecta a esa fila, columna 0.
	for rowIdx, sl := range rows {
		r, c := softLineAt(src, width, sl.in)
		if r != rowIdx || c != 0 {
			t.Fatalf("at(in de la fila %d) = (%d,%d), esperaba (%d,0)", rowIdx, r, c, rowIdx)
		}
	}
	// toByte de col 0 de cada fila cae en esa fila...
	// y la primera celda de cada fila mapea a un byte dentro de ella.
	for rowIdx := range rows {
		b := softLineToByte(src, width, rowIdx, 0)
		r, _ := softLineAt(src, width, b)
		if r != rowIdx {
			t.Fatalf("toByte(fila %d,0) = byte %d que cae en la fila %d", rowIdx, b, r)
		}
	}
}
