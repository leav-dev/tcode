package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// cellRuneAt devuelve la runa principal de la celda (x, y) de la pantalla
// simulada, o 0 si la celda está vacía. GetContents() lee el front buffer, así
// que las aserciones van después de Show().
func cellRuneAt(s tcell.SimulationScreen, x, y int) rune {
	cells, w, _ := s.GetContents()
	c := cells[y*w+x]
	if len(c.Runes) == 0 {
		return 0
	}
	return c.Runes[0]
}

// cellReverse dice si la celda (x, y) está en estilo invertido. tcell no
// expone un getter de Reverse: hay que descomponer el estilo y mirar los
// atributos.
func cellReverse(s tcell.SimulationScreen, x, y int) bool {
	cells, w, _ := s.GetContents()
	_, _, attr := cells[y*w+x].Style.Decompose()
	return attr&tcell.AttrReverse != 0
}

// TestOffsetSurfaceTranslatesAllFourMethods: la superficie suma su origen a
// Put, SetContent y ShowCursor, y HideCursor delega tal cual.
func TestOffsetSurfaceTranslatesAllFourMethods(t *testing.T) {
	s := newTestScreen(t, 20, 12)
	o := NewOffsetSurface(s)
	o.SetRegion(2, 1, 10, 5)

	// Put: (0,0) de la región es (2,1) de la pantalla.
	if rest, used := o.Put(0, 0, "ab", tcell.StyleDefault); rest != "" || used != 2 {
		t.Fatalf("Put devolvió (%q, %d), se esperaba (\"\", 2)", rest, used)
	}
	// SetContent: (1,2) de la región es (3,3) de la pantalla.
	o.SetContent(1, 2, 'x', nil, tcell.StyleDefault)
	// ShowCursor: (0,0) de la región es (2,1) de la pantalla.
	o.ShowCursor(0, 0)

	s.Show()
	if got := cellRuneAt(s, 2, 1); got != 'a' {
		t.Fatalf("(2,1) = %q, se esperaba 'a' (Put traducido)", got)
	}
	if got := cellRuneAt(s, 3, 1); got != 'b' {
		t.Fatalf("(3,1) = %q, se esperaba 'b'", got)
	}
	if got := cellRuneAt(s, 3, 3); got != 'x' {
		t.Fatalf("(3,3) = %q, se esperaba 'x' (SetContent traducido)", got)
	}
	// Nada se escribió fuera de la región: el origen solo se suma, no se resta.
	if got := cellRuneAt(s, 0, 0); got != 0 {
		t.Fatalf("(0,0) = %q, se esperaba vacío (la región no se dibuja en el origen)", got)
	}

	if x, y, vis := s.GetCursor(); !vis || x != 2 || y != 1 {
		t.Fatalf("cursor = (%d,%d,vis=%v), se esperaba (2,1,true) (ShowCursor traducido)", x, y, vis)
	}

	o.HideCursor()
	if _, _, vis := s.GetCursor(); vis {
		t.Fatal("HideCursor debe delegar y ocultar el cursor")
	}
}

