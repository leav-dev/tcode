package view

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// newList devuelve n nodos-raíz de prueba con nombres estables ("e00", "e01",
// ...) y rutas inventadas (archivos). Los tests de la vista construyen las
// entradas a mano: el explorador solo dibuja, navega y colapsa, nunca lee el
// filesystem.
func newList(n int) []Entry {
	entries := make([]Entry, n)
	for i := range entries {
		entries[i] = Entry{
			Name: fmt.Sprintf("e%02d", i),
			Path: fmt.Sprintf("/cwd/e%02d.txt", i),
		}
	}
	return entries
}

// TestFileBrowserDrawsIndentedWithPrefixes: cada nodo se dibuja con su
// indentación (2 celdas por nivel) y su prefijo —"▸ " dir colapsado, "▾ "
// dir expandido, "  " archivo—, y los directorios con el sufijo "/".
func TestFileBrowserDrawsIndentedWithPrefixes(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(20, 6)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{
		{Name: "docs", Path: "/cwd/docs", IsDir: true},
		{Name: "notas.txt", Path: "/cwd/notas.txt"},
	})

	s := newTestScreen(t, 20, 6)
	fb.Draw(s)
	s.Show()

	if got := screenLines(s)[0]; got != "▸ docs/" {
		t.Fatalf("fila 0 = %q, se esperaba %q (dir colapsado con prefijo y sufijo)", got, "▸ docs/")
	}
	if got := screenLines(s)[1]; got != "  notas.txt" {
		t.Fatalf("fila 1 = %q, se esperaba %q (archivo con prefijo de 2 celdas)", got, "  notas.txt")
	}
	if got := screenLines(s)[2]; got != "" {
		t.Fatalf("fila 2 = %q, se esperaba vacía: el árbol termina", got)
	}
}

// TestFileBrowserHighlightsTheCursorRow: la fila del cursor va con la barra de
// selección (fondo de acento, TreeCursor) a todo el ancho y las demás con el
// fondo del tema (Text ya pinta su propio fondo, ya no el de la terminal); el
// cursor por defecto es el primer nodo. La barra se verifica por fondo
// explícito —no por Reverse—: el fondo no depende de los colores default de la
// terminal y es lo que hace visible la selección.
func TestFileBrowserHighlightsTheCursorRow(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(20, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{
		{Name: "a.txt", Path: "/cwd/a.txt"},
		{Name: "b.txt", Path: "/cwd/b.txt"},
	})

	bar := tcell.PaletteColor(24)        // el fondo del TreeCursor por defecto
	docBg := tcell.NewHexColor(0x1E1E1E) // el fondo del documento en Dark+ (Text)
	s := newTestScreen(t, 20, 5)
	fb.Draw(s)
	s.Show()

	if cellBg(s, 0, 0) != bar {
		t.Fatal("la fila del cursor debe llevar el fondo de la barra de selección")
	}
	// La barra de selección es de ancho completo: el final de la fila 0 también
	// lleva el fondo, aunque la entrada sea corta.
	if cellBg(s, 10, 0) != bar {
		t.Fatal("la fila del cursor debe resaltarse a todo el ancho del panel")
	}
	if cellBg(s, 0, 1) != docBg {
		t.Fatal("las demás filas deben ir con el fondo del tema, ya no el de la terminal")
	}

	action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if action != ActionMove || !handled {
		t.Fatalf("Down devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	fb.Draw(s)
	s.Show()
	if cellBg(s, 0, 1) != bar {
		t.Fatal("tras Down, la fila 1 debe llevar la barra de selección")
	}
	if cellBg(s, 0, 0) != docBg {
		t.Fatal("tras Down, la fila 0 pierde la barra de selección y queda con el fondo del tema")
	}
}

// TestFileBrowserActiveFileRowShowsMarker: la fila activa de un ARCHIVO lleva
// el marcador "> " en lugar del prefijo "  " (misma semántica de 2 celdas),
// y al mover el cursor el marcador viaja con él. Los DIRECTORIOS conservan su
// caret ▸/▾ también en la fila activa: el caret es el indicador de expansión,
// reemplazarlo perdería el estado expandido/colapsado.
func TestFileBrowserActiveFileRowShowsMarker(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(20, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{
		{Name: "a.txt", Path: "/cwd/a.txt"},
		{Name: "b.txt", Path: "/cwd/b.txt"},
	})

	s := newTestScreen(t, 20, 5)
	fb.Draw(s)
	s.Show()
	if got := screenLines(s)[0]; got != "> a.txt" {
		t.Fatalf("fila activa de archivo = %q, se esperaba %q (marcador de posición)", got, "> a.txt")
	}
	if got := screenLines(s)[1]; got != "  b.txt" {
		t.Fatalf("fila inactiva de archivo = %q, se esperaba %q (prefijo sin marcador)", got, "  b.txt")
	}

	fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	fb.Draw(s)
	s.Show()
	if got := screenLines(s)[0]; got != "  a.txt" {
		t.Fatalf("fila inactiva de archivo = %q, se esperaba %q (prefijo sin marcador)", got, "  a.txt")
	}
	if got := screenLines(s)[1]; got != "> b.txt" {
		t.Fatalf("fila activa tras Down = %q, se esperaba %q (el marcador viaja con el cursor)", got, "> b.txt")
	}

	// Un dir en la fila activa conserva su caret, sin marcador "> ".
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "docs", Path: "/cwd/docs", IsDir: true}})
	fb.Draw(s)
	s.Show()
	if got := screenLines(s)[0]; got != "▸ docs/" {
		t.Fatalf("fila activa de dir = %q, se esperaba %q (caret de colapsado, no marcador)", got, "▸ docs/")
	}
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})
	fb.Draw(s)
	s.Show()
	if got := screenLines(s)[0]; got != "▾ docs/" {
		t.Fatalf("fila activa de dir expandido = %q, se esperaba %q (caret de expandido)", got, "▾ docs/")
	}
}

