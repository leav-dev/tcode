package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// mouseEvent crea un evento de mouse del botón dado en la celda (x, y).
func mouseEvent(x, y int, btns tcell.ButtonMask, mod tcell.ModMask) *tcell.EventMouse {
	return tcell.NewEventMouse(x, y, btns, mod)
}

// modEvent crea una tecla con opción de Shift.
func modEvent(key tcell.Key, shift bool) *tcell.EventKey {
	m := tcell.ModNone
	if shift {
		m = tcell.ModShift
	}
	return tcell.NewEventKey(key, 0, m)
}

// --- selección con MOUSE: drag, clic y Shift+clic (gutter-aware) ---

// TestMouseDragSelectsWithGutterRespected: presionar el botón 1 en una celda
// de TEXTO ancla (sin seleccionar todavía); arrastrar extiende una selección
// viva desde el ancla; soltar la deja marcada. Las coordenadas de pantalla se
// traducen restando el gutter: la celda gutterWidth+3 es la columna 3 del
// texto, es decir el offset de documento 3. Un press sobre el gutter se ignora
// entero: no ancla, no arma arrastre.
func TestMouseDragSelectsWithGutterRespected(t *testing.T) {
	v := newTestView(t, "hola mundo", 20, 2)
	gutter := v.gutterWidth()

	// El press sobre el gutter (x < gutterWidth) no arranca selección.
	if v.HandleEvent(mouseEvent(gutter-1, 0, tcell.Button1, tcell.ModNone)) {
		t.Fatal("un press sobre el gutter no debe manejarse")
	}
	if v.mouseDown {
		t.Fatal("el press sobre el gutter no debe armar el arrastre")
	}
	if v.SelectionActive() {
		t.Fatal("el press sobre el gutter no debe seleccionar")
	}

	// Press en la celda 0 del texto (x = gutterWidth): ancla, sin rango aún.
	v.HandleEvent(mouseEvent(gutter, 0, tcell.Button1, tcell.ModNone))
	if v.SelectionActive() {
		t.Fatal("el press solo no debe seleccionar todavía (ancla sin rango)")
	}
	if got := v.cursor.ByteCol; got != 0 {
		t.Fatalf("cursor col = %d, esperaba 0 (celda de texto 0)", got)
	}

	// Arrastre a la columna 3 del texto: la celda gutterWidth+3.
	v.HandleEvent(mouseEvent(gutter+3, 0, tcell.Button1, tcell.ModNone))
	if start, end, ok := v.SelectionRange(); !ok || start != 0 || end != 3 {
		t.Fatalf("rango tras arrastrar = [%d,%d) ok=%v, esperaba [0,3)", start, end, ok)
	}
	if got := v.SelectionText(); got != "hol" {
		t.Fatalf("selección = %q, esperaba %q", got, "hol")
	}

	// Soltar el botón: el drag termina y la selección queda marcada.
	v.HandleEvent(mouseEvent(gutter+3, 0, tcell.ButtonNone, tcell.ModNone))
	if !v.SelectionActive() {
		t.Fatal("al soltar la selección debe quedar marcada")
	}
	if got := v.SelectionText(); got != "hol" {
		t.Fatalf("selección final = %q, esperaba %q", got, "hol")
	}
}

// TestMouseClickClearsSelection: un clic simple (press + release sin motion)
// posiciona el cursor y LIMPIA la selección previa: sin drag no queda marca.
func TestMouseClickClearsSelection(t *testing.T) {
	v := newTestView(t, "hola mundo", 20, 2)
	gutter := v.gutterWidth()

	// Armar una selección con un drag: [0,3) = "hol".
	v.HandleEvent(mouseEvent(gutter, 0, tcell.Button1, tcell.ModNone))
	v.HandleEvent(mouseEvent(gutter+3, 0, tcell.Button1, tcell.ModNone))
	v.HandleEvent(mouseEvent(gutter+3, 0, tcell.ButtonNone, tcell.ModNone))
	if !v.SelectionActive() {
		t.Fatal("el drag previo debió dejar selección activa")
	}

	// Clic simple en la columna 5 del texto (celda gutterWidth+5).
	at := gutter + 5
	v.HandleEvent(mouseEvent(at, 0, tcell.Button1, tcell.ModNone))
	v.HandleEvent(mouseEvent(at, 0, tcell.ButtonNone, tcell.ModNone))

	if v.SelectionActive() {
		t.Fatal("un clic sin motion debe limpiar la selección")
	}
	if got := v.cursor.ByteCol; got != 5 {
		t.Fatalf("cursor col = %d, esperaba 5 (columna del texto clickeada)", got)
	}
}

