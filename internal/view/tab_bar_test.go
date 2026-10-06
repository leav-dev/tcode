package view

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/model"
)

// newTabWorkspace crea un workspace con archivos reales en t.TempDir(), en el
// orden de apertura. Devuelve el workspace con el ÚLTIMO abierto activo.
func newTabWorkspace(t *testing.T, names ...string) *model.Workspace {
	t.Helper()
	dir := t.TempDir()
	ws := model.NewWorkspace()
	if err := ws.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot falló: %v", err)
	}
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatalf("no se pudo crear %s: %v", name, err)
		}
		if _, err := ws.Open(path); err != nil {
			t.Fatalf("Open(%s) falló: %v", name, err)
		}
	}
	// Cerrar los buffers al terminar: en Windows un mmap vivo impide borrar el
	// archivo y el cleanup de t.TempDir() falla. Los tests de la vista ya lo
	// hacen con su PieceTable; acá el workspace cierra por todos.
	t.Cleanup(func() { ws.CloseAll() })
	return ws
}

// drawTabBar dibuja la barra y hace flush al front buffer: GetContents() lee
// el front, así que Show() es obligatorio (igual que en las demás vistas).
func drawTabBar(tb *TabBar, ws *model.Workspace, s tcell.SimulationScreen, width int) {
	tb.Draw(s, ws, width)
	s.Show()
}

