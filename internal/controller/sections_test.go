package controller

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestSetSectionStoresAndPushes: setSection escribe la sección de la extensión
// en la barra del buffer activo, sin pisar a las demás.
func TestSetSectionStoresAndPushes(t *testing.T) {
	app, _ := newToastApp(t)

	if err := app.SetSection("tcode.gitchanges", "Git: 3 files"); err != nil {
		t.Fatalf("SetSection falló: %v", err)
	}
	if err := app.SetSection("tcode.linter", "2 issues"); err != nil {
		t.Fatalf("SetSection falló: %v", err)
	}

	got := app.statusBar.Sections()
	if len(got) != 2 {
		t.Fatalf("secciones = %v, esperaba 2", got)
	}
	if got["tcode.gitchanges"] != "Git: 3 files" {
		t.Fatalf("sección gitchanges = %q", got["tcode.gitchanges"])
	}
	if got["tcode.linter"] != "2 issues" {
		t.Fatalf("sección linter = %q", got["tcode.linter"])
	}
}

// TestSetSectionEmptyTextRemoves: el texto vacío remueve la sección de la
// extensión, como documenta la API.
func TestSetSectionEmptyTextRemoves(t *testing.T) {
	app, _ := newToastApp(t)

	app.SetSection("tcode.gitchanges", "Git: 3 files")
	if err := app.SetSection("tcode.gitchanges", ""); err != nil {
		t.Fatalf("SetSection falló: %v", err)
	}

	if got := app.statusBar.Sections(); len(got) != 0 {
		t.Fatalf("secciones = %v, esperaba vacío tras remover", got)
	}
}

// TestSetSectionRejectsEmptyID: el id vacío es un error legible, como el resto
// de la API tcode.*.
func TestSetSectionRejectsEmptyID(t *testing.T) {
	app, _ := newToastApp(t)

	if err := app.SetSection("", "texto"); err == nil {
		t.Fatal("SetSection con id vacío debe devolver error")
	}
}

// TestSetSectionRequiresBuffer: sin buffer activo no hay dónde escribir la
// sección.
func TestSetSectionRequiresBuffer(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla: %v", err)
	}
	s.SetSize(40, 10)

	app, err := NewAppWithScreen(s, "")
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	defer func() { app.ws.CloseAll(); s.Fini() }()

	if err := app.SetSection("tcode.gitchanges", "Git: 3 files"); err == nil {
		t.Fatal("SetSection sin buffer debe devolver error")
	}
}

// TestSyncStatusPushesActiveBufferSections: al cambiar de pestaña, la barra
// muestra las secciones del buffer activo —sin datos stale del anterior.
func TestSyncStatusPushesActiveBufferSections(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno\n", "dos\n")

	app.SetSection("tcode.gitchanges", "Git: 3 files")
	if got := app.statusBar.Sections()["tcode.gitchanges"]; got != "Git: 3 files" {
		t.Fatalf("sección = %q, se esperaba la del buffer activo", got)
	}

	// Cambiar de pestaña: el otro buffer no tiene secciones todavía.
	app.switchTab(app.ws.Next)
	if got := app.statusBar.Sections(); len(got) != 0 {
		t.Fatalf("secciones = %q, se esperaba vacío en el buffer nuevo", got)
	}

	// Escribir en el nuevo buffer: su propia sección, sin pisar la del otro.
	app.SetSection("tcode.gitchanges", "Git: 1 file")
	if got := app.statusBar.Sections()["tcode.gitchanges"]; got != "Git: 1 file" {
		t.Fatalf("sección = %q, se esperaba la del buffer nuevo", got)
	}

	// Volver al primero: su sección sigue ahí.
	app.switchTab(app.ws.Prev)
	if got := app.statusBar.Sections()["tcode.gitchanges"]; got != "Git: 3 files" {
		t.Fatalf("sección = %q, se esperaba la del buffer original", got)
	}
}

// TestCloseTabDropsSections: al cerrar un buffer, sus secciones se borran y
// sobreviven las del buffer que queda.
func TestCloseTabDropsSections(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno\n", "dos\n")

	// Secciones en ambos buffers.
	app.SetSection("tcode.gitchanges", "Git: 3 files")
	app.switchTab(app.ws.Next)
	app.SetSection("tcode.gitchanges", "Git: 1 file")

	// Cerrar el buffer activo: solo se borra la entrada del cerrado.
	app.closeTab()

	if len(app.sections) != 1 {
		t.Fatalf("secciones por buffer = %d, esperaba 1 (el buffer vivo)", len(app.sections))
	}
	if got := app.statusBar.Sections()["tcode.gitchanges"]; got != "Git: 3 files" {
		t.Fatalf("sección = %q, se esperaba la del buffer que sobrevivió", got)
	}
}
