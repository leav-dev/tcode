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
	// Text lleva el FONDO del documento: el tema pinta su propio fondo y la
	// terminal deja de mandar; StyleForRole lo propaga a los roles de sintaxis;
	// TabIdle comparte el par para que la fila de pestañas no desentone.
	Text         tcell.Style // texto del documento sin resaltar
	CursorLineBg tcell.Color // fondo de la línea del cursor
	TabActive    tcell.Style // pestaña activa
	TabIdle      tcell.Style // pestañas inactivas
	TreeCursor   tcell.Style // fila activa del árbol: la barra de selección
	Status       tcell.Style // barra de estado
	Message      tcell.Style // mensaje transitorio de la barra
	Modified     tcell.Style // marca [+] de documento sucio
	Comment      tcell.Style // comentarios (sintaxis)
	Keyword      tcell.Style // palabras clave (sintaxis)
	String       tcell.Style // cadenas (sintaxis)
	Number       tcell.Style // números (sintaxis)
	Type         tcell.Style // tipos y primitivas (sintaxis)
	Function     tcell.Style // nombres de función (sintaxis)
	Variable     tcell.Style // identificadores comunes (sintaxis)
	Punct        tcell.Style // puntuación/otros (sintaxis)

	// Gutter lleva SOLO el frente de la columna de números de línea; el uso (el
	// dibujo del editor) le pone el fondo del documento con docBg. Los Diag*
	// llevan el color de severidad de los marcadores del gutter (y del
	// subrayado en el hito siguiente), también solo frente.
	Gutter      tcell.Style // números de línea del gutter (gris suave)
	DiagError   tcell.Style // diagnóstico: error
	DiagWarning tcell.Style // diagnóstico: advertencia
	DiagInfo    tcell.Style // diagnóstico: info
}

// fg tiñe el frente de un estilo sobre el default: el rol de color de la
// mayoría de las marcas de texto y de los roles de sintaxis.
func fg(c tcell.Color) tcell.Style { return tcell.StyleDefault.Foreground(c) }

// on compone un frente y un fondo EXPLÍCITOS: la barra de selección y los
// acentos de la barra de estado llevan los dos, así la selección no depende de
// los colores "default" de la terminal, que es lo que hacía difícil ver el
// nodo activo.
func on(fg, bg tcell.Color) tcell.Style { return tcell.StyleDefault.Foreground(fg).Background(bg) }

// active es la pestaña activa: conserva el atributo Reverse del diseño
// original y le suma un color de acento.
func active(c tcell.Color) tcell.Style { return tcell.StyleDefault.Reverse(true).Foreground(c) }

// docBg es el fondo del documento del tema: el que StyleForRole propaga a la
// sintaxis.
func (t Theme) docBg() tcell.Color {
	_, bg, _ := t.Text.Decompose()
	return bg
}

// DefaultTheme es la paleta de arranque y el fallback del tema Custom
// (~/.tcode/theme.json): la misma paleta oscura tipo VSCode Dark+ de
// DarkTheme. LoadTheme y themeOr dependen de ella (base del JSON del usuario y
// valor del zero-check), por eso vive en la única fábrica del par: su fondo ya
// no es el de la terminal, es el de Dark+ (#1E1E1E).
func DefaultTheme() Theme {
	return DarkTheme()
}

// DarkTheme es la paleta oscura tipo VSCode Dark+ del selector: la misma
// paleta de DefaultTheme (el arranque y el fallback del Custom). Text lleva el
// par del documento Dark+ —frente #D4D4D4 sobre fondo #1E1E1E—, que TabIdle
// comparte para que la fila de pestañas no desentone.
func DarkTheme() Theme {
	return Theme{
		Text:         on(tcell.NewHexColor(0xD4D4D4), tcell.NewHexColor(0x1E1E1E)),
		CursorLineBg: tcell.PaletteColor(236),
		TabActive:    active(tcell.PaletteColor(45)),
		TabIdle:      on(tcell.NewHexColor(0xD4D4D4), tcell.NewHexColor(0x1E1E1E)),
		TreeCursor:   on(tcell.PaletteColor(15), tcell.PaletteColor(24)),
		Status:       on(tcell.PaletteColor(15), tcell.PaletteColor(24)),
		Message:      on(tcell.PaletteColor(220), tcell.PaletteColor(24)),
		Modified:     fg(tcell.PaletteColor(208)),
		Comment:      fg(tcell.PaletteColor(244)),
		Keyword:      fg(tcell.PaletteColor(213)),
		String:       fg(tcell.PaletteColor(173)),
		Number:       fg(tcell.PaletteColor(114)),
		Type:         fg(tcell.PaletteColor(79)),
		Function:     fg(tcell.PaletteColor(187)),
		Variable:     fg(tcell.PaletteColor(117)),
		Punct:        fg(tcell.PaletteColor(250)),
		Gutter:       fg(tcell.PaletteColor(246)),
		DiagError:    fg(tcell.PaletteColor(208)),
		DiagWarning:  fg(tcell.PaletteColor(220)),
		DiagInfo:     fg(tcell.PaletteColor(45)),
	}
}

