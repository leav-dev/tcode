package controller

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Aserciones de RENDER: los tests del controlador miraban el estado (¿se abrió
// la ventana?), no lo que la UI DIBUJA. Esa diferencia importa — una vista puede
// tener el estado correcto y dibujar mal, y el usuario ve el problema antes que
// el test. Acá se leen las celdas de la pantalla simulada, que es la verificación
// que no necesita TTY ni parsear secuencias de escape.

// screenRows devuelve las filas de la pantalla simulada como texto.
func screenRows(app *App) []string {
	sim := app.screen.(tcell.SimulationScreen)
	cells, w, h := sim.GetContents()
	rows := make([]string, h)
	for y := 0; y < h; y++ {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				sb.WriteRune(' ')
				continue
			}
			sb.WriteRune(c.Runes[0])
		}
		rows[y] = sb.String()
	}
	return rows
}

// screenHas dice si alguna fila dibujada contiene s.
func screenHas(app *App, s string) bool {
	for _, row := range screenRows(app) {
		if strings.Contains(row, s) {
			return true
		}
	}
	return false
}

// screenDump vuelve legible el fallo: sin esto, un assert de render solo dice
// "no está", y no hay forma de ver qué se dibujó en su lugar.
func screenDump(app *App) string {
	var sb strings.Builder
	for i, row := range screenRows(app) {
		if strings.TrimSpace(row) == "" {
			continue
		}
		sb.WriteString("\nfila ")
		sb.WriteString(strings.TrimSpace(strings.Repeat(" ", 2)))
		sb.WriteString(itoa(i))
		sb.WriteString(" |")
		sb.WriteString(strings.TrimRight(row, " "))
		sb.WriteString("|")
	}
	return sb.String()
}

// itoa evita importar strconv solo para esto.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// TestUIRendersTheExtensionsRowInTheConfigMenu: la fila Extensiones no solo
// existe en el modelo — se DIBUJA, con su valor "abrir". Es la aserción que
// hubiera respondido al instante "no me aparece la opción en el menú".
func TestUIRendersTheExtensionsRowInTheConfigMenu(t *testing.T) {
	resetConfigVars(t)
	app, _ := newTestApp(t, "hola")
	resizeApp(app, 80, 25)

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	if !app.configActive {
		t.Fatal("Ctrl+P debe abrir la configuración")
	}
	if !screenHas(app, "Configuración") {
		t.Fatalf("la ventana no dibuja su título:%s", screenDump(app))
	}
	if !screenHas(app, "Extensiones") {
		t.Fatalf("la ventana de configuración no dibuja la fila Extensiones:%s", screenDump(app))
	}
	if !screenHas(app, "abrir") {
		t.Fatalf("la fila Extensiones no dibuja su valor de acción:%s", screenDump(app))
	}
}

// TestUIRendersTheExtensionWindowTabs: abrir la fila Extensiones dibuja la
// ventana con sus cuatro pestañas.
func TestUIRendersTheExtensionWindowTabs(t *testing.T) {
	resetConfigVars(t)
	app, _ := newTestApp(t, "hola")
	resizeApp(app, 80, 25)

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	// End deja el cursor en la última fila, que es la de acción.
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.extActive {
		t.Fatalf("Enter en la fila Extensiones debe abrir la ventana:%s", screenDump(app))
	}
	for _, tab := range []string{"Instaladas", "Actualizables", "Disponibles", "Proveedores"} {
		if !screenHas(app, tab) {
			t.Fatalf("la ventana no dibuja la pestaña %q:%s", tab, screenDump(app))
		}
	}
}

// TestUIRendersTheCreatePrompt: Ctrl+N dibuja el pedido de nombre en la barra.
func TestUIRendersTheCreatePrompt(t *testing.T) {
	resetConfigVars(t)
	app, _ := newCreateApp(t)
	resizeApp(app, 80, 25)

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlN, 0, tcell.ModNone))
	if !app.promptActive {
		t.Fatal("Ctrl+N debe abrir el pedido de nombre")
	}
	if !screenHas(app, "Nuevo archivo") {
		t.Fatalf("el pedido de creación no se dibuja:%s", screenDump(app))
	}
}

// TestUIRendersTheDeletePrompt: Delete sobre el nodo del cursor dibuja el pedido
// sí/no con el nombre del archivo.
func TestUIRendersTheDeletePrompt(t *testing.T) {
	resetConfigVars(t)
	app, _ := newDeleteApp(t, map[string]string{"borrame.txt": "x"})
	resizeApp(app, 80, 25)

	app.handleEvent(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone))
	if !app.promptActive {
		t.Fatal("Delete debe abrir el pedido de confirmación")
	}
	if !screenHas(app, "Borrar") {
		t.Fatalf("el pedido de borrado no dibuja el verbo:%s", screenDump(app))
	}
	if !screenHas(app, "borrame.txt") {
		t.Fatalf("el pedido de borrado no dibuja el nombre del archivo:%s", screenDump(app))
	}
}
