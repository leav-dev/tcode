package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// TestRunRecoversPanicAndLogsTheCrash: un pánico en el despacho de un evento
// (acá, un comando de extensión que explota) no deja el proceso muerto con la
// terminal destruida; Run lo recupera, restaura la pantalla, vuelca el stack a
// crash.log y devuelve un error legible.
func TestRunRecoversPanicAndLogsTheCrash(t *testing.T) {
	dir := t.TempDir()
	oldPath, oldPaper := crashLogPath, setCrashPaper
	crashLogPath = func() string { return filepath.Join(dir, "crash.log") }
	setCrashPaper = func() {} // el crash paper abre el archivo de por vida; no en tests
	defer func() { crashLogPath, setCrashPaper = oldPath, oldPaper }()

	app, _ := newTestApp(t, "uno")
	if err := app.ext.Registry().Register("test.panic", func() error { panic("boom-ctrl-w") }); err != nil {
		t.Fatalf("Register falló: %v", err)
	}
	addExtension(t, app, `{
		"id": "demo.crash",
		"name": "Crash",
		"version": "1.0.0",
		"activation": ["onStartup"],
		"contributes": {
			"keybindings": [{"key": "ctrl+k", "command": "test.panic"}]
		}
	}`)

	if err := app.screen.PostEvent(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone)); err != nil {
		t.Fatalf("PostEvent falló: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- app.Run() }()

	var runErr error
	select {
	case runErr = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run no terminó tras el pánico (¿la pantalla simulada no entregó el evento?)")
	}
	if runErr == nil {
		t.Fatal("Run devolvió nil pese al pánico: el crash no se convirtió en error")
	}
	if !strings.Contains(runErr.Error(), "crash.log") || !strings.Contains(runErr.Error(), "boom-ctrl-w") {
		t.Errorf("error = %q, esperaba ruta de crash y valor del pánico", runErr)
	}

	data, err := os.ReadFile(filepath.Join(dir, "crash.log"))
	if err != nil {
		t.Fatalf("no se escribió crash.log: %v", err)
	}
	if !strings.Contains(string(data), "boom-ctrl-w") {
		t.Errorf("crash.log no contiene el valor del pánico:\n%s", data)
	}
	if !strings.Contains(string(data), "goroutine") {
		t.Errorf("crash.log no contiene el stack:\n%s", data)
	}
}

// TestRunNormalPathDoesNotWriteCrashLog: el camino feliz no toca el registro y
// puede recibir eventos normales (un Ctrl+W limpio) sin generar crimen.
func TestRunNormalPathDoesNotWriteCrashLog(t *testing.T) {
	dir := t.TempDir()
	oldPath, oldPaper := crashLogPath, setCrashPaper
	crashLogPath = func() string { return filepath.Join(dir, "crash.log") }
	setCrashPaper = func() {} // el crash paper abre el archivo de por vida; no en tests
	defer func() { crashLogPath, setCrashPaper = oldPath, oldPaper }()

	app, _ := newTestApp(t, "uno")
	// Escape con el buffer limpio cierra el editor: el camino feliz termina sin
	// tocar el registro de crashes.
	if err := app.screen.PostEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); err != nil {
		t.Fatalf("PostEvent falló: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run del camino feliz devolvió error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run no terminó en el camino feliz")
	}

	if _, err := os.Stat(filepath.Join(dir, "crash.log")); !os.IsNotExist(err) {
		t.Errorf("el camino feliz no debería escribir crash.log (err=%v)", err)
	}
}
