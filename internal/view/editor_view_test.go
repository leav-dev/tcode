package view

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/model"
)

// newTestScreen crea una pantalla simulada de tcell (sin TTY) del tamaño dado.
func newTestScreen(t *testing.T, width, height int) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla simulada: %v", err)
	}
	s.SetSize(width, height)
	t.Cleanup(s.Fini)
	return s
}

func newTestView(t *testing.T, content string, width, height int) *EditorView {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo de prueba: %v", err)
	}

	pt := model.NewPieceTable()
	if err := pt.LoadFile(path); err != nil {
		t.Fatalf("LoadFile falló: %v", err)
	}
	t.Cleanup(func() { pt.Close() })

	return NewEditorView(pt, height, width)
}

// draw renderiza la vista y hace flush al front buffer de la pantalla simulada.
// GetContents() lee el front buffer, por lo que Show() es obligatorio.
func draw(v *EditorView, s tcell.SimulationScreen) {
	v.Draw(s)
	s.Show()
}

// screenLines extrae el texto visible de la pantalla simulada como líneas.
//
// Ojo: la celda de continuación de un carácter ancho queda sin runas y esta
// función la reemplaza por un espacio, así que "日b" se reconstruye como "日 b".
// Para aserciones de posición conviene usar GetContent directamente.
func screenLines(s tcell.SimulationScreen) []string {
	cells, width, height := s.GetContents()
	lines := make([]string, height)
	for y := 0; y < height; y++ {
		var sb strings.Builder
		for x := 0; x < width; x++ {
			c := cells[y*width+x]
			if len(c.Runes) == 0 {
				sb.WriteRune(' ')
				continue
			}
			sb.WriteRune(c.Runes[0])
		}
		lines[y] = strings.TrimRight(sb.String(), " ")
	}
	return lines
}

func TestDrawRendersVisibleLines(t *testing.T) {
	s := newTestScreen(t, 20, 3)
	v := newTestView(t, "uno\ndos\ntres\ncuatro", 20, 3)

	draw(v, s)

	got := screenLines(s)
	// El gutter (2 columnas: número + separador) precede al texto.
	want := []string{"1 uno", "2 dos", "3 tres"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("línea %d = %q, se esperaba %q", i, got[i], want[i])
		}
	}
}

func TestDrawOnlyRendersLinesInsideViewport(t *testing.T) {
	s := newTestScreen(t, 20, 2)
	v := newTestView(t, "uno\ndos\ntres\ncuatro", 20, 2)

	// Simula estar escrolleado hasta la tercera línea (índice 2).
	v.viewport.TopLine = 2
	draw(v, s)

	got := screenLines(s)
	if got[0] != "3 tres" || got[1] != "4 cuatro" {
		t.Fatalf("viewport mal renderizado: %q", got)
	}
}

// TestArrowKeysMoveCursor reemplaza al viejo test de scroll por teclado: las
// flechas ahora mueven el cursor y el viewport lo acompaña.
func TestArrowKeysMoveCursor(t *testing.T) {
	v := newTestView(t, "uno\ndos\ntres\ncuatro", 20, 2)

	if v.cursor.Line != 0 || v.cursor.ByteCol != 0 {
		t.Fatalf("el cursor debe arrancar en (0,0), está en (%d,%d)", v.cursor.Line, v.cursor.ByteCol)
	}

	if !v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) {
		t.Fatal("KeyDown debería mover el cursor")
	}
	if v.cursor.Line != 1 {
		t.Fatalf("cursor.Line = %d, se esperaba 1", v.cursor.Line)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if v.cursor.ByteCol != 1 {
		t.Fatalf("cursor.ByteCol = %d, se esperaba 1", v.cursor.ByteCol)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if v.cursor.ByteCol != 0 {
		t.Fatalf("cursor.ByteCol = %d, se esperaba 0", v.cursor.ByteCol)
	}

	// Las letras ya no scrollean: se insertan como texto en el cursor.
	if !v.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone)) {
		t.Fatal("'j' debe insertarse como texto")
	}
	if got := string(v.model.LineContent(1)); got != "jdos" {
		t.Fatalf("línea 1 = %q, se esperaba %q", got, "jdos")
	}
	if v.viewport.TopLine != 0 {
		t.Fatalf("TopLine = %d: insertar no debe scrollear el viewport", v.viewport.TopLine)
	}
}

func TestScrollUpAtTopIsNoOp(t *testing.T) {
	v := newTestView(t, "uno\ndos\ntres", 20, 2)

	if v.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)) {
		t.Fatal("scrollear hacia arriba en el tope no debería cambiar nada")
	}
	if v.viewport.TopLine != 0 {
		t.Fatalf("TopLine = %d, se esperaba 0", v.viewport.TopLine)
	}
}