// LightTheme es la paleta clara del selector (tipo VSCode Light): texto
// oscuro sobre terminal clara, con la barra de estado y la selección en azul
// oscuro como la default. Modified y String coinciden a propósito (rojo
// #A31515): es un empate aceptable de la paleta original, documentado acá.
// Text lleva el par del documento claro: #000000 sobre #FFFFFF.
func LightTheme() Theme {
	return Theme{
		Text:         on(tcell.NewHexColor(0x000000), tcell.NewHexColor(0xFFFFFF)),
		CursorLineBg: tcell.NewHexColor(0xE7F4FD),
		TabActive:    active(tcell.PaletteColor(21)),
		TabIdle:      on(tcell.NewHexColor(0x000000), tcell.NewHexColor(0xFFFFFF)),
		TreeCursor:   on(tcell.PaletteColor(15), tcell.PaletteColor(21)),
		Status:       on(tcell.PaletteColor(15), tcell.PaletteColor(24)),
		Message:      on(tcell.PaletteColor(220), tcell.PaletteColor(24)),
		Modified:     fg(tcell.NewHexColor(0xA31515)),
		Comment:      fg(tcell.NewHexColor(0x008000)),
		Keyword:      fg(tcell.NewHexColor(0x0000FF)),
		String:       fg(tcell.NewHexColor(0xA31515)),
		Number:       fg(tcell.NewHexColor(0x098658)),
		Type:         fg(tcell.NewHexColor(0x267F99)),
		Function:     fg(tcell.NewHexColor(0x795E26)),
		Variable:     fg(tcell.NewHexColor(0x001080)),
		Punct:        fg(tcell.NewHexColor(0x000000)),
		Gutter:       fg(tcell.NewHexColor(0x808080)),
		DiagError:    fg(tcell.NewHexColor(0xD6140F)),
		DiagWarning:  fg(tcell.NewHexColor(0xB25D00)),
		DiagInfo:     fg(tcell.NewHexColor(0x005FB8)),
	}
}

// LightHighContrastTheme (id "light-hc") es la paleta clara de alto contraste:
// línea del cursor amarilla, selección negra sobre amarillo y sintaxis con
// saturación máxima sobre fondo claro.
func LightHighContrastTheme() Theme {
	return Theme{
		Text:         on(tcell.NewHexColor(0x000000), tcell.NewHexColor(0xFFFFFF)),
		CursorLineBg: tcell.NewHexColor(0xFFFF00),
		TabActive:    active(tcell.PaletteColor(0)),
		TabIdle:      on(tcell.NewHexColor(0x000000), tcell.NewHexColor(0xFFFFFF)),
		TreeCursor:   on(tcell.PaletteColor(0), tcell.PaletteColor(226)),
		Status:       on(tcell.PaletteColor(255), tcell.PaletteColor(0)),
		Message:      on(tcell.PaletteColor(0), tcell.PaletteColor(226)),
		Modified:     fg(tcell.PaletteColor(1)),
		Comment:      fg(tcell.NewHexColor(0x808080)),
		Keyword:      fg(tcell.NewHexColor(0x0000CC)),
		String:       fg(tcell.NewHexColor(0xCC0000)),
		Number:       fg(tcell.NewHexColor(0x098658)),
		Type:         fg(tcell.NewHexColor(0x000080)),
		Function:     fg(tcell.NewHexColor(0x804000)),
		Variable:     fg(tcell.NewHexColor(0x000000)),
		Punct:        fg(tcell.NewHexColor(0x000000)),
		Gutter:       fg(tcell.NewHexColor(0x808080)),
		DiagError:    fg(tcell.PaletteColor(1)),
		DiagWarning:  fg(tcell.PaletteColor(11)),
		DiagInfo:     fg(tcell.PaletteColor(12)),
	}
}

