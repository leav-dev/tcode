package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestTypeOpenerInsertsPair: tipear `(` inserta el par y deja el cursor en el
// medio, en un solo paso.
func TestTypeOpenerInsertsPair(t *testing.T) {
	v := newTestView(t, "", 20, 3)

	typeRune(v, '(')
	if got := contentOf(t, v); got != "()" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "()")
	}
	if v.cursor.ByteCol != 1 {
		t.Fatalf("ByteCol = %d, se esperaba 1 (en el medio del par)", v.cursor.ByteCol)
	}
	typeRune(v, 'x')
	if got := contentOf(t, v); got != "(x)" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "(x)")
	}
}

// TestTypeCloserSkipsOver: tipear el cierre cuando el siguiente es el mismo
// avanza sin duplicar.
func TestTypeCloserSkipsOver(t *testing.T) {
	v := newTestView(t, "", 20, 3)

	typeRune(v, '(')
	typeRune(v, ')')
	if got := contentOf(t, v); got != "()" {
		t.Fatalf("contenido = %q, se esperaba %q sin duplicar", got, "()")
	}
	if v.cursor.ByteCol != 2 {
		t.Fatalf("ByteCol = %d, se esperaba 2 (tras el par)", v.cursor.ByteCol)
	}
}

// TestBackspaceDeletesEmptyPair: Backspace entre un par vacío borra los dos
// juntos.
func TestBackspaceDeletesEmptyPair(t *testing.T) {
	v := newTestView(t, "", 20, 3)

	typeRune(v, '[')
	pressKey(v, tcell.KeyBackspace)
	if got := contentOf(t, v); got != "" {
		t.Fatalf("contenido = %q, se esperaba vacío", got)
	}
}

// TestEnterBetweenPairSplits: Enter justo entre `{}` parte en dos líneas con
// el cierre en la siguiente e indent extra en el medio.
func TestEnterBetweenPairSplits(t *testing.T) {
	v := newTestView(t, "", 20, 4)

	typeString(v, "f() {")
	pressKey(v, tcell.KeyEnter)
	want := "f() {\n    \n}"
	if got := contentOf(t, v); got != want {
		t.Fatalf("contenido = %q, se esperaba %q", got, want)
	}
	if v.cursor.Line != 1 {
		t.Fatalf("línea = %d, se esperaba 1 (la del medio)", v.cursor.Line)
	}
}

// TestQuotesPairAndSkip: `"` arma el par y un segundo `"` lo salta; `'` tras
// letra inserta simple (don't).
func TestQuotesPairAndSkip(t *testing.T) {
	v := newTestView(t, "", 20, 3)

	typeRune(v, '"')
	if got := contentOf(t, v); got != `""` {
		t.Fatalf("contenido = %q, se esperaba par de comillas", got)
	}
	typeRune(v, '"')
	if got := contentOf(t, v); got != `""` {
		t.Fatalf("contenido = %q, el segundo \" debe saltar sin duplicar", got)
	}

	v2 := newTestView(t, "don", 20, 3)
	pressKey(v2, tcell.KeyEnd)
	typeRune(v2, '\'')
	if got := contentOf(t, v2); got != "don'" {
		t.Fatalf("contenido = %q, tras palabra la comilla va simple", got)
	}
}

// TestSurroundSelection: tipear una apertura con selección envuelve el texto.
func TestSurroundSelection(t *testing.T) {
	v := newTestView(t, "hola", 20, 3)

	pressKey(v, tcell.KeyHome)
	v.HandleEvent(modEvent(tcell.KeyRight, true))
	v.HandleEvent(modEvent(tcell.KeyRight, true))
	typeRune(v, '(')
	if got := contentOf(t, v); got != "(ho)la" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "(ho)la")
	}
}