func TestScrollDownClampsAtDocumentEnd(t *testing.T) {
	// 4 líneas, viewport de 2 → maxTopLine = 2.
	v := newTestView(t, "uno\ndos\ntres\ncuatro", 20, 2)

	for i := 0; i < 10; i++ {
		v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	if v.viewport.TopLine != 2 {
		t.Fatalf("TopLine = %d, se esperaba 2 (clamp al final del documento)", v.viewport.TopLine)
	}
}

// TestPageDownMovesCursorByViewportHeight: PgDn mueve el cursor y el viewport lo
// sigue CENTRÁNDOLO: la línea 3 queda en el medio del alto 3 (3 - 3/2 = 2).
func TestPageDownMovesCursorByViewportHeight(t *testing.T) {
	v := newTestView(t, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10", 20, 3)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if v.cursor.Line != 3 {
		t.Fatalf("cursor.Line = %d, se esperaba 3 tras PgDn", v.cursor.Line)
	}
	if v.viewport.TopLine != 2 {
		t.Fatalf("TopLine = %d, se esperaba 2: la línea 3 queda centrada tras PgDn", v.viewport.TopLine)
	}
}

// TestCtrlHomeAndCtrlEndMoveCursorToDocumentBounds: Home y End ahora son de
// línea; para el documento completo están las variantes con Ctrl.
func TestCtrlHomeAndCtrlEndMoveCursorToDocumentBounds(t *testing.T) {
	v := newTestView(t, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10", 20, 4)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))
	if v.cursor.Line != 9 {
		t.Fatalf("cursor.Line = %d, se esperaba 9 tras Ctrl+End", v.cursor.Line)
	}
	if v.viewport.TopLine != 6 {
		t.Fatalf("TopLine = %d, se esperaba 6 tras Ctrl+End", v.viewport.TopLine)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModCtrl))
	if v.cursor.Line != 0 || v.cursor.ByteCol != 0 {
		t.Fatalf("cursor = (%d,%d), se esperaba (0,0) tras Ctrl+Home", v.cursor.Line, v.cursor.ByteCol)
	}
	if v.viewport.TopLine != 0 {
		t.Fatalf("TopLine = %d, se esperaba 0 tras Ctrl+Home", v.viewport.TopLine)
	}
}

func TestMouseWheelScrollsViewport(t *testing.T) {
	v := newTestView(t, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10", 20, 3)

	if !v.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, tcell.ModNone)) {
		t.Fatal("la rueda hacia abajo debería cambiar el viewport")
	}
	if v.viewport.TopLine != 3 {
		t.Fatalf("TopLine = %d, se esperaba 3 tras la rueda", v.viewport.TopLine)
	}

	v.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelUp, tcell.ModNone))
	if v.viewport.TopLine != 0 {
		t.Fatalf("TopLine = %d, se esperaba 0 tras la rueda hacia arriba", v.viewport.TopLine)
	}
}

func TestResizeClampsViewportToDocument(t *testing.T) {
	// Con 4 líneas y alto 1, maxTopLine = 3.
	v := newTestView(t, "uno\ndos\ntres\ncuatro", 20, 1)

	// Ctrl+End lleva el cursor al final del documento.
	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))
	if v.viewport.TopLine != 3 {
		t.Fatalf("TopLine = %d, se esperaba 3", v.viewport.TopLine)
	}

	// Al agrandar la ventana, el tope máximo baja a 1 y el viewport debe reajustarse.
	v.Resize(20, 3)
	if v.viewport.TopLine != 1 {
		t.Fatalf("TopLine = %d, se esperaba 1 tras el resize", v.viewport.TopLine)
	}
}

func TestHorizontalScrollSkipsLeadingColumns(t *testing.T) {
	// Estos tests fijan el scroll HORIZONTAL: sin wrap (el wrap es la
	// alternativa y envuelve la línea, anulando el desplazamiento).
	oldWrap := wordWrapEnabled
	wordWrapEnabled = false
	defer func() { wordWrapEnabled = oldWrap }()
	s := newTestScreen(t, 5, 1)
	v := newTestView(t, "abcdefghij", 5, 1)
	v.viewport.LeftColumn = 1

	draw(v, s)
	// El gutter (2 columnas) precede al texto y reduce el área de texto a 3
	// columnas: la fila muestra "bcd" después del número.
	if got := screenLines(s)[0]; got != "1 bcd" {
		t.Fatalf("contenido horizontal = %q, se esperaba %q", got, "1 bcd")
	}
}

func TestDrawDecodesUTF8WithoutSplittingRunes(t *testing.T) {
	s := newTestScreen(t, 10, 1)
	v := newTestView(t, "café ☕\nsegunda", 10, 1)

	draw(v, s)
	if got := screenLines(s)[0]; got != "1 café ☕" {
		t.Fatalf("contenido UTF-8 = %q, se esperaba %q", got, "1 café ☕")
	}
}

func TestEventThatDoesNotChangeViewportReturnsFalse(t *testing.T) {
	v := newTestView(t, "uno\ndos", 20, 2)

	// F1 no tiene acción asignada.
	if v.HandleEvent(tcell.NewEventKey(tcell.KeyF1, 0, tcell.ModNone)) {
		t.Fatal("una tecla sin acción no debería pedir redibujado")
	}
	if v.HandleEvent(tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone)) {
		t.Fatal("un evento de mouse sin rueda no debería pedir redibujado")
	}
}
