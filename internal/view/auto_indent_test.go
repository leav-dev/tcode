package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/leav-dev/tcode/internal/model"
)

// eventos de teclado simulados de la sesión de edición.
func evRune(r rune) tcell.Event { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }
func evEnter() tcell.Event      { return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone) }
func evTab() tcell.Event        { return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone) }

// lineContent devuelve la línea i del modelo, para aserciones post-Enter.
func lineContent(t *testing.T, pt *model.PieceTable, line int) string {
	t.Helper()
	if line >= pt.LineCount() {
		t.Fatalf("línea %d fuera del documento (%d líneas)", line, pt.LineCount())
	}
	return string(pt.LineContent(line))
}

// typeAndEnter tipea pre y presiona Enter, el flujo de edición real.
func typeAndEnter(t *testing.T, v *EditorView, pre string) {
	t.Helper()
	for _, r := range pre {
		if !v.handleKey(evRune(r).(*tcell.EventKey)) {
			t.Fatalf("handleKey rechazó la runa %q", r)
		}
	}
	if !v.handleKey(evEnter().(*tcell.EventKey)) {
		t.Fatal("handleKey rechazó el Enter")
	}
}

// TestEnterInheritsIndent: Enter hereda el prefijo exacto de la línea de origen.
func TestEnterInheritsIndent(t *testing.T) {
	pt := model.NewPieceTable()
	v := NewEditorView(pt, 10, 30)
	typeAndEnter(t, v, "    foo")

	if got := lineContent(t, pt, 1); got != "    " {
		t.Fatalf("línea nueva = %q, esperaba el prefijo heredado", got)
	}
}

// TestEnterAfterOpenBraceAddsIndent: tras una línea que termina en {, la línea
// nueva lleva indentUnit (la línea de origen no tenía prefijo).
func TestEnterAfterOpenBraceAddsIndent(t *testing.T) {
	pt := model.NewPieceTable()
	v := NewEditorView(pt, 10, 30)
	typeAndEnter(t, v, "if (x) {")

	if got := lineContent(t, pt, 1); got != indentUnit {
		t.Fatalf("línea nueva = %q, esperaba indentUnit (%q)", got, indentUnit)
	}
}

// TestEnterAfterColonAddsIndent: la regla incluye el caso de Python (termina en :).
func TestEnterAfterColonAddsIndent(t *testing.T) {
	pt := model.NewPieceTable()
	v := NewEditorView(pt, 10, 30)
	typeAndEnter(t, v, "if x:")

	if got := lineContent(t, pt, 1); got != indentUnit {
		t.Fatalf("línea nueva = %q, esperaba indentUnit (%q)", got, indentUnit)
	}
}

// TestEnterAfterBracketAddsIndent: tras [, el nivel extra también.
func TestEnterAfterBracketAddsIndent(t *testing.T) {
	pt := model.NewPieceTable()
	v := NewEditorView(pt, 10, 30)
	typeAndEnter(t, v, "arr = [")

	if got := lineContent(t, pt, 1); got != indentUnit {
		t.Fatalf("línea nueva = %q, esperaba indentUnit (%q)", got, indentUnit)
	}
}

// TestEnterPlainLineNoIndent: una línea plana no agrega nada.
func TestEnterPlainLineNoIndent(t *testing.T) {
	pt := model.NewPieceTable()
	v := NewEditorView(pt, 10, 30)
	typeAndEnter(t, v, "hello")

	if got := lineContent(t, pt, 1); got != "" {
		t.Fatalf("línea nueva = %q, esperaba vacía", got)
	}
}

// TestEnterAfterTrailingSpacesStillDetectsBrace: el whitespace de cola no
// esconde el carácter que abre el bloque. Con auto-cierre, tipear `{` trae
// su `}`: se lo salta con Right y se lo borra con Backspace para dejar la
// línea como antes (`if (x) {   `) y comprobar que el Enter igual indenta.
func TestEnterAfterTrailingSpacesStillDetectsBrace(t *testing.T) {
	pt := model.NewPieceTable()
	v := NewEditorView(pt, 10, 30)
	// Tipear hasta la llave (el Enter va después): el auto-cierre trae su `}`.
	for _, r := range "if (x) {" {
		if !v.handleKey(evRune(r).(*tcell.EventKey)) {
			t.Fatalf("handleKey rechazó la runa %q", r)
		}
	}
	// Quitar el cierre auto-insertado: Right lo salta, Backspace lo borra.
	if !v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)) {
		t.Fatal("Right no saltó el cierre")
	}
	if !v.HandleEvent(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone)) {
		t.Fatal("Backspace no borró el cierre")
	}
	typeAndEnter(t, v, "   ")

	if got := lineContent(t, pt, 1); got != indentUnit {
		t.Fatalf("línea nueva = %q, esperaba indentUnit", got)
	}
}

// TestTabFollowsIndentUnitChange: cambiar indentUnit cambia el Tab y el nivel.
func TestTabFollowsIndentUnitChange(t *testing.T) {
	old := indentUnit
	indentUnit = "\t"
	defer func() { indentUnit = old }()

	pt := model.NewPieceTable()
	v := NewEditorView(pt, 10, 30)
	v.handleKey(evTab().(*tcell.EventKey))
	if got := lineContent(t, pt, 0); got != "\t" {
		t.Fatalf("línea = %q, esperaba el tab de la unidad cambiada", got)
	}
}
