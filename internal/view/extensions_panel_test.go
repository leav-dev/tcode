package view

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// drawPanel dibuja el panel y hace flush al front buffer: GetContents() lee el
// front, así que Show() es obligatorio.
func drawPanel(p *ExtensionsPanel, s tcell.SimulationScreen) {
	p.Draw(s)
	s.Show()
}

// newExtensionEntry es una entrada de catálogo de prueba.
func newExtensionEntry(id, name, version, subdir string, installed bool) ExtensionEntry {
	return ExtensionEntry{ID: id, Name: name, Version: version, Subdir: subdir, Installed: installed}
}

// TestExtensionsPanelHeight: el alto del panel es las entradas + el item final
// + el marco (2 filas).
func TestExtensionsPanelHeight(t *testing.T) {
	p := NewExtensionsPanel()
	if got := p.Height(); got != 3 {
		t.Fatalf("Height() sin entradas = %d, se esperaba 3 (solo el item final + marco)", got)
	}
	p.SetEntries([]ExtensionEntry{
		newExtensionEntry("a", "A", "1.0.0", "a", false),
		newExtensionEntry("b", "B", "2.0.0", "b", true),
	})
	if got := p.Height(); got != 5 {
		t.Fatalf("Height() con 2 entradas = %d, se esperaba 5 (2 + item final + marco)", got)
	}
}

// TestExtensionsPanelSetEntriesReplacesAndClears: SetEntries reemplaza el
// catálogo, limpia las marcas, el cursor y el scroll.
func TestExtensionsPanelSetEntriesReplacesAndClears(t *testing.T) {
	p := NewExtensionsPanel()
	p.SetEntries([]ExtensionEntry{newExtensionEntry("a", "A", "1.0.0", "a", false)})
	// Marcar y mover el cursor.
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	// SetEntries nuevo: sin marcas, cursor y scroll al principio.
	p.SetEntries([]ExtensionEntry{newExtensionEntry("b", "B", "2.0.0", "b", false)})
	if got := len(p.Entries()); got != 1 {
		t.Fatalf("Entries() = %d entradas, se esperaba 1", got)
	}
	if p.Entries()[0].ID != "b" {
		t.Fatalf("Entries()[0].ID = %q, se esperaba %q", p.Entries()[0].ID, "b")
	}
	if p.cursor != 0 || p.top != 0 {
		t.Fatalf("tras SetEntries cursor=%d top=%d, se esperaba 0/0", p.cursor, p.top)
	}
	// Sin marcas: Enter en el item final instala la extensión (fallback).
	p.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	_, ids, _ := p.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if len(ids) != 1 || ids[0] != "b" {
		t.Fatalf("Enter sin marcas devolvió ids=%v, se esperaba [b]", ids)
	}
}

// TestExtensionsPanelNavigatesWithScroll: Up/Down mueven el cursor con scroll
// en ambos sentidos, Home/End saltan a los extremos y PgUp/PgDn saltan página.
func TestExtensionsPanelNavigatesWithScroll(t *testing.T) {
	p := NewExtensionsPanel()
	entries := make([]ExtensionEntry, 0, 10)
	for i := range 10 {
		entries = append(entries, newExtensionEntry(fmt.Sprintf("id%d", i), fmt.Sprintf("Ext %d", i), "1.0.0", fmt.Sprintf("d%d", i), false))
	}
	p.SetEntries(entries)
	p.Resize(30, 5) // interior de 3 filas: 10 extensiones + item final = 11 filas

	// End: cursor al final (item final, índice 10) y scroll hasta el final.
	if handled, _, _ := p.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)); !handled {
		t.Fatal("End debe ser del panel")
	}
	if p.cursor != 10 {
		t.Fatalf("cursor tras End = %d, se esperaba 10 (item final)", p.cursor)
	}
	if p.top != 8 { // 11 filas, 3 visibles → top = 8
		t.Fatalf("top tras End = %d, se esperaba 8 (scroll al final)", p.top)
	}

	// Home: cursor 0, scroll al principio.
	p.HandleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if p.cursor != 0 || p.top != 0 {
		t.Fatalf("tras Home cursor=%d top=%d, se esperaba 0/0", p.cursor, p.top)
	}

	// PgDn: salta página (3 filas) → cursor 3.
	p.HandleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if p.cursor != 3 {
		t.Fatalf("cursor tras PgDn = %d, se esperaba 3 (salto de página)", p.cursor)
	}
	// PgDn de nuevo: cursor 6.
	p.HandleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if p.cursor != 6 {
		t.Fatalf("cursor tras 2° PgDn = %d, se esperaba 6", p.cursor)
	}
	// PgUp: cursor 3.
	p.HandleEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	if p.cursor != 3 {
		t.Fatalf("cursor tras PgUp = %d, se esperaba 3", p.cursor)
	}

	// Down hasta el final: el cursor se detiene en el item final y el scroll
	// lo deja visible.
	for range 20 {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	if p.cursor != 10 {
		t.Fatalf("cursor tras muchos Down = %d, se esperaba 10 (item final)", p.cursor)
	}
	if p.top != 8 {
		t.Fatalf("top tras muchos Down = %d, se esperaba 8", p.top)
	}

	// Up hasta el principio.
	for range 20 {
		p.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	}
	if p.cursor != 0 || p.top != 0 {
		t.Fatalf("tras muchos Up cursor=%d top=%d, se esperaba 0/0", p.cursor, p.top)
	}
}

