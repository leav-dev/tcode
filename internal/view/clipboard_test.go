package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestCopySelectionWritesClipboard: copiar con selección escribe el texto en el
// portapapeles (mock) y no toca el documento.
func TestCopySelectionWritesClipboard(t *testing.T) {
	var wrote string
	old := writeClipboard
	writeClipboard = func(text string) error { wrote = text; return nil }
	defer func() { writeClipboard = old }()

	v := newTestView(t, "hola mundo", 20, 2)
	v.setCursorAt(0)
	v.HandleEvent(modEvent(tcell.KeyRight, true))
	v.HandleEvent(modEvent(tcell.KeyRight, true))

	if err := v.CopySelection(); err != nil {
		t.Fatalf("CopySelection falló: %v", err)
	}
	if wrote != "ho" {
		t.Fatalf("portapapeles = %q, esperaba %q", wrote, "ho")
	}
	if got := v.model.GetContent(); got != "hola mundo" {
		t.Fatalf("el documento cambió tras copiar: %q", got)
	}
}

// TestCopyWithoutSelectionDoesNothing: sin selección no toca el portapapeles.
func TestCopyWithoutSelectionDoesNothing(t *testing.T) {
	called := false
	old := writeClipboard
	writeClipboard = func(text string) error { called = true; return nil }
	defer func() { writeClipboard = old }()

	v := newTestView(t, "hola", 20, 2)
	if err := v.CopySelection(); err != nil {
		t.Fatalf("CopySelection sin selección falló: %v", err)
	}
	if called {
		t.Fatal("sin selección no debe escribir el portapapeles")
	}
}

// TestPasteInsertsClipboardText: Ctrl+V inserta el contenido del portapapeles.
func TestPasteInsertsClipboardText(t *testing.T) {
	old := readClipboard
	readClipboard = func() (string, error) { return "pegado", nil }
	defer func() { readClipboard = old }()

	v := newTestView(t, "hola", 20, 2)
	v.handleKey(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModNone))

	if got := v.model.GetContent(); got != "pegadohola" {
		t.Fatalf("contenido = %q, esperaba %q (el cursor quedó al inicio)", got, "pegadohola")
	}
	if v.SelectionActive() {
		t.Fatal("el pegado no debe dejar selección")
	}
}

// TestPasteReplacesSelection: pegar con selección la reemplaza.
func TestPasteReplacesSelection(t *testing.T) {
	old := readClipboard
	readClipboard = func() (string, error) { return "XX", nil }
	defer func() { readClipboard = old }()

	v := newTestView(t, "hola", 20, 2)
	v.setCursorAt(0)
	v.HandleEvent(modEvent(tcell.KeyRight, true))
	v.HandleEvent(modEvent(tcell.KeyRight, true)) // sel "ho"
	v.handleKey(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModNone))

	if got := v.model.GetContent(); got != "XXla" {
		t.Fatalf("contenido = %q, esperaba %q", got, "XXla")
	}
}