// TestFileBrowserExpandingShowsChildrenIndented: SetChildren inyecta los hijos
// del dir del cursor con depth+1, el dir queda expandido ("▾"), los hijos se
// aplanan a continuación con su indentación y el cursor queda en el dir.
func TestFileBrowserExpandingShowsChildrenIndented(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(20, 8)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "src", Path: "/cwd/src", IsDir: true}})

	action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if action != ActionExpand || !handled {
		t.Fatalf("Enter sobre el dir colapsado devolvió (action=%v, handled=%v), se esperaba (ActionExpand, true)", action, handled)
	}
	// El controlador leyó el dir y deposita los hijos: el cursor sigue sobre el
	// dir expandido, con sus hijos a continuación.
	fb.SetChildren([]Entry{
		{Name: "main.go", Path: "/cwd/src/main.go"},
		{Name: "internal", Path: "/cwd/src/internal", IsDir: true},
	})
	if got := fb.CursorPath(); got != "/cwd/src" {
		t.Fatalf("CursorPath() = %q tras expandir, se esperaba %q (el cursor en el dir)", got, "/cwd/src")
	}

	// Nivel 2: bajar a internal y expandir con SetChildren.
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) // main.go
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) // internal
	fb.SetChildren([]Entry{{Name: "code.go", Path: "/cwd/src/internal/code.go"}})

	s := newTestScreen(t, 20, 8)
	fb.Draw(s)
	s.Show()
	lines := screenLines(s)
	// Un nodo de profundidad d lleva d*2 celdas de indentación MÁS las 2 del
	// prefijo (la columna de la flecha se alinea en todos los niveles): un
	// archivo de profundidad 1 va con 4 espacios y uno de profundidad 2 con 6.
	want := []string{
		"▾ src/",
		"    main.go",
		"  ▾ internal/",
		"      code.go",
	}
	for i, w := range want {
		if lines[i] != w {
			t.Fatalf("fila %d = %q, se esperaba %q", i, lines[i], w)
		}
	}
	if lines[4] != "" {
		t.Fatalf("fila 4 = %q, se esperaba vacía: el árbol termina", lines[4])
	}
}

// TestFileBrowserCollapsingHidesChildrenAndKeepsTheCursorOnTheDir: Left sobre
// el dir expandido del cursor lo colapsa INTERNAMENTE (sin E/S): el aplanado
// se reconstruye sin los hijos y el cursor queda en el dir colapsado, con su
// ruta intacta. Los hijos se CONSERVAN en el nodo: re-expandir no los relee.
func TestFileBrowserCollapsingHidesChildrenAndKeepsTheCursorOnTheDir(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(20, 8)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "docs", Path: "/cwd/docs", IsDir: true}})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})

	// Bajar al hijo y volver al dir: el colapso es sobre el dir del cursor.
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))

	action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if action != ActionMove || !handled {
		t.Fatalf("Left sobre el dir expandido devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if got := fb.CursorPath(); got != "/cwd/docs" {
		t.Fatalf("CursorPath() = %q, se esperaba %q (el cursor en el dir colapsado)", got, "/cwd/docs")
	}

	s := newTestScreen(t, 20, 8)
	fb.Draw(s)
	s.Show()
	if got := screenLines(s)[0]; got != "▸ docs/" {
		t.Fatalf("fila 0 = %q, se esperaba %q (dir colapsado)", got, "▸ docs/")
	}
	if got := screenLines(s)[1]; got != "" {
		t.Fatalf("fila 1 = %q, se esperaba vacía: los hijos desaparecieron del aplanado", got)
	}

	// Re-expandir sin re-leer: SetChildren de nuevo no duplica los hijos que el
	// nodo conserva; solo re-expande.
	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})
	fb.Draw(s)
	s.Show()
	if got := screenLines(s)[0]; got != "▾ docs/" {
		t.Fatalf("fila 0 = %q tras re-expandir, se esperaba %q (dir expandido)", got, "▾ docs/")
	}
	if got := screenLines(s)[1]; got != "    a.txt" {
		t.Fatalf("fila 1 = %q tras re-expandir, se esperaba %q (única, con su indentación)", got, "    a.txt")
	}
	if got := screenLines(s)[2]; got != "" {
		t.Fatalf("fila 2 = %q, se esperaba vacía: los hijos no se duplican", got)
	}
}

