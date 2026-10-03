package model

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// --- Insert ---

func TestInsertAtStart(t *testing.T) {
	pt := loadTable(t, "mundo")

	if err := pt.Insert(0, "hola "); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if got := pt.GetContent(); got != "hola mundo" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "hola mundo")
	}
	if pt.Len() != 10 {
		t.Fatalf("Len() = %d, se esperaba 10", pt.Len())
	}
}

func TestInsertInMiddleSplitsPiece(t *testing.T) {
	pt := loadTable(t, "holamundo")

	if err := pt.Insert(4, " "); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if got := pt.GetContent(); got != "hola mundo" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "hola mundo")
	}
	// Al partir la pieza original quedan: original + insertada + original.
	if len(pt.pieces) != 3 {
		t.Fatalf("piezas = %d, se esperaba 3 tras partir", len(pt.pieces))
	}
}

func TestInsertAtEnd(t *testing.T) {
	pt := loadTable(t, "hola")

	if err := pt.Insert(4, " mundo"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if got := pt.GetContent(); got != "hola mundo" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "hola mundo")
	}
}

func TestInsertEmptyTextIsNoOp(t *testing.T) {
	pt := loadTable(t, "hola")

	if err := pt.Insert(2, ""); err != nil {
		t.Fatalf("Insert de texto vacío falló: %v", err)
	}
	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "hola")
	}
	if len(pt.pieces) != 1 {
		t.Fatalf("piezas = %d, se esperaba 1: un insert vacío no debe tocar la tabla", len(pt.pieces))
	}
}

func TestInsertOutOfRangeFails(t *testing.T) {
	pt := loadTable(t, "hola")

	for _, off := range []int{-1, 5, 100} {
		if err := pt.Insert(off, "x"); err == nil {
			t.Fatalf("Insert(%d) debería fallar", off)
		}
	}
	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("un Insert inválido no debe alterar el documento: %q", got)
	}
}

// --- Delete ---

func TestDeleteAtStart(t *testing.T) {
	pt := loadTable(t, "hola mundo")

	n, err := pt.Delete(0, 5)
	if err != nil {
		t.Fatalf("Delete falló: %v", err)
	}
	if n != 5 {
		t.Fatalf("Delete devolvió %d, se esperaba 5", n)
	}
	if got := pt.GetContent(); got != "mundo" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "mundo")
	}
}

func TestDeleteInMiddleTrimmsPiece(t *testing.T) {
	pt := loadTable(t, "hola mundo")

	// Borra solo el espacio: la pieza original queda recortada en dos.
	if _, err := pt.Delete(4, 5); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}
	if got := pt.GetContent(); got != "holamundo" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "holamundo")
	}
}

func TestDeleteAtEnd(t *testing.T) {
	pt := loadTable(t, "hola mundo")

	if _, err := pt.Delete(4, 10); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}
	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "hola")
	}
}

func TestDeleteEmptyRangeIsNoOp(t *testing.T) {
	pt := loadTable(t, "hola")

	n, err := pt.Delete(2, 2)
	if err != nil {
		t.Fatalf("Delete de rango vacío falló: %v", err)
	}
	if n != 0 {
		t.Fatalf("Delete devolvió %d, se esperaba 0", n)
	}
	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "hola")
	}
}

func TestDeleteOutOfRangeFails(t *testing.T) {
	pt := loadTable(t, "hola")

	for _, r := range [][2]int{{-1, 2}, {0, 5}, {3, 2}} {
		if _, err := pt.Delete(r[0], r[1]); err == nil {
			t.Fatalf("Delete(%d,%d) debería fallar", r[0], r[1])
		}
	}
	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("un Delete inválido no debe alterar el documento: %q", got)
	}
}

func TestDeleteWholeDocumentLeavesItEmpty(t *testing.T) {
	pt := loadTable(t, "hola\nmundo")

	if _, err := pt.Delete(0, pt.Len()); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}
	if got := pt.GetContent(); got != "" {
		t.Fatalf("contenido = %q, se esperaba vacío", got)
	}
	if pt.Len() != 0 {
		t.Fatalf("Len() = %d, se esperaba 0", pt.Len())
	}
	if got := pt.LineCount(); got != 0 {
		t.Fatalf("LineCount() = %d, se esperaba 0", got)
	}
}

// --- Interacción con el índice de líneas ---

func TestInsertNewlineCreatesLineAndShiftsTheRest(t *testing.T) {
	pt := loadTable(t, "uno\ndos")

	if err := pt.Insert(3, "\nX"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if got := pt.GetContent(); got != "uno\nX\ndos" {
		t.Fatalf("contenido = %q", got)
	}
	if got := pt.LineCount(); got != 3 {
		t.Fatalf("LineCount() = %d, se esperaba 3", got)
	}
	if want := []int{0, 4, 6}; !equalInts(pt.lineOffsets, want) {
		t.Fatalf("lineOffsets = %v, se esperaba %v", pt.lineOffsets, want)
	}
}

func TestDeleteSpanningNewlineMergesLines(t *testing.T) {
	pt := loadTable(t, "uno\ndos")

	// Borra "o\nd": las dos líneas se funden en una.
	if _, err := pt.Delete(2, 5); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}
	if got := pt.GetContent(); got != "unos" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "unos")
	}
	if got := pt.LineCount(); got != 1 {
		t.Fatalf("LineCount() = %d, se esperaba 1", got)
	}
	if want := []int{0}; !equalInts(pt.lineOffsets, want) {
		t.Fatalf("lineOffsets = %v, se esperaba %v", pt.lineOffsets, want)
	}
}