// TestExtensionsPanelMarksAndInstalls: Espacio marca/desmarca la extensión del
// cursor (el item final no se marca), Enter sobre una extensión instala esa y
// Enter sobre el item final instala las marcadas (o la extensión sin marcas).
func TestExtensionsPanelMarksAndInstalls(t *testing.T) {
	p := NewExtensionsPanel()
	p.SetEntries([]ExtensionEntry{
		newExtensionEntry("a", "Alpha", "1.0.0", "alpha", false),
		newExtensionEntry("b", "Beta", "2.0.0", "beta", false),
	})
	p.Resize(30, 6)

	// Espacio marca la extensión del cursor (a).
	if handled, ids, _ := p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)); !handled || ids != nil {
		t.Fatalf("Espacio devolvió (handled=%v, ids=%v), se esperaba (true, nil)", handled, ids)
	}
	// Enter sobre la extensión del cursor: instala esa.
	_, ids, _ := p.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if len(ids) != 1 || ids[0] != "a" {
		t.Fatalf("Enter sobre Alpha devolvió ids=%v, se esperaba [a]", ids)
	}

	// Espacio sobre el item final no marca nada.
	p.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)) // cursor al item final
	if handled, ids, _ := p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)); !handled || ids != nil {
		t.Fatalf("Espacio en el item final devolvió (handled=%v, ids=%v), se esperaba (true, nil)", handled, ids)
	}

	// Enter sobre el item final con una marca: instala la marcada (a).
	_, ids, _ = p.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if len(ids) != 1 || ids[0] != "a" {
		t.Fatalf("Enter en el item final devolvió ids=%v, se esperaba [a] (la marcada)", ids)
	}

	// Sin marcas: Enter en el item final instala la extensión (fallback).
	p.SetEntries([]ExtensionEntry{
		newExtensionEntry("a", "Alpha", "1.0.0", "alpha", false),
		newExtensionEntry("b", "Beta", "2.0.0", "beta", false),
	})
	p.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	_, ids, _ = p.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if len(ids) != 1 || ids[0] != "b" {
		t.Fatalf("Enter sin marcas devolvió ids=%v, se esperaba [b] (la extensión)", ids)
	}
}

// TestExtensionsPanelClosesOnEscapeAndFallsThroughOnCtrlPage: Escape cierra el
// panel (close=true) y Ctrl+PageUp/PageDown caen al controlador (cambian de
// pestaña, no son del panel).
func TestExtensionsPanelClosesOnEscapeAndFallsThroughOnCtrlPage(t *testing.T) {
	p := NewExtensionsPanel()
	p.SetEntries([]ExtensionEntry{newExtensionEntry("a", "Alpha", "1.0.0", "alpha", false)})
	p.Resize(30, 6)

	if handled, ids, close := p.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); !handled || ids != nil || !close {
		t.Fatalf("Escape devolvió (handled=%v, ids=%v, close=%v), se esperaba (true, nil, true)", handled, ids, close)
	}
	for _, key := range []tcell.Key{tcell.KeyPgUp, tcell.KeyPgDn} {
		ev := tcell.NewEventKey(key, 0, tcell.ModCtrl)
		if handled, ids, close := p.HandleEvent(ev); handled || ids != nil || close {
			t.Fatalf("Ctrl+%v devolvió (handled=%v, ids=%v, close=%v), se esperaba (false, nil, false)", key, handled, ids, close)
		}
	}
	// Una tecla ajena también cae al controlador.
	if handled, _, _ := p.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)); handled {
		t.Fatal("una tecla ajena no debe ser del panel")
	}
}

