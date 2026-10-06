package controller

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/leav-dev/tcode/internal/view"
)

// TestCtrlRReloadsCleanBufferDirectly: Ctrl+R sobre un buffer limpio recarga
// sin confirmación (recargar no pierde nada).
func TestCtrlRReloadsCleanBufferDirectly(t *testing.T) {
	app, _ := newTestApp(t, "uno")
	press(app, tcell.KeyCtrlR)

	if msg := app.statusBar.Message(); !strings.Contains(msg, "Recargado") {
		t.Errorf("mensaje = %q, esperaba el aviso de recarga", msg)
	}
	if app.ws.Active().Modified() {
		t.Fatal("un buffer limpio recargado no puede quedar modificado")
	}
}

// TestCtrlRAsksTwiceOnDirtyBuffer: con ediciones sin guardar, la primera Ctrl+R
// solo avisa y la segunda confirma; la edición desaparece (el disco gana).
func TestCtrlRAsksTwiceOnDirtyBuffer(t *testing.T) {
	app, _ := newTestApp(t, "uno\n")
	typeRune(app, 'X') // sucio

	press(app, tcell.KeyCtrlR)
	if msg := app.statusBar.Message(); strings.Contains(msg, "Recargado") {
		t.Fatal("la primera Ctrl+R no debe recargar sobre un buffer sucio")
	}
	if !app.ws.Active().Modified() {
		t.Fatal("el buffer debe seguir sucio tras el primer aviso")
	}

	press(app, tcell.KeyCtrlR)
	if app.ws.Active().Modified() {
		t.Fatal("la segunda Ctrl+R debió recargar (descarta las ediciones)")
	}
	if got := string(app.ws.Active().LineContent(0)); got != "uno" {
		t.Fatalf("contenido = %q, esperaba el del disco (la X debe desaparecer)", got)
	}
	if msg := app.statusBar.Message(); !strings.Contains(msg, "Recargado") {
		t.Errorf("mensaje = %q, esperaba el aviso de recarga", msg)
	}
}

// TestCtrlRArmedIsCancelledByAnotherKey: otra tecla desarma la confirmación;
// la siguiente Ctrl+R vuelve a preguntar en lugar de recargar.
func TestCtrlRArmedIsCancelledByAnotherKey(t *testing.T) {
	app, _ := newTestApp(t, "uno\n")
	typeRune(app, 'X')

	press(app, tcell.KeyCtrlR) // arma
	typeRune(app, 'y')         // desarma (y escribe)
	press(app, tcell.KeyCtrlR) // vuelve a armar, NO recarga

	if !app.ws.Active().Modified() {
		t.Fatal("la tecla canceladora no debió permitir la recarga: el buffer sigue sucio")
	}
	if msg := app.statusBar.Message(); strings.Contains(msg, "Recargado") {
		t.Errorf("mensaje = %q: la confirmación desarmada no puede recargar", msg)
	}
}

// TestExternalChangeAutoReloadsCleanBuffer: un archivo que cambió por fuera
// (acá: el mtime; Windows bloquea escribir contenido sobre un archivo mapeado,
// pero el cambio de fecha es metadata y sí pasa) con buffer limpio recarga solo
// ante la actividad del usuario: la marca de cambio externo desaparece.
func TestExternalChangeAutoReloadsCleanBuffer(t *testing.T) {
	app, path := newTestApp(t, "uno")
	buf := app.ws.Active()

	future := time.Now().Add(3 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("Chtimes falló: %v", err)
	}
	if !buf.ChangedOnDisk() {
		t.Fatal("el test requiere la marca de cambio externo")
	}

	press(app, tcell.KeyDown) // actividad que no escribe: el check debe recargar y refrescar la marca

	if buf.ChangedOnDisk() {
		t.Fatal("tras la actividad, un buffer limpio se recargó y la marca se refrescó")
	}
	if buf.Modified() {
		t.Fatal("la recarga automática no puede dejar el buffer modificado")
	}
}

// TestDirtyBufferDoesNotAutoReload: con ediciones sin guardar, la detección
// automática no recarga: la decisión es del Ctrl+R (confirmado).
func TestDirtyBufferDoesNotAutoReload(t *testing.T) {
	app, path := newTestApp(t, "uno\n")
	buf := app.ws.Active()
	typeRune(app, 'X') // sucio

	future := time.Now().Add(3 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("Chtimes falló: %v", err)
	}

	typeRune(app, 'y') // actividad: el check no debe tocar un buffer sucio

	if !buf.Modified() {
		t.Fatal("la actividad no debe recargar un buffer sucio")
	}
	// La X se inserta en el cursor (inicio del documento) y la y después de ella.
	if got := string(buf.LineContent(0)); got != "Xyuno" {
		t.Fatalf("contenido = %q, esperaba las ediciones intactas", got)
	}
}

// TestWrapToggleKey: Ctrl+Shift+W alterna el salto de palabra con su mensaje,
// sin caer al documento.
func TestWrapToggleKey(t *testing.T) {
	app, _ := newTestApp(t, "uno")

	want := !view.WordWrapEnabled()
	hmm := tcell.NewEventKey(tcell.KeyRune, 'w', tcell.ModCtrl|tcell.ModShift)
	app.handleEvent(hmm)

	if view.WordWrapEnabled() != want {
		t.Fatalf("WordWrapEnabled() = %v, esperaba %v tras la tecla", view.WordWrapEnabled(), want)
	}
	msg := app.statusBar.Message()
	if (want && !strings.Contains(msg, "activado")) || (!want && !strings.Contains(msg, "desactivado")) {
		t.Errorf("mensaje = %q, esperaba el aviso del estado nuevo", msg)
	}
	if got := app.ws.Active().GetContent(); got != "uno" {
		t.Fatalf("la tecla del toggle no debe escribir en el documento: %q", got)
	}
}
