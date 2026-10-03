package view

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// newList devuelve n entradas de prueba con nombres estables ("e00", "e01",
// ...) y rutas inventadas. Los tests de la vista construyen las entradas a
// mano: el explorador solo dibuja y mueve el cursor, nunca lee el filesystem.
func newList(n int) []Entry {
	entries := make([]Entry, n)
	for i := range entries {
		entries[i] = Entry{
			Name: fmt.Sprintf("e%02d", i),
			Path: fmt.Sprintf("/tmp/e%02d.txt", i),
		}
	}
	return entries
}

// TestFileBrowserDrawsDirsWithSlash: los directorios se dibujan con el sufijo
// "/" y los archivos sin él, cada uno en su fila.
func TestFileBrowserDrawsDirsWithSlash(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(20, 5)
	fb.SetEntries([]Entry{
		{Name: "docs", Path: "/cwd/docs", IsDir: true},
		{Name: "notas.txt", Path: "/cwd/notas.txt"},
	})

	s := newTestScreen(t, 20, 5)
	fb.Draw(s)
	s.Show()

	if got := screenLines(s)[0]; got != "docs/" {
		t.Fatalf("fila 0 = %q, se esperaba %q (directorio con sufijo)", got, "docs/")
	}
	if got := screenLines(s)[1]; got != "notas.txt" {
		t.Fatalf("fila 1 = %q, se esperaba %q (archivo sin sufijo)", got, "notas.txt")
	}
	if got := screenLines(s)[2]; got != "" {
		t.Fatalf("fila 2 = %q, se esperaba vacía: la lista termina", got)
	}
}

// TestFileBrowserHighlightsTheCursorRow: la fila del cursor va en estilo
// invertido (a todo el ancho) y las demás con el estilo por defecto; el cursor
// por defecto es la primera entrada.
func TestFileBrowserHighlightsTheCursorRow(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(20, 5)
	fb.SetEntries([]Entry{
		{Name: "a.txt", Path: "/cwd/a.txt"},
		{Name: "b.txt", Path: "/cwd/b.txt"},
	})

	s := newTestScreen(t, 20, 5)
	fb.Draw(s)
	s.Show()

	if !cellReverse(s, 0, 0) {
		t.Fatal("la fila del cursor debe ir en estilo invertido")
	}
	// La barra de selección es de ancho completo: el final de la fila 0 también
	// queda invertido, aunque la entrada sea corta.
	if !cellReverse(s, 10, 0) {
		t.Fatal("la fila del cursor debe resaltarse a todo el ancho del panel")
	}
	if cellReverse(s, 0, 1) {
		t.Fatal("las demás filas deben ir con el estilo por defecto")
	}

	handled, activate := fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if !handled || activate {
		t.Fatalf("Down devolvió (handled=%v, activate=%v), se esperaba (true, false)", handled, activate)
	}
	fb.Draw(s)
	s.Show()
	if !cellReverse(s, 0, 1) {
		t.Fatal("tras Down, la fila 1 debe ser la resaltada")
	}
	if cellReverse(s, 0, 0) {
		t.Fatal("tras Down, la fila 0 pierde el resaltado")
	}
}

// TestFileBrowserScrollKeepsTheActiveVisible: con más entradas que el alto,
// mover el cursor corre el scroll vertical lo mínimo para que la activa siga
// visible, y la fila visible del cursor siempre existe.
func TestFileBrowserScrollKeepsTheActiveVisible(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetEntries(newList(30))

	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if fb.cursor != 29 {
		t.Fatalf("cursor = %d tras End, se esperaba 29", fb.cursor)
	}
	if fb.top != 25 {
		t.Fatalf("top = %d tras End, se esperaba 25 (la activa en la última fila)", fb.top)
	}

	s := newTestScreen(t, 10, 5)
	fb.Draw(s)
	s.Show()
	if got := screenLines(s)[4]; got != "e29" {
		t.Fatalf("última fila visible = %q, se esperaba la entrada 29", got)
	}
	if !cellReverse(s, 0, 4) {
		t.Fatal("la entrada activa 29 debe dibujarse resaltada en la última fila")
	}

	// Subir una: la activa sigue dentro de la ventana [25,30), así que el
	// scroll NO se mueve todavía —el scroll mínimo corre top solo cuando el
	// cursor saldría de la ventana, como el editor.
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if fb.cursor != 28 || fb.top != 25 {
		t.Fatalf("tras Up: cursor=%d top=%d, se esperaba 28 y 25 (sin mover el top)", fb.cursor, fb.top)
	}

	// Subir hasta que la activa salga por arriba: top la acompaña.
	for range 5 {
		fb.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	}
	if fb.cursor != 23 || fb.top != 23 {
		t.Fatalf("tras subir hasta salir de la ventana: cursor=%d top=%d, se esperaba 23 y 23", fb.cursor, fb.top)
	}
}