// DarkHighContrastTheme (id "dark-hc") es la paleta oscura de alto contraste:
// misma línea del cursor amarilla que la clara, sintaxis de saturación máxima
// con acentos cian y blanco.
func DarkHighContrastTheme() Theme {
	return Theme{
		Text:         on(tcell.NewHexColor(0xFFFFFF), tcell.NewHexColor(0x000000)),
		CursorLineBg: tcell.NewHexColor(0xFFFF00),
		TabActive:    active(tcell.PaletteColor(226)),
		TabIdle:      on(tcell.NewHexColor(0xFFFFFF), tcell.NewHexColor(0x000000)),
		TreeCursor:   on(tcell.PaletteColor(0), tcell.PaletteColor(226)),
		Status:       on(tcell.PaletteColor(226), tcell.PaletteColor(0)),
		Message:      on(tcell.PaletteColor(226), tcell.PaletteColor(0)),
		Modified:     fg(tcell.PaletteColor(9)),
		Comment:      fg(tcell.NewHexColor(0xCCCCCC)),
		Keyword:      fg(tcell.NewHexColor(0xFFFF00)),
		String:       fg(tcell.NewHexColor(0x00FF00)),
		Number:       fg(tcell.NewHexColor(0x00FFFF)),
		Type:         fg(tcell.NewHexColor(0x00FFFF)),
		Function:     fg(tcell.NewHexColor(0xFFFFFF)),
		Variable:     fg(tcell.NewHexColor(0xFFFFFF)),
		Punct:        fg(tcell.NewHexColor(0xFFFFFF)),
		Gutter:       fg(tcell.PaletteColor(248)),
		DiagError:    fg(tcell.PaletteColor(9)),
		DiagWarning:  fg(tcell.PaletteColor(11)),
		DiagInfo:     fg(tcell.PaletteColor(12)),
	}
}

// TokyoNightTheme (id "tokyo-night") es la paleta inspirada en Tokyo Night:
// fondo azul profundo, acentos violeta y cian, sintaxis saturada.
func TokyoNightTheme() Theme {
	return Theme{
		Text:         on(tcell.NewHexColor(0xC0CAF5), tcell.NewHexColor(0x1A1B26)),
		CursorLineBg: tcell.NewHexColor(0x16161E),
		TabActive:    active(tcell.PaletteColor(69)),
		TabIdle:      on(tcell.NewHexColor(0xC0CAF5), tcell.NewHexColor(0x1A1B26)),
		TreeCursor:   on(tcell.PaletteColor(15), tcell.PaletteColor(24)),
		Status:       on(tcell.PaletteColor(15), tcell.PaletteColor(24)),
		Message:      on(tcell.PaletteColor(215), tcell.PaletteColor(24)),
		Modified:     fg(tcell.NewHexColor(0xF7768E)),
		Comment:      fg(tcell.NewHexColor(0x565F89)),
		Keyword:      fg(tcell.NewHexColor(0xBB9AF7)),
		String:       fg(tcell.NewHexColor(0x9ECE6A)),
		Number:       fg(tcell.NewHexColor(0xFF9E64)),
		Type:         fg(tcell.NewHexColor(0x2AC3DE)),
		Function:     fg(tcell.NewHexColor(0x7AA2F7)),
		Variable:     fg(tcell.NewHexColor(0xC0CAF5)),
		Punct:        fg(tcell.NewHexColor(0x89DDFF)),
		Gutter:       fg(tcell.NewHexColor(0x565F89)),
		DiagError:    fg(tcell.NewHexColor(0xF7768E)),
		DiagWarning:  fg(tcell.NewHexColor(0xE0AF68)),
		DiagInfo:     fg(tcell.NewHexColor(0x7AA2F7)),
	}
}

// DraculaTheme (id "dracula") es la paleta inspirada en Dracula: fondo gris
// pizarra para la selección y la barra, sintaxis pastel saturada. Variable y
// Punct comparten el blanco #F8F8F2, como en la paleta original.
func DraculaTheme() Theme {
	return Theme{
		Text:         on(tcell.NewHexColor(0xF8F8F2), tcell.NewHexColor(0x282A36)),
		CursorLineBg: tcell.NewHexColor(0x44475A),
		TabActive:    active(tcell.PaletteColor(212)),
		TabIdle:      on(tcell.NewHexColor(0xF8F8F2), tcell.NewHexColor(0x282A36)),
		TreeCursor:   on(tcell.PaletteColor(15), tcell.NewHexColor(0x44475A)),
		Status:       on(tcell.PaletteColor(15), tcell.NewHexColor(0x44475A)),
		Message:      on(tcell.PaletteColor(228), tcell.NewHexColor(0x44475A)),
		Modified:     fg(tcell.NewHexColor(0xFF5555)),
		Comment:      fg(tcell.NewHexColor(0x6272A4)),
		Keyword:      fg(tcell.NewHexColor(0xFF79C6)),
		String:       fg(tcell.NewHexColor(0xF1FA8C)),
		Number:       fg(tcell.NewHexColor(0xBD93F9)),
		Type:         fg(tcell.NewHexColor(0x8BE9FD)),
		Function:     fg(tcell.NewHexColor(0x50FA7B)),
		Variable:     fg(tcell.NewHexColor(0xF8F8F2)),
		Punct:        fg(tcell.NewHexColor(0xF8F8F2)),
		Gutter:       fg(tcell.NewHexColor(0x6272A4)),
		DiagError:    fg(tcell.NewHexColor(0xFF5555)),
		DiagWarning:  fg(tcell.NewHexColor(0xF1FA8C)),
		DiagInfo:     fg(tcell.NewHexColor(0x8BE9FD)),
	}
}

