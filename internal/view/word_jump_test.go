package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func ctrlKey(key tcell.Key) tcell.EventKey {
	return *tcell.NewEventKey(key, 0, tcell.ModCtrl)
}

func ctrlShiftKey(key tcell.Key) tcell.EventKey {
	return *tcell.NewEventKey(key, 0, tcell.ModCtrl|tcell.ModShift)
}

func TestCtrlRightJumpsToWordEnds(t *testing.T) {
	v := newTestView(t, "foo bar", 20, 2)

	ev := ctrlKey(tcell.KeyRight)
	v.HandleEvent(&ev)
	if v.cursor.Line != 0 || v.cursor.ByteCol != 3 {
		t.Fatalf("cursor = (%d,%d), se esperaba (0,3) fin de foo", v.cursor.Line, v.cursor.ByteCol)
	}
	ev2 := ctrlKey(tcell.KeyRight)
	v.HandleEvent(&ev2)
	if v.cursor.ByteCol != 7 {
		t.Fatalf("ByteCol = %d, se esperaba 7 fin de bar", v.cursor.ByteCol)
	}
}

func TestCtrlLeftJumpsToWordStarts(t *testing.T) {
	v := newTestView(t, "foo bar", 20, 2)
	// Llevar al final con dos saltos.
	ev := ctrlKey(tcell.KeyRight)
	v.HandleEvent(&ev)
	ev2 := ctrlKey(tcell.KeyRight)
	v.HandleEvent(&ev2)

	back := ctrlKey(tcell.KeyLeft)
	v.HandleEvent(&back)
	if v.cursor.ByteCol != 4 {
		t.Fatalf("ByteCol = %d, se esperaba 4 inicio de bar", v.cursor.ByteCol)
	}
	back2 := ctrlKey(tcell.KeyLeft)
	v.HandleEvent(&back2)
	if v.cursor.ByteCol != 0 {
		t.Fatalf("ByteCol = %d, se esperaba 0 inicio de foo", v.cursor.ByteCol)
	}
}

func TestCtrlRightTreatsSymbolsAsSeparators(t *testing.T) {
	v := newTestView(t, "foo.bar(baz)", 30, 2)

	ev := ctrlKey(tcell.KeyRight)
	v.HandleEvent(&ev)
	if v.cursor.ByteCol != 3 {
		t.Fatalf("ByteCol = %d, se esperaba 3 fin de foo", v.cursor.ByteCol)
	}
	ev2 := ctrlKey(tcell.KeyRight)
	v.HandleEvent(&ev2)
	if v.cursor.ByteCol != 7 {
		t.Fatalf("ByteCol = %d, se esperaba 7 fin de bar", v.cursor.ByteCol)
	}
}

func TestCtrlArrowsCrossLines(t *testing.T) {
	v := newTestView(t, "foo\nbar", 20, 3)

	// Desde el fin de foo, el salto cruza a fin de bar.
	ev := ctrlKey(tcell.KeyRight)
	v.HandleEvent(&ev) // (0,3) fin de foo
	ev2 := ctrlKey(tcell.KeyRight)
	v.HandleEvent(&ev2)
	if v.cursor.Line != 1 || v.cursor.ByteCol != 3 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,3) fin de bar", v.cursor.Line, v.cursor.ByteCol)
	}

	back := ctrlKey(tcell.KeyLeft)
	v.HandleEvent(&back)
	if v.cursor.Line != 1 || v.cursor.ByteCol != 0 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,0) inicio de bar", v.cursor.Line, v.cursor.ByteCol)
	}
}

func TestCtrlShiftRightExtendsSelection(t *testing.T) {
	v := newTestView(t, "foo bar", 20, 2)

	ev := ctrlShiftKey(tcell.KeyRight)
	v.HandleEvent(&ev)
	if !v.SelectionActive() {
		t.Fatal("se esperaba selección activa con Ctrl+Shift+Right")
	}
	start, end, _ := v.SelectionRange()
	if start != 0 || end != 3 {
		t.Fatalf("selección = [%d,%d), se esperaba [0,3)", start, end)
	}
}