// TestFileBrowserSetChildrenIsDefensive: SetChildren es no-op si el nodo del
// cursor no es un dir, y si el dir ya cargó sus hijos no los duplica. Sin
// nodos no hace nada.
func TestFileBrowserSetChildrenIsDefensive(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "a.txt", Path: "/cwd/a.txt"}})

	// El nodo del cursor es un archivo: SetChildren no inyecta nada.
	fb.SetChildren([]Entry{{Name: "hijo", Path: "/cwd/a.txt/hijo"}})
	if got := fb.CursorPath(); got != "/cwd/a.txt" {
		t.Fatalf("CursorPath() = %q, el nodo archivo no debía cambiar", got)
	}

	// Dir expandido con hijos: un SetChildren posterior no duplica.
	fb.SetRootEntries([]Entry{{Name: "docs", Path: "/cwd/docs", IsDir: true}})
	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})
	fb.SetChildren([]Entry{{Name: "b.txt", Path: "/cwd/docs/b.txt"}})

	s := newTestScreen(t, 10, 5)
	fb.Draw(s)
	s.Show()
	if got := screenLines(s)[1]; got != "    a.txt" {
		t.Fatalf("fila 1 = %q, se esperaba %q (un solo hijo, con su indentación)", got, "    a.txt")
	}
	if got := screenLines(s)[2]; got != "" {
		t.Fatalf("fila 2 = %q, se esperaba vacía: el segundo SetChildren no duplicó", got)
	}

	// Sin nodos: no-op total.
	fb.SetRootEntries(nil)
	fb.SetChildren([]Entry{{Name: "x", Path: "/cwd/x"}})
	if got := fb.CursorPath(); got != "" {
		t.Fatalf("CursorPath() = %q sin nodos, se esperaba \"\"", got)
	}
}

// TestFileBrowserSetRootResetsTheTree: SetRoot descarta el estado anterior del
// árbol —nodos, cursor y scroll—; el nuevo primer nivel se carga con
// SetRootEntries y el cursor arranca en 0.
func TestFileBrowserSetRootResetsTheTree(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "docs", Path: "/cwd/docs", IsDir: true}})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))

	fb.SetRoot("/otro")
	if got := fb.CursorPath(); got != "" {
		t.Fatalf("CursorPath() = %q tras SetRoot, se esperaba \"\" (el árbol quedó vacío)", got)
	}
	if fb.cursor != 0 || fb.top != 0 {
		t.Fatalf("tras SetRoot: cursor=%d top=%d, se esperaban 0 y 0", fb.cursor, fb.top)
	}

	fb.SetRootEntries([]Entry{{Name: "b.txt", Path: "/otro/b.txt"}})
	if got := fb.CursorPath(); got != "/otro/b.txt" {
		t.Fatalf("CursorPath() = %q tras SetRootEntries, se esperaba el nuevo primer nivel", got)
	}
}

// TestFileBrowserEnterTogglesAnExpandedDirectory: Enter sobre un dir colapsado
// pide sus hijos con ActionExpand; sobre el mismo dir ya expandido lo COLAPSA
// (toggle: la misma tecla abre y cierra, sin E/S ni relectura) y la selección
// queda en el dir.
func TestFileBrowserEnterTogglesAnExpandedDirectory(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "docs", Path: "/cwd/docs", IsDir: true}})

	action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if action != ActionExpand || !handled {
		t.Fatalf("Enter sobre un dir colapsado devolvió (action=%v, handled=%v), se esperaba (ActionExpand, true)", action, handled)
	}

	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})
	action, handled = fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if action != ActionMove || !handled {
		t.Fatalf("Enter sobre un dir ya expandido devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if got := fb.CursorPath(); got != "/cwd/docs" {
		t.Fatalf("CursorPath() = %q tras el toggle, se esperaba %q (la selección queda en el dir)", got, "/cwd/docs")
	}
	if n := len(fb.nodes); n != 1 {
		t.Fatalf("aplanado = %d nodos tras el toggle, se esperaba 1 (los hijos colapsaron)", n)
	}
	// Y un Enter más lo vuelve a expandir sin releer.
	action, handled = fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if action != ActionExpand || !handled {
		t.Fatalf("el tercer Enter devolvió (action=%v, handled=%v), se esperaba (ActionExpand, true)", action, handled)
	}
}

