package model

import (
	"testing"
)

// typing es el caso normal: una ráfaga de inserciones contiguas de una runa, como
// la que produce el teclado.
func typing(pt *PieceTable, offset int, text string) {
	for _, r := range text {
		// Se ignoran los errores a propósito: los tests que usan este helper
		// verifican el contenido después, no cada paso.
		_ = pt.Insert(offset, string(r))
		offset += len(string(r))
	}
}

func TestTypingCoalescesIntoOneUndoStep(t *testing.T) {
	pt := loadTable(t, "")

	typing(pt, 0, "hola")
	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("contenido = %q", got)
	}

	// Un solo deshacer tiene que borrar la palabra entera.
	c := mustUndo(t, pt)

	if got := pt.GetContent(); got != "" {
		t.Fatalf("tras un deshacer, contenido = %q, se esperaba vacío", got)
	}
	if c.Offset != 0 || c.Inserted != "hola" {
		t.Fatalf("el cambio fusionado = %+v, se esperaba offset 0 con %q", c, "hola")
	}
	if pt.CanUndo() {
		t.Fatal("la palabra entera debe ser un solo paso")
	}
}

func TestTypingCoalescesWithWideAndCombiningCharacters(t *testing.T) {
	pt := loadTable(t, "")

	typing(pt, 0, "café 日")
	mustUndo(t, pt)

	if got := pt.GetContent(); got != "" {
		t.Fatalf("tras un deshacer, contenido = %q, se esperaba vacío", got)
	}
}