// TestFileBrowserSetEntriesClampsTheCursor: una lista nueva clampa cursor y
// top al rango (la activa no puede quedar fuera), y una lista vacía vuelve el
// cursor a 0 y no dibuja nada.
func TestFileBrowserSetEntriesClampsTheCursor(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetEntries(newList(10))
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if fb.cursor != 9 {
		t.Fatalf("cursor = %d tras End, se esperaba 9", fb.cursor)
	}

	fb.SetEntries(newList(3))
	if fb.cursor != 2 {
		t.Fatalf("cursor = %d tras recortar a 3 entradas, se esperaba 2", fb.cursor)
	}
	if fb.top != 0 {
		t.Fatalf("top = %d tras recortar, se esperaba 0", fb.top)
	}

	fb.SetEntries(nil)
	if fb.cursor != 0 || fb.top != 0 {
		t.Fatalf("con lista vacía cursor=%d top=%d, se esperaban 0 y 0", fb.cursor, fb.top)
	}

	// Con lista vacía no se dibuja nada, aunque el panel tenga alto.
	s := newTestScreen(t, 10, 5)
	fb.Draw(s)
	s.Show()
	for y := 0; y < 5; y++ {
		if got := cellRuneAt(s, 0, y); got != 0 {
			t.Fatalf("fila %d = %q, con lista vacía no debe dibujarse nada", y, got)
		}
	}
}

// TestFileBrowserPages: PageUp/PageDown saltan una página (el alto del panel)
// y se clamps a los bordes de la lista.
func TestFileBrowserPages(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetEntries(newList(20))

	handled, activate := fb.HandleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if !handled || activate {
		t.Fatalf("PgDn devolvió (handled=%v, activate=%v), se esperaba (true, false)", handled, activate)
	}
	if fb.cursor != 5 {
		t.Fatalf("cursor = %d tras PgDn, se esperaba 5 (una página)", fb.cursor)
	}

	fb.HandleEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	if fb.cursor != 0 {
		t.Fatalf("cursor = %d tras PgUp, se esperaba 0", fb.cursor)
	}

	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if fb.cursor != 19 {
		t.Fatalf("cursor = %d tras PgDn al final, se esperaba 19 (clamp a la última)", fb.cursor)
	}
}

// TestFileBrowserMouseSelectsAndScrolls: el clic selecciona la fila y la rueda
// scrollea la lista sin tocar el cursor fuera de la ventana.
func TestFileBrowserMouseSelectsAndScrolls(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetEntries(newList(15))

	handled, activate := fb.HandleEvent(tcell.NewEventMouse(1, 2, tcell.Button1, tcell.ModNone))
	if !handled || activate {
		t.Fatalf("clic devolvió (handled=%v, activate=%v), se esperaba (true, false)", handled, activate)
	}
	if fb.cursor != 2 {
		t.Fatalf("cursor = %d tras el clic en la fila 2, se esperaba 2", fb.cursor)
	}

	// Un clic fuera de las filas del panel no selecciona nada.
	fb.HandleEvent(tcell.NewEventMouse(1, 50, tcell.Button1, tcell.ModNone))
	if fb.cursor != 2 {
		t.Fatalf("cursor = %d tras el clic fuera del panel, no debía cambiar", fb.cursor)
	}

	// La rueda mueve la selección y la activa sigue visible: 3 filas abajo.
	handled, _ = fb.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, tcell.ModNone))
	if !handled {
		t.Fatal("la rueda debe manejarse")
	}
	if fb.cursor != 5 {
		t.Fatalf("cursor = %d tras la rueda, se esperaba 5", fb.cursor)
	}
	if fb.top != 1 {
		t.Fatalf("top = %d tras la rueda, se esperaba 1 (la activa en la última fila)", fb.top)
	}
}