// TestOffsetSurfaceClipsAgainstTheRegion: todo lo que cae fuera de la región
// se descarta, contra los cuatro bordes; un texto que desborda por la derecha
// se recorta devolviendo el sobrante.
func TestOffsetSurfaceClipsAgainstTheRegion(t *testing.T) {
	s := newTestScreen(t, 20, 10)
	o := NewOffsetSurface(s)
	o.SetRegion(0, 0, 5, 5)

	for _, c := range []struct{ x, y int }{
		{-1, 0}, // izquierda
		{0, -1}, // arriba
		{5, 0},  // derecha
		{0, 5},  // abajo
	} {
		if rest, used := o.Put(c.x, c.y, "ab", tcell.StyleDefault); rest != "ab" || used != 0 {
			t.Fatalf("Put(%d,%d) devolvió (%q,%d), se esperaba (\"ab\",0) descartado", c.x, c.y, rest, used)
		}
		o.SetContent(c.x, c.y, 'x', nil, tcell.StyleDefault)
		o.ShowCursor(c.x, c.y)
	}

	// Nada de lo descartado se escribió dentro de la región.
	s.Show()
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			if got := cellRuneAt(s, x, y); got != 0 {
				t.Fatalf("la celda (%d,%d) = %q, no debía escribirse nada", x, y, got)
			}
		}
	}
	if _, _, vis := s.GetCursor(); vis {
		t.Fatal("ShowCursor fuera de la región debe esconder el cursor")
	}

	// Un texto que arranca dentro de la región pero la desborda por la derecha
	// se recorta: solo entran las columnas de la región y el sobrante vuelve
	// como devolución, igual que en el borde de la pantalla.
	if rest, used := o.Put(4, 0, "abcde", tcell.StyleDefault); rest != "bcde" || used != 1 {
		t.Fatalf("Put(4,0,\"abcde\") devolvió (%q,%d), se esperaba (\"bcde\",1)", rest, used)
	}
	s.Show()
	if got := cellRuneAt(s, 4, 0); got != 'a' {
		t.Fatalf("(4,0) = %q, se esperaba 'a'", got)
	}
	if got := cellRuneAt(s, 5, 0); got != 0 {
		t.Fatalf("(5,0) = %q, se esperaba vacío: la región termina en x=4", got)
	}
}

// TestOffsetSurfaceHidesCursorOutsideTheRegion: el cursor fuera de la región
// no puede quedar flotando sobre otra pane; dentro, se muestra traducido.
func TestOffsetSurfaceHidesCursorOutsideTheRegion(t *testing.T) {
	s := newTestScreen(t, 20, 10)
	o := NewOffsetSurface(s)
	o.SetRegion(0, 0, 5, 5)

	for _, c := range []struct{ x, y int }{{-1, 2}, {2, -1}, {5, 2}, {2, 5}} {
		o.ShowCursor(c.x, c.y)
		if _, _, vis := s.GetCursor(); vis {
			t.Fatalf("ShowCursor(%d,%d) fuera de la región no debe mostrar el cursor", c.x, c.y)
		}
	}

	o.ShowCursor(2, 3)
	if x, y, vis := s.GetCursor(); !vis || x != 2 || y != 3 {
		t.Fatalf("cursor dentro de la región = (%d,%d,vis=%v), se esperaba (2,3,true)", x, y, vis)
	}
}

// TestOffsetSurfaceSetRegionReframes: la misma instancia se reencuadra con
// SetRegion —origen y región nuevos— sin reasignar, que es como la reusa el
// controlador entre redibujos.
func TestOffsetSurfaceSetRegionReframes(t *testing.T) {
	s := newTestScreen(t, 20, 10)
	o := NewOffsetSurface(s)

	o.SetRegion(0, 0, 5, 5)
	o.Put(0, 0, "a", tcell.StyleDefault)

	o.SetRegion(5, 6, 4, 3)
	o.Put(0, 0, "b", tcell.StyleDefault)
	o.Put(3, 0, "c", tcell.StyleDefault)
	// Fuera del NUEVO encuadre no se escribe: x=4 es el borde derecho.
	o.Put(4, 0, "z", tcell.StyleDefault)

	s.Show()
	if got := cellRuneAt(s, 0, 0); got != 'a' {
		t.Fatalf("(0,0) = %q, se esperaba 'a' del primer encuadre", got)
	}
	if got := cellRuneAt(s, 5, 6); got != 'b' {
		t.Fatalf("(5,6) = %q, se esperaba 'b' del segundo encuadre", got)
	}
	if got := cellRuneAt(s, 8, 6); got != 'c' {
		t.Fatalf("(8,6) = %q, se esperaba 'c' (origen sumado al recorte)", got)
	}
	if got := cellRuneAt(s, 9, 6); got != 0 {
		t.Fatalf("(9,6) = %q, se esperaba vacío: el nuevo encuadre recorta en x=4", got)
	}
}
