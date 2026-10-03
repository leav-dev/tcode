package view

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/model"
)

// newGoTable abre un archivo .go en memoria (para el resaltado por extensión).
func newGoTable(t *testing.T, content string) *model.PieceTable {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pt := model.NewPieceTable()
	if err := pt.LoadFile(path); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	t.Cleanup(func() { pt.Close() })
	return pt
}

// fgOfCell y bgOfCell extraen los colores del estilo de una celda pintada.
func fgOfCell(st tcell.Style) tcell.Color { c, _, _ := st.Decompose(); return c }
func bgOfCell(st tcell.Style) tcell.Color { _, c, _ := st.Decompose(); return c }

// TestCursorLineGetsItsBackground: los clusters de la línea del cursor llevan
// el fondo del rol CursorLineBg.
func TestCursorLineGetsItsBackground(t *testing.T) {
	s := newTestScreen(t, 20, 3)
	v := newTestView(t, "una\n", 20, 3) // el cursor arranca en la línea 0
	draw(v, s)

	cells, width, _ := s.GetContents()
	got := bgOfCell(cells[0*width+0].Style) // celda (0,0) = 'u'
	if got != DefaultTheme().CursorLineBg {
		t.Fatalf("fondo de la línea del cursor = %v, esperaba %v", got, DefaultTheme().CursorLineBg)
	}
}

// TestCursorMovedLineKeepsBackground: al bajar el cursor, la nueva línea del
// cursor es la marcada y la anterior vuelve al fondo normal.
func TestCursorMovedLineKeepsBackground(t *testing.T) {
	s := newTestScreen(t, 20, 3)
	v := newTestView(t, "uno\ndos\n", 20, 3)

	if !v.moveCursorToCell(0, 1) {
		t.Fatal("no se pudo mover el cursor a la línea 1")
	}
	draw(v, s)

	cells, width, _ := s.GetContents()
	if bgOfCell(cells[1*width+0].Style) != DefaultTheme().CursorLineBg {
		t.Fatal("la línea 1 (cursor) debe llevar el fondo")
	}
	if bgOfCell(cells[0*width+0].Style) == DefaultTheme().CursorLineBg {
		t.Fatal("la línea 0 ya no es la del cursor")
	}
}

// TestKeywordStyledInRender: en un archivo .go la palabra clave se pinta con el
// rol del tema; el texto plano no.
func TestKeywordStyledInRender(t *testing.T) {
	s := newTestScreen(t, 30, 3)
	pt := newGoTable(t, "func main() {\n")
	v := NewEditorView(pt, 3, 30)
	draw(v, s)

	cells, width, _ := s.GetContents()
	if got := fgOfCell(cells[0*width+0].Style); got != fgOf(DefaultTheme().Keyword) {
		t.Fatalf("'func' = %v, esperaba el fg de Keyword (%v)", got, fgOf(DefaultTheme().Keyword))
	}
	// 'main' seguido de ( es el rol Function (regla nueva), no keyword.
	if got := fgOfCell(cells[0*width+5].Style); got != fgOf(DefaultTheme().Function) {
		t.Fatalf("'main' = %v, esperaba texto", got)
	}
}

// TestCommentStyledInRender: un comentario // se pinta con el rol Comment.
func TestCommentStyledInRender(t *testing.T) {
	s := newTestScreen(t, 30, 3)
	pt := newGoTable(t, "x := 1 // nota\n")
	v := NewEditorView(pt, 3, 30)
	draw(v, s)

	cells, width, _ := s.GetContents()
	if got := fgOfCell(cells[0*width+11].Style); got != fgOf(DefaultTheme().Comment) {
		t.Fatalf("comentario = %v, esperaba el fg de Comment", got)
	}
}

// TestThemeOverrideReachesRender: un tema custom re-mapea el render del editor.
func TestThemeOverrideReachesRender(t *testing.T) {
	s := newTestScreen(t, 30, 3)
	pt := newGoTable(t, "func main() {\n")
	v := NewEditorView(pt, 3, 30)
	th := DefaultTheme()
	th.Keyword = tcell.StyleDefault.Foreground(tcell.ColorRed)
	v.SetTheme(th)
	draw(v, s)

	cells, width, _ := s.GetContents()
	if got := fgOfCell(cells[0*width+0].Style); got != tcell.ColorRed {
		t.Fatalf("'func' con tema custom = %v, esperaba red", got)
	}
}

// TestStatusBarUsesTheme: la barra de estado dibuja con el rol Status del tema
// (y el mensaje con el rol Message).
func TestStatusBarUsesTheme(t *testing.T) {
	s := newTestScreen(t, 20, 1)
	sb := NewStatusBar()
	th := DefaultTheme()
	th.Status = tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorNavy)
	sb.SetTheme(th)
	sb.SetFile("doc.go", false)
	sb.Draw(s, 0, 20)
	s.Show()

	cells, _, _ := s.GetContents()
	fg, bg, _ := cells[0].Style.Decompose()
	if fg != tcell.ColorRed || bg != tcell.ColorNavy {
		t.Fatalf("celda 0 = fg %v bg %v, esperaba el tema custom (red on navy)", fg, bg)
	}
}

// TestStatusBarDefaultWithoutSetTheme: sin SetTheme la barra usa la default.
func TestStatusBarDefaultWithoutSetTheme(t *testing.T) {
	s := newTestScreen(t, 20, 1)
	sb := NewStatusBar()
	sb.SetFile("doc.go", false)
	sb.Draw(s, 0, 20)
	s.Show()

	cells, _, _ := s.GetContents()
	if cells[0].Style != DefaultTheme().Status {
		t.Fatalf("la barra sin tema debe usar el default")
	}
}

// TestTypeFunctionVariableStyledInRender: los roles nuevos llegan a la celda.
func TestTypeFunctionVariableStyledInRender(t *testing.T) {
	s := newTestScreen(t, 40, 4)
	pt := newGoTable(t, "var x int\nfoo(x)\n")
	v := NewEditorView(pt, 4, 40)
	draw(v, s)

	cells, width, _ := s.GetContents()
	// "var x int": 'var' keyword (0), 'x' variable (4), 'int' type (6).
	if fgOf(cells[0*width+6].Style) != fgOf(DefaultTheme().Type) {
		t.Fatal("'int' debe pintarse con el rol Type")
	}
	if fgOf(cells[0*width+4].Style) != fgOf(DefaultTheme().Variable) {
		t.Fatal("'x' debe pintarse con el rol Variable")
	}
	// "foo(x)": 'foo' función (0).
	if fgOf(cells[1*width+0].Style) != fgOf(DefaultTheme().Function) {
		t.Fatal("'foo' debe pintarse con el rol Function")
	}
}