// TestFileBrowserEnterActivatesTheActiveEntry: Enter sobre una entrada devuelve
// (true, true) —la señal para que el controlador abra o descienda—; sin
// entradas, Enter y cualquier tecla ajena caen al flujo normal con (false,
// false). Las teclas de movimiento se consumen aunque el cursor no se mueva:
// con el foco en el panel, Up/Down son del explorador, no del documento.
func TestFileBrowserEnterActivatesTheActiveEntry(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetEntries([]Entry{{Name: "a.txt", Path: "/cwd/a.txt"}})

	handled, activate := fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !handled || !activate {
		t.Fatalf("Enter devolvió (handled=%v, activate=%v), se esperaba (true, true)", handled, activate)
	}

	// Sin entradas: Enter no activa nada.
	fb.SetEntries(nil)
	if handled, activate := fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); handled || activate {
		t.Fatalf("Enter sin entradas devolvió (handled=%v, activate=%v), se esperaba (false, false)", handled, activate)
	}

	// Una tecla de texto y Escape no son del explorador.
	fb.SetEntries(newList(3))
	if handled, _ := fb.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)); handled {
		t.Fatal("una runa no debe manejarla el explorador")
	}
	if handled, _ := fb.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); handled {
		t.Fatal("Escape no debe manejarlo el explorador")
	}
	if handled, _ := fb.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone)); handled {
		t.Fatal("Ctrl+S no debe manejarlo el explorador")
	}

	// Up en el borde superior se consume igual: el foco está en el panel.
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if handled, activate := fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)); !handled || activate {
		t.Fatalf("Down al final devolvió (handled=%v, activate=%v), se esperaba (true, false)", handled, activate)
	}
}

// TestFileBrowserLetsCtrlPageKeysFallToTheController: Ctrl+PageUp/PageDown
// cambian de pestaña y son del controlador (U2b); el explorador no los
// consume —el mismo guard defensivo que el editor— u el cambio de pestaña
// moriría con el foco en el panel.
func TestFileBrowserLetsCtrlPageKeysFallToTheController(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetEntries(newList(20))
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	cursorBefore := fb.cursor

	for _, key := range []tcell.Key{tcell.KeyPgUp, tcell.KeyPgDn} {
		if handled, activate := fb.HandleEvent(tcell.NewEventKey(key, 0, tcell.ModCtrl)); handled || activate {
			t.Fatalf("%v con Ctrl devolvió (handled=%v, activate=%v), se esperaba (false, false)", key, handled, activate)
		}
	}
	if fb.cursor != cursorBefore {
		t.Fatalf("el cursor del panel no debe moverse con Ctrl+PgUp/PgDn: quedó %d", fb.cursor)
	}
}

// TestFileBrowserCursorPath: la ruta de la entrada activa, o "" sin entradas.
func TestFileBrowserCursorPath(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	if got := fb.CursorPath(); got != "" {
		t.Fatalf("CursorPath() = %q sin entradas, se esperaba \"\"", got)
	}

	fb.SetEntries([]Entry{
		{Name: "b.txt", Path: "/cwd/b.txt"},
		{Name: "c.txt", Path: "/cwd/c.txt"},
	})
	// El cursor arranca en la primera entrada.
	if got := fb.CursorPath(); got != "/cwd/b.txt" {
		t.Fatalf("CursorPath() = %q, se esperaba la primera entrada", got)
	}
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if got := fb.CursorPath(); got != "/cwd/c.txt" {
		t.Fatalf("CursorPath() = %q tras Down, se esperaba la segunda entrada", got)
	}
}