// TestFileBrowserRightActsOnFilesAndDirs: Right es la otra tecla de avance:
// sobre un archivo activa (ActionActivate), sobre un dir colapsado expande
// (ActionExpand) y sobre un dir expandido no hace nada (ActionNone).
func TestFileBrowserRightActsOnFilesAndDirs(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{
		{Name: "docs", Path: "/cwd/docs", IsDir: true},
		{Name: "a.txt", Path: "/cwd/a.txt"},
	})

	// Right sobre un archivo → ActionActivate.
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) // a.txt
	if action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); action != ActionActivate || !handled {
		t.Fatalf("Right sobre un archivo devolvió (action=%v, handled=%v), se esperaba (ActionActivate, true)", action, handled)
	}

	// Right sobre un dir colapsado → ActionExpand.
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)) // docs
	if action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); action != ActionExpand || !handled {
		t.Fatalf("Right sobre un dir colapsado devolvió (action=%v, handled=%v), se esperaba (ActionExpand, true)", action, handled)
	}

	// Right sobre el dir ya expandido → lo colapsa (toggle, como Enter).
	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})
	action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if action != ActionMove || !handled {
		t.Fatalf("Right sobre un dir ya expandido devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	// El árbol raíz tiene docs + a.txt: tras el toggle solo se ocultan los
	// hijos del dir colapsado.
	if n := len(fb.nodes); n != 2 {
		t.Fatalf("aplanado = %d nodos tras el toggle con Right, se esperaba 2", n)
	}
}

// TestFileBrowserLeftClosesDirectories: ← colapsa el dir expandido del cursor
// (ActionMove); con la selección en un hijo primero sube al padre y el
// siguiente ← colapsa (la forma de cerrar la carpeta estando dentro); y con el
// foco en el panel la flecha es SIEMPRE del árbol —también sin nada que
// colapsar ni subir—, nunca cae al editor.
func TestFileBrowserLeftClosesDirectories(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{
		{Name: "docs", Path: "/cwd/docs", IsDir: true},
		{Name: "a.txt", Path: "/cwd/a.txt"},
	})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	fb.SetChildren([]Entry{{Name: "hijo.txt", Path: "/cwd/docs/hijo.txt"}})

	// Cursor en un hijo: el primer ← sube la selección al padre (sigue
	// expandido) y el segundo lo colapsa.
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) // hijo.txt
	if action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); action != ActionMove || !handled {
		t.Fatalf("Left sobre un hijo devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if got := fb.CursorPath(); got != "/cwd/docs" {
		t.Fatalf("CursorPath() = %q tras el primer ←, se esperaba el padre %q", got, "/cwd/docs")
	}
	if n := len(fb.nodes); n != 3 {
		t.Fatalf("aplanado = %d nodos tras subir, se esperaba 3 (el dir sigue expandido)", n)
	}

	if action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); action != ActionMove || !handled {
		t.Fatalf("el segundo ← devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if got := fb.CursorPath(); got != "/cwd/docs" {
		t.Fatalf("CursorPath() = %q tras colapsar, se esperaba %q", got, "/cwd/docs")
	}
	if n := len(fb.nodes); n != 2 {
		t.Fatalf("aplanado = %d nodos tras colapsar, se esperaba 2", n)
	}

	// Cursor en un archivo del nivel raíz: no hay padre ni dir que colapsar,
	// pero la flecha se come igual (no escapa al editor).
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)) // a.txt
	if action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); action != ActionMove || !handled {
		t.Fatalf("Left sobre un archivo raíz devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if got := fb.CursorPath(); got != "/cwd/a.txt" {
		t.Fatalf("CursorPath() = %q, no debía moverse en el nivel raíz", got)
	}

	// Re-expandir (el dir conserva sus hijos; el controlador re-lee igual y
	// llama SetChildren, que no duplica) y colapsar de nuevo: idéntico.
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)) // docs (colapsado)
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	fb.SetChildren([]Entry{{Name: "hijo.txt", Path: "/cwd/docs/hijo.txt"}})
	if n := len(fb.nodes); n != 3 {
		t.Fatalf("aplanado = %d nodos al re-expandir, se esperaba 3", n)
	}
	if action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); action != ActionMove || !handled {
		t.Fatalf("Left tras re-expandir devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if n := len(fb.nodes); n != 2 {
		t.Fatalf("aplanado = %d nodos tras re-colapsar, se esperaba 2", n)
	}
}

