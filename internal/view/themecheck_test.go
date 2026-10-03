package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestUserThemeFileLoads valida EXACTAMENTE el theme.json de ejemplo del
// usuario: todos los roles deben parsear (ninguno vuelve al default por error).
func TestUserThemeFileLoads(t *testing.T) {
	src := `{
  "text": "default",
  "cursorLine": "236",
  "tabActive": "45",
  "treeCursor": "237",
  "status": "white",
  "message": "220",
  "modified": "208",
  "comment": "244",
  "keyword": "#C586C0",
  "string": "#CE9178",
  "number": "#B5CEA8",
  "type": "#4EC9B0",
  "function": "#DCDCA3",
  "variable": "#9CDCFE",
  "punct": "250"
}`
	th := LoadTheme([]byte(src))
	want := map[string]tcell.Color{
		"comment":  tcell.PaletteColor(244),
		"keyword":  tcell.NewHexColor(0xc586c0),
		"string":   tcell.NewHexColor(0xce9178),
		"number":   tcell.NewHexColor(0xb5cea8),
		"type":     tcell.NewHexColor(0x4ec9b0),
		"function": tcell.NewHexColor(0xdcdca3),
		"variable": tcell.NewHexColor(0x9cdcfe),
	}
	got := map[string]tcell.Color{
		"comment":  fgOf(th.Comment),
		"keyword":  fgOf(th.Keyword),
		"string":   fgOf(th.String),
		"number":   fgOf(th.Number),
		"type":     fgOf(th.Type),
		"function": fgOf(th.Function),
		"variable": fgOf(th.Variable),
	}
	for name, c := range want {
		if got[name] != c {
			t.Errorf("%s = %v, esperaba %v", name, got[name], c)
		}
	}
	if th.Text != tcell.StyleDefault {
		t.Error("text debe seguir siendo default")
	}
}
