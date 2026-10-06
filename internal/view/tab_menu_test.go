package view

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/leav-dev/tcode/internal/model"
)

// menuWorkspace abre n archivos de prueba como n pestañas (newTabWorkspace
// devuelve la ÚLTIMA activa).
func menuWorkspace(t *testing.T, n int) *model.Workspace {
	t.Helper()
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("t%02d.txt", i)
	}
	return newTabWorkspace(t, names...)
}

// drawTabMenu dibuja el menú y hace flush al front buffer: GetContents() lee
// el front, así que Show() es obligatorio.
func drawTabMenu(m *TabMenu, ws *model.Workspace, s tcell.SimulationScreen, width int) {
	m.Draw(s, ws, width)
	s.Show()
}

// TestTabMenuDrawsLabelsAndHighlightsTheCursorRow: cada fila es la etiqueta de
// la pestaña (nombre base, "[+]" si está sucia —el mismo tabLabel de la
// TabBar—) y la fila del cursor va resaltada a todo el ancho.
func TestTabMenuDrawsLabelsAndHighlightsTheCursorRow(t *testing.T) {
	ws := newTabWorkspace(t, "a.txt", "b.txt")
	if err := ws.BufferAt(1).Insert(0, "x"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	ws.SetActive(0)

	m := NewTabMenu()
	m.Resize(20, 5)
	m.Open(ws)

	s := newTestScreen(t, 20, 5)
	drawTabMenu(m, ws, s, 20)

	if got := screenLines(s)[0]; got != "> a.txt" {
		t.Fatalf("fila 0 = %q, se esperaba %q (marcador de la pestaña activa)", got, "> a.txt")
	}
	// La sucia lleva el mismo marcador que la TabBar, con indent de no-activa.
	if got := screenLines(s)[1]; got != "  b.txt [+]" {
		t.Fatalf("fila 1 = %q, se esperaba %q", got, "  b.txt [+]")
	}
	if !cellReverse(s, 0, 0) {
		t.Fatal("la fila del cursor debe ir en estilo invertido")
	}
	// La barra de resaltado es de ancho completo: el final de la fila del
	// cursor también queda invertido, aunque la etiqueta sea corta.
	if !cellReverse(s, 10, 0) {
		t.Fatal("la fila del cursor debe resaltarse a todo el ancho del menú")
	}
	if cellReverse(s, 0, 1) {
		t.Fatal("las demás filas deben ir con el estilo por defecto")
	}
	// Con 2 pestañas y 5 filas no hay nada debajo de la segunda.
	if got := cellRuneAt(s, 0, 2); got != 0 {
		t.Fatalf("fila 2 = %q, no debe dibujarse nada tras la última pestaña", got)
	}

	// Down mueve el cursor: la fila 1 queda resaltada y la 0 pierde el brillo.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	drawTabMenu(m, ws, s, 20)
	if !cellReverse(s, 0, 1) {
		t.Fatal("tras Down, la fila 1 debe ser la resaltada")
	}
	if cellReverse(s, 0, 0) {
		t.Fatal("tras Down, la fila 0 pierde el resaltado")
	}
}

// TestTabMenuOpenPlacesTheCursorOnTheActiveTab: Open deja el cursor sobre la
// pestaña activa del workspace, con el scroll corrido lo mínimo para verla.
func TestTabMenuOpenPlacesTheCursorOnTheActiveTab(t *testing.T) {
	ws := newTabWorkspace(t, "a.txt", "b.txt", "c.txt")
	ws.SetActive(1)

	m := NewTabMenu()
	m.Open(ws)
	if got := m.Selected(); got != 1 {
		t.Fatalf("Selected() = %d tras Open sobre la activa 1, se esperaba 1", got)
	}
}

// TestTabMenuScrollKeepsTheCursorVisible: con más pestañas que el alto, el
// cursor sobre la última corre el scroll vertical lo mínimo para que siga
// visible, y el movimiento la mantiene dentro de la ventana.
func TestTabMenuScrollKeepsTheCursorVisible(t *testing.T) {
	// 30 pestañas con la última activa (pos 29), alto 5: top = 25.
	ws := menuWorkspace(t, 30)
	m := NewTabMenu()
	m.Resize(20, 5)
	m.Open(ws)

	if m.cursor != 29 || m.top != 25 {
		t.Fatalf("Open: cursor=%d top=%d, se esperaba 29 y 25", m.cursor, m.top)
	}

	s := newTestScreen(t, 20, 5)
	drawTabMenu(m, ws, s, 20)
	if got := screenLines(s)[4]; got != "> t29.txt" {
		t.Fatalf("última fila visible = %q, se esperaba %q (activa marcada)", got, "> t29.txt")
	}
	if !cellReverse(s, 0, 4) {
		t.Fatal("la pestaña activa 29 debe dibujarse resaltada en la última fila")
	}

	// Subir una: la activa sigue dentro de la ventana [25,30), así que el
	// scroll NO se mueve todavía —scroll mínimo, como el explorador.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if m.cursor != 28 || m.top != 25 {
		t.Fatalf("tras Up: cursor=%d top=%d, se esperaba 28 y 25 (sin mover el top)", m.cursor, m.top)
	}

	// Subir hasta que la activa salga por arriba: el top la acompaña.
	for range 5 {
		m.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	}
	if m.cursor != 23 || m.top != 23 {
		t.Fatalf("tras subir hasta salir de la ventana: cursor=%d top=%d, se esperaba 23 y 23", m.cursor, m.top)
	}
}

