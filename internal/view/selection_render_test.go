package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestSelectionStyledInRender: los clusters dentro del rango se pintan con el
// rol Selection; los de afuera, con su estilo normal.
func TestSelectionStyledInRender(t *testing.T) {
	s := newTestScreen(t, 20, 2)
	v := newTestView(t, "hola mundo", 20, 2)

	// Seleccionar "hola" (bytes 0..4).
	v.setCursorAt(0)
	v.HandleEvent(modEvent(tcell.KeyRight, true))
	v.HandleEvent(modEvent(tcell.KeyRight, true))
	v.HandleEvent(modEvent(tcell.KeyRight, true))
	v.HandleEvent(modEvent(tcell.KeyRight, true))
	draw(v, s)

	cells, width, _ := s.GetContents()
	if cells[0*width+0].Style != DefaultTheme().Selection {
		t.Fatal("la 'h' seleccionada debe llevar el estilo Selection")
	}
	if cells[0*width+2].Style != DefaultTheme().Selection {
		t.Fatal("la 'l' seleccionada debe llevar el estilo Selection")
	}
	if cells[0*width+5].Style == DefaultTheme().Selection {
		t.Fatal("el espacio fuera del rango no debe estar seleccionado")
	}
}

// TestTypingReplacesSelection: escribir con selección borra el rango e inserta
// el texto nuevo en su lugar (VSCode-like).
func TestTypingReplacesSelection(t *testing.T) {
	v := newTestView(t, "hola mundo", 20, 2)

	v.setCursorAt(0)
	v.HandleEvent(modEvent(tcell.KeyRight, true)) // sel [0,1]
	v.handleKey(evRune('X').(*tcell.EventKey))    // reemplaza la 'h'

	if got := v.model.GetContent(); got != "Xola mundo" {
		t.Fatalf("contenido = %q, esperaba %q", got, "Xola mundo")
	}
	if v.SelectionActive() {
		t.Fatal("tras reemplazar no debe quedar selección")
	}
}

// TestBackspaceReplacesSelection: Backspace con selección borra el rango.
func TestBackspaceReplacesSelection(t *testing.T) {
	v := newTestView(t, "hola mundo", 20, 2)

	v.setCursorAt(0)
	v.HandleEvent(modEvent(tcell.KeyRight, true))
	v.HandleEvent(modEvent(tcell.KeyRight, true)) // sel [0,2] "ho"
	v.backspace()

	if got := v.model.GetContent(); got != "la mundo" {
		t.Fatalf("contenido = %q, esperaba %q", got, "la mundo")
	}
}

// TestDeleteReplacesSelection: Delete con selección borra el rango.
func TestDeleteReplacesSelection(t *testing.T) {
	v := newTestView(t, "hola mundo", 20, 2)

	v.setCursorAt(5)
	v.HandleEvent(modEvent(tcell.KeyRight, true))
	v.HandleEvent(modEvent(tcell.KeyRight, true)) // sel [5,7] "mu"
	v.deleteForward()

	if got := v.model.GetContent(); got != "hola ndo" {
		t.Fatalf("contenido = %q, esperaba %q", got, "hola ndo")
	}
}

// TestMouseDragSelects: presionar el botón 1 ancla y arrastrar extiende la
// selección desde ese punto; soltar la deja marcada.
func TestMouseDragSelects(t *testing.T) {
	v := newTestView(t, "hola mundo", 20, 2)

	press := tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone)
	v.HandleEvent(press) // ancla en (0,0)
	if v.SelectionActive() {
		t.Fatal("el press solo no debe seleccionar todavía")
	}

	drag := tcell.NewEventMouse(3, 0, tcell.Button1, tcell.ModNone)
	v.HandleEvent(drag) // arrastre hasta (3,0)
	if got := v.SelectionText(); got != "hol" {
		t.Fatalf("selección tras arrastrar = %q, esperaba %q", got, "hol")
	}

	release := tcell.NewEventMouse(3, 0, tcell.ButtonNone, tcell.ModNone)
	v.HandleEvent(release) // soltar: termina el arrastre
	if !v.SelectionActive() {
		t.Fatal("al soltar la selección debe quedar marcada")
	}
	if got := v.SelectionText(); got != "hol" {
		t.Fatalf("selección final = %q, esperaba %q", got, "hol")
	}
}

// TestMouseClickDoesNotSelect: un clic (press + release sin movimiento) no
// arma selección y solo posiciona.
func TestMouseClickDoesNotSelect(t *testing.T) {
	v := newTestView(t, "hola mundo", 20, 2)

	v.HandleEvent(tcell.NewEventMouse(1, 0, tcell.Button1, tcell.ModNone))
	v.HandleEvent(tcell.NewEventMouse(1, 0, tcell.ButtonNone, tcell.ModNone))

	if v.SelectionActive() {
		t.Fatal("un clic no debe dejar selección")
	}
	if v.cursor.ByteCol != 1 {
		t.Fatalf("cursor col = %d, esperaba 1", v.cursor.ByteCol)
	}
}