// namedTheme es una entrada del registry: el id y el nombre del selector más
// la fábrica que construye la paleta. El orden del registry ES el orden del
// selector; los getters construyen sus slices a partir de él.
type namedTheme struct {
	id    string
	name  string
	build func() Theme
}

// themeRegistry lista las paletas del selector de la ventana de configuración,
// en orden estable (el orden del selector). No incluye al Custom (que no es
// una paleta fija, sino el tema del usuario o el default).
var themeRegistry = []namedTheme{
	{"light", "Light", LightTheme},
	{"dark", "Dark", DarkTheme},
	{"light-hc", "Light HC", LightHighContrastTheme},
	{"dark-hc", "Dark HC", DarkHighContrastTheme},
	{"tokyo-night", "Tokyo Night", TokyoNightTheme},
	{"dracula", "Dracula", DraculaTheme},
}

// ThemeIDs devuelve los ids del registry en el orden del selector.
func ThemeIDs() []string {
	ids := make([]string, len(themeRegistry))
	for i, nt := range themeRegistry {
		ids[i] = nt.id
	}
	return ids
}

// ThemeNames devuelve los nombres del registry en el orden del selector.
func ThemeNames() []string {
	names := make([]string, len(themeRegistry))
	for i, nt := range themeRegistry {
		names[i] = nt.name
	}
	return names
}

// ThemeByID devuelve el tema del id dado y true, o el tema cero y false si el
// id no está en el registry (los callers distinguen el Custom por el id vacío).
func ThemeByID(id string) (Theme, bool) {
	for _, nt := range themeRegistry {
		if nt.id == id {
			return nt.build(), true
		}
	}
	return Theme{}, false
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
			} else if ok || v == "default" {
				// "default" deja el texto en StyleDefault: el tema Custom deja
				// de mandar sobre el par del documento (fondo, y por lo tanto los
				// roles, vuelven a depender de la terminal).
				*dst = tcell.StyleDefault
			}
		}
	}

	fg(&t.TabActive, "tabActive")
	fg(&t.TabIdle, "tabIdle")
	// treeCursor es un color de FONDO, como cursorLine: la fila activa del
	// árbol lleva la barra de selección visible siempre, y el texto queda en el
	// fg por defecto (se adapta a terminales claras y oscuras). "default" —o
	// un valor inválido— conserva la barra por defecto.
	if v := raw["treeCursor"]; v != "" {
		if c, ok := parseThemeColor(v); ok && c != tcell.ColorDefault {
			t.TreeCursor = tcell.StyleDefault.Background(c)
		}
	}
	fg(&t.Status, "status")
	fg(&t.Message, "message")
	fg(&t.Modified, "modified")
	fg(&t.Comment, "comment")
	fg(&t.Keyword, "keyword")
	fg(&t.String, "string")
	fg(&t.Number, "number")
	fg(&t.Type, "type")
	fg(&t.Function, "function")
	fg(&t.Variable, "variable")
	fg(&t.Punct, "punct")
	fg(&t.Gutter, "gutter")
	fg(&t.DiagError, "diagError")
	fg(&t.DiagWarning, "diagWarning")
	fg(&t.DiagInfo, "diagInfo")
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
// donde los roles del highlighter se vuelven colores. Cada caso propaga el
// fondo del documento (docBg) sobre el rol: ningún otro consumidor usa los
// roles directos, así los tokens heredan el fondo del tema —y con bg ==
// ColorDefault (un tema Custom con "text": "default") el comportamiento queda
// idéntico al actual, dependiendo de la terminal—.
func (t Theme) StyleForRole(r Role) tcell.Style {
	switch r {
	case RoleComment:
		return t.Comment.Background(t.docBg())
	case RoleKeyword:
		return t.Keyword.Background(t.docBg())
	case RoleString:
		return t.String.Background(t.docBg())
	case RoleNumber:
		return t.Number.Background(t.docBg())
	case RoleType:
		return t.Type.Background(t.docBg())
	case RoleFunction:
		return t.Function.Background(t.docBg())
	case RoleVariable:
		return t.Variable.Background(t.docBg())
	case RolePunct:
		return t.Punct.Background(t.docBg())
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