// TestTabMenuPages: PageUp/PageDown saltan una página (el alto del menú) y se
// clamps a los bordes de la lista, como el explorador.
func TestTabMenuPages(t *testing.T) {
	ws := menuWorkspace(t, 20)
	ws.SetActive(0) // el cursor de Open cae en la activa: 0, para saltar desde arriba
	m := NewTabMenu()
	m.Resize(10, 5)
	m.Open(ws)

	handled, activate := m.HandleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if !handled || activate {
		t.Fatalf("PgDn devolvió (handled=%v, activate=%v), se esperaba (true, false)", handled, activate)
	}
	if m.cursor != 5 {
		t.Fatalf("cursor = %d tras PgDn, se esperaba 5 (una página)", m.cursor)
	}

	m.HandleEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	if m.cursor != 0 {
		t.Fatalf("cursor = %d tras PgUp, se esperaba 0", m.cursor)
	}

	m.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	m.HandleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if m.cursor != 19 {
		t.Fatalf("cursor = %d tras PgDn al final, se esperaba 19 (clamp a la última)", m.cursor)
	}
}

// TestTabMenuMovementKeysAreConsumedEvenAtTheEdge: las teclas de movimiento se
// consumen aunque el cursor no se mueva —con el menú abierto, Up/Down son del
// menú y no del documento—, Home/End clamps a los bordes y Selected sigue al
// cursor.
func TestTabMenuMovementKeysAreConsumedEvenAtTheEdge(t *testing.T) {
	ws := newTabWorkspace(t, "a.txt", "b.txt", "c.txt")
	ws.SetActive(1)
	m := NewTabMenu()
	m.Open(ws)

	if handled, activate := m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)); !handled || activate {
		t.Fatalf("Down devolvió (handled=%v, activate=%v), se esperaba (true, false)", handled, activate)
	}
	if got := m.Selected(); got != 2 {
		t.Fatalf("Selected() = %d tras Down, se esperaba 2", got)
	}
	if handled, activate := m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)); !handled || activate {
		t.Fatalf("Down al final devolvió (handled=%v, activate=%v), se esperaba (true, false)", handled, activate)
	}
	if handled, activate := m.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)); !handled || activate {
		t.Fatalf("Up devolvió (handled=%v, activate=%v), se esperaba (true, false)", handled, activate)
	}

	m.HandleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if got := m.Selected(); got != 0 {
		t.Fatalf("Selected() = %d tras Home, se esperaba 0", got)
	}
	m.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if got := m.Selected(); got != 2 {
		t.Fatalf("Selected() = %d tras End, se esperaba 2", got)
	}
}