// TestExtensionsPanelDrawsFrameTitleAndRows: el panel dibuja su marco con el
// título "Extensions", cada fila con la marca, el nombre y la versión alineada
// a la derecha (más "(installed)" si ya está), el item final alineado como las
// demás filas y la fila del cursor con la barra de selección a todo el ancho
// interior.
func TestExtensionsPanelDrawsFrameTitleAndRows(t *testing.T) {
	p := NewExtensionsPanel()
	p.SetEntries([]ExtensionEntry{
		newExtensionEntry("a", "Alpha", "1.0.0", "alpha", false),
		newExtensionEntry("b", "Beta", "2.3.4", "beta", true),
	})
	p.Resize(30, 6)

	s := newTestScreen(t, 30, 6)
	drawPanel(p, s)

	// Marco y título.
	if got := cellRuneAt(s, 0, 0); got != '┌' {
		t.Fatalf("(0,0) = %q, se esperaba '┌'", got)
	}
	if got := cellRuneAt(s, 29, 5); got != '┘' {
		t.Fatalf("(29,5) = %q, se esperaba '┘'", got)
	}
	lines := screenLines(s)
	if !strings.Contains(lines[0], "Extensions") {
		t.Fatalf("borde superior = %q, debe contener el título %q", lines[0], "Extensions")
	}

	// Fila 0 (cursor): "[ ] Alpha" + padding + "v1.0.0", con la barra de
	// selección (TreeCursor) a todo el ancho interior.
	for x := 1; x < 29; x++ {
		if bg := cellBg(s, x, 1); bg != tcell.PaletteColor(24) {
			t.Fatalf("fondo de la fila del cursor en x=%d = %v, se esperaba TreeCursor", x, bg)
		}
	}
	if line := lines[1]; !strings.Contains(line, "[ ] Alpha") || !strings.HasSuffix(line, "v1.0.0│") {
		t.Fatalf("fila 0 = %q, se esperaba \"[ ] Alpha\" + padding + \"v1.0.0\"", line)
	}

	// Fila 1: "[ ] Beta" + padding + "v2.3.4 (installed)".
	if line := lines[2]; !strings.Contains(line, "[ ] Beta") || !strings.HasSuffix(line, "v2.3.4 (installed)│") {
		t.Fatalf("fila 1 = %q, se esperaba \"[ ] Beta\" + padding + \"v2.3.4 (installed)\"", line)
	}

	// Fila 2 (item final): "Install (under cursor)" alineado como las demás.
	if line := lines[3]; !strings.Contains(line, "Install (under cursor)") {
		t.Fatalf("fila final = %q, se esperaba \"Install (under cursor)\"", line)
	}
}

// TestExtensionsPanelDrawsLoadingErrorAndEmpty: mientras consulta muestra
// "Consultando el catálogo…", con error "Sin conexión" y sin entradas "Sin
// extensiones disponibles".
func TestExtensionsPanelDrawsLoadingErrorAndEmpty(t *testing.T) {
	// Loading.
	p := NewExtensionsPanel()
	p.SetLoading()
	p.Resize(30, 5)
	s := newTestScreen(t, 30, 5)
	drawPanel(p, s)
	if line := screenLines(s)[1]; !strings.Contains(line, "Consultando el catálogo…") {
		t.Fatalf("fila de carga = %q, se esperaba \"Consultando el catálogo…\"", line)
	}

	// Error.
	p2 := NewExtensionsPanel()
	p2.SetError("HTTP 404")
	p2.Resize(30, 5)
	s2 := newTestScreen(t, 30, 5)
	drawPanel(p2, s2)
	if line := screenLines(s2)[1]; !strings.Contains(line, "Sin conexión") {
		t.Fatalf("fila de error = %q, se esperaba \"Sin conexión\"", line)
	}

	// Vacío.
	p3 := NewExtensionsPanel()
	p3.Resize(30, 5)
	s3 := newTestScreen(t, 30, 5)
	drawPanel(p3, s3)
	if line := screenLines(s3)[1]; !strings.Contains(line, "Sin extensiones disponibles") {
		t.Fatalf("fila vacía = %q, se esperaba \"Sin extensiones disponibles\"", line)
	}
}
