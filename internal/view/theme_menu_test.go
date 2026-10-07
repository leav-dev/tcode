package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestExtensionThemesRegisterAndResolve(t *testing.T) {
	RegisterExtensionThemes(nil)
	defer RegisterExtensionThemes(nil)

	RegisterExtensionThemes([]ExtensionTheme{
		{ID: "demo.rosa", Label: "Rosa", From: "demo.temas", Data: []byte(`{"keyword":"#ff79c6"}`)},
	})
	th, ok := ThemeByID("demo.rosa")
	if !ok {
		t.Fatal("ThemeByID(demo.rosa) debe existir tras registrar")
	}
	fg, _, _ := th.Keyword.Decompose()
	if fg != tcell.NewHexColor(0xFF79C6) {
		t.Fatalf("keyword fg = %v, esperaba #ff79c6", fg)
	}
	// SetActiveThemeID acepta el id de extensión.
	SetActiveThemeID("demo.rosa")
	if ActiveThemeID() != "demo.rosa" {
		t.Fatalf("ActiveThemeID() = %q, esperaba demo.rosa", ActiveThemeID())
	}
	SetActiveThemeID("")
}

func TestExtensionThemesDoNotShadowBuiltins(t *testing.T) {
	RegisterExtensionThemes(nil)
	defer RegisterExtensionThemes(nil)

	RegisterExtensionThemes([]ExtensionTheme{
		{ID: "dark", Label: "Falso dark", From: "demo.temas", Data: []byte(`{}`)},
		{ID: "demo.a", Label: "A", From: "demo.temas", Data: []byte(`{}`)},
		{ID: "demo.a", Label: "A2", From: "demo.temas", Data: []byte(`{}`)},
	})
	if _, ok := ThemeByID("demo.a"); !ok {
		t.Fatal("demo.a debe existir")
	}
	th, _ := ThemeByID("dark")
	fg, bg, _ := th.Text.Decompose()
	wantFG, wantBG, _ := DarkTheme().Text.Decompose()
	if fg != wantFG || bg != wantBG {
		t.Fatal("una extensión no puede pisar el dark built-in")
	}
}

func TestAvailableThemesListsAll(t *testing.T) {
	RegisterExtensionThemes(nil)
	defer RegisterExtensionThemes(nil)
	RegisterExtensionThemes([]ExtensionTheme{
		{ID: "demo.rosa", Label: "Rosa", From: "demo.temas", Data: []byte(`{}`)},
	})
	opts := AvailableThemes()
	if len(opts) != len(ThemeIDs())+2 {
		t.Fatalf("opciones = %d, esperaba built-ins + ext + Custom", len(opts))
	}
	if opts[len(opts)-1].ID != "" || opts[len(opts)-1].Name != "Custom" {
		t.Fatalf("última opción = %+v, esperaba Custom", opts[len(opts)-1])
	}
	found := false
	for _, o := range opts {
		if o.ID == "demo.rosa" && o.Source == "demo.temas" {
			found = true
		}
	}
	if !found {
		t.Fatalf("falta demo.rosa en %+v", opts)
	}
}

func TestThemeMenuSelects(t *testing.T) {
	RegisterExtensionThemes(nil)
	defer RegisterExtensionThemes(nil)
	SetActiveThemeID("")
	defer SetActiveThemeID("")

	m := NewThemeMenu()
	m.Resize(40, 12)
	SetActiveThemeID("light")
	m.Open()
	if len(m.Items()) == 0 {
		t.Fatal("la ventana debe listar temas")
	}
	// Bajar una fila y elegir con Enter.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if m.Cursor() != 1 {
		t.Fatalf("cursor = %d, esperaba 1", m.Cursor())
	}
	handled, selected := m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !handled || !selected {
		t.Fatal("Enter debe devolver (true, true)")
	}
	opt, ok := m.Selected()
	if !ok || opt.ID == "" && len(m.Items()) > 1 {
		// El cursor 1 es un built-in: el id no debe ser vacío.
		t.Fatalf("selección = %+v ok=%v", opt, ok)
	}
	// Tecla ajena cierra.
	if handled, _ := m.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)); handled {
		t.Fatal("una tecla ajena no debe manejarse")
	}
}