// TestDeleteCollapsingLineStartDoesNotDuplicate cubre el caso en que el inicio de
// línea borrado cae justo en el borde del rango: el inicio que sobrevive y el
// inicio desplazado terminan en la misma posición.
func TestDeleteCollapsingLineStartDoesNotDuplicate(t *testing.T) {
	pt := loadTable(t, "a\nb\nc")

	if _, err := pt.Delete(2, 4); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}
	if got := pt.GetContent(); got != "a\nc" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "a\nc")
	}
	if want := []int{0, 2}; !equalInts(pt.lineOffsets, want) {
		t.Fatalf("lineOffsets = %v, se esperaba %v (sin duplicados)", pt.lineOffsets, want)
	}
}

// TestDeleteMiddleNewlineKeepsLineStartsValid cubre el caso que rompía el índice:
// borrar un rango que contiene un '\n' no debe dejar un inicio de línea apuntando
// a una posición que no está precedida por '\n'.
func TestDeleteMiddleNewlineKeepsLineStartsValid(t *testing.T) {
	pt := loadTable(t, "a\n\nb")

	// Borra el primer '\n' (offset 1). "a\n\nb" tiene arranques [0,2,3].
	// El arranque en 2 no debe sobrevivir mapeado a 1, porque 'a' precede a 1.
	if _, err := pt.Delete(1, 2); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}
	if got := pt.GetContent(); got != "a\nb" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "a\nb")
	}
	if want := []int{0, 2}; !equalInts(pt.lineOffsets, want) {
		t.Fatalf("lineOffsets = %v, se esperaba %v", pt.lineOffsets, want)
	}
	if got := pt.LineCount(); got != 2 {
		t.Fatalf("LineCount() = %d, se esperaba 2", got)
	}
}

func TestGetRangeAfterEdits(t *testing.T) {
	pt := loadTable(t, "uno\ndos\ntres")

	if err := pt.Insert(0, "cero\n"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if _, err := pt.Delete(5, 9); err != nil { // borra "uno\n"
		t.Fatalf("Delete falló: %v", err)
	}

	if got := pt.GetContent(); got != "cero\ndos\ntres" {
		t.Fatalf("contenido = %q", got)
	}
	if got := string(pt.GetRange(0, 1)); got != "cero\n" {
		t.Fatalf("GetRange(0,1) = %q", got)
	}
	if got := string(pt.GetRange(1, 2)); got != "dos\n" {
		t.Fatalf("GetRange(1,2) = %q", got)
	}
	if got := string(pt.GetRange(2, 3)); got != "tres" {
		t.Fatalf("GetRange(2,3) = %q", got)
	}
}

// TestEditingDoesNotMutateTheMappedFile verifica que el mmap se mantiene intacto:
// las ediciones solo crean piezas sobre newBuffer, nunca escriben el original.
func TestEditingDoesNotMutateTheMappedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	const original = "contenido original\nsin tocar\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	pt := NewPieceTable()
	if err := pt.LoadFile(path); err != nil {
		t.Fatalf("LoadFile falló: %v", err)
	}
	defer pt.Close()

	if err := pt.Insert(0, "PREFIJO "); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if _, err := pt.Delete(10, 18); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo releer el archivo: %v", err)
	}
	if string(onDisk) != original {
		t.Fatalf("el archivo en disco cambió: %q", onDisk)
	}
}

// --- Test diferencial contra un string de referencia ---