// TestFileBrowserLeftCollapsesASubdirectoryInPlace: ← sobre una SUBCARPETA
// desplegada (nivel >= 1) la colapsa y la selección queda EN ELLA, sin saltar
// al directorio padre: el colapso nunca "se lleva" el cursor a otro nivel.
func TestFileBrowserLeftCollapsesASubdirectoryInPlace(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(30, 12)
	fb.SetRoot("/repo")
	fb.SetRootEntries([]Entry{
		{Name: "internal", Path: "/repo/internal", IsDir: true},
		{Name: "main.go", Path: "/repo/main.go"},
	})

	// internal → view → sus hijos: dos niveles anidados expandidos.
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)) // internal
	fb.SetChildren([]Entry{
		{Name: "view", Path: "/repo/internal/view", IsDir: true},
		{Name: "model", Path: "/repo/internal/model", IsDir: true},
	})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))  // view
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)) // expandir view
	fb.SetChildren([]Entry{
		{Name: "file_browser.go", Path: "/repo/internal/view/file_browser.go"},
		{Name: "surface.go", Path: "/repo/internal/view/surface.go"},
	})

	// Cursor sobre view (expandida) y ←: view colapsa, sus hijos desaparecen
	// y el cursor queda EN view —no salta al abuelo internal—.
	if action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); action != ActionMove || !handled {
		t.Fatalf("Left sobre la subcarpeta devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if got := fb.CursorPath(); got != "/repo/internal/view" {
		t.Fatalf("CursorPath() = %q tras colapsar view, se esperaba %q (la selección no se va al padre)", got, "/repo/internal/view")
	}
	// El aplanado: internal, view, model, main.go — sin los hijos de view.
	if n := len(fb.nodes); n != 4 {
		t.Fatalf("aplanado = %d nodos tras colapsar view, se esperaba 4", n)
	}
}

// TestFileBrowserClickOnTheExpansionArrowToggles: el clic sobre la flecha
// (▸/▾) de un directorio alterna colapsado ↔ expandido; el clic en el resto de
// la fila solo selecciona, sin alternar nada.
func TestFileBrowserClickOnTheExpansionArrowToggles(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{
		{Name: "docs", Path: "/cwd/docs", IsDir: true},
		{Name: "a.txt", Path: "/cwd/a.txt"},
	})

	// docs (dir, depth 0): su flecha ocupa las columnas 0..1 de la fila 0.
	// Clic en la flecha de un dir colapsado → el árbol pide sus hijos.
	action, handled := fb.HandleEvent(tcell.NewEventMouse(1, 0, tcell.Button1, tcell.ModNone))
	if action != ActionExpand || !handled {
		t.Fatalf("clic en la flecha de un dir colapsado devolvió (action=%v, handled=%v), se esperaba (ActionExpand, true)", action, handled)
	}
	if got := fb.CursorPath(); got != "/cwd/docs" {
		t.Fatalf("CursorPath() = %q tras el clic en la flecha, se esperaba el dir", got)
	}

	// El controlador deposita los hijos y el clic en la flecha del dir ya
	// expandido lo colapsa.
	fb.SetChildren([]Entry{{Name: "hijo.txt", Path: "/cwd/docs/hijo.txt"}})
	if n := len(fb.nodes); n != 3 {
		t.Fatalf("aplanado = %d nodos al expandir, se esperaba 3", n)
	}
	action, handled = fb.HandleEvent(tcell.NewEventMouse(1, 0, tcell.Button1, tcell.ModNone))
	if action != ActionMove || !handled {
		t.Fatalf("clic en la flecha de un dir expandido devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if n := len(fb.nodes); n != 2 {
		t.Fatalf("aplanado = %d nodos al colapsar con el clic, se esperaba 2", n)
	}

	// Clic sobre el nombre (fuera de la flecha, x=3) solo selecciona: el dir
	// sigue colapsado y no se alterna nada.
	action, handled = fb.HandleEvent(tcell.NewEventMouse(3, 0, tcell.Button1, tcell.ModNone))
	if action != ActionMove || !handled {
		t.Fatalf("clic en el nombre devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if n := len(fb.nodes); n != 2 {
		t.Fatalf("aplanado = %d nodos tras el clic en el nombre, se esperaba 2 (sin toggle)", n)
	}
}

// TestFileBrowserEnterActivatesFilesAndFallsOtherwise: Enter sobre un archivo
// devuelve (ActionActivate, true); sin nodos, Enter y toda tecla ajena caen al
// flujo normal con (ActionNone, false); el movimiento se consume aunque el
// cursor no se mueva: con el foco en el panel, Up/Down son del explorador, no
// del documento.
func TestFileBrowserEnterActivatesFilesAndFallsOtherwise(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "a.txt", Path: "/cwd/a.txt"}})

	action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if action != ActionActivate || !handled {
		t.Fatalf("Enter sobre un archivo devolvió (action=%v, handled=%v), se esperaba (ActionActivate, true)", action, handled)
	}

	// Sin nodos: Enter no activa nada.
	fb.SetRootEntries(nil)
	if action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); action != ActionNone || handled {
		t.Fatalf("Enter sin nodos devolvió (action=%v, handled=%v), se esperaba (ActionNone, false)", action, handled)
	}

	// Una tecla de texto, Escape y Ctrl+S no son del explorador.
	fb.SetRootEntries(newList(3))
	for _, ev := range []tcell.Event{
		tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone),
	} {
		if action, handled := fb.HandleEvent(ev); action != ActionNone || handled {
			t.Fatalf("%v devolvió (action=%v, handled=%v), se esperaba (ActionNone, false)", ev, action, handled)
		}
	}

	// Down al final se consume igual: el foco está en el panel.
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	fb.SetRootEntries(newList(3))
	if action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)); action != ActionMove || !handled {
		t.Fatalf("Down al final devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
}

// TestFileBrowserPages: PageUp/PageDown saltan una página (el alto del panel)
// y se clamps a los bordes del árbol.
func TestFileBrowserPages(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries(newList(20))

	action, handled := fb.HandleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if action != ActionMove || !handled {
		t.Fatalf("PgDn devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
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

// TestFileBrowserMouseSelectsAndScrollsAtAnyDepth: el clic selecciona la fila
// —también un nodo anidado que esté en el aplanado— y la rueda scrollea sin
// tocar el cursor fuera de la ventana.
func TestFileBrowserMouseSelectsAndScrollsAtAnyDepth(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "docs", Path: "/cwd/docs", IsDir: true}})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})
	// Aplanado: [docs, a.txt].

	action, handled := fb.HandleEvent(tcell.NewEventMouse(1, 1, tcell.Button1, tcell.ModNone))
	if action != ActionMove || !handled {
		t.Fatalf("clic devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if got := fb.CursorPath(); got != "/cwd/docs/a.txt" {
		t.Fatalf("CursorPath() = %q tras el clic en la fila 1, se esperaba el nodo anidado", got)
	}

	// Un clic fuera de las filas del panel no selecciona nada.
	action, handled = fb.HandleEvent(tcell.NewEventMouse(1, 50, tcell.Button1, tcell.ModNone))
	if action != ActionNone || handled {
		t.Fatalf("clic fuera del panel devolvió (action=%v, handled=%v), se esperaba (ActionNone, false)", action, handled)
	}
	if got := fb.CursorPath(); got != "/cwd/docs/a.txt" {
		t.Fatalf("CursorPath() = %q tras el clic fuera del panel, no debía cambiar", got)
	}

	// La rueda mueve la selección y la activa sigue visible: 3 filas abajo del
	// clic. SetRootEntries resetea el cursor a 0, así que el clic en la fila 2
	// lo deja en 2 y la rueda lo lleva a 5, con la activa en la última fila.
	fb.SetRootEntries(newList(15))
	action, handled = fb.HandleEvent(tcell.NewEventMouse(1, 2, tcell.Button1, tcell.ModNone))
	if action != ActionMove || !handled {
		t.Fatalf("clic devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if fb.cursor != 2 {
		t.Fatalf("cursor = %d tras el clic en la fila 2, se esperaba 2", fb.cursor)
	}
	action, handled = fb.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, tcell.ModNone))
	if action != ActionMove || !handled {
		t.Fatalf("la rueda devolvió (action=%v, handled=%v), se esperaba (ActionMove, true)", action, handled)
	}
	if fb.cursor != 5 {
		t.Fatalf("cursor = %d tras la rueda, se esperaba 5", fb.cursor)
	}
	if fb.top != 1 {
		t.Fatalf("top = %d tras la rueda, se esperaba 1 (la activa en la última fila)", fb.top)
	}
}