// TestTabBarDrawsLabelsAndModifiedMarker: cada pestaña es el nombre base de la
// ruta, y la sucia lleva el mismo marcador "[+]" que la barra de estado.
func TestTabBarDrawsLabelsAndModifiedMarker(t *testing.T) {
	ws := newTabWorkspace(t, "a.txt", "b.txt")
	if err := ws.BufferAt(1).Insert(0, "x"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	ws.SetActive(0)

	tb := NewTabBar()
	s := newTestScreen(t, 40, 1)
	drawTabBar(tb, ws, s, 40)

	want := "[a.txt]│b.txt [+]"
	if got := screenLines(s)[0]; got != want {
		t.Fatalf("fila 0 = %q, se esperaba %q", got, want)
	}
}

// TestTabBarUntitledBufferShowsSinNombre: un buffer sin ruta no puede mostrar
// un nombre base; se muestra como "(sin nombre)".
func TestTabBarUntitledBufferShowsSinNombre(t *testing.T) {
	ws := model.NewWorkspace()
	ws.NewUntitled()

	tb := NewTabBar()
	s := newTestScreen(t, 40, 1)
	drawTabBar(tb, ws, s, 40)

	if got := screenLines(s)[0]; got != "[(sin nombre)]" {
		t.Fatalf("fila 0 = %q, se esperaba %q", got, "[(sin nombre)]")
	}
}

// TestTabBarHighlightsTheActiveTab: la activa va en estilo invertido y las
// demás con el estilo por defecto; el separador de una columna también es
// neutral.
func TestTabBarHighlightsTheActiveTab(t *testing.T) {
	ws := newTabWorkspace(t, "a.txt", "b.txt")
	ws.SetActive(1)

	tb := NewTabBar()
	s := newTestScreen(t, 40, 1)
	drawTabBar(tb, ws, s, 40)

	// "a.txt" ocupa 0..4, el separador la columna 5, "b.txt" arranca en la 6.
	if cellReverse(s, 0, 0) {
		t.Fatal("la pestaña inactiva no debe ir en estilo invertido")
	}
	if !cellReverse(s, 6, 0) {
		t.Fatal("la pestaña activa debe ir en estilo invertido")
	}
	if cellReverse(s, 5, 0) {
		t.Fatal("el separador entre pestañas no debe ir en estilo invertido")
	}
}

// TestTabBarScrollShowsArrowsAndKeepsActiveVisible: cuando no entran todas,
// la ventana se corre (EnsureActive), la activa queda visible y resaltada, y
// las flechas marcan el desborde en los dos bordes.
func TestTabBarScrollShowsArrowsAndKeepsActiveVisible(t *testing.T) {
	ws := newTabWorkspace(t, "t0.txt", "t1.txt", "t2.txt", "t3.txt", "t4.txt")
	ws.SetActive(3) // la penúltima: a 15 columnas no entra desde el inicio

	tb := NewTabBar()
	tb.EnsureActive(ws, 15)
	if tb.start != 3 {
		t.Fatalf("start = %d tras EnsureActive, se esperaba 3 (lo mínimo para ver la activa con corchetes)", tb.start)
	}

	s := newTestScreen(t, 15, 1)
	drawTabBar(tb, ws, s, 15)

	if got := cellRuneAt(s, 0, 0); got != '<' {
		t.Fatalf("(0,0) = %q, se esperaba '<' (pestañas cortadas a la izquierda)", got)
	}
	if got := cellRuneAt(s, 14, 0); got != '>' {
		t.Fatalf("(14,0) = %q, se esperaba '>' (la última sigue fuera a la derecha)", got)
	}
	// La activa está visible y resaltada: "[t3.txt]" arranca en la columna 1.
	if got := cellRuneAt(s, 2, 0); got != 't' {
		t.Fatalf("(2,0) = %q, se esperaba el inicio de la pestaña activa \"[t3.txt]\"", got)
	}
	if !cellReverse(s, 2, 0) {
		t.Fatal("la pestaña activa visible debe ir en estilo invertido")
	}

	want := "<[t3.txt]│t4.…>"
	if got := screenLines(s)[0]; got != want {
		t.Fatalf("fila 0 = %q, se esperaba %q", got, want)
	}
}

// TestEnsureActiveIsIdempotent: si la activa ya entra entera, EnsureActive no
// mueve nada; y si la activa quedó a la izquierda del inicio, vuelve a ella.
func TestEnsureActiveIsIdempotent(t *testing.T) {
	ws := newTabWorkspace(t, "t0.txt", "t1.txt", "t2.txt", "t3.txt", "t4.txt")
	ws.SetActive(3)

	tb := NewTabBar()
	tb.EnsureActive(ws, 15)
	first := tb.start

	tb.EnsureActive(ws, 15)
	if tb.start != first {
		t.Fatalf("EnsureActive repetido movió start: %d -> %d", first, tb.start)
	}

	// La activa 0 ya se ve con la ventana en 0: idempotente también ahí.
	ws.SetActive(0)
	tb.EnsureActive(ws, 15)
	if tb.start != 0 {
		t.Fatalf("start = %d con la activa en 0, se esperaba 0", tb.start)
	}
}

// TestEnsureActiveOnEmptyWorkspaceIsHarmless: el resize llama a EnsureActive
// siempre, también con el workspace vacío.
func TestEnsureActiveOnEmptyWorkspaceIsHarmless(t *testing.T) {
	tb := NewTabBar()
	tb.EnsureActive(model.NewWorkspace(), 20)
	if tb.start != 0 {
		t.Fatalf("start = %d sobre workspace vacío, se esperaba 0", tb.start)
	}
}

// TestTabBarWideMiddleTabStillDrawsTheActive: una pestaña intermedia muy
// ancha no puede cortar la fila antes de la activa. Si la activa entra según
// EnsureActive, todo el prefijo hasta ella entra, así que la intermedia —más
// corta que ese prefijo— entra entera; Draw y EnsureActive tienen que acordar
// en la frontera justa.
func TestTabBarWideMiddleTabStillDrawsTheActive(t *testing.T) {
	// "x.txt" (5) + sep (1) + intermedia (20) + sep (1) + "[y.txt]" (7) = 34.
	ws := newTabWorkspace(t, "x.txt", "nombre-muy-largo.txt", "y.txt")
	ws.SetActive(2)

	tb := NewTabBar()
	tb.EnsureActive(ws, 35)
	if tb.start != 0 {
		t.Fatalf("start = %d, se esperaba 0 (todo entra en 35 columnas)", tb.start)
	}

	s := newTestScreen(t, 35, 1)
	drawTabBar(tb, ws, s, 35)

	if got := cellRuneAt(s, 6, 0); got != 'n' {
		t.Fatalf("(6,0) = %q, se esperaba el inicio de la pestaña intermedia", got)
	}
	// La activa "[y.txt]" arranca en la columna 27: está visible pese a la
	// intermedia ancha, y no hay flechas (no sobra nada a los lados).
	if got := cellRuneAt(s, 28, 0); got != 'y' {
		t.Fatalf("(28,0) = %q, se esperaba el inicio de la pestaña activa", got)
	}
	if got := screenLines(s)[0]; got != "x.txt│nombre-muy-largo.txt│[y.txt]" {
		t.Fatalf("fila 0 = %q", got)
	}
}

// TestTabBarSlidesPastAWideMiddleTab: si la activa no entra con la intermedia
// ancha en la ventana, EnsureActive corre start MÁS ALLÁ de la intermedia —no
// la trunca en el medio— y la activa queda visible con las flechas marcando
// el desborde.
func TestTabBarSlidesPastAWideMiddleTab(t *testing.T) {
	ws := newTabWorkspace(t, "x.txt", "nombre-muy-largo.txt", "y.txt")
	ws.SetActive(2)

	tb := NewTabBar()
	tb.EnsureActive(ws, 30)
	if tb.start != 1 {
		t.Fatalf("start = %d tras EnsureActive en 30 columnas, se esperaba 1 (saltar la intermedia)", tb.start)
	}

	s := newTestScreen(t, 30, 1)
	drawTabBar(tb, ws, s, 30)

	// "<" en 0, la intermedia en 1..20, "[y.txt]" activa en 22..28: la ventana
	// cabe entera desde la 1, así que a la derecha no hay nada cortado (sin
	// '>'); la cortada a la izquierda la marca el '<'.
	if got := cellRuneAt(s, 0, 0); got != '<' {
		t.Fatalf("(0,0) = %q, se esperaba '<' — x.txt quedó a la izquierda", got)
	}
	if got := cellRuneAt(s, 23, 0); got != 'y' {
		t.Fatalf("(23,0) = %q, se esperaba el inicio de la pestaña activa visible", got)
	}
	if !cellReverse(s, 23, 0) {
		t.Fatal("la pestaña activa visible debe ir en estilo invertido")
	}
	if got := cellRuneAt(s, 29, 0); got != ' ' {
		t.Fatalf("(29,0) = %q, se esperaba vacío: no hay desborde a la derecha", got)
	}
}

// TestTabBarTruncatesATabWiderThanTheRow: una pestaña más ancha que la fila no
// entra ni arrancando en ella; se muestra su inicio truncado, con la elipsis
// en la última celda del contenido y la flecha de desborde a la derecha.
func TestTabBarTruncatesATabWiderThanTheRow(t *testing.T) {
	const label = "un-nombre-muy-largo.txt"
	ws := newTabWorkspace(t, label)

	tb := NewTabBar()
	tb.EnsureActive(ws, 10)

	s := newTestScreen(t, 10, 1)
	drawTabBar(tb, ws, s, 10)

	// 10 columnas: 8 de texto truncado (la etiqueta mide 23) + "…" + ">".
	want := "un-nombr…>"
	if got := screenLines(s)[0]; got != want {
		t.Fatalf("fila 0 = %q, se esperaba %q", got, want)
	}
}

// TestTabBarClickOnATabReturnsItsIndex: el clic sobre una pestaña devuelve su
// índice para activarla; el hit box se come el separador siguiente (sin zonas
// muertas entre pestañas) y un clic fuera de la fila no es de la barra.
func TestTabBarClickOnATabReturnsItsIndex(t *testing.T) {
	ws := newTabWorkspace(t, "a.txt", "b.txt", "c.txt")
	ws.SetActive(0)
	tb := NewTabBar()

	// A 40 columnas con la activa entre corchetes: "[a.txt]" en 0..6,
	// separador en 7, "b.txt" en 8..12, separador en 13, "c.txt" en 14..18.
	for _, tc := range []struct{ x, want int }{
		{0, 0}, {6, 0}, {7, 0}, // el separador pertenece a la pestaña anterior
		{8, 1}, {12, 1}, {13, 1},
		{14, 2}, {18, 2},
	} {
		idx, handled := tb.HandleMouse(tc.x, 0, tcell.Button1, ws, 40)
		if !handled || idx != tc.want {
			t.Fatalf("clic en x=%d devolvió (idx=%d, handled=%v), se esperaba (%d, true)", tc.x, idx, handled, tc.want)
		}
	}
	// Fuera de la fila: no es de la barra.
	if _, handled := tb.HandleMouse(3, 1, tcell.Button1, ws, 40); handled {
		t.Fatal("un clic fuera de la fila de pestañas no debe manejarlo la barra")
	}
	// En la fila pero a la derecha de la última pestaña: se come el evento sin
	// activar nada.
	if idx, handled := tb.HandleMouse(30, 0, tcell.Button1, ws, 40); !handled || idx != -1 {
		t.Fatalf("clic en el vacío de la fila devolvió (idx=%d, handled=%v), se esperaba (-1, true)", idx, handled)
	}
}

// TestTabBarArrowClicksScrollTheStrip: con más pestañas que ancho, el clic en
// '>' y '<' corre la ventana del strip sin cambiar la pestaña activa.
func TestTabBarArrowClicksScrollTheStrip(t *testing.T) {
	ws := newTabWorkspace(t, "t0.txt", "t1.txt", "t2.txt", "t3.txt", "t4.txt", "t5.txt")
	ws.SetActive(0)
	tb := NewTabBar()
	const width = 18

	// '>' en la última columna: hay pestañas fuera a la derecha.
	if idx, handled := tb.HandleMouse(width-1, 0, tcell.Button1, ws, width); !handled || idx != -1 {
		t.Fatalf("clic en '>' devolvió (idx=%d, handled=%v), se esperaba (-1, true)", idx, handled)
	}
	if tb.start != 1 {
		t.Fatalf("start = %d tras el clic en '>', se esperaba 1", tb.start)
	}
	// '<' en la columna 0: vuelve la ventana.
	if idx, handled := tb.HandleMouse(0, 0, tcell.Button1, ws, width); !handled || idx != -1 {
		t.Fatalf("clic en '<' devolvió (idx=%d, handled=%v), se esperaba (-1, true)", idx, handled)
	}
	if tb.start != 0 {
		t.Fatalf("start = %d tras el clic en '<', se esperaba 0", tb.start)
	}
	// Sin pestañas la fila no es de nadie.
	if _, handled := tb.HandleMouse(0, 0, tcell.Button1, model.NewWorkspace(), width); handled {
		t.Fatal("sin pestañas el clic no debe manejarlo la barra")
	}
}

// TestTabBarWheelSwitchesTabs: la rueda sobre la fila cambia de pestaña
// (arriba = anterior, abajo = siguiente, con wrap), como en un navegador.
func TestTabBarWheelSwitchesTabs(t *testing.T) {
	ws := newTabWorkspace(t, "a.txt", "b.txt", "c.txt")
	ws.SetActive(0)
	tb := NewTabBar()

	if idx, handled := tb.HandleMouse(2, 0, tcell.WheelDown, ws, 40); !handled || idx != 1 {
		t.Fatalf("rueda abajo devolvió (idx=%d, handled=%v), se esperaba (1, true)", idx, handled)
	}
	if idx, handled := tb.HandleMouse(2, 0, tcell.WheelUp, ws, 40); !handled || idx != 2 {
		t.Fatalf("rueda arriba desde la primera devolvió (idx=%d, handled=%v), se esperaba (2, true) con wrap", idx, handled)
	}
}
