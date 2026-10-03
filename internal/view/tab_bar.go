package view

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/model"
)

// TabBar es la fila 0 de la composición: las pestañas abiertas del workspace.
// Dibuja la activa en estilo invertido y se desplaza horizontalmente cuando no
// entran todas, marcando el desborde con '<' y '>' en los bordes.
type TabBar struct {
	// start es el índice de la primera pestaña visible, la ventana de
	// desplazamiento horizontal. Lo mueve EnsureActive, nunca Draw.
	start int
}

func NewTabBar() *TabBar {
	return &TabBar{}
}

// tabLabel es el texto de una pestaña: el nombre base de la ruta con la marca
// de modificación, con el mismo formato que la barra de estado. Un buffer sin
// ruta se muestra como "(sin nombre)".
func tabLabel(buf *model.PieceTable) string {
	name := buf.Path()
	if name == "" {
		name = "(sin nombre)"
	} else {
		name = filepath.Base(name)
	}
	if buf.Modified() {
		return name + " [+]"
	}
	return name
}

// tabLabelWidth es el ancho en celdas de la etiqueta de la pestaña i. No se
// llama tabWidth: ese nombre ya es la constante de columnas de una tabulación
// del editor.
func tabLabelWidth(ws *model.Workspace, i int) int {
	return displayWidth(tabLabel(ws.BufferAt(i)))
}

// Draw pinta la fila 0 entera de su superficie: las pestañas de ws. La activa
// va en estilo invertido y las demás con el estilo por defecto; entre pestañas
// hay un separador de una columna. Si no entran todas, la ventana (`start`) se
// recorre y las flechas '<' y '>' marcan que hay más a cada lado. Una pestaña
// que no entra entera se trunca mostrando su inicio —con '…' en la última
// celda si hay lugar— y corta la fila.
//
// Dibuja sobre una Surface y no sobre la pantalla para que quien compone —el
// controlador— la desplace: las pestañas viven sobre el área del editor, no
// sobre el panel del árbol, y el offset lo pone el OffsetSurface igual que
// para el editor.
func (tb *TabBar) Draw(sc Surface, ws *model.Workspace, width int) {
	if width <= 0 {
		return
	}

	// La fila se limpia entera con el estilo por defecto: las pestañas no
	// tienen fondo propio más allá del del terminal.
	for x := 0; x < width; x++ {
		sc.SetContent(x, 0, ' ', nil, tcell.StyleDefault)
	}
	if ws.Len() == 0 {
		return
	}

	active := ws.ActiveIndex()
	start := tb.start

	// La flecha izquierda ocupa la columna 0 cuando la ventana está corrida;
	// las pestañas entonces arrancan en la columna 1.
	contentStart := 0
	if start > 0 {
		sc.SetContent(0, 0, '<', nil, tcell.StyleDefault)
		contentStart = 1
	}

	// ¿Quedan pestañas fuera por la derecha? Se mide el total desde start con
	// un separador por pestaña: si no entra, la última columna queda reservada
	// para '>'.
	total := 0
	for i := start; i < ws.Len(); i++ {
		if i > start {
			total++
		}
		total += tabLabelWidth(ws, i)
	}
	right := total > width-contentStart
	contentEnd := width - 1
	if right {
		contentEnd--
	}

	x := contentStart
	for i := start; i < ws.Len(); i++ {
		if x > contentEnd {
			break
		}
		label := tabLabel(ws.BufferAt(i))
		style := tcell.StyleDefault
		if i == active {
			style = tcell.StyleDefault.Reverse(true)
		}

		remaining := contentEnd - x + 1
		w := displayWidth(label)
		if w > remaining {
			// La pestaña no entra entera: se trunca mostrando su inicio y la
			// fila corta acá. Si queda al menos una celda además del inicio,
			// la última de la fila muestra la elipsis que marca el corte.
			if remaining >= 2 {
				writeString(sc, x, 0, label, style, remaining-1)
				sc.SetContent(contentEnd, 0, '…', nil, style)
			} else {
				writeString(sc, x, 0, label, style, remaining)
			}
			break
		}
		writeString(sc, x, 0, label, style, remaining)
		x += w
		if i+1 < ws.Len() {
			x++ // separador de una columna entre pestañas
		}
	}

	if right {
		sc.SetContent(width-1, 0, '>', nil, tcell.StyleDefault)
	}
}

// EnsureActive corre start lo mínimo a la derecha hasta que la pestaña activa
// entre entera; si ni así entra —es más ancha que toda la fila—, start queda
// en la activa y su comienzo es lo único que se dibuja (truncado). Si ya es
// visible, no mueve nada: es idempotente. Lineal y sin alocar estructuras: la
// etiqueta de cada pestaña se mide una sola vez.
func (tb *TabBar) EnsureActive(ws *model.Workspace, width int) {
	if width <= 0 {
		return
	}
	n := ws.Len()
	if n == 0 {
		tb.start = 0
		return
	}
	active := ws.ActiveIndex()
	if active < 0 {
		tb.start = 0
		return
	}

	// La ventana nunca arranca después de la activa: si la activa quedó a la
	// izquierda del inicio (se cerraron pestañas anteriores), el inicio vuelve
	// a ella.
	if tb.start > active {
		tb.start = active
	}

	// totalNeed es el ancho de todas las pestañas desde start, con separador
	// de una columna entre cada par. Por cada paso del inicio a la derecha se
	// descuenta una pestaña, sin recalcular desde cero.
	totalNeed := 0
	for i := tb.start; i < n; i++ {
		if i > tb.start {
			totalNeed++
		}
		totalNeed += tabLabelWidth(ws, i)
	}

	start := tb.start
	for {
		contentStart := 0
		if start > 0 {
			contentStart = 1 // '<' en la columna 0
		}
		right := totalNeed > width-contentStart
		contentEnd := width - 1
		if right {
			contentEnd-- // reservar la última columna para '>'
		}

		// Posición de la activa dibujando desde start.
		x := contentStart
		for i := start; i < active; i++ {
			x += tabLabelWidth(ws, i) + 1
		}
		w := tabLabelWidth(ws, active)

		if x+w-1 <= contentEnd {
			tb.start = start
			return // ya entra entera (si start no cambió, no se movió nada)
		}
		if start == active {
			// Ni arrancando en la activa entra: es más ancha que la fila.
			tb.start = active
			return
		}
		totalNeed -= tabLabelWidth(ws, start) + 1
		start++
	}
}