// TestFileBrowserScrollKeepsTheActiveVisible: con más nodos que el alto, mover
// el cursor corre el scroll vertical lo mínimo para que la activa siga
// visible, y la fila visible del cursor siempre existe.
func TestFileBrowserScrollKeepsTheActiveVisible(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries(newList(30))

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
	if got := screenLines(s)[4]; got != "> e29" {
		t.Fatalf("última fila visible = %q, se esperaba %q (entrada activa con marcador)", got, "> e29")
	}
	if cellBg(s, 0, 4) != tcell.PaletteColor(24) {
		t.Fatal("la entrada activa 29 debe dibujarse con la barra de selección en la última fila")
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

// TestFileBrowserSetRootEntriesClampsTheCursor: un primer nivel nuevo clampa
// cursor y top al rango (la activa no puede quedar fuera), y un nivel vacío
// vuelve el cursor a 0 y no dibuja nada.
func TestFileBrowserSetRootEntriesClampsTheCursor(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries(newList(10))
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if fb.cursor != 9 {
		t.Fatalf("cursor = %d tras End, se esperaba 9", fb.cursor)
	}

	// SetRootEntries RESETEA cursor y scroll (a diferencia del SetEntries de U3,
	// que clampaba): el primer nivel nuevo es un estado nuevo, no un recorte.
	fb.SetRootEntries(newList(3))
	if fb.cursor != 0 {
		t.Fatalf("cursor = %d tras SetRootEntries con 3 entradas, se esperaba 0 (reseteado)", fb.cursor)
	}
	if fb.top != 0 {
		t.Fatalf("top = %d tras SetRootEntries, se esperaba 0", fb.top)
	}

	fb.SetRootEntries(nil)
	if fb.cursor != 0 || fb.top != 0 {
		t.Fatalf("con lista vacía cursor=%d top=%d, se esperaban 0 y 0", fb.cursor, fb.top)
	}

	// Con el primer nivel vacío no se dibuja nada, aunque el panel tenga alto.
	s := newTestScreen(t, 10, 5)
	fb.Draw(s)
	s.Show()
	for y := 0; y < 5; y++ {
		if got := cellRuneAt(s, 0, y); got != 0 {
			t.Fatalf("fila %d = %q, con el árbol vacío no debe dibujarse nada", y, got)
		}
	}
}

// TestFileBrowserLetsCtrlPageKeysFallToTheController: Ctrl+PageUp/PageDown
// cambian de pestaña y son del controlador (U2b); el explorador no los
// consume —el mismo guard defensivo que el editor— o el cambio de pestaña
// moriría con el foco en el panel.
func TestFileBrowserLetsCtrlPageKeysFallToTheController(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries(newList(20))
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	cursorBefore := fb.cursor

	for _, key := range []tcell.Key{tcell.KeyPgUp, tcell.KeyPgDn} {
		if action, handled := fb.HandleEvent(tcell.NewEventKey(key, 0, tcell.ModCtrl)); action != ActionNone || handled {
			t.Fatalf("%v con Ctrl devolvió (action=%v, handled=%v), se esperaba (ActionNone, false)", key, action, handled)
		}
	}
	if fb.cursor != cursorBefore {
		t.Fatalf("el cursor del panel no debe moverse con Ctrl+PgUp/PgDn: quedó %d", fb.cursor)
	}
}

// TestFileBrowserCursorPath: la ruta del nodo activo —del primer nivel o de un
// nodo anidado visible—, o "" sin nodos.
func TestFileBrowserCursorPath(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	if got := fb.CursorPath(); got != "" {
		t.Fatalf("CursorPath() = %q sin nodos, se esperaba \"\"", got)
	}

	fb.SetRootEntries([]Entry{
		{Name: "b.txt", Path: "/cwd/b.txt"},
		{Name: "c.txt", Path: "/cwd/c.txt"},
	})
	// El cursor arranca en el primer nodo.
	if got := fb.CursorPath(); got != "/cwd/b.txt" {
		t.Fatalf("CursorPath() = %q, se esperaba el primer nodo", got)
	}
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if got := fb.CursorPath(); got != "/cwd/c.txt" {
		t.Fatalf("CursorPath() = %q tras Down, se esperaba el segundo nodo", got)
	}

	// Un nodo anidado (hijo de un dir expandido) también tiene su ruta.
	fb.SetRootEntries([]Entry{{Name: "docs", Path: "/cwd/docs", IsDir: true}})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if got := fb.CursorPath(); got != "/cwd/docs/a.txt" {
		t.Fatalf("CursorPath() = %q sobre un nodo anidado, se esperaba %q", got, "/cwd/docs/a.txt")
	}
}

// TestFileBrowserRevealSelectsAVisibleNode: Reveal de un nodo ya visible lo
// selecciona y termina; Reveal de un path que NO está en el árbol (ni bajo
// ningún nodo visible) termina sin mover el cursor.
func TestFileBrowserRevealSelectsAVisibleNode(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{
		{Name: "a.txt", Path: "/cwd/a.txt"},
		{Name: "b.txt", Path: "/cwd/b.txt"},
	})

	done, dir := fb.Reveal("/cwd/b.txt")
	if !done || dir != "" {
		t.Fatalf("Reveal de un nodo visible = (done=%v, dir=%q), se esperaba (true, \"\")", done, dir)
	}
	if got := fb.CursorPath(); got != "/cwd/b.txt" {
		t.Fatalf("CursorPath() = %q tras Reveal, se esperaba el nodo pedido", got)
	}

	// Un path fuera del árbol (ni siquiera bajo un dir visible): termina sin
	// mover la selección.
	done, dir = fb.Reveal("/otro/x.txt")
	if !done || dir != "" {
		t.Fatalf("Reveal fuera del árbol = (done=%v, dir=%q), se esperaba (true, \"\")", done, dir)
	}
	if got := fb.CursorPath(); got != "/cwd/b.txt" {
		t.Fatalf("CursorPath() = %q tras un Reveal fuera del árbol, no debía moverse", got)
	}
}

// TestFileBrowserRevealExpandsACollapsedAncestor: Reveal de un archivo bajo un
// dir colapsado pide expandir ese dir ((false, dirPath)); el controlador
// deposita los hijos con ExpandDir y el Reveal siguiente termina con el cursor
// sobre el archivo. El acoplamiento vista/controlador es el mismo que
// Expand/Collapse: la vista nunca lee el filesystem.
func TestFileBrowserRevealExpandsACollapsedAncestor(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "docs", Path: "/cwd/docs", IsDir: true}})

	done, dir := fb.Reveal("/cwd/docs/a.txt")
	if done || dir != "/cwd/docs" {
		t.Fatalf("Reveal bajo un dir colapsado = (done=%v, dir=%q), se esperaba (false, \"/cwd/docs\")", done, dir)
	}

	// ExpandDir es por RUTA (el cursor no tiene que estar sobre el dir) y no
	// pierde el estado del árbol.
	if ok := fb.ExpandDir(dir, []Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}}); !ok {
		t.Fatal("ExpandDir debe devolver true para un dir del árbol")
	}
	if got := fb.CursorPath(); got != "/cwd/docs" {
		t.Fatalf("ExpandDir no debe mover el cursor: quedó en %q", got)
	}

	done, dir = fb.Reveal("/cwd/docs/a.txt")
	if !done || dir != "" {
		t.Fatalf("Reveal tras expandir = (done=%v, dir=%q), se esperaba (true, \"\")", done, dir)
	}
	if got := fb.CursorPath(); got != "/cwd/docs/a.txt" {
		t.Fatalf("CursorPath() = %q tras el reveal completo, se esperaba el archivo", got)
	}
}

