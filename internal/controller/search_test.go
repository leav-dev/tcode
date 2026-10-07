package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestFindOffsetsByLine(t *testing.T) {
	app, _ := newTestApp(t, "hola mundo\nhola de nuevo\nhola")
	buf := app.ws.Active()
	got := findOffsets(buf, "hola")
	if len(got) != 3 {
		t.Fatalf("matches = %v, esperaba 3", got)
	}
	if findOffsets(buf, "") != nil {
		t.Fatal("query vacía debe dar nil")
	}
	if len(findOffsets(buf, "chau")) != 0 {
		t.Fatal("sin ocurrencias debe dar vacío")
	}
}

func TestNextMatchIndexIsCircular(t *testing.T) {
	m := []int{5, 15, 25}
	if nextMatchIndex(m, 0, 0) != 0 {
		t.Fatal("después de 0 debe ir el índice 0 (offset 5)")
	}
	if nextMatchIndex(m, 5, 0) != 1 {
		t.Fatal("después de 5 debe ir el índice 1")
	}
	if nextMatchIndex(m, 99, 0) != 0 {
		t.Fatal("al final debe volver al inicio (circular)")
	}
	if nextMatchIndex(m, 99, 1) != 0 {
		t.Fatal("al final debe volver al primero aunque el índice previo no sea 0")
	}
	if nextMatchIndex(nil, 0, 0) != -1 {
		t.Fatal("sin matches debe dar -1")
	}
}

func TestCtrlFFindsAndCycles(t *testing.T) {
	app, _ := newTestApp(t, "uno dos uno dos uno")
	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlF, 0, tcell.ModNone))
	if !app.searchActive {
		t.Fatal("Ctrl+F debe abrir el pedido de búsqueda")
	}
	for _, r := range "uno" {
		app.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if app.searchBuf != "uno" {
		t.Fatalf("searchBuf = %q, esperaba uno", app.searchBuf)
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	ed := app.activeEditor()
	if ed.CursorOffset() != 8 {
		t.Fatalf("tras el primer Enter el cursor debe ir al siguiente 'uno' (8), quedó en %d", ed.CursorOffset())
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ed.CursorOffset() != 16 {
		t.Fatalf("el segundo Enter debe ciclar al tercer 'uno' (16), quedó en %d", ed.CursorOffset())
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if app.searchActive {
		t.Fatal("Escape debe cerrar la búsqueda")
	}
}

func TestCtrlFWithoutMatches(t *testing.T) {
	app, _ := newTestApp(t, "hola mundo")
	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlF, 0, tcell.ModNone))
	for _, r := range "chau" {
		app.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.searchActive {
		t.Fatal("sin coincidencias el pedido debe seguir abierto")
	}
	if len(app.searchMatches) != 0 {
		t.Fatalf("matches = %v, esperaba vacío", app.searchMatches)
	}
}

func TestRepoSearchOpensWindowAndJumps(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hola mundo\notra linea\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("nada acá\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	matches := searchWorkspace(dir, "mundo")
	if len(matches) != 1 {
		t.Fatalf("matches = %+v, esperaba 1", matches)
	}
	if matches[0].Line != 0 {
		t.Fatalf("línea = %d, esperaba 0", matches[0].Line)
	}

	app, err := newTestAppOnDir(t, dir)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	// Ctrl+Shift+F llega como KeyRune 'F' con Ctrl+Shift.
	app.handleEvent(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModCtrl|tcell.ModShift))
	if !app.repoPromptActive {
		t.Fatal("Ctrl+Shift+F debe abrir el pedido del repo")
	}
	for _, r := range "mundo" {
		app.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.repoResultsActive {
		t.Fatal("Enter debe abrir la ventana de resultados")
	}
	if app.repoResults.Len() != 1 {
		t.Fatalf("resultados = %d, esperaba 1", app.repoResults.Len())
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.repoResultsActive {
		t.Fatal("Enter en la ventana debe saltar y cerrar")
	}
	buf := app.ws.Active()
	if buf == nil {
		t.Fatal("tras el salto debe haber un buffer activo")
	}
	ed := app.activeEditor()
	if ed.CursorOffset() != 5 {
		t.Fatalf("cursor = %d, esperaba 5 (inicio de 'mundo')", ed.CursorOffset())
	}
}

func TestRepoSearchSkipsGitAndBinary(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "x.txt"), []byte("secreto\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("secreto\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	matches := searchWorkspace(dir, "secreto")
	if len(matches) != 1 {
		t.Fatalf("matches = %+v, esperaba solo ok.txt", matches)
	}
}
