package model

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo de prueba: %v", err)
	}
	return path
}

func loadTable(t *testing.T, content string) *PieceTable {
	t.Helper()
	pt := NewPieceTable()
	if err := pt.LoadFile(writeTempFile(t, content)); err != nil {
		t.Fatalf("LoadFile falló: %v", err)
	}
	t.Cleanup(func() { pt.Close() })
	return pt
}

func TestLineCountWithoutTrailingNewline(t *testing.T) {
	pt := loadTable(t, "uno\ndos\ntres")
	if got := pt.LineCount(); got != 3 {
		t.Fatalf("LineCount() = %d, se esperaba 3", got)
	}
}

// TestLineCountIncludesTrailingEmptyLine: un documento que termina en '\n' tiene
// una línea vacía final direccionable, que es donde el cursor cae tras un Enter
// al final. Antes se excluía como "fantasma" y eso dejaba al cursor sin línea.
func TestLineCountIncludesTrailingEmptyLine(t *testing.T) {
	pt := loadTable(t, "uno\ndos\n")
	if got := pt.LineCount(); got != 3 {
		t.Fatalf("LineCount() = %d, se esperaba 3 (\"uno\", \"dos\" y la línea vacía final)", got)
	}
	if got := pt.LineStart(2); got != 8 {
		t.Fatalf("LineStart(2) = %d, se esperaba 8", got)
	}
	if got := len(pt.LineContent(2)); got != 0 {
		t.Fatalf("LineContent(2) = %q, se esperaba vacía", got)
	}
}

func TestGetRangeReturnsRequestedLines(t *testing.T) {
	pt := loadTable(t, "uno\ndos\ntres\ncuatro")

	got := string(pt.GetRange(1, 3))
	if got != "dos\ntres\n" {
		t.Fatalf("GetRange(1,3) = %q, se esperaba %q", got, "dos\ntres\n")
	}
}

func TestGetRangeIsZeroCopyViewIntoMmap(t *testing.T) {
	pt := loadTable(t, "uno\ndos\ntres")

	got := pt.GetRange(0, 1)
	if len(got) == 0 {
		t.Fatal("GetRange devolvió un slice vacío")
	}
	// El slice debe ser una vista del mmap, no una copia: su primer byte
	// coincide exactamente con el primer byte del buffer mapeado.
	if &got[0] != &pt.originalBuffer[0] {
		t.Fatal("GetRange copió datos en lugar de devolver una vista del mmap")
	}
}

func TestGetRangeOutOfBoundsReturnsNil(t *testing.T) {
	pt := loadTable(t, "uno\ndos")

	if got := pt.GetRange(5, 9); got != nil {
		t.Fatalf("GetRange fuera de rango = %q, se esperaba nil", got)
	}
	if got := pt.GetRange(1, 1); got != nil {
		t.Fatalf("GetRange con rango vacío = %q, se esperaba nil", got)
	}
	if got := pt.GetRange(-1, 2); got != nil {
		t.Fatalf("GetRange con start negativo = %q, se esperaba nil", got)
	}
}

func TestGetRangeClampsEndLineToDocumentEnd(t *testing.T) {
	pt := loadTable(t, "uno\ndos")

	got := string(pt.GetRange(1, 99))
	if got != "dos" {
		t.Fatalf("GetRange(1,99) = %q, se esperaba %q", got, "dos")
	}
}

func TestEmptyFileHasNoLines(t *testing.T) {
	pt := loadTable(t, "")
	if got := pt.LineCount(); got != 0 {
		t.Fatalf("LineCount() en archivo vacío = %d, se esperaba 0", got)
	}
	if got := pt.GetRange(0, 1); got != nil {
		t.Fatalf("GetRange en archivo vacío = %q, se esperaba nil", got)
	}
}

func TestGetContentStillReturnsWholeDocument(t *testing.T) {
	content := "uno\ndos\ntres"
	pt := loadTable(t, content)
	if got := pt.GetContent(); got != content {
		t.Fatalf("GetContent() = %q, se esperaba %q", got, content)
	}
}
