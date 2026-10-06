package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/view"
)

// newToastApp arma una app sobre un archivo nuevo en un temp dir, con una
// pantalla simulada chica.
func newToastApp(t *testing.T) (*App, tcell.SimulationScreen) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nota.txt")
	if err := os.WriteFile(path, []byte("uno\n"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla: %v", err)
	}
	s.SetSize(40, 10)
	app, err := NewAppWithScreen(s, path)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	t.Cleanup(func() { app.ws.CloseAll(); s.Fini() })
	app.redraw()
	return app, s
}

// TestSaveShowsSuccessToast: Ctrl+S confirma con el toast superior derecho y
// deja la barra de estado sin mensaje —la confirmación se movió, no se duplicó.
func TestSaveShowsSuccessToast(t *testing.T) {
	app, _ := newToastApp(t)
	typeString(app, "dos")
	press(app, tcell.KeyCtrlS)

	if !app.toast.Visible() {
		t.Fatal("tras guardar el toast debe estar visible")
	}
	if got := app.toast.Message(); got != "Guardado" {
		t.Fatalf("toast = %q, se esperaba %q", got, "Guardado")
	}
	if app.toast.Kind() != view.ToastSuccess {
		t.Fatalf("kind = %d, se esperaba ToastSuccess", app.toast.Kind())
	}
	if msg := app.statusBar.Message(); strings.Contains(msg, "Guardado") {
		t.Fatalf("barra = %q, la confirmación debe vivir solo en el toast", msg)
	}
}

// TestSaveErrorShowsErrorToast: un guardado que falla avisa en el toast con
// el kind error, no en la barra.
func TestSaveErrorShowsErrorToast(t *testing.T) {
	app, _ := newToastApp(t)
	typeString(app, "dos")

	// Save As a un directorio inexistente: la escritura falla y el error va al
	// toast.
	app.startPrompt()
	app.promptBuf = filepath.Join(t.TempDir(), "no-existe", "archivo.txt")
	app.handlePromptKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if !app.toast.Visible() {
		t.Fatal("tras un guardado fallido el toast debe estar visible")
	}
	if app.toast.Kind() != view.ToastError {
		t.Fatalf("kind = %d, se esperaba ToastError", app.toast.Kind())
	}
	if got := app.toast.Message(); !strings.HasPrefix(got, "Error al guardar como: ") {
		t.Fatalf("toast = %q, se esperaba el aviso de error", got)
	}
	if msg := app.statusBar.Message(); strings.Contains(msg, "Error al guardar") {
		t.Fatalf("barra = %q, el error debe vivir solo en el toast", msg)
	}
}

// TestSaveAsShowsToast: Save As confirma en el toast igual que Ctrl+S.
func TestSaveAsShowsToast(t *testing.T) {
	app, _ := newToastApp(t)
	typeString(app, "dos")

	dest := filepath.Join(t.TempDir(), "copia.txt")
	app.startPrompt()
	app.promptBuf = dest
	app.handlePromptKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if !app.toast.Visible() {
		t.Fatal("tras Save As el toast debe estar visible")
	}
	if got := app.toast.Message(); got != "Guardado en copia.txt" {
		t.Fatalf("toast = %q, se esperaba %q", got, "Guardado en copia.txt")
	}
	if app.toast.Kind() != view.ToastSuccess {
		t.Fatalf("kind = %d, se esperaba ToastSuccess", app.toast.Kind())
	}
}

// TestToastExpiresOnItsOwn: el timer del toast lo borra solo a los segundos.
// El timer dispara en su goroutine y vuelve por PostEvent: el bucle lo atiende
// y limpia.
func TestToastExpiresOnItsOwn(t *testing.T) {
	app, s := newToastApp(t)

	old := view.ToastDuration
	view.ToastDuration = 100 * time.Millisecond
	defer func() { view.ToastDuration = old }()

	app.showToast("Guardado", view.ToastSuccess)
	if !app.toast.Visible() {
		t.Fatal("recién mostrado el toast debe estar visible")
	}

	// Esperar el evento del timer con el bucle despierto: PollEvent no bloquea
	// y el timer llega en su goroutine. La cola trae también el snapshot de
	// extensiones (posteado al arrancar): se procesa y se sigue esperando. La
	// expiración por reloj no borra el mensaje —solo el evento del timer lo
	// limpia—, así que el loop termina cuando el toast queda sin mensaje.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ev := s.PollEvent(); ev != nil {
			app.handleEvent(ev)
			if app.toast.Message() == "" {
				break
			}
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}

	if app.toast.Message() != "" {
		t.Fatal("el timer del toast no limpió la notificación")
	}
}

// TestAppNotifyKinds: Notify (ScriptAPI) muestra el toast con el kind
// pedido —ausente y "success" son éxito, "error" es error— y rechaza un kind
// desconocido con un error que la extensión ve.
func TestAppNotifyKinds(t *testing.T) {
	app, _ := newToastApp(t)

	if err := app.Notify("lista", ""); err != nil {
		t.Fatalf("Notify vacío falló: %v", err)
	}
	if !app.toast.Visible() || app.toast.Kind() != view.ToastSuccess {
		t.Fatalf("Notify sin kind: visible=%v kind=%d, se esperaba toast de éxito", app.toast.Visible(), app.toast.Kind())
	}

	app.Notify("bien", "success")
	if app.toast.Kind() != view.ToastSuccess {
		t.Fatalf("Notify success: kind=%d, se esperaba ToastSuccess", app.toast.Kind())
	}

	app.Notify("falló", "error")
	if !app.toast.Visible() || app.toast.Kind() != view.ToastError {
		t.Fatalf("Notify error: visible=%v kind=%d, se esperaba toast de error", app.toast.Visible(), app.toast.Kind())
	}

	if err := app.Notify("x", "raro"); err == nil {
		t.Fatal("Notify con kind desconocido debe devolver error")
	}
}

// TestStaleToastTimerIgnoresNewToast: el timer de un toast viejo no puede
// borrar al toast nuevo —el seq del evento no coincide con el vigente.
func TestStaleToastTimerIgnoresNewToast(t *testing.T) {
	app, _ := newToastApp(t)

	app.showToast("primero", view.ToastSuccess)
	app.showToast("segundo", view.ToastSuccess)

	// Llega el timer del primero: el toast vigente es el segundo.
	app.handleEvent(tcell.NewEventInterrupt(toastEvent{Seq: 1}))
	if !app.toast.Visible() {
		t.Fatal("el timer del toast viejo borró al nuevo")
	}
	if got := app.toast.Message(); got != "segundo" {
		t.Fatalf("toast = %q, se esperaba %q", got, "segundo")
	}

	// Llega el timer del segundo: ahora sí expira.
	app.handleEvent(tcell.NewEventInterrupt(toastEvent{Seq: 2}))
	if app.toast.Visible() {
		t.Fatal("el timer del toast vigente no lo borró")
	}
}
