package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// --- movimiento horizontal ---

func TestCursorMovesByGraphemeClusterNotByRune(t *testing.T) {
	// "e" + acento combinante + "x": el acento no es un paso propio.
	v := newTestView(t, "e\u0301x", 20, 1)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if v.cursor.ByteCol != 3 {
		t.Fatalf("ByteCol = %d, se esperaba 3: el cluster 'e+acento' ocupa 3 bytes y es un paso", v.cursor.ByteCol)
	}
	v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if v.cursor.ByteCol != 4 {
		t.Fatalf("ByteCol = %d, se esperaba 4", v.cursor.ByteCol)
	}
}

func TestCursorStepsOverWideCharactersAsOneUnit(t *testing.T) {
	v := newTestView(t, "日a", 20, 1)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if v.cursor.ByteCol != 3 {
		t.Fatalf("ByteCol = %d, se esperaba 3: '日' ocupa 3 bytes y es un paso", v.cursor.ByteCol)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if v.cursor.ByteCol != 0 {
		t.Fatalf("ByteCol = %d, se esperaba 0", v.cursor.ByteCol)
	}
}

func TestCursorWrapsBetweenLines(t *testing.T) {
	v := newTestView(t, "uno\ndos", 20, 2)

	// Derecha desde el final de la línea 0 pasa al inicio de la línea 1.
	for i := 0; i < 3; i++ {
		v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	if v.cursor.Line != 0 || v.cursor.ByteCol != 3 {
		t.Fatalf("cursor = (%d,%d), se esperaba (0,3)", v.cursor.Line, v.cursor.ByteCol)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if v.cursor.Line != 1 || v.cursor.ByteCol != 0 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,0) al cruzar de línea", v.cursor.Line, v.cursor.ByteCol)
	}

	// Izquierda desde el inicio de la línea 1 vuelve al final de la línea 0.
	v.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if v.cursor.Line != 0 || v.cursor.ByteCol != 3 {
		t.Fatalf("cursor = (%d,%d), se esperaba (0,3)", v.cursor.Line, v.cursor.ByteCol)
	}
}

func TestCursorStopsAtDocumentBounds(t *testing.T) {
	v := newTestView(t, "uno\ndos", 20, 2)

	if v.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)) {
		t.Fatal("izquierda al inicio del documento no debería cambiar nada")
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))
	if v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)) {
		t.Fatal("derecha al final del documento no debería cambiar nada")
	}
}

// --- movimiento vertical y columna deseada ---

func TestVerticalMovementKeepsDesiredColumn(t *testing.T) {
	// La línea 0 es larga, la 1 es corta y la 2 vuelve a ser larga.
	v := newTestView(t, "abcdefgh\nab\nabcdefgh", 20, 3)

	for i := 0; i < 6; i++ {
		v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	if v.cursor.ByteCol != 6 {
		t.Fatalf("ByteCol = %d, se esperaba 6", v.cursor.ByteCol)
	}

	// Baja a la línea corta: el cursor queda al final, pero recuerda la columna 6.
	v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if v.cursor.Line != 1 || v.cursor.ByteCol != 2 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,2) clampeado a la línea corta", v.cursor.Line, v.cursor.ByteCol)
	}

	// Baja a la línea larga: recupera la columna 6, no se queda pegado en la 2.
	v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if v.cursor.Line != 2 || v.cursor.ByteCol != 6 {
		t.Fatalf("cursor = (%d,%d), se esperaba (2,6): la columna deseada debe sobrevivir", v.cursor.Line, v.cursor.ByteCol)
	}
}

func TestHorizontalMovementResetsDesiredColumn(t *testing.T) {
	v := newTestView(t, "abcdefgh\nabcdefgh", 20, 2)

	for i := 0; i < 6; i++ {
		v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if v.cursor.ByteCol != 6 {
		t.Fatalf("ByteCol = %d, se esperaba 6", v.cursor.ByteCol)
	}

	// Un movimiento horizontal reancla la columna deseada.
	v.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if v.cursor.ByteCol != 5 {
		t.Fatalf("ByteCol = %d, se esperaba 5", v.cursor.ByteCol)
	}
	v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	v.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if v.cursor.ByteCol != 5 {
		t.Fatalf("ByteCol = %d, se esperaba 5: la columna deseada se reancló", v.cursor.ByteCol)
	}
}

func TestVerticalMovementUsesDisplayColumnsForWideCharacters(t *testing.T) {
	// La línea 0 tiene un carácter ancho; la 1 es ASCII de igual ancho visual.
	v := newTestView(t, "日b\nxyz", 20, 2)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if v.cursor.ByteCol != 4 {
		t.Fatalf("ByteCol = %d, se esperaba 4", v.cursor.ByteCol)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	// "日b" mide 3 columnas de pantalla, así que en "xyz" el cursor cae en 3.
	if v.cursor.ByteCol != 3 {
		t.Fatalf("ByteCol = %d, se esperaba 3: se compara en columnas de pantalla, no en bytes", v.cursor.ByteCol)
	}
}

// --- Home / End ---

func TestHomeAndEndMoveWithinTheLine(t *testing.T) {
	v := newTestView(t, "uno\ndos\ntres", 20, 3)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if v.cursor.Line != 1 || v.cursor.ByteCol != 3 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,3) tras End", v.cursor.Line, v.cursor.ByteCol)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if v.cursor.Line != 1 || v.cursor.ByteCol != 0 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,0) tras Home", v.cursor.Line, v.cursor.ByteCol)
	}
}

