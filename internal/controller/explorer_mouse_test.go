package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestMouseClickOnArrowExpandsDirectory: un clic con Button1 sobre la flecha
// de expansión (▸/▾) de un directorio colapsado lo EXPANDE igual que Enter:
// el controlador atiende ActionExpand también en la ruta del mouse (no solo
// con el foco en el panel), lee los hijos y los muestra indentados.
func TestMouseClickOnArrowExpandsDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "carpeta")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("no se pudo crear el directorio: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "fuente.txt"), []byte("adentro"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	app := newExplorerApp(t, dir)
	app.redraw()
	if got := panelRow(app, 0); got != "▸ carpeta/" {
		t.Fatalf("fila 0 del panel = %q, se esperaba %q (dir colapsado)", got, "▸ carpeta/")
	}

	// Clic sobre la flecha: celda x=1 de la fila 0 del panel (la flecha ocupa
	// [depth*2, depth*2+2) = [0,2)); la y de pantalla suma la fila de pestañas.
	app.handleEvent(tcell.NewEventMouse(1, tabBarHeight, tcell.Button1, tcell.ModNone))
	app.redraw()

	if got := app.explorer.CursorPath(); got != sub {
		t.Fatalf("CursorPath() = %q, se esperaba %q (el cursor en el dir)", got, sub)
	}
	if got := panelRow(app, 0); got != "▾ carpeta/" {
		t.Fatalf("fila 0 del panel = %q, se esperaba %q (dir expandido)", got, "▾ carpeta/")
	}
	if got := panelRow(app, 1); got != "  · fuente.txt" {
		t.Fatalf("fila 1 del panel = %q, se esperaba %q (el hijo, con la indentación de su nivel)", got, "  · fuente.txt")
	}
}
