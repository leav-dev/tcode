package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// modEvent crea una tecla con opción de Shift.
func modEvent(key tcell.Key, shift bool) *tcell.EventKey {
	m := tcell.ModNone
	if shift {
		m = tcell.ModShift
	}
	return tcell.NewEventKey(key, 0, m)
}

// TestShiftArrowsExtendSelection: Shift+derecha marca, y la extensión sigue
// desde el ancla con los movimientos posteriores.
func TestShiftArrowsExtendSelection(t *testing.T) {
	v := newTestView(t, "abcd\nefgh", 20, 4)

	v.HandleEvent(modEvent(tcell.KeyRight, true)) // (0,1), sel [0,1]
	v.HandleEvent(modEvent(tcell.KeyRight, true)) // (0,2), sel [0,2]
	if got := v.SelectionText(); got != "ab" {
		t.Fatalf("selección = %q, esperaba %q", got, "ab")
	}
	// Shift+Down extiende desde el ancla (0,0) hasta (1,2): offset 6.
	v.HandleEvent(modEvent(tcell.KeyDown, true))
	if got := v.SelectionText(); got != "abcd\nef" {
		t.Fatalf("selección = %q, esperaba %q", got, "abcd\nef")
	}
}

// TestMoveWithoutShiftClearsSelection: cualquier movimiento sin Shift limpia.
func TestMoveWithoutShiftClearsSelection(t *testing.T) {
	v := newTestView(t, "abcd\nefgh", 20, 4)

	v.HandleEvent(modEvent(tcell.KeyRight, true))
	if !v.SelectionActive() {
		t.Fatal("la selección debería estar activa tras Shift+derecha")
	}
	v.HandleEvent(modEvent(tcell.KeyRight, false))
	if v.SelectionActive() {
		t.Fatal("un movimiento sin Shift debe limpiar la selección")
	}
	if got := v.cursor.ByteCol; got != 2 {
		t.Fatalf("cursor col = %d, esperaba 2", got)
	}
}

// TestShiftHomeSelectsTheLineStart: Shift+Home arrastra el cursor al inicio de
// la línea y la selección cubre [inicio, ancla).
func TestShiftHomeSelectsTheLineStart(t *testing.T) {
	v := newTestView(t, "abcd\nefgh", 20, 4)
	v.setCursorAt(7) // (1,2): la línea 1 arranca en el byte 5 ('f'=6, 'g'=7)

	v.HandleEvent(modEvent(tcell.KeyHome, true))
	if got := v.SelectionText(); got != "ef" {
		t.Fatalf("selección = %q, esperaba %q", got, "ef")
	}
}

// TestCtrlASelectsEverything: Ctrl+A marca todo y deja el cursor al final.
func TestCtrlASelectsEverything(t *testing.T) {
	v := newTestView(t, "abcd\nefgh", 20, 4)

	if !v.handleKey(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModNone)) {
		t.Fatal("Ctrl+A no se manejó")
	}
	docLen := v.model.Len()
	if got := v.SelectionText(); got != "abcd\nefgh" {
		t.Fatalf("selección = %q, esperaba el documento completo", got)
	}
	if v.cursor.Line != 1 || v.cursor.ByteCol != 4 {
		t.Fatalf("cursor = (%d,%d), esperaba el final del doc", v.cursor.Line, v.cursor.ByteCol)
	}
	if v.sel.Start != 0 || v.sel.End != docLen {
		t.Fatalf("sel = [%d,%d), esperaba [0,%d)", v.sel.Start, v.sel.End, docLen)
	}
}