// --- viewport ---

func TestViewportFollowsCursorDown(t *testing.T) {
	v := newTestView(t, "1\n2\n3\n4\n5\n6\n7\n8", 20, 3)

	// Baja hasta la línea 4: debe entrar en pantalla con el mínimo desplazamiento.
	for i := 0; i < 4; i++ {
		v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	if v.cursor.Line != 4 {
		t.Fatalf("cursor.Line = %d, se esperaba 4", v.cursor.Line)
	}
	if v.viewport.TopLine != 2 {
		t.Fatalf("TopLine = %d, se esperaba 2 para que la línea 4 sea visible", v.viewport.TopLine)
	}
}

func TestViewportFollowsCursorUp(t *testing.T) {
	v := newTestView(t, "1\n2\n3\n4\n5\n6\n7\n8", 20, 3)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))
	if v.viewport.TopLine != 5 {
		t.Fatalf("TopLine = %d, se esperaba 5", v.viewport.TopLine)
	}

	for i := 0; i < 5; i++ {
		v.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	}
	if v.cursor.Line != 2 {
		t.Fatalf("cursor.Line = %d, se esperaba 2", v.cursor.Line)
	}
	if v.viewport.TopLine != 2 {
		t.Fatalf("TopLine = %d, se esperaba 2: el viewport sube junto al cursor", v.viewport.TopLine)
	}
}

func TestViewportScrollsHorizontallyToKeepCursorVisible(t *testing.T) {
	// Estos tests fijan el scroll HORIZONTAL: sin wrap (el wrap es la
	// alternativa y envuelve la línea, anulando el desplazamiento).
	oldWrap := wordWrapEnabled
	wordWrapEnabled = false
	defer func() { wordWrapEnabled = oldWrap }()
	v := newTestView(t, "abcdefghij", 5, 1)

	for i := 0; i < 6; i++ {
		v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	if v.cursor.ByteCol != 6 {
		t.Fatalf("ByteCol = %d, se esperaba 6", v.cursor.ByteCol)
	}
	// El área de texto mide 3 columnas (5 de widget - 2 de gutter): la
	// columna 6 queda visible con LeftColumn 4 (6 - 3 + 1).
	if v.viewport.LeftColumn != 4 {
		t.Fatalf("LeftColumn = %d, se esperaba 4 para que la columna 6 sea visible", v.viewport.LeftColumn)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModCtrl))
	if v.viewport.LeftColumn != 0 {
		t.Fatalf("LeftColumn = %d, se esperaba 0 al volver al inicio", v.viewport.LeftColumn)
	}
}

func TestMouseWheelScrollDoesNotDragTheCursor(t *testing.T) {
	v := newTestView(t, "1\n2\n3\n4\n5\n6\n7\n8", 20, 3)

	v.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, tcell.ModNone))
	if v.viewport.TopLine != 3 {
		t.Fatalf("TopLine = %d, se esperaba 3", v.viewport.TopLine)
	}
	// El cursor no se movió: scrollear con la rueda no debe arrastrarlo.
	if v.cursor.Line != 0 || v.cursor.ByteCol != 0 {
		t.Fatalf("cursor = (%d,%d), se esperaba (0,0)", v.cursor.Line, v.cursor.ByteCol)
	}
}

// --- hit testing del mouse ---

func TestMouseClickPositionsCursor(t *testing.T) {
	v := newTestView(t, "uno\ndos\ntres", 20, 3)

	// La columna 2 del texto vive en la celda de pantalla gutterWidth+2.
	if !v.HandleEvent(tcell.NewEventMouse(2+v.gutterWidth(), 1, tcell.Button1, tcell.ModNone)) {
		t.Fatal("el clic debería mover el cursor")
	}
	if v.cursor.Line != 1 || v.cursor.ByteCol != 2 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,2)", v.cursor.Line, v.cursor.ByteCol)
	}
}

// TestMouseClickOnWideCharacterSnapsToItsStart es el caso que el render por runas
// rompía: clickear la mitad de un carácter ancho debe anclar en su inicio.
func TestMouseClickOnWideCharacterSnapsToItsStart(t *testing.T) {
	v := newTestView(t, "日ab", 20, 1)
	gutter := v.gutterWidth()

	// La columna lógica 1 cae en la mitad de '日' (que ocupa 0 y 1): en
	// pantalla es la celda gutter+1.
	v.HandleEvent(tcell.NewEventMouse(gutter+1, 0, tcell.Button1, tcell.ModNone))
	if v.cursor.ByteCol != 0 {
		t.Fatalf("ByteCol = %d, se esperaba 0: el clic debe anclar en el inicio del carácter ancho", v.cursor.ByteCol)
	}

	// La columna lógica 2 ya es el carácter siguiente.
	v.HandleEvent(tcell.NewEventMouse(gutter+2, 0, tcell.Button1, tcell.ModNone))
	if v.cursor.ByteCol != 3 {
		t.Fatalf("ByteCol = %d, se esperaba 3", v.cursor.ByteCol)
	}
}

