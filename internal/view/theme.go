package view

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Theme agrupa los estilos por rol visual del editor. Cada componente consume
// el rol que le toca; ningún módulo hardcodea un color (salvo los defaults de
// acá). Es la costura donde un día entra un tema de sintaxis real.
type Theme struct {
	Text         tcell.Style // texto del documento sin resaltar
	CursorLineBg tcell.Color // fondo de la línea del cursor
	TabActive    tcell.Style // pestaña activa
	TabIdle      tcell.Style // pestañas inactivas
	TreeCursor   tcell.Style // nodo activo del árbol
	Status       tcell.Style // barra de estado
	Message      tcell.Style // mensaje transitorio de la barra
	Modified     tcell.Style // marca [+] de documento sucio
	Comment      tcell.Style // comentarios (sintaxis)
	Keyword      tcell.Style // palabras clave (sintaxis)
	String       tcell.Style // cadenas (sintaxis)
	Number       tcell.Style // números (sintaxis)
	Punct        tcell.Style // puntuación/otros (sintaxis)
}

// DefaultTheme es la paleta por defecto: estilo oscuro, tipo VSCode Dark+.
// Los roles "activos" (TabActive, TreeCursor) conservan el atributo Reverse
// del diseño original y le suman un color de acento: así el "de qué está
// seleccionado" nunca depende solo del color de la terminal.
func DefaultTheme() Theme {
	fg := func(c tcell.Color) tcell.Style { return tcell.StyleDefault.Foreground(c) }
	on := func(fg, bg tcell.Color) tcell.Style { return tcell.StyleDefault.Foreground(fg).Background(bg) }
	active := func(c tcell.Color) tcell.Style { return tcell.StyleDefault.Reverse(true).Foreground(c) }
	return Theme{
		Text:         tcell.StyleDefault,
		CursorLineBg: tcell.PaletteColor(236),
		TabActive:    active(tcell.PaletteColor(45)),
		TabIdle:      tcell.StyleDefault,
		TreeCursor:   active(tcell.ColorDefault),
		Status:       on(tcell.PaletteColor(15), tcell.PaletteColor(24)),
		Message:      on(tcell.PaletteColor(220), tcell.PaletteColor(24)),
		Modified:     fg(tcell.PaletteColor(208)),
		Comment:      fg(tcell.PaletteColor(244)),
		Keyword:      fg(tcell.PaletteColor(213)),
		String:       fg(tcell.PaletteColor(173)),
		Number:       fg(tcell.PaletteColor(114)),
		Punct:        fg(tcell.PaletteColor(250)),
	}
}

// LoadTheme construye un tema desde el JSON de configuración del usuario
// ({"keyword": "#569cd6", ...}). Los roles ausentes o con valores no
// parseables conservan el default; un JSON roto devuelve el default completo.
// El tema del usuario jamás rompe el editor.
func LoadTheme(data []byte) Theme {
	t := DefaultTheme()
	raw := map[string]string{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return t
	}

	// Cada rol se aplica solo si el valor es un color válido; la mayoría son
	// colores de frente, CursorLineBg es de fondo.
	fg := func(dst *tcell.Style, key string) {
		if v := raw[key]; v != "" {
			if c, ok := parseThemeColor(v); ok {
				*dst = tcell.StyleDefault.Foreground(c)
			}
		}
	}
	txt := func(dst *tcell.Style, key string) {
		if v := raw[key]; v != "" {
			if c, ok := parseThemeColor(v); ok && c != tcell.ColorDefault {
				*dst = tcell.StyleDefault.Foreground(c)
			} else if ok {
				*dst = tcell.StyleDefault
			}
		}
	}

	fg(&t.TabActive, "tabActive")
	fg(&t.TabIdle, "tabIdle")
	fg(&t.TreeCursor, "treeCursor")
	fg(&t.Status, "status")
	fg(&t.Message, "message")
	fg(&t.Modified, "modified")
	fg(&t.Comment, "comment")
	fg(&t.Keyword, "keyword")
	fg(&t.String, "string")
	fg(&t.Number, "number")
	fg(&t.Punct, "punct")
	txt(&t.Text, "text")
	if v := raw["cursorLine"]; v != "" {
		if c, ok := parseThemeColor(v); ok {
			t.CursorLineBg = c
		}
	}
	return t
}

// themeOr devuelve el tema activo: el inyectado por SetTheme o el default.
// Los componentes dibujan con esto; el valor cero del campo significa default.
func themeOr(th Theme) Theme {
	if th == (Theme{}) {
		return DefaultTheme()
	}
	return th
}

// StyleForRole mapea un rol de sintaxis al estilo del tema: es el único punto
// donde los roles del highlighter se vuelven colores.
func (t Theme) StyleForRole(r Role) tcell.Style {
	switch r {
	case RoleComment:
		return t.Comment
	case RoleKeyword:
		return t.Keyword
	case RoleString:
		return t.String
	case RoleNumber:
		return t.Number
	case RolePunct:
		return t.Punct
	default:
		return t.Text
	}
}

// parseThemeColor resuelve un color del JSON: nombre tcell ("red", "navy"…),
// hex ("#569cd6") o índice ANSI ("0"–"255").
func parseThemeColor(s string) (tcell.Color, bool) {
	if strings.HasPrefix(s, "#") {
		if len(s) == 7 {
			if n, err := strconv.ParseInt(s[1:], 16, 32); err == nil {
				return tcell.NewHexColor(int32(n)), true
			}
		}
		return 0, false
	}
	if c := tcell.GetColor(strings.ToLower(s)); c.Valid() {
		return c, true
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 255 {
		return tcell.PaletteColor(n), true
	}
	return 0, false
}