// TestFileBrowserRevealWalksNestedCollapsedDirs: un camino de dos niveles
// colapsados se reavela en dos pasadas (una por dir que pedir), con el cursor
// estable fuera del dir que se expande; el reveal no depende del cursor.
func TestFileBrowserRevealWalksNestedCollapsedDirs(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 6)
	fb.SetRoot("/repo")
	fb.SetRootEntries([]Entry{
		{Name: "a.txt", Path: "/repo/a.txt"},
		{Name: "internal", Path: "/repo/internal", IsDir: true},
	})
	// El cursor queda en a.txt, lejos de la rama que el reveal va a abrir.
	if got := fb.CursorPath(); got != "/repo/a.txt" {
		t.Fatalf("CursorPath() = %q, se esperaba a.txt (cursor de arranque)", got)
	}

	done, dir := fb.Reveal("/repo/internal/view/editor.go")
	if done || dir != "/repo/internal" {
		t.Fatalf("pasada 1 = (done=%v, dir=%q), se esperaba (false, /repo/internal)", done, dir)
	}
	fb.ExpandDir(dir, []Entry{
		{Name: "view", Path: "/repo/internal/view", IsDir: true},
		{Name: "model", Path: "/repo/internal/model", IsDir: true},
	})

	done, dir = fb.Reveal("/repo/internal/view/editor.go")
	if done || dir != "/repo/internal/view" {
		t.Fatalf("pasada 2 = (done=%v, dir=%q), se esperaba (false, /repo/internal/view)", done, dir)
	}
	fb.ExpandDir(dir, []Entry{{Name: "editor.go", Path: "/repo/internal/view/editor.go"}})

	done, dir = fb.Reveal("/repo/internal/view/editor.go")
	if !done || dir != "" {
		t.Fatalf("pasada final = (done=%v, dir=%q), se esperaba (true, \"\")", done, dir)
	}
	if got := fb.CursorPath(); got != "/repo/internal/view/editor.go" {
		t.Fatalf("CursorPath() = %q, se esperaba el archivo revelado", got)
	}
}

