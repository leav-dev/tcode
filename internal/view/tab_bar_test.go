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

	want := "a.txt b.txt [+]"
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

	if got := screenLines(s)[0]; got != "(sin nombre)" {
		t.Fatalf("fila 0 = %q, se esperaba %q", got, "(sin nombre)")
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
	if tb.start != 2 {
		t.Fatalf("start = %d tras EnsureActive, se esperaba 2 (lo mínimo para ver la activa)", tb.start)
	}

	s := newTestScreen(t, 15, 1)
	drawTabBar(tb, ws, s, 15)

	if got := cellRuneAt(s, 0, 0); got != '<' {
		t.Fatalf("(0,0) = %q, se esperaba '<' (pestañas cortadas a la izquierda)", got)
	}
	if got := cellRuneAt(s, 14, 0); got != '>' {
		t.Fatalf("(14,0) = %q, se esperaba '>' (la última sigue fuera a la derecha)", got)
	}
	// La activa está visible y resaltada: "t3.txt" arranca en la columna 8.
	if got := cellRuneAt(s, 8, 0); got != 't' {
		t.Fatalf("(8,0) = %q, se esperaba el inicio de la pestaña activa \"t3.txt\"", got)
	}
	if !cellReverse(s, 8, 0) {
		t.Fatal("la pestaña activa visible debe ir en estilo invertido")
	}

	want := "<t2.txt t3.txt>"
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
	// "x.txt" (5) + sep (1) + intermedia (20) + sep (1) + "y.txt" (5) = 32.
	ws := newTabWorkspace(t, "x.txt", "nombre-muy-largo.txt", "y.txt")
	ws.SetActive(2)

	tb := NewTabBar()
	tb.EnsureActive(ws, 33)
	if tb.start != 0 {
		t.Fatalf("start = %d, se esperaba 0 (todo entra en 33 columnas)", tb.start)
	}

	s := newTestScreen(t, 33, 1)
	drawTabBar(tb, ws, s, 33)

	if got := cellRuneAt(s, 6, 0); got != 'n' {
		t.Fatalf("(6,0) = %q, se esperaba el inicio de la pestaña intermedia", got)
	}
	// La activa "y.txt" arranca en la columna 27: está visible pese a la
	// intermedia ancha, y no hay flechas (no sobra nada a los lados).
	if got := cellRuneAt(s, 27, 0); got != 'y' {
		t.Fatalf("(27,0) = %q, se esperaba el inicio de la pestaña activa", got)
	}
	if got := screenLines(s)[0]; got != "x.txt nombre-muy-largo.txt y.txt" {
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

	// "<" en 0, la intermedia en 1..20, "y.txt" activa en 22..26: la ventana
	// cabe entera desde la 1, así que a la derecha no hay nada cortado (sin
	// '>'); la cortada a la izquierda la marca el '<'.
	if got := cellRuneAt(s, 0, 0); got != '<' {
		t.Fatalf("(0,0) = %q, se esperaba '<' — x.txt quedó a la izquierda", got)
	}
	if got := cellRuneAt(s, 22, 0); got != 'y' {
		t.Fatalf("(22,0) = %q, se esperaba el inicio de la pestaña activa visible", got)
	}
	if !cellReverse(s, 22, 0) {
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
