package model

import (
	"testing"
)

// setUndoCap baja el tope del historial para el test y lo restaura al final.
func setUndoCap(t *testing.T, n int) {
	t.Helper()
	old := maxUndoHistory
	maxUndoHistory = n
	t.Cleanup(func() { maxUndoHistory = old })
}

// TestUndoHistoryIsCapped: el historial no crece sin límite; los cambios que
// caen del frente ya no se pueden deshacer, pero su efecto queda en el
// documento (deshacer todo no vuelve al inicio).
func TestUndoHistoryIsCapped(t *testing.T) {
	setUndoCap(t, 3)

	pt := NewPieceTable()
	for i := 0; i < 5; i++ {
		if err := pt.Insert(pt.Len(), "x\n"); err != nil {
			t.Fatalf("Insert %d falló: %v", i, err)
		}
		pt.BreakTypingGroup() // saltos de línea ya no se fusionan; el break lo garantiza
	}

	if len(pt.undo) != 3 {
		t.Fatalf("undo = %d, esperaba el tope (3)", len(pt.undo))
	}

	for pt.CanUndo() {
		if _, ok, err := pt.Undo(); err != nil || !ok {
			t.Fatalf("Undo falló: ok=%v err=%v", ok, err)
		}
	}

	// Los 3 últimos se deshicieron; los 2 primeros quedaron aplicados para
	// siempre (ya no hay historial para volver a ese estado).
	if got := docText(pt); got != "x\nx\n" {
		t.Fatalf("doc = %q, esperaba el efecto de los 2 cambios descartados", got)
	}
}

// TestUndoCapAdjustsSavedPoint: al descartar del frente, el punto de guardado
// (índice en undo) corre con el historial; cuando el guardado era el más viejo
// y cae, pasa a noSavedAt y el documento queda legítimamente modificado.
func TestUndoCapAdjustsSavedPoint(t *testing.T) {
	setUndoCap(t, 3)

	pt := NewPieceTable()
	for i := 0; i < 3; i++ {
		if err := pt.Insert(pt.Len(), "a"); err != nil {
			t.Fatalf("Insert %d falló: %v", i, err)
		}
		pt.BreakTypingGroup()
	}
	// Estado guardado: el documento coincide con el historial completo (3).
	pt.savedAt = len(pt.undo)
	if pt.Modified() {
		t.Fatal("recién guardado no puede estar modificado")
	}

	// 4to cambio: el más viejo cae del frente y savedAt corre de 3 a 2.
	pt.Insert(pt.Len(), "b")
	pt.BreakTypingGroup()
	if pt.savedAt != 2 {
		t.Fatalf("savedAt = %d, esperaba 2 tras el primer descarte", pt.savedAt)
	}

	// 5to: savedAt 2 -> 1.
	pt.Insert(pt.Len(), "c")
	pt.BreakTypingGroup()
	if pt.savedAt != 1 {
		t.Fatalf("savedAt = %d, esperaba 1 tras el segundo descarte", pt.savedAt)
	}

	// 6to: el punto vive en el índice 1 y corre a 0. savedAt 0 sigue siendo
	// EXACTAMENTE alcanzable: deshaciendo d, c y b el documento vuelve al
	// estado guardado (a1..a3 quedaron aplicados en la base, caídos del
	// historial, y no hace falta deshacerlos).
	pt.Insert(pt.Len(), "d")
	pt.BreakTypingGroup()
	if pt.savedAt != 0 {
		t.Fatalf("savedAt = %d, esperaba 0 (todavía alcanzable)", pt.savedAt)
	}
	pt.Insert(pt.Len(), "e")
	pt.BreakTypingGroup()
	if pt.savedAt != noSavedAt {
		t.Fatalf("savedAt = %d, esperaba noSavedAt al caer el último cambio del estado guardado", pt.savedAt)
	}
	if !pt.Modified() {
		t.Fatal("sin punto de guardado el documento debe estar modificado")
	}
}

// TestRedoWithinCapStillWorks: deshacer/rehacer dentro del tope se comporta
// exactamente como antes; el tope no toca la semántica viva del historial.
func TestRedoWithinCapStillWorks(t *testing.T) {
	setUndoCap(t, 3)

	pt := NewPieceTable()
	for i := 0; i < 3; i++ {
		if err := pt.Insert(pt.Len(), "a"); err != nil {
			t.Fatalf("Insert %d falló: %v", i, err)
		}
		pt.BreakTypingGroup()
	}

	for pt.CanUndo() {
		if _, ok, err := pt.Undo(); err != nil || !ok {
			t.Fatalf("Undo falló: %v", err)
		}
	}
	if got := docText(pt); got != "" {
		t.Fatalf("tras deshacer todo el doc debería estar vacío, es %q", got)
	}

	for pt.CanRedo() {
		if _, ok, err := pt.Redo(); err != nil || !ok {
			t.Fatalf("Redo falló: %v", err)
		}
	}
	if got := docText(pt); got != "aaa" {
		t.Fatalf("tras rehacer todo el doc = %q, esperaba aaa", got)
	}
	if len(pt.undo) != 3 || len(pt.redo) != 0 {
		t.Fatalf("historiales = undo %d / redo %d, esperaba 3/0", len(pt.undo), len(pt.redo))
	}
}
