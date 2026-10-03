package view

import (
	"testing"
)

// TestClampCursorKeepsCursorInsideShorterDocument: tras recargar un archivo
// más corto el cursor debe recortarse a la última línea existente y a la última
// columna válida; un LineContent fuera de rango paniquearía en el próximo
// dibujo, así que el clamp es obligatorio.
func TestClampCursorKeepsCursorInsideShorterDocument(t *testing.T) {
	// Sin \n final: la última línea es "sola línea" y el clamp debe caer ahí.
	v := newTestView(t, "una\nsola línea", 20, 5)

	// Cursor artificial fuera de rango, como quedaría tras recargar un doc corto.
	v.cursor.Line = 9
	v.cursor.ByteCol = 100
	v.cursor.desiredCol = 100

	v.ClampCursor()

	if v.cursor.Line != 1 {
		t.Errorf("cursor.Line = %d, esperaba la última línea (1)", v.cursor.Line)
	}
	if v.cursor.ByteCol != len(v.model.LineContent(1)) {
		t.Errorf("ByteCol = %d, esperaba el final de la línea (%d)", v.cursor.ByteCol, len(v.model.LineContent(1)))
	}
	if v.cursor.ByteCol > len(v.model.LineContent(1)) {
		t.Fatal("el cursor quedó fuera de su línea")
	}
}

// TestClampCursorMovesViewportIntoRange: el viewport vertical acompaña al
// cursor recortado (TopLine ya no puede quedar indicando una línea inexistente).
func TestClampCursorMovesViewportIntoRange(t *testing.T) {
	v := newTestView(t, "uno\ndos\ntres\ncuatro\n", 20, 5)

	v.cursor.Line = 50
	v.cursor.ByteCol = 1
	v.viewport.TopLine = 45 // fuera del documento

	v.ClampCursor()

	if v.cursor.Line >= v.lineCount() {
		t.Fatalf("el cursor quedó fuera del documento: %d", v.cursor.Line)
	}
	if v.viewport.TopLine > v.cursor.Line || v.viewport.TopLine+v.viewport.Height <= v.cursor.Line && v.viewport.Height > 0 {
		t.Errorf("viewport (%d) no contiene al cursor (%d) de alto %d", v.viewport.TopLine, v.cursor.Line, v.viewport.Height)
	}
	if v.viewport.TopLine < 0 {
		t.Errorf("TopLine = %d, no puede ser negativo", v.viewport.TopLine)
	}
}

// TestClampCursorOnEmptyDocument: un documento vacío deja el cursor en el
// origen sin paniquear (no se lee ninguna línea).
func TestClampCursorOnEmptyDocument(t *testing.T) {
	v := newTestView(t, "", 20, 5)
	v.cursor.Line = 3
	v.cursor.ByteCol = 7

	v.ClampCursor()

	if v.cursor.Line != 0 || v.cursor.ByteCol != 0 {
		t.Errorf("cursor = (%d, %d), esperaba (0, 0)", v.cursor.Line, v.cursor.ByteCol)
	}
	if v.viewport.TopLine != 0 || v.viewport.LeftColumn != 0 {
		t.Errorf("viewport = (%d, %d), esperaba (0, 0)", v.viewport.TopLine, v.viewport.LeftColumn)
	}
}
