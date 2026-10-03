package view

import (
	"github.com/rivo/uniseg"
)

// wordWrapEnabled es la configuración del salto de palabra: mutable (la
// futura configuración del editor la expondrá) y alternada en vivo por
// Ctrl+Shift+W. Cuando está apagado, el editor se comporta como antes: la
// línea excede el ancho y se corta contra el borde con scroll horizontal.
var wordWrapEnabled = true

// WordWrapEnabled reporta la configuración actual del salto de palabra.
func WordWrapEnabled() bool { return wordWrapEnabled }

// ToggleWordWrap alterna la configuración y devuelve el estado nuevo. La usa
// el controlador en la tecla del toggle (Ctrl+Shift+W).
func ToggleWordWrap() bool {
	wordWrapEnabled = !wordWrapEnabled
	return wordWrapEnabled
}

// softLine es una fila visual: un rebanado de la línea lógica que cabe en
// width celdas, con el byte de inicio dentro de la línea (para traducir
// coordenadas). El texto envuelto nunca se copia: son slices de la línea.
// El espacio del corte queda AL FINAL de la fila anterior, invisible al
// pintar (su celda está fuera del ancho), y así la concatenación de las filas
// es siempre la línea original.
type softLine struct {
	text string
	in   int
}

// softLines envuelve line en filas visuales de hasta width celdas, cortando por
// palabra: la fila se rompe justo después del último espacio que entra (la
// palabra que no cupo va ENTERA a la fila siguiente) y una palabra más larga
// que width se parte en el clúster exacto (VSCode-like). Mide por grapheme
// cluster con el ancho real (CJK = 2). Un espacio/tab nunca desborda: se pega
// al borde, marca el corte y se conserva en la fila anterior.
func softLines(line string, width int) []softLine {
	if width < 1 {
		width = 1
	}
	var out []softLine
	rest := line
	base := 0 // desplazamiento de rest dentro de line

	for {
		rowBegin := 0   // inicio de la fila visual actual (relativo a rest)
		col := 0        // columnas ocupadas en la fila
		breakAfter := 0 // byte (rel. a rest) justo después del último espacio
		g := uniseg.NewGraphemes(rest)
		cuts := false

		for g.Next() {
			cl := g.Str()
			from, to := g.Positions()
			w := wrapClusterWidth(cl, col)
			if w == 0 {
				continue // combinante huérfano: sin celda propia
			}
			if cl == " " || cl == "\t" {
				// Un espacio nunca desborda: se pega al borde, marca el corte y
				// queda al final de la fila anterior (invisible al pintar).
				col += w
				breakAfter = to
				continue
			}
			if col+w > width {
				at := from
				if breakAfter > rowBegin && breakAfter <= len(rest) {
					at = breakAfter // la palabra que seguía al espacio va abajo entera
				}
				if at > rowBegin {
					out = append(out, softLine{text: rest[rowBegin:at], in: base + rowBegin})
				}
				rest = rest[at:]
				base += at
				cuts = true
				break
			}
			col += w
		}

		if !cuts {
			if rowBegin < len(rest) || len(out) == 0 {
				out = append(out, softLine{text: rest[rowBegin:], in: base + rowBegin})
			}
			break
		}
	}
	return out
}

// softLineCount devuelve cuántas filas visuales ocupa la línea al ancho dado.
func softLineCount(line string, width int) int {
	return len(softLines(line, width))
}

// softLineAt ubica el byte byteCol de la línea en (fila visual, columna
// visible). Un byte que cae dentro de un clúster ancho cuenta la columna desde
// su inicio.
func softLineAt(line string, width, byteCol int) (row, col int) {
	if byteCol > len(line) {
		byteCol = len(line)
	}
	if byteCol < 0 {
		byteCol = 0
	}
	ls := softLines(line, width)
	for r, sl := range ls {
		// Límite estricto salvo en la ÚLTIMA fila: el byte justo en la frontera
		// de una fila intermedia pertenece a la siguiente (el texto de la fila
		// es [in, in+len)), pero el FINAL de la línea (cursor al final del
		// documento) debe caer en la última fila con su columna completa.
		if byteCol < sl.in+len(sl.text) || (r == len(ls)-1 && byteCol <= sl.in+len(sl.text)) {
			min := byteCol - sl.in
			if min > len(sl.text) {
				min = len(sl.text)
			}
			return r, uniseg.StringWidth(sl.text[:min])
		}
	}
	if len(ls) == 0 {
		return 0, 0
	}
	return len(ls) - 1, 0
}

// softLineToByte traduce (fila visual, columna visible) de la línea al byte de
// la línea lógica; una columna en medio de un clúster se redondea a él.
func softLineToByte(line string, width, row, col int) int {
	ls := softLines(line, width)
	if row >= len(ls) {
		row = len(ls) - 1
	}
	if row < 0 {
		return 0
	}
	sl := ls[row]
	c := 0
	g := uniseg.NewGraphemes(sl.text)
	for g.Next() {
		from, _ := g.Positions()
		w := wrapClusterWidth(g.Str(), c)
		if w > 0 {
			if col < c+w {
				return sl.in + from
			}
			c += w
		}
	}
	return sl.in + len(sl.text)
}

// wrapClusterWidth es el ancho en celdas de un cluster a partir de la columna
// col: las tabulaciones se expanden contra la columna y el resto usa el ancho
// real del grapheme (uniseg). 0 = sin celda propia (combinante huérfano).
func wrapClusterWidth(cl string, col int) int {
	if cl == "\t" {
		return tabWidth - col%tabWidth
	}
	if w := uniseg.StringWidth(cl); w > 0 {
		return w
	}
	return 0
}
