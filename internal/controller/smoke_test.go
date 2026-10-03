package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestFullSessionSmoke recorre una sesión realista completa sobre una pantalla
// simulada: escribir, saltos de línea, borrado, clic del mouse, rueda, páginas,
// deshacer, rehacer, resize y guardar. No busca verificar lógica fina, que ya está
// cubierta por los otros tests, sino descartar que algún camino de eventos se caiga.
func TestFullSessionSmoke(t *testing.T) {
	const content = "primera línea\nsegunda línea con 日本語\ntercera\n"
	path := filepath.Join(t.TempDir(), "smoke.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla: %v", err)
	}
	s.SetSize(40, 12)

	app, err := NewAppWithScreen(s, path)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	defer func() { app.model.Close(); s.Fini() }()

	app.redraw()

	// Navegar al final, escribir, saltos de línea y un carácter ancho.
	app.handleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))
	typeString(app, " agregado")
	press(app, tcell.KeyEnter)
	typeRune(app, '日')
	typeRune(app, 'x')

	// Borrar hacia atrás y hacia adelante.
	press(app, tcell.KeyBackspace)
	press(app, tcell.KeyDelete)
	press(app, tcell.KeyBackspace)

	// Mouse: clic en una celda del medio y rueda.
	app.handleEvent(tcell.NewEventMouse(5, 2, tcell.Button1, tcell.ModNone))
	app.handleEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, tcell.ModNone))
	app.handleEvent(tcell.NewEventMouse(0, 0, tcell.WheelUp, tcell.ModNone))

	// Movimiento por páginas y por líneas.
	press(app, tcell.KeyPgUp)
	press(app, tcell.KeyPgDn)
	press(app, tcell.KeyUp)
	press(app, tcell.KeyDown)
	press(app, tcell.KeyHome)
	press(app, tcell.KeyEnd)

	// Deshacer y rehacer varias veces.
	for i := 0; i < 5; i++ {
		press(app, tcell.KeyCtrlZ)
	}
	for i := 0; i < 3; i++ {
		press(app, tcell.KeyCtrlY)
	}

	// Redimensionar a algo más chico que el contenido.
	app.handleEvent(tcell.NewEventResize(20, 3))
	app.redraw()

	// Escribir de nuevo y guardar.
	typeRune(app, '!')
	press(app, tcell.KeyCtrlS)

	if app.model.Modified() {
		t.Fatal("tras Ctrl+S el documento no debe quedar modificado")
	}
	// La invariante que importa: lo que quedó en memoria es lo que está en disco.
	if got, want := readFile(t, path), app.model.GetContent(); got != want {
		t.Fatalf("disco y memoria divergen\ndisco:   %q\nmemoria: %q", got, want)
	}
}

// TestEmptyDocumentSmoke cubre el arranque sobre un archivo vacío, que es el caso
// donde varias operaciones no tienen nada sobre lo que operar.
func TestEmptyDocumentSmoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vacio.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla: %v", err)
	}
	s.SetSize(30, 6)

	app, err := NewAppWithScreen(s, path)
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	defer func() { app.model.Close(); s.Fini() }()

	app.redraw()

	// Nada de esto tiene sobre qué operar y no debe romper.
	press(app, tcell.KeyBackspace)
	press(app, tcell.KeyDelete)
	press(app, tcell.KeyUp)
	press(app, tcell.KeyDown)
	press(app, tcell.KeyLeft)
	press(app, tcell.KeyRight)
	press(app, tcell.KeyCtrlZ)
	press(app, tcell.KeyCtrlY)
	app.handleEvent(tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone))

	// Y se tiene que poder empezar a escribir.
	typeString(app, "hola")
	press(app, tcell.KeyCtrlS)

	if got := readFile(t, path); got != "hola" {
		t.Fatalf("archivo = %q, se esperaba %q", got, "hola")
	}
}
