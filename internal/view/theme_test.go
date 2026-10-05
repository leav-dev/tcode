package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// fgOf extrae el color de frente de un estilo (Decompose es el getter de tcell).
func fgOf(st tcell.Style) tcell.Color {
	fg, _, _ := st.Decompose()
	return fg
}

// bgOf extrae el color de fondo de un estilo.
func bgOf(st tcell.Style) tcell.Color {
	_, bg, _ := st.Decompose()
	return bg
}

// TestThemeDefaultsAreSet: el tema por defecto define todos los roles; los que
// deben diferenciarse se distinguen, y el fondo de la línea del cursor es
// válido.
func TestThemeDefaultsAreSet(t *testing.T) {
	th := DefaultTheme()

	if !th.CursorLineBg.Valid() {
		t.Fatal("CursorLineBg debe ser un color válido")
	}
	if th.Status == th.Text {
		t.Fatal("la barra de estado debe diferenciarse del texto")
	}
	if th.TabActive == th.TabIdle {
		t.Fatal("la pestaña activa debe diferenciarse de la inactiva")
	}
	if th.Comment == th.Text || th.Keyword == th.Text || th.String == th.Text {
		t.Fatal("los roles de sintaxis deben diferenciarse del texto")
	}
}

// TestLoadThemeParsesRoles: el JSON re-mapea los roles nombrados.
func TestLoadThemeParsesRoles(t *testing.T) {
	src := `{
		"text": "default",
		"cursorLine": "236",
		"keyword": "#569cd6",
		"string": "173",
		"status": "white",
		"message": "red",
		"comment": "244",
		"number": "114",
		"tabActive": "36",
		"modified": "208",
		"punct": "250",
		"treeCursor": "237"
	}`
	th := LoadTheme([]byte(src))

	if want := tcell.PaletteColor(244); fgOf(th.Comment) != want {
		t.Errorf("comment fg = %v, esperaba el índice 244", fgOf(th.Comment))
	}
	if fgOf(th.Keyword) != tcell.NewHexColor(0x569cd6) {
		t.Errorf("keyword fg = %v, esperaba #569cd6", fgOf(th.Keyword))
	}
	if fgOf(th.String) != tcell.PaletteColor(173) {
		t.Errorf("string fg = %v, esperaba índice 173", fgOf(th.String))
	}
	if th.CursorLineBg != tcell.PaletteColor(236) {
		t.Errorf("cursorLine = %v, esperaba 236", th.CursorLineBg)
	}
	if fgOf(th.TabActive) != tcell.PaletteColor(36) {
		t.Errorf("tabActive fg = %v, esperaba índice 36", fgOf(th.TabActive))
	}
	// treeCursor es ahora un color de FONDO, como cursorLine: la fila activa
	// lleva la barra explícita y el texto en el fg por defecto (que se adapta a
	// terminales claras y oscuras).
	if bgOf(th.TreeCursor) != tcell.PaletteColor(237) {
		t.Errorf("treeCursor bg = %v, esperaba índice 237", bgOf(th.TreeCursor))
	}
	if fgOf(th.TreeCursor) != tcell.ColorDefault {
		t.Errorf("treeCursor fg = %v, esperaba default (el texto de la fila activa)", fgOf(th.TreeCursor))
	}
	if th.Text != tcell.StyleDefault {
		t.Errorf("text = %v, esperaba default", th.Text)
	}
}

// TestLoadThemeBrokenJSONFallsBack: un JSON roto devuelve el default y nada
// más: el tema del usuario jamás rompe el editor.
func TestLoadThemeBrokenJSONFallsBack(t *testing.T) {
	th := LoadTheme([]byte("{ json roto"))
	if th.Text != DefaultTheme().Text {
		t.Error("el JSON roto debe conservar el default")
	}
	if th.Keyword != DefaultTheme().Keyword {
		t.Error("el JSON roto debe conservar el default de keyword")
	}
}

// TestLoadThemeUnknownValueKeepsDefault: un rol con valor no parseable
// conserva su default (y no rompe el resto).
func TestLoadThemeUnknownValueKeepsDefault(t *testing.T) {
	src := `{"keyword": "noexiste", "string": "173", "treeCursor": "noexiste"}`
	th := LoadTheme([]byte(src))

	if th.Keyword != DefaultTheme().Keyword {
		t.Error("el valor inválido de keyword debe conservar el default")
	}
	if fgOf(th.String) != tcell.PaletteColor(173) {
		t.Error("el rol válido guardado junto al inválido debe aplicarse")
	}
	if th.TreeCursor != DefaultTheme().TreeCursor {
		t.Error("el valor inválido de treeCursor debe conservar la barra por defecto")
	}
}

// TestLoadThemeTreeCursorDefaultKeepsTheBar: "default" en treeCursor conserva
// la barra de selección por defecto (no la borra ni la vuelve invisible).
func TestLoadThemeTreeCursorDefaultKeepsTheBar(t *testing.T) {
	th := LoadTheme([]byte(`{"treeCursor": "default"}`))
	if th.TreeCursor != DefaultTheme().TreeCursor {
		t.Error("treeCursor: \"default\" debe conservar la barra por defecto")
	}
}