func TestMouseClickRespectsHorizontalScroll(t *testing.T) {
	v := newTestView(t, "abcdefghij", 5, 1)
	v.viewport.LeftColumn = 3

	// La columna 0 del texto vive en la celda gutterWidth: ahí cae la columna
	// lógica 3 (LeftColumn).
	v.HandleEvent(tcell.NewEventMouse(v.gutterWidth(), 0, tcell.Button1, tcell.ModNone))
	if v.cursor.ByteCol != 3 {
		t.Fatalf("ByteCol = %d, se esperaba 3: la columna del texto es la 3 del documento", v.cursor.ByteCol)
	}
}

func TestMouseClickBelowTheLastLineIsIgnored(t *testing.T) {
	v := newTestView(t, "uno", 20, 5)

	if v.HandleEvent(tcell.NewEventMouse(v.gutterWidth(), 4, tcell.Button1, tcell.ModNone)) {
		t.Fatal("un clic por debajo de la última línea no debería mover el cursor")
	}
}

// TestMoveCursorToOffsetScrolleIntoView cubre el camino que usan deshacer y
// rehacer: el cursor salta a un offset de documento y el viewport lo acompaña.
func TestMoveCursorToOffsetScrolleIntoView(t *testing.T) {
	v := newTestView(t, "uno\ndos\ntres\ncuatro", 20, 2)

	v.MoveCursorToOffset(11) // 's' de "tres"

	if v.cursor.Line != 2 || v.cursor.ByteCol != 3 {
		t.Fatalf("cursor = (%d,%d), se esperaba (2,3)", v.cursor.Line, v.cursor.ByteCol)
	}
	if v.viewport.TopLine != 1 {
		t.Fatalf("TopLine = %d, se esperaba 1 para que la línea 2 sea visible", v.viewport.TopLine)
	}
}

func TestMoveCursorToOffsetOnEmptyDocument(t *testing.T) {
	v := newTestView(t, "", 20, 2)

	v.MoveCursorToOffset(0)

	if v.cursor.Line != 0 || v.cursor.ByteCol != 0 {
		t.Fatalf("cursor = (%d,%d), se esperaba (0,0)", v.cursor.Line, v.cursor.ByteCol)
	}
}

// TestMoveCursorToOffsetIgnoresOffsetPastTheEnd evita que un offset fuera de
// rango deje el cursor en un lugar inexistente.
func TestMoveCursorToOffsetClampsPastTheEnd(t *testing.T) {
	v := newTestView(t, "uno\ndos", 20, 2)

	v.MoveCursorToOffset(999)

	if v.cursor.Line != 1 || v.cursor.ByteCol != 3 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,3): el final del documento", v.cursor.Line, v.cursor.ByteCol)
	}
}

// --- dibujado del cursor ---

func TestDrawShowsCursorAtItsCell(t *testing.T) {
	s := newTestScreen(t, 20, 3)
	v := newTestView(t, "uno\ndos\ntres", 20, 3)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	draw(v, s)

	x, y, visible := s.GetCursor()
	if !visible {
		t.Fatal("el cursor debería ser visible")
	}
	// La celda de la columna 1 del texto es gutterWidth+1 en pantalla.
	if x != 3 || y != 1 {
		t.Fatalf("cursor en (%d,%d), se esperaba (3,1)", x, y)
	}
}

func TestDrawPlacesCursorAfterWideCharacters(t *testing.T) {
	s := newTestScreen(t, 20, 1)
	v := newTestView(t, "日a", 20, 1)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	draw(v, s)

	x, _, visible := s.GetCursor()
	if !visible {
		t.Fatal("el cursor debería ser visible")
	}
	if x != 4 {
		t.Fatalf("cursor en columnas %d, se esperaba 4: '日' ocupa dos celdas y el texto arranca tras el gutter", x)
	}
}

func TestDrawHidesCursorWhenScrolledOutOfView(t *testing.T) {
	lines := ""
	for i := 0; i < 20; i++ {
		lines += "linea\n"
	}
	s := newTestScreen(t, 20, 3)
	v := newTestView(t, lines, 20, 3)

	// Con el cursor en la línea 0, scrollear hacia abajo lo deja fuera de pantalla.
	v.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, tcell.ModNone))
	if v.viewport.TopLine == 0 {
		t.Fatal("el test necesita que el viewport se haya desplazado")
	}
	draw(v, s)

	if _, _, visible := s.GetCursor(); visible {
		t.Fatal("con el cursor fuera de pantalla no debería dibujarse")
	}
}
