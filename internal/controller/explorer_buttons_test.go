package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/leav-dev/tcode/internal/view"
)

// footerClick manda un clic del mouse en la columna x del pie del explorador,
// en coordenadas de pantalla: el panel arranca bajo la fila de pestañas y su
// última fila es el pie, justo arriba de la barra de estado.
func footerClick(app *App, x int) {
	_, h := app.screen.Size()
	app.handleEvent(tcell.NewEventMouse(x, tabBarHeight+editorHeight(h)-1, tcell.Button1, tcell.ModNone))
}

// footerFolderX es la columna del chip de carpeta: el pie arranca en 1, cada
// chip lleva un espacio de padding a cada lado y los separa una celda.
func footerFolderX() int { return 1 + 1 + len(view.NewFileLabel) + 1 + 1 }

// TestExplorerFooterFileButtonOpensThePrompt: el botón del pie abre el MISMO
// pedido de Ctrl+N, con el destino contextual del cursor, y la creación real
// termina en disco.
func TestExplorerFooterFileButtonOpensThePrompt(t *testing.T) {
	app, root := newCreateApp(t)

	footerClick(app, 1)

	if !app.explorerFocused {
		t.Fatal("el clic en el pie debe enfocar el explorador")
	}
	if !app.promptActive || !strings.Contains(app.statusBar.Label(), "Nuevo archivo") {
		t.Fatalf("el clic debe abrir el pedido de archivo (etiqueta %q)", app.statusBar.Label())
	}
	typeString(app, "nuevo.txt")
	press(app, tcell.KeyEnter)

	if _, err := os.Stat(filepath.Join(root, "nuevo.txt")); err != nil {
		t.Fatalf("el archivo no se creó en la raíz del cursor: %v", err)
	}
}

// TestExplorerFooterFolderButtonCreatesInTheCursorDirectory: el botón de
// carpeta crea DENTRO de la carpeta del cursor, igual que Ctrl+Shift+N.
func TestExplorerFooterFolderButtonCreatesInTheCursorDirectory(t *testing.T) {
	app, root := newCreateApp(t)
	docs := filepath.Join(root, "docs")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatalf("no se pudo crear el dir de prueba: %v", err)
	}
	app.explorer.SetRootEntries([]view.Entry{{Name: "docs", Path: docs, IsDir: true}})

	footerClick(app, footerFolderX())

	if !app.promptActive || !strings.Contains(app.statusBar.Label(), "Nueva carpeta") {
		t.Fatalf("el clic debe abrir el pedido de carpeta (etiqueta %q)", app.statusBar.Label())
	}
	typeString(app, "borradores")
	press(app, tcell.KeyEnter)

	info, err := os.Stat(filepath.Join(docs, "borradores"))
	if err != nil {
		t.Fatalf("la carpeta no se creó dentro de docs: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("lo creado debe ser un directorio")
	}
}

// TestExplorerFooterClickOutsideTheButtonsDoesNothing: un clic en el pie pero
// fuera de los chips no abre ningún pedido.
func TestExplorerFooterClickOutsideTheButtonsDoesNothing(t *testing.T) {
	app, _ := newCreateApp(t)

	footerClick(app, 0)

	if app.promptActive {
		t.Fatal("un clic en el pie fuera de los botones no debe abrir ningún pedido")
	}
}