// TestMouseShiftClickExtendsSelection: Shift+clic NO limpia: extiende la
// selección desde el ancla previo (VSCode-like), o la arranca desde el cursor
// cuando no hay ancla.
func TestMouseShiftClickExtendsSelection(t *testing.T) {
	v := newTestView(t, "hola mundo", 20, 2)
	gutter := v.gutterWidth()

	// Selección previa con un drag: [0,2) = "ho", el ancla queda en 0.
	v.HandleEvent(mouseEvent(gutter, 0, tcell.Button1, tcell.ModNone))
	v.HandleEvent(mouseEvent(gutter+2, 0, tcell.Button1, tcell.ModNone))
	v.HandleEvent(mouseEvent(gutter+2, 0, tcell.ButtonNone, tcell.ModNone))
	if got := v.SelectionText(); got != "ho" {
		t.Fatalf("selección previa = %q, esperaba %q", got, "ho")
	}

	// Shift+clic en la columna 6 del texto: el rango crece del ancla 0 a la
	// columna 6 → [0,6) = "hola m".
	v.HandleEvent(mouseEvent(gutter+6, 0, tcell.Button1, tcell.ModShift))
	if got := v.SelectionText(); got != "hola m" {
		t.Fatalf("selección tras Shift+clic = %q, esperaba %q", got, "hola m")
	}
	v.HandleEvent(mouseEvent(gutter+6, 0, tcell.ButtonNone, tcell.ModNone))

	// Sin ancla previo (el clic normal limpió), Shift+clic la ARRANCA desde
	// el cursor: clic en la columna 3, luego Shift+clic en la 8 → [3,8).
	v.HandleEvent(mouseEvent(gutter+3, 0, tcell.Button1, tcell.ModNone))
	v.HandleEvent(mouseEvent(gutter+3, 0, tcell.ButtonNone, tcell.ModNone))
	if v.SelectionActive() {
		t.Fatal("el clic normal debió limpiar la selección")
	}
	v.HandleEvent(mouseEvent(gutter+8, 0, tcell.Button1, tcell.ModShift))
	v.HandleEvent(mouseEvent(gutter+8, 0, tcell.ButtonNone, tcell.ModNone))
	if start, end, ok := v.SelectionRange(); !ok || start != 3 || end != 8 {
		t.Fatalf("rango = [%d,%d) ok=%v, esperaba [3,8)", start, end, ok)
	}
}

// TestMouseDragSelectionStyledInRender: la fila seleccionada por DRAG se pinta
// con el rol Selection en las celdas del rango; el gutter conserva sus
// números, y la celda fuera del rango no queda marcada.
func TestMouseDragSelectionStyledInRender(t *testing.T) {
	s := newTestScreen(t, 20, 2)
	v := newTestView(t, "hola mundo", 20, 2)
	gutter := v.gutterWidth()

	// Drag de [0,3): "hol".
	v.HandleEvent(mouseEvent(gutter, 0, tcell.Button1, tcell.ModNone))
	v.HandleEvent(mouseEvent(gutter+3, 0, tcell.Button1, tcell.ModNone))
	v.HandleEvent(mouseEvent(gutter+3, 0, tcell.ButtonNone, tcell.ModNone))
	draw(v, s)

	cells, width, _ := s.GetContents()
	for _, x := range []int{gutter, gutter + 1, gutter + 2} {
		if cells[0*width+x].Style != DefaultTheme().Selection {
			t.Fatalf("la celda %d del rango debe llevar el estilo Selection", x)
		}
	}
	if cells[0*width+gutter+3].Style == DefaultTheme().Selection {
		t.Fatal("la celda fuera del rango no debe estar seleccionada")
	}
	// El gutter sigue pintando sus números (la selección no lo pisa).
	if got := cellRuneAt(s, 0, 0); got != '1' {
		t.Fatalf("gutter (0,0) = %q, se esperaba '1'", got)
	}
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