// TestFileBrowserRevealKeepsLoadedChildren: un dir con hijos ya cargados —pero
// colapsado— se re-expande por ExpandDir SIN duplicar sus hijos (mismo diseño
// de re-expandir sin re-leer), y un Reveal posterior no re-expande nada.
func TestFileBrowserRevealKeepsLoadedChildren(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 6)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "docs", Path: "/cwd/docs", IsDir: true}})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)) // colapsar (guarda los hijos)

	if ok := fb.ExpandDir("/cwd/docs", []Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}}); !ok {
		t.Fatal("ExpandDir debe re-expandir el dir con hijos cargados")
	}
	fb.ExpandDir("/cwd/docs", []Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}}) // de nuevo: no duplica

	if n := len(fb.nodes); n != 2 {
		t.Fatalf("aplanado = %d nodos, se esperaba 2 (docs + su único hijo, sin duplicados)", n)
	}
	done, dir := fb.Reveal("/cwd/docs/a.txt")
	if !done || dir != "" {
		t.Fatalf("Reveal del archivo ya visible = (done=%v, dir=%q), se esperaba (true, \"\")", done, dir)
	}
}

// TestFileBrowserRevealStopsWhenTheAncestorCannotContainIt: un ancestro ya
// expandido cuyo contenido cargado no incluye el target termina el reveal sin
// mover el cursor: el archivo no existe bajo el árbol actual (el dir se leyó
// en su momento y no estaba ahí).
func TestFileBrowserRevealStopsWhenTheAncestorCannotContainIt(t *testing.T) {
	fb := NewFileBrowser()
	fb.Resize(10, 5)
	fb.SetRoot("/cwd")
	fb.SetRootEntries([]Entry{{Name: "docs", Path: "/cwd/docs", IsDir: true}})
	fb.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	fb.SetChildren([]Entry{{Name: "a.txt", Path: "/cwd/docs/a.txt"}})

	done, dir := fb.Reveal("/cwd/docs/otro.txt")
	if !done || dir != "" {
		t.Fatalf("Reveal de un inexistente en un dir expandido = (done=%v, dir=%q), se esperaba (true, \"\")", done, dir)
	}
	if got := fb.CursorPath(); got != "/cwd/docs" {
		t.Fatalf("CursorPath() = %q, no debía moverse (ni siquiera a la fila del dir)", got)
	}
}