// TestParseThemeColorAceptsFormats: nombre, hex e índice ANSI; rechaza el resto.
func TestParseThemeColorAceptsFormats(t *testing.T) {
	cases := []struct {
		in   string
		want tcell.Color
		ok   bool
	}{
		{"red", tcell.ColorRed, true},
		{"#569cd6", tcell.NewHexColor(0x569cd6), true},
		{"173", tcell.PaletteColor(173), true},
		{"0", tcell.PaletteColor(0), true},
		{"256", 0, false},
		{"-1", 0, false},
		{"", 0, false},
		{"neon!!", 0, false},
		{"#xyz", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := parseThemeColor(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Errorf("parseThemeColor(%q) = (%v, %v), esperaba (%v, %v)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestThemeSyntaxRolesAreDistinct: los roles de sintaxis nuevos se distinguen
// del texto y entre sí en el default.
func TestThemeSyntaxRolesAreDistinct(t *testing.T) {
	th := DefaultTheme()
	pairs := [][2]tcell.Style{
		{th.Type, th.Text},
		{th.Function, th.Text},
		{th.Variable, th.Text},
		{th.Type, th.Keyword},
		{th.Function, th.Keyword},
	}
	for _, p := range pairs {
		if p[0] == p[1] {
			t.Fatalf("el par de roles no se distingue: %v vs %v", p[0], p[1])
		}
	}
}

// TestLoadThemeParsesNewSyntaxRoles: el JSON re-mapea type/function/variable.
func TestLoadThemeParsesNewSyntaxRoles(t *testing.T) {
	src := `{"type": "117", "function": "179", "variable": "117"}`
	th := LoadTheme([]byte(src))
	if fgOf(th.Type) != tcell.PaletteColor(117) {
		t.Errorf("type = %v, esperaba índice 117", fgOf(th.Type))
	}
	if fgOf(th.Function) != tcell.PaletteColor(179) {
		t.Errorf("function = %v, esperaba índice 179", fgOf(th.Function))
	}
	if fgOf(th.Variable) != tcell.PaletteColor(117) {
		t.Errorf("variable = %v, esperaba índice 117", fgOf(th.Variable))
	}
}

// TestThemeRegistryIsStableAndValid: el registry expone los seis temas en un
// orden fijo (el del selector), cada id existe con un tema válido (CursorLineBg
// real y barra de estado diferenciada del texto) y un id ajeno no existe.
func TestThemeRegistryIsStableAndValid(t *testing.T) {
	wantIDs := [...]string{"light", "dark", "light-hc", "dark-hc", "tokyo-night", "dracula"}
	ids := ThemeIDs()
	if len(ids) != len(wantIDs) {
		t.Fatalf("ThemeIDs() = %v, se esperaban %d ids", ids, len(wantIDs))
	}
	for i, want := range wantIDs {
		if ids[i] != want {
			t.Fatalf("ThemeIDs()[%d] = %q, se esperaba %q (orden estable del selector)", i, ids[i], want)
		}
	}

	wantNames := [...]string{"Light", "Dark", "Light HC", "Dark HC", "Tokyo Night", "Dracula"}
	names := ThemeNames()
	if len(names) != len(wantNames) {
		t.Fatalf("ThemeNames() = %v, se esperaban %d nombres", names, len(wantNames))
	}
	for i, want := range wantNames {
		if names[i] != want {
			t.Fatalf("ThemeNames()[%d] = %q, se esperaba %q", i, names[i], want)
		}
	}

	for _, id := range ids {
		th, ok := ThemeByID(id)
		if !ok {
			t.Fatalf("ThemeByID(%q) debe existir en el registry", id)
		}
		if !th.CursorLineBg.Valid() {
			t.Fatalf("ThemeByID(%q): CursorLineBg debe ser un color válido", id)
		}
		if th.Status == th.Text {
			t.Fatalf("ThemeByID(%q): la barra de estado debe diferenciarse del texto", id)
		}
	}
	if _, ok := ThemeByID("noexiste"); ok {
		t.Fatal("ThemeByID con un id ajeno al registry debe devolver false")
	}
}

// TestThemesAreDistinct: las seis paletas se distinguen entre sí —el selector
// tiene que ofrecer opciones que se noten— y el fondo de la línea del cursor
// fija las expectativas reales de cada paleta, con claras (Light, Light HC) y
// oscuras (Dark, Tokyo Night, Dracula, Dark HC).
func TestThemesAreDistinct(t *testing.T) {
	dark := DarkTheme()
	light := LightTheme()
	lasthc := LightHighContrastTheme()
	darkhc := DarkHighContrastTheme()
	tokyo := TokyoNightTheme()
	dracula := DraculaTheme()

	// Keyword: el rol con más peso visual diferencia las paletas por pares clave.
	pairs := [][2]tcell.Color{
		{fgOf(dark.Keyword), fgOf(light.Keyword)},
		{fgOf(light.Keyword), fgOf(tokyo.Keyword)},
		{fgOf(tokyo.Keyword), fgOf(dracula.Keyword)},
		{fgOf(dark.Keyword), fgOf(lasthc.Keyword)},
		{fgOf(lasthc.Keyword), fgOf(darkhc.Keyword)},
	}
	for _, p := range pairs {
		if p[0] == p[1] {
			t.Fatalf("el Keyword de dos paletas no se distingue: %v vs %v", p[0], p[1])
		}
	}

	// CursorLineBg: el valor real de cada paleta, claro u oscuro por diseño.
	cases := []struct {
		name string
		got  tcell.Color
		want tcell.Color
	}{
		{"light", light.CursorLineBg, tcell.NewHexColor(0xE7F4FD)},
		{"light-hc", lasthc.CursorLineBg, tcell.NewHexColor(0xFFFF00)},
		{"dark", dark.CursorLineBg, tcell.PaletteColor(236)},
		{"dark-hc", darkhc.CursorLineBg, tcell.NewHexColor(0xFFFF00)},
		{"tokyo-night", tokyo.CursorLineBg, tcell.NewHexColor(0x16161E)},
		{"dracula", dracula.CursorLineBg, tcell.NewHexColor(0x44475A)},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("CursorLineBg de %s = %v, se esperaba %v", tc.name, tc.got, tc.want)
		}
	}
}
