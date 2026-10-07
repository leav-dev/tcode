package controller

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func escKey() *tcell.EventKey { return tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone) }

// openConfigExtManager abre la configuración y entra a la ventana de
// extensiones con el teclado (Ctrl+P, Down×4 hasta Extensiones, Enter).
func openConfigExtManager(t *testing.T, app *App) {
	t.Helper()
	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	for i := 0; i < 4; i++ {
		app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.extActive {
		t.Fatal("Enter en Extensiones debe abrir la ventana de extensiones")
	}
}

// openConfigThemeMenu abre la configuración y entra a la ventana de temas
// (Ctrl+P, Down×3 hasta Theme, Enter).
func openConfigThemeMenu(t *testing.T, app *App) {
	t.Helper()
	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	for i := 0; i < 3; i++ {
		app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.themeMenuActive {
		t.Fatal("Enter en Theme debe abrir la ventana de temas")
	}
}

// TestEscInExtManagerReturnsToConfig: Esc en la ventana de extensiones vuelve
// a la configuración que la abrió (no cierra todo); otro Esc cierra la
// configuración; ningún Esc de ventana cierra el editor.
func TestEscInExtManagerReturnsToConfig(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	openConfigExtManager(t, app)

	if quit := app.handleEvent(escKey()); quit {
		t.Fatal("Esc en extensiones no debe cerrar el editor")
	}
	if app.extActive || !app.configActive {
		t.Fatal("Esc en extensiones debe volver a la configuración")
	}
	if !app.lastEscape.IsZero() {
		t.Fatal("el Esc de ventana debe invalidar la doble presión de salida")
	}
	if quit := app.handleEvent(escKey()); quit {
		t.Fatal("Esc en configuración no debe cerrar el editor")
	}
	if app.configActive {
		t.Fatal("el segundo Esc debe cerrar la configuración")
	}
}

// TestEscInThemeMenuReturnsToConfig: el espejo para la ventana de temas.
func TestEscInThemeMenuReturnsToConfig(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	openConfigThemeMenu(t, app)

	if quit := app.handleEvent(escKey()); quit {
		t.Fatal("Esc en temas no debe cerrar el editor")
	}
	if app.themeMenuActive || !app.configActive {
		t.Fatal("Esc en temas debe volver a la configuración")
	}
	if quit := app.handleEvent(escKey()); quit {
		t.Fatal("Esc en configuración no debe cerrar el editor")
	}
	if app.configActive {
		t.Fatal("el segundo Esc debe cerrar la configuración")
	}
}

// TestWindowEscBreaksQuitCountdown: un Escape del documento seguido de
// navegación por ventanas no suma para el quit. Con reloj fijo: Esc en el
// documento arma la cuenta, abrir y cerrar la configuración con Esc la
// invalida, y el siguiente Esc en el documento rearma (no cierra).
func TestWindowEscBreaksQuitCountdown(t *testing.T) {
	oldClock := clockNow
	oldWin := quitEscapeWindow
	quitEscapeWindow = 500 * time.Millisecond
	now := time.Unix(0, 0).Add(time.Hour)
	clockNow = func() time.Time { return now }
	defer func() { clockNow, quitEscapeWindow = oldClock, oldWin }()

	app, _ := newTestApp(t, "uno")

	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("el primer Escape arma la cuenta, no cierra")
	}
	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	if quit := app.handleEvent(escKey()); quit {
		t.Fatal("Esc en configuración no debe cerrar el editor")
	}
	// Sin la invalidación, este Escape caería dentro de la ventana del
	// primero y cerraría el editor: la navegación por ventanas debe
	// cortar la cuenta.
	if quit := press(app, tcell.KeyEscape); quit {
		t.Fatal("tras navegar ventanas con Esc, el siguiente Escape debe rearmar, no cerrar")
	}
}
