package view

import (
	"testing"
)

// TestLanguageForPath detecta el lenguaje por extensión; lo desconocido es None.
func TestLanguageForPath(t *testing.T) {
	cases := []struct {
		path string
		want Language
	}{
		{"main.go", LangGo},
		{"mod.py", LangPython},
		{"ui.ts", LangJS},
		{"c.js", LangJS},
		{"x.c", LangCLike},
		{"y.h", LangCLike},
		{"datos.json", LangJSON},
		{"readme.txt", LangNone},
		{"Makefile", LangNone},
		{"", LangNone},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := languageForPath(tc.path); got != tc.want {
				t.Errorf("languageForPath(%q) = %v, esperaba %v", tc.path, got, tc.want)
			}
		})
	}
}

// roleAtIdx devuelve el rol del byte pos de la línea con el highlighter dado.
func roleAtIdx(t *testing.T, h *highlighter, line string, pos int) Role {
	t.Helper()
	return h.styleAt([]byte(line), pos)
}

// TestHighlightGoKeywords: las palabras clave del lenguaje se marcan, con
// límite de token completo (funcXYZ no es func).
func TestHighlightGoKeywords(t *testing.T) {
	h := newHighlighter("main.go")
	if got := roleAtIdx(t, h, "func main()", 0); got != RoleKeyword {
		t.Fatalf("'func' = %v, esperaba keyword", got)
	}
	// El prefijo de palabra clave no aplica a un identificador más largo.
	h2 := newHighlighter("m.go")
	if got := roleAtIdx(t, h2, "funcXYZ", 0); got != RoleText {
		t.Fatalf("'funcXYZ' = %v, esperaba texto (límite de token)", got)
	}
}

// TestHighlightGoComment: // colorea el resto de la línea.
func TestHighlightGoComment(t *testing.T) {
	h := newHighlighter("main.go")
	if got := roleAtIdx(t, h, "x := 1 // nota", 7); got != RoleComment {
		t.Fatalf("posición del comentario = %v, esperaba Comment", got)
	}
	if got := roleAtIdx(t, h, "// todo", 6); got != RoleComment {
		t.Fatalf("cola del comentario = %v, esperaba Comment", got)
	}
}

// TestHighlightPythonComment: en Python el comentario es #.
func TestHighlightPythonComment(t *testing.T) {
	h := newHighlighter("x.py")
	if got := roleAtIdx(t, h, "x = 1 # nota", 6); got != RoleComment {
		t.Fatalf("'#' de Python = %v, esperaba Comment", got)
	}
	if got := roleAtIdx(t, h, "# solo", 0); got == RoleText {
		t.Fatal("línea de comentario debe empezar en Comment")
	}
}

// TestHighlightString: las comillas encierran una string; sin cierre, el resto
// de la línea es string (límite documentado).
func TestHighlightString(t *testing.T) {
	h := newHighlighter("main.go")
	if got := roleAtIdx(t, h, `x := "hola"`, 6); got != RoleString {
		t.Fatalf("comilla abierta = %v, esperaba String", got)
	}
	if got := roleAtIdx(t, h, `x := "hola"`, 7); got != RoleString {
		t.Fatalf("contenido = %v, esperaba String", got)
	}
	if got := roleAtIdx(t, h, `x := "hola"`, 9); got != RoleString {
		t.Fatalf("cierre de comilla = %v, esperaba String", got)
	}
	h2 := newHighlighter("s.py")
	if got := roleAtIdx(t, h2, `s = 'ab`, 5); got != RoleString {
		t.Fatalf("string sin cerrar = %v, esperaba String hasta el fin", got)
	}
}

// TestHighlightNumber: enteros, decimales y hex se marcan como Number.
func TestHighlightNumber(t *testing.T) {
	h := newHighlighter("m.go")
	cases := []struct {
		line string
		pos  int
	}{
		{"n := 123", 6},
		{"pi := 3.14", 8},
		{"b := 0xff", 7},
	}
	for _, c := range cases {
		t.Run(c.line, func(t *testing.T) {
			if got := roleAtIdx(t, h, c.line, c.pos); got != RoleNumber {
				t.Errorf("posición %d de %q = %v, esperaba Number", c.pos, c.line, got)
			}
		})
	}
	if got := roleAtIdx(t, h, "abc123", 0); got != RoleText {
		t.Fatalf("'abc123' arranca en texto, no en Number: %v", got)
	}
}

// TestHighlightJSON: strings y true/false/null; sin comentarios.
func TestHighlightJSON(t *testing.T) {
	h := newHighlighter("d.json")
	if got := roleAtIdx(t, h, `{"ok": true}`, 2); got != RoleString {
		t.Fatalf("clave JSON = %v, esperaba String", got)
	}
	if got := roleAtIdx(t, h, `{"ok": true}`, 8); got != RoleKeyword {
		t.Fatalf("'true' = %v, esperaba Keyword", got)
	}
}

// TestHighlightUnknownLanguageIsPlainText: sin lenguaje no hay roles.
func TestHighlightUnknownLanguageIsPlainText(t *testing.T) {
	h := newHighlighter("readme.txt")
	if got := roleAtIdx(t, h, "func X // nota", 5); got != RoleText {
		t.Fatalf("en txt todo debe ser texto: %v", got)
	}
}

// TestHighlightPunctRole: la puntuación sola queda en Punct, no en Text.
func TestHighlightPunctRole(t *testing.T) {
	h := newHighlighter("m.go")
	if got := roleAtIdx(t, h, "a+b;", 1); got != RolePunct {
		t.Fatalf("'+' = %v, esperaba Punct", got)
	}
}