// TestNewlineBreaksTheTypingGroup: el Enter es un paso propio y además corta el
// grupo, así que lo de antes y lo de después son dos pasos de deshacer.
func TestNewlineBreaksTheTypingGroup(t *testing.T) {
	pt := loadTable(t, "")

	typing(pt, 0, "ab")
	if err := pt.Insert(2, "\n"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	typing(pt, 3, "cd")

	if got := pt.GetContent(); got != "ab\ncd" {
		t.Fatalf("contenido = %q", got)
	}

	mustUndo(t, pt)
	if got := pt.GetContent(); got != "ab\n" {
		t.Fatalf("tras un deshacer, %q, se esperaba %q", got, "ab\n")
	}
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "ab" {
		t.Fatalf("tras dos deshaceres, %q, se esperaba %q", got, "ab")
	}
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "" {
		t.Fatalf("tras tres deshaceres, %q", got)
	}
}

// TestNonContiguousTypingDoesNotCoalesce: si el cursor se movió, lo que se escribe
// después no puede fusionarse aunque el texto quede pegado.
func TestNonContiguousTypingDoesNotCoalesce(t *testing.T) {
	pt := loadTable(t, "")

	typing(pt, 0, "uno")
	// Se escribe en otra posición: no es contiguo con lo anterior.
	typing(pt, 0, "dos")

	if got := pt.GetContent(); got != "dosuno" {
		t.Fatalf("contenido = %q", got)
	}

	mustUndo(t, pt)
	if got := pt.GetContent(); got != "uno" {
		t.Fatalf("tras un deshacer, %q, se esperaba %q", got, "uno")
	}
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "" {
		t.Fatalf("tras dos deshaceres, %q", got)
	}
}

// TestCursorMovementBreaksTheTypingGroup es el caso que la contigüidad sola no
// cubre: moverse y volver a la misma posición tiene que separar los grupos igual.
func TestCursorMovementBreaksTheTypingGroup(t *testing.T) {
	pt := loadTable(t, "")

	typing(pt, 0, "abc")

	// El cursor se mueve y vuelve al mismo lugar: el view llama a esto en cada
	// movimiento. El offset de la próxima inserción es contiguo, así que sin el
	// corte explícito se fusionaría.
	pt.BreakTypingGroup()

	typing(pt, 3, "d")
	if got := pt.GetContent(); got != "abcd" {
		t.Fatalf("contenido = %q", got)
	}

	mustUndo(t, pt)
	if got := pt.GetContent(); got != "abc" {
		t.Fatalf("tras un deshacer, %q, se esperaba %q: el movimiento separó los grupos", got, "abc")
	}
}

func TestDeletingBreaksTheTypingGroup(t *testing.T) {
	pt := loadTable(t, "")

	typing(pt, 0, "ab")
	applyDelete(t, pt, 1, 2) // borra la 'b'
	typing(pt, 1, "c")

	if got := pt.GetContent(); got != "ac" {
		t.Fatalf("contenido = %q", got)
	}

	// Tres pasos: la ráfaga "ab", el borrado y la inserción posterior. Que el
	// borrado sea un paso propio es justamente lo que se quiere verificar.
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "a" {
		t.Fatalf("tras un deshacer, %q, se esperaba %q", got, "a")
	}
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "ab" {
		t.Fatalf("tras dos deshaceres, %q, se esperaba %q", got, "ab")
	}
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "" {
		t.Fatalf("tras tres deshaceres, %q, se esperaba vacío", got)
	}
}

// TestUndoBreaksTheTypingGroup evita que lo escrito después de deshacer se fusione
// con un cambio que ya quedó atrás en el historial.
func TestUndoBreaksTheTypingGroup(t *testing.T) {
	pt := loadTable(t, "")

	typing(pt, 0, "ab")
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "" {
		t.Fatalf("tras deshacer, %q", got)
	}

	typing(pt, 0, "cd")
	mustUndo(t, pt)

	if got := pt.GetContent(); got != "" {
		t.Fatalf("tras deshacer lo nuevo, %q, se esperaba vacío", got)
	}
	// Y el historial tiene que haber conservado la rama deshecha para rehacer.
	if !pt.CanRedo() {
		t.Fatal("debería haber rama para rehacer")
	}
}

func TestRedoBreaksTheTypingGroup(t *testing.T) {
	pt := loadTable(t, "")

	typing(pt, 0, "ab")
	mustUndo(t, pt)
	mustRedo(t, pt) // vuelve "ab" y corta el grupo

	typing(pt, 2, "c")

	if got := pt.GetContent(); got != "abc" {
		t.Fatalf("contenido = %q", got)
	}
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "ab" {
		t.Fatalf("tras un deshacer, %q, se esperaba %q", got, "ab")
	}
}

// TestCoalescedTypingStaysInvertible comprueba que fusionar no rompa la
// reversibilidad: deshacer y rehacer una palabra fusionada tiene que ser exacto.
func TestCoalescedTypingStaysInvertible(t *testing.T) {
	const initial = "base\n"
	pt := loadTable(t, initial)

	typing(pt, 0, "prefijo ")
	typing(pt, len("prefijo base\n"), " sufijo")

	want := "prefijo base\n sufijo"
	if got := pt.GetContent(); got != want {
		t.Fatalf("contenido = %q, se esperaba %q", got, want)
	}

	mustUndo(t, pt)
	mustUndo(t, pt)
	if got := pt.GetContent(); got != initial {
		t.Fatalf("tras deshacer todo, %q, se esperaba %q", got, initial)
	}

	mustRedo(t, pt)
	mustRedo(t, pt)
	if got := pt.GetContent(); got != want {
		t.Fatalf("tras rehacer todo, %q, se esperaba %q", got, want)
	}
}

// TestCoalescedTypingKeepsTheLineIndexInShape: fusionar no debe saltarse el
// mantenimiento del índice de líneas, que ocurre en insertRaw y no en record.
func TestCoalescedTypingKeepsTheLineIndexInShape(t *testing.T) {
	pt := loadTable(t, "uno\ndos")

	// Inserta un salto y después texto contiguo: el salto corta el grupo, pero el
	// índice tiene que quedar bien en todos los pasos.
	typing(pt, 7, "\ntres")

	want := "uno\ndos\ntres"
	if got := pt.GetContent(); got != want {
		t.Fatalf("contenido = %q, se esperaba %q", got, want)
	}
	if got, expected := pt.LineCount(), len(expectedLines(want)); got != expected {
		t.Fatalf("LineCount() = %d, se esperaba %d", got, expected)
	}
	checkLineOffsetsInvariant(t, 0, pt, want)

	mustUndo(t, pt) // el texto después del salto
	if got := pt.GetContent(); got != "uno\ndos\n" {
		t.Fatalf("tras un deshacer, %q", got)
	}
	mustUndo(t, pt) // el salto
	if got := pt.GetContent(); got != "uno\ndos" {
		t.Fatalf("tras dos deshaceres, %q", got)
	}
	if got := pt.LineCount(); got != 2 {
		t.Fatalf("LineCount() = %d, se esperaba 2", got)
	}
}