// TestTabMenuEnterActivatesTheCursor: Enter (y KeyLF) devuelven (true, true) —
// la señal para que el controlador cambie a la pestaña del cursor—; sin
// pestañas el conteo es 0 (capturado en Open) y Enter no activa nada.
func TestTabMenuEnterActivatesTheCursor(t *testing.T) {
	ws := newTabWorkspace(t, "a.txt", "b.txt")
	ws.SetActive(1)
	m := NewTabMenu()
	m.Open(ws)

	for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyLF} {
		handled, activate := m.HandleEvent(tcell.NewEventKey(key, 0, tcell.ModNone))
		if !handled || !activate {
			t.Fatalf("%v devolvió (handled=%v, activate=%v), se esperaba (true, true)", key, handled, activate)
		}
	}

	m.Open(model.NewWorkspace())
	if handled, activate := m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); handled || activate {
		t.Fatalf("Enter sin pestañas devolvió (handled=%v, activate=%v), se esperaba (false, false)", handled, activate)
	}
}

// TestTabMenuEscapeAndForeignKeysAreNotHandled: Escape y Ctrl+C —y cualquier
// tecla ajena— NO los maneja el menú: devuelven (false, false) para que el
// controlador cierre el menú descartando, exactamente como el explorador no
// maneja Escape.
func TestTabMenuEscapeAndForeignKeysAreNotHandled(t *testing.T) {
	m := NewTabMenu()
	m.Resize(10, 5)
	m.Open(menuWorkspace(t, 3))
	before := m.cursor // Open deja el cursor sobre la activa: la última del workspace

	for _, ev := range []tcell.Event{
		tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone),
	} {
		if handled, activate := m.HandleEvent(ev); handled || activate {
			t.Fatalf("%v devolvió (handled=%v, activate=%v), se esperaba (false, false)", ev, handled, activate)
		}
	}
	if m.cursor != before {
		t.Fatalf("el cursor no debe moverse con teclas ajenas: quedó %d, se esperaba %d", m.cursor, before)
	}
}

// TestTabMenuLetsCtrlPageKeysFallThrough: Ctrl+PageUp/PageDown cambian de
// pestaña y son del controlador (U2b); el menú no los consume —el mismo guard
// defensivo que el explorador y el editor—.
func TestTabMenuLetsCtrlPageKeysFallThrough(t *testing.T) {
	m := NewTabMenu()
	m.Resize(10, 5)
	m.Open(menuWorkspace(t, 20))
	m.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	before := m.cursor

	for _, key := range []tcell.Key{tcell.KeyPgUp, tcell.KeyPgDn} {
		if handled, activate := m.HandleEvent(tcell.NewEventKey(key, 0, tcell.ModCtrl)); handled || activate {
			t.Fatalf("%v con Ctrl devolvió (handled=%v, activate=%v), se esperaba (false, false)", key, handled, activate)
		}
	}
	if m.cursor != before {
		t.Fatalf("el cursor del menú no debe moverse con Ctrl+PgUp/PgDn: quedó %d", m.cursor)
	}
}

// TestTabMenuDrawsNothingWithNoTabs: sin pestañas (o sin alto) no se dibuja
// nada, aunque el menú tenga región.
func TestTabMenuDrawsNothingWithNoTabs(t *testing.T) {
	empty := model.NewWorkspace()
	m := NewTabMenu()
	m.Resize(10, 5)
	m.Open(empty)

	s := newTestScreen(t, 10, 5)
	drawTabMenu(m, empty, s, 10)
	for y := 0; y < 5; y++ {
		if got := cellRuneAt(s, 0, y); got != 0 {
			t.Fatalf("fila %d = %q, sin pestañas no debe dibujarse nada", y, got)
		}
	}
}
