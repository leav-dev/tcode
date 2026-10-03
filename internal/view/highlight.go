package view

import (
	"bytes"
	"path/filepath"
	"strings"
)

// Language identifica el lenguaje del buffer por extensión para el resaltado
// básico. El highlighter es mecánico y por LÍNEA: sin estado entre líneas (un
// /* abierto o un string sin cerrar colorean solo su línea; límite anotado).
type Language int

const (
	LangNone Language = iota
	LangGo
	LangPython
	LangJS
	LangCLike
	LangJSON
)

// Role es la identidad de sintaxis de un byte (mapeada a un estilo por el
// Theme). El highlighter produce roles, nunca colores.
type Role int

const (
	RoleText Role = iota
	RoleComment
	RoleKeyword
	RoleString
	RoleNumber
	RolePunct
)

// langByExt mapea extensiones a lenguajes conocidos.
var langByExt = map[string]Language{
	".go": LangGo, ".py": LangPython,
	".js": LangJS, ".jsx": LangJS, ".ts": LangJS, ".tsx": LangJS,
	".mjs": LangJS, ".cjs": LangJS,
	".c": LangCLike, ".h": LangCLike, ".cc": LangCLike,
	".cpp": LangCLike, ".hpp": LangCLike, ".java": LangCLike, ".rs": LangCLike,
	".json": LangJSON,
}

// languageForPath detecta el lenguaje por la extensión; desconocido es None.
func languageForPath(path string) Language {
	if l, ok := langByExt[filepath.Ext(path)]; ok {
		return l
	}
	return LangNone
}

var keywords = map[Language]map[string]bool{
	LangGo:     wordSet("break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var"),
	LangPython: wordSet("and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield"),
	LangJS:     wordSet("as async await break case catch class const continue debugger default delete do else enum export extends false finally for from function get if import in instanceof let new null of return set static super switch this throw true try typeof undefined var void while with yield"),
	LangCLike:  wordSet("auto bool break case char class const continue default delete do double else enum extern false float for goto if inline int long namespace new nullptr operator private protected public register return short signed sizeof static struct switch template this throw true typedef typename union unsigned using virtual void volatile while"),
	LangJSON:   wordSet("true false null"),
}

func wordSet(s string) map[string]bool {
	m := make(map[string]bool, 16)
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// highlighter colorea una línea según el lenguaje del buffer.
type highlighter struct {
	lang Language
}

// newHighlighter crea el resaltador para el archivo dado.
func newHighlighter(path string) *highlighter {
	return &highlighter{lang: languageForPath(path)}
}

// styleAt devuelve el rol del byte en pos de la línea. Es puro: reescanea la
// línea en cada llamada (el editor pregunta por los clusters visibles, que son
// pocos y chicos).
func (h *highlighter) styleAt(line []byte, pos int) Role {
	if pos < 0 || pos >= len(line) {
		return RoleText
	}

	lang := h.lang
	lineComment := ""
	hasBlock := false
	switch lang {
	case LangGo, LangJS, LangCLike:
		lineComment, hasBlock = "//", true
	case LangPython:
		lineComment = "#"
	case LangJSON:
		// solo strings y keywords; sin comentarios en v1
	}
	ks := keywords[lang]

	i := 0
	for i < len(line) {
		if lineComment != "" && bytes.HasPrefix(line[i:], []byte(lineComment)) {
			return RoleComment // desde el marcador hasta el fin
		}
		if hasBlock && i+1 < len(line) && line[i] == '/' && line[i+1] == '*' {
			if end := bytes.Index(line[i+2:], []byte("*/")); end < 0 {
				return RoleComment // sin cierre en la línea
			} else {
				i += 2 + end + 2
				continue
			}
		}
		if line[i] == '"' || line[i] == '\'' {
			q := line[i]
			j := i + 1
			for j < len(line) {
				if line[j] == '\\' {
					j += 2
					continue
				}
				if line[j] == q {
					break
				}
				j++
			}
			if j >= len(line) {
				// string sin cierre: el resto de la línea es string
				if pos >= i {
					return RoleString
				}
				return RoleText
			}
			if pos >= i && pos <= j {
				return RoleString
			}
			i = j + 1
			continue
		}
		if isIdentStart(line[i]) {
			j := i
			for j < len(line) && isIdentChar(line[j]) {
				j++
			}
			if ks[string(line[i:j])] && pos >= i && pos < j {
				return RoleKeyword
			}
			i = j
			continue
		}
		if isDigit(line[i]) {
			j := i
			if line[j] == '0' && j+1 < len(line) && (line[j+1] == 'x' || line[j+1] == 'X') {
				j += 2
				for j < len(line) && isHex(line[j]) {
					j++
				}
			} else {
				for j < len(line) && (isDigit(line[j]) || line[j] == '.') {
					j++
				}
			}
			if pos >= i && pos < j {
				return RoleNumber
			}
			i = j
			continue
		}
		if line[i] != ' ' && line[i] != '\t' && pos == i {
			return RolePunct
		}
		i++
	}
	return RoleText
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
func isIdentChar(c byte) bool { return isIdentStart(c) || isDigit(c) }
func isDigit(c byte) bool     { return c >= '0' && c <= '9' }
func isHex(c byte) bool       { return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') }
