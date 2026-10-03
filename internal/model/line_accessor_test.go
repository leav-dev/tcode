package model

import "testing"

func TestLineStartAndContent(t *testing.T) {
	pt := loadTable(t, "uno\ndos\ntres")

	for _, tc := range []struct {
		line    int
		start   int
		content string
	}{
		{0, 0, "uno"},
		{1, 4, "dos"},
		{2, 8, "tres"},
	} {
		if got := pt.LineStart(tc.line); got != tc.start {
			t.Fatalf("LineStart(%d) = %d, se esperaba %d", tc.line, got, tc.start)
		}
		if got := string(pt.LineContent(tc.line)); got != tc.content {
			t.Fatalf("LineContent(%d) = %q, se esperaba %q", tc.line, got, tc.content)
		}
	}
}

func TestLineContentExcludesLineBreak(t *testing.T) {
	pt := loadTable(t, "uno\ndos\n")

	// El salto existe (el inicio de la línea 1 está en 4), pero no es contenido.
	if got := string(pt.LineContent(0)); got != "uno" {
		t.Fatalf("LineContent(0) = %q, se esperaba %q", got, "uno")
	}
	if got := pt.LineBreakLen(0); got != 1 {
		t.Fatalf("LineBreakLen(0) = %d, se esperaba 1", got)
	}
}

func TestLineBreakLenHandlesCRLF(t *testing.T) {
	pt := loadTable(t, "uno\r\ndos")

	if got := pt.LineBreakLen(0); got != 2 {
		t.Fatalf("LineBreakLen(0) = %d, se esperaba 2 para CRLF", got)
	}
	if got := string(pt.LineContent(0)); got != "uno" {
		t.Fatalf("LineContent(0) = %q, se esperaba %q: CRLF es salto, no contenido", got, "uno")
	}
	if got := pt.LineStart(1); got != 5 {
		t.Fatalf("LineStart(1) = %d, se esperaba 5", got)
	}
}

func TestLineBreakLenIsZeroOnLastLineWithoutNewline(t *testing.T) {
	pt := loadTable(t, "uno\ndos")

	if got := pt.LineBreakLen(1); got != 0 {
		t.Fatalf("LineBreakLen(1) = %d, se esperaba 0", got)
	}
}

func TestLineAtMapsOffsetsToLines(t *testing.T) {
	pt := loadTable(t, "uno\ndos\ntres")

	for _, tc := range []struct {
		offset int
		line   int
	}{
		{0, 0},
		{2, 0},
		{3, 0}, // el '\n' pertenece a la línea que cierra
		{4, 1},
		{8, 2},
		{11, 2}, // último byte
	} {
		if got := pt.LineAt(tc.offset); got != tc.line {
			t.Fatalf("LineAt(%d) = %d, se esperaba %d", tc.offset, got, tc.line)
		}
	}
}

// TestLineAccessorsSurviveEdits comprueba que los accesores sigan al índice de
// líneas después de insertar y borrar, en lugar de quedar calculados sobre el
// archivo original.
func TestLineAccessorsSurviveEdits(t *testing.T) {
	pt := loadTable(t, "uno\ndos")

	if err := pt.Insert(0, "cero\n"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if _, err := pt.Delete(5, 9); err != nil { // borra "uno\n"
		t.Fatalf("Delete falló: %v", err)
	}

	if got := pt.GetContent(); got != "cero\ndos" {
		t.Fatalf("contenido = %q", got)
	}
	if got := string(pt.LineContent(0)); got != "cero" {
		t.Fatalf("LineContent(0) = %q, se esperaba %q", got, "cero")
	}
	if got := string(pt.LineContent(1)); got != "dos" {
		t.Fatalf("LineContent(1) = %q, se esperaba %q", got, "dos")
	}
	if got := pt.LineStart(1); got != 5 {
		t.Fatalf("LineStart(1) = %d, se esperaba 5", got)
	}
	if got := pt.LineAt(6); got != 1 {
		t.Fatalf("LineAt(6) = %d, se esperaba 1", got)
	}
}

func TestLineAccessorsOnEmptyDocument(t *testing.T) {
	pt := loadTable(t, "")

	if got := pt.LineContent(0); got != nil {
		t.Fatalf("LineContent(0) en documento vacío = %q, se esperaba nil", got)
	}
	if got := pt.LineStart(0); got != 0 {
		t.Fatalf("LineStart(0) = %d, se esperaba 0", got)
	}
	if got := pt.LineBreakLen(0); got != 0 {
		t.Fatalf("LineBreakLen(0) = %d, se esperaba 0", got)
	}
	if got := pt.LineAt(0); got != 0 {
		t.Fatalf("LineAt(0) = %d, se esperaba 0", got)
	}
}