// TestEditsMatchReferenceString aplica una secuencia determinista de ediciones a
// la PieceTable y a un string común, y compara en cada paso. Es el test que
// atrapa errores de índices: cantidad de líneas, contenido y el invariante de
// lineOffsets se verifican en cada iteración.
func TestEditsMatchReferenceString(t *testing.T) {
	const initial = "linea uno\nlinea dos\nlinea tres\nlinea cuatro\n"
	pt := loadTable(t, initial)
	ref := initial

	// LCG determinista: nada de rand, para que un fallo sea reproducible.
	seed := uint64(0x2545F4914F6CDD1D)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		if n <= 0 {
			return 0
		}
		return int((seed >> 33) % uint64(n))
	}
	fragments := []string{"a", "b", " ", "\n", "é", "日", "XY", "\n\n"}

	for i := 0; i < 400; i++ {
		if next(2) == 0 && len(ref) > 0 {
			start := next(len(ref))
			end := start + next(len(ref)-start)
			if _, err := pt.Delete(start, end); err != nil {
				t.Fatalf("iter %d: Delete(%d,%d) falló: %v", i, start, end, err)
			}
			ref = ref[:start] + ref[end:]
		} else {
			off := next(len(ref) + 1)
			text := fragments[next(len(fragments))]
			if err := pt.Insert(off, text); err != nil {
				t.Fatalf("iter %d: Insert(%d,%q) falló: %v", i, off, text, err)
			}
			ref = ref[:off] + text + ref[off:]
		}

		if got := pt.GetContent(); got != ref {
			t.Fatalf("iter %d: contenido divergente\ngot: %q\nref: %q", i, got, ref)
		}
		if got, want := pt.Len(), len(ref); got != want {
			t.Fatalf("iter %d: Len() = %d, se esperaba %d", i, got, want)
		}
		if got, want := pt.LineCount(), expectedLineCount(ref); got != want {
			t.Fatalf("iter %d: LineCount() = %d, se esperaba %d (contenido %q)", i, got, want, ref)
		}
		checkLineOffsetsInvariant(t, i, pt, ref)

		want := expectedLines(ref)
		if got := pt.LineCount(); got != len(want) {
			t.Fatalf("iter %d: LineCount() = %d, expectedLines = %d", i, got, len(want))
		}
		for li, line := range want {
			if got := string(pt.GetRange(li, li+1)); got != line {
				t.Fatalf("iter %d: GetRange(%d,%d) = %q, se esperaba %q (contenido %q)",
					i, li, li+1, got, line, ref)
			}
		}
	}
}

// expectedLines parte el texto en líneas con la misma semántica que la tabla: un
// '\n' final no genera una línea vacía adicional.
func expectedLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func expectedLineCount(s string) int { return len(expectedLines(s)) }

// checkLineOffsetsInvariant valida el contrato de lineOffsets contra el texto:
// empieza en 0, es estrictamente creciente, no se pasa del documento y cada
// entrada apunta justo después de un '\n'.
func checkLineOffsetsInvariant(t *testing.T, iter int, pt *PieceTable, ref string) {
	t.Helper()

	if len(pt.lineOffsets) == 0 {
		t.Fatalf("iter %d: lineOffsets vacío", iter)
	}
	if pt.lineOffsets[0] != 0 {
		t.Fatalf("iter %d: lineOffsets[0] = %d, se esperaba 0", iter, pt.lineOffsets[0])
	}
	if !sort.IntsAreSorted(pt.lineOffsets) {
		t.Fatalf("iter %d: lineOffsets desordenado: %v", iter, pt.lineOffsets)
	}
	for i := 1; i < len(pt.lineOffsets); i++ {
		if pt.lineOffsets[i] == pt.lineOffsets[i-1] {
			t.Fatalf("iter %d: lineOffsets tiene duplicados: %v", iter, pt.lineOffsets)
		}
	}
	for _, off := range pt.lineOffsets {
		if off < 0 || off > len(ref) {
			t.Fatalf("iter %d: offset de línea %d fuera del documento (%d bytes)", iter, off, len(ref))
		}
		if off > 0 && (off > len(ref) || ref[off-1] != '\n') {
			t.Fatalf("iter %d: offset %d no está precedido por '\\n' (contenido %q)", iter, off, ref)
		}
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestInsertThenDeleteRoundTrips comprueba que insertar y borrar lo insertado
// devuelve el documento al estado original, sin residuos en la tabla.
func TestInsertThenDeleteRoundTrips(t *testing.T) {
	const original = "uno\ndos\ntres"
	pt := loadTable(t, original)

	if err := pt.Insert(4, "insertado\n"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if _, err := pt.Delete(4, 4+len("insertado\n")); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}

	if got := pt.GetContent(); got != original {
		t.Fatalf("contenido = %q, se esperaba %q", got, original)
	}
	if want := []int{0, 4, 8}; !equalInts(pt.lineOffsets, want) {
		t.Fatalf("lineOffsets = %v, se esperaba %v", pt.lineOffsets, want)
	}
}

// TestManyEditsKeepPiecesConsistent verifica que la suma de piezas coincida con
// docLen después de una ráfaga de ediciones.
func TestManyEditsKeepPiecesConsistent(t *testing.T) {
	pt := loadTable(t, strings.Repeat("abcdefghij\n", 20))

	for i := 0; i < 50; i++ {
		if err := pt.Insert(i*3, "Z"); err != nil {
			t.Fatalf("Insert en iteración %d falló: %v", i, err)
		}
	}
	for i := 0; i < 25; i++ {
		if _, err := pt.Delete(i*2, i*2+3); err != nil {
			t.Fatalf("Delete en iteración %d falló: %v", i, err)
		}
	}

	sum := 0
	for _, p := range pt.pieces {
		if p.Len <= 0 {
			t.Fatalf("pieza con longitud %d: las piezas vacías no deben existir", p.Len)
		}
		sum += p.Len
	}
	if sum != pt.docLen {
		t.Fatalf("suma de piezas = %d, docLen = %d", sum, pt.docLen)
	}
	if len(pt.GetContent()) != pt.docLen {
		t.Fatalf("GetContent tiene %d bytes, docLen = %d", len(pt.GetContent()), pt.docLen)
	}
}
