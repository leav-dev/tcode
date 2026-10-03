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
	want := []string{"uno", "dos", "tres"}
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
	if got[0] != "tres" || got[1] != "cuatro" {
		t.Fatalf("viewport mal renderizado: %q", got)
	}
}

func TestScrollByKeyboardMovesViewport(t *testing.T) {
	v := newTestView(t, "uno\ndos\ntres\ncuatro", 20, 2)

	if !v.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) {
		t.Fatal("KeyDown debería haber cambiado el viewport")
	}
	if v.viewport.TopLine != 1 {
		t.Fatalf("TopLine = %d, se esperaba 1", v.viewport.TopLine)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	if v.viewport.TopLine != 2 {
		t.Fatalf("TopLine = %d, se esperaba 2", v.viewport.TopLine)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	if v.viewport.TopLine != 1 {
		t.Fatalf("TopLine = %d, se esperaba 1 tras 'k'", v.viewport.TopLine)
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

func TestPageDownScrollsByViewportHeight(t *testing.T) {
	v := newTestView(t, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10", 20, 3)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if v.viewport.TopLine != 3 {
		t.Fatalf("TopLine = %d, se esperaba 3 tras PgDn", v.viewport.TopLine)
	}
}

func TestEndAndHomeJumpToDocumentBounds(t *testing.T) {
	v := newTestView(t, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10", 20, 4)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if v.viewport.TopLine != 6 {
		t.Fatalf("TopLine = %d, se esperaba 6 tras End", v.viewport.TopLine)
	}

	v.HandleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if v.viewport.TopLine != 0 {
		t.Fatalf("TopLine = %d, se esperaba 0 tras Home", v.viewport.TopLine)
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

	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
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
	s := newTestScreen(t, 5, 1)
	v := newTestView(t, "abcdefghij", 5, 1)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone))
	if v.viewport.LeftColumn != 1 {
		t.Fatalf("LeftColumn = %d, se esperaba 1", v.viewport.LeftColumn)
	}

	draw(v, s)
	if got := screenLines(s)[0]; got != "bcdef" {
		t.Fatalf("contenido horizontal = %q, se esperaba %q", got, "bcdef")
	}
}

func TestDrawDecodesUTF8WithoutSplittingRunes(t *testing.T) {
	s := newTestScreen(t, 10, 1)
	v := newTestView(t, "café ☕\nsegunda", 10, 1)

	draw(v, s)
	if got := screenLines(s)[0]; got != "café ☕" {
		t.Fatalf("contenido UTF-8 = %q, se esperaba %q", got, "café ☕")
	}
}

func TestEventThatDoesNotChangeViewportReturnsFalse(t *testing.T) {
	v := newTestView(t, "uno\ndos", 20, 2)

	if v.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)) {
		t.Fatal("una tecla sin acción no debería pedir redibujado")
	}
	if v.HandleEvent(tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone)) {
		t.Fatal("un evento de mouse sin rueda no debería pedir redibujado")
	}
}
