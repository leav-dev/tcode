package view

import "github.com/gdamore/tcell/v2"

// wheelScroll es el desplazamiento de la rueda del mouse en filas, el mismo
// paso de 3 que usa el editor.
const wheelScroll = 3

// Entry es una fila del explorador: el nombre visible, la ruta completa y si
// es un directorio (los directorios se dibujan con el sufijo "/"). La entrada
// sintética ".." es Entry{"..", dirPadre, true} y la compone el controlador,
// que también es quien rellena Path; la vista solo dibuja lo que recibe.
type Entry struct {
	Name  string
	Path  string
	IsDir bool
}

// FileBrowser es el listado de una sola columna del directorio actual.
//
// La vista NO abre archivos (eso es decisión de ws.Open en el controlador) ni
// lee el filesystem: el controlador lee con os.ReadDir y deposita el listado
// con SetEntries. Quien compone —el controlador— elige dónde cae el panel con
// la Surface; la vista dibuja desde (0,0) en coordenadas propias, como el
// editor.
//
// Un árbol recursivo exigiría cargar subdirectorios por nodo; un listado con
// Enter para descender, ".." para subir (bordeado por el root de la sesión) y
// scroll cumple la misma función con un viewport virtual simple: el mismo
// patrón de viewports que el editor.
type FileBrowser struct {
	entries []Entry
	cursor  int // índice de la entrada activa
	top     int // primera entrada visible
	width   int
	height  int
}

func NewFileBrowser() *FileBrowser {
	return &FileBrowser{}
}

// SetEntries reemplaza el listado y mantiene la activa visible: el cursor se
// clampa al rango nuevo y el scroll se corre lo mínimo para que siga en
// pantalla (el mismo trato que el editor ante un resize). Con el listado vacío
// el cursor vuelve a 0 y Draw no pinta nada.
func (fb *FileBrowser) SetEntries(entries []Entry) {
	fb.entries = entries
	fb.clamp()
	fb.ensureCursorVisible()
}

// clamp mantiene cursor y top dentro del listado actual. Sobre un listado
// vacío ambos vuelven a 0.
func (fb *FileBrowser) clamp() {
	if len(fb.entries) == 0 {
		fb.cursor, fb.top = 0, 0
		return
	}
	n := len(fb.entries)
	fb.cursor = min(max(fb.cursor, 0), n-1)
	maxTop := n - fb.height
	if maxTop < 0 {
		maxTop = 0
	}
	fb.top = min(max(fb.top, 0), maxTop)
}

// ensureCursorVisible corre top lo mínimo para que la entrada activa quede
// dentro del alto del panel, como hace el editor con su viewport.
func (fb *FileBrowser) ensureCursorVisible() {
	if len(fb.entries) == 0 || fb.height <= 0 {
		return
	}
	if fb.cursor < fb.top {
		fb.top = fb.cursor
	}
	if fb.cursor >= fb.top+fb.height {
		fb.top = fb.cursor - fb.height + 1
	}
	fb.clamp()
}

// setCursor coloca el cursor en el índice i (clampeado) y mantiene la activa
// visible.
func (fb *FileBrowser) setCursor(i int) {
	fb.cursor = i
	fb.clamp()
	fb.ensureCursorVisible()
}

// moveCursor desplaza el cursor en delta y devuelve si cambió de posición.
func (fb *FileBrowser) moveCursor(delta int) bool {
	n := len(fb.entries)
	if n == 0 {
		return false
	}
	target := fb.cursor + delta
	if target < 0 {
		target = 0
	}
	if target >= n {
		target = n - 1
	}
	if target == fb.cursor {
		return false
	}
	fb.cursor = target
	fb.ensureCursorVisible()
	return true
}

// page es el salto de página: lo que cabe en el alto del panel, mínimo 1.
func (fb *FileBrowser) page() int {
	if fb.height > 1 {
		return fb.height
	}
	return 1
}

// Resize actualiza las dimensiones del panel y reencuadra el scroll, como el
// resize del editor: la activa queda visible y el top dentro del rango.
func (fb *FileBrowser) Resize(width, height int) {
	if width > 0 {
		fb.width = width
	}
	if height > 0 {
		fb.height = height
	}
	fb.clamp()
	fb.ensureCursorVisible()
}

// CursorPath devuelve la ruta de la entrada activa, o "" si no hay entradas.
func (fb *FileBrowser) CursorPath() string {
	if len(fb.entries) == 0 {
		return ""
	}
	return fb.entries[fb.cursor].Path
}

// HandleEvent procesa el teclado y el mouse del panel y devuelve (handled,
// activate): handled dice si el evento era del panel y activate si la entrada
// activa se activó (Enter) para que el controlador la abra o descienda a ella.
//
// Up/Down/PageUp/PageDown/Home/End mueven el cursor y devuelven (true, false)
// aunque el cursor no se mueva: con el foco en el explorador, esas teclas son
// del panel y no del documento. Enter sobre una entrada devuelve (true, true);
// el mouse selecciona con Button1 y scrollea con la rueda. Toda otra tecla o
// evento devuelve (false, false) y cae al flujo normal del controlador (atajos,
// documento, salida).
func (fb *FileBrowser) HandleEvent(ev tcell.Event) (handled, activate bool) {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyUp:
			fb.moveCursor(-1)
			return true, false
		case tcell.KeyDown:
			fb.moveCursor(1)
			return true, false
		case tcell.KeyPgUp:
			// Ctrl+PageUp/PageDown cambian de pestaña y son del controlador
			// (U2b), no scroll de página: el panel los deja caer, igual que el
			// editor los descarta con ModCtrl para que ninguna variante de
			// terminal escrolle por accidente.
			if ev.Modifiers()&tcell.ModCtrl != 0 {
				return false, false
			}
			fb.moveCursor(-fb.page())
			return true, false
		case tcell.KeyPgDn:
			if ev.Modifiers()&tcell.ModCtrl != 0 {
				return false, false
			}
			fb.moveCursor(fb.page())
			return true, false
		case tcell.KeyHome:
			fb.setCursor(0)
			return true, false
		case tcell.KeyEnd:
			fb.setCursor(len(fb.entries) - 1)
			return true, false
		case tcell.KeyEnter, tcell.KeyLF:
			if len(fb.entries) == 0 {
				return false, false
			}
			return true, true
		}

	case *tcell.EventMouse:
		_, y := ev.Position()
		switch {
		case ev.Buttons()&tcell.Button1 != 0:
			// El clic selecciona la fila; el controlador enfoca el panel al
			// enterarse de que el panel lo manejó. Un clic fuera de las filas
			// del panel no selecciona nada.
			if len(fb.entries) == 0 || y < 0 || y >= fb.height {
				return false, false
			}
			fb.setCursor(fb.top + y)
			return true, false
		case ev.Buttons()&tcell.WheelUp != 0:
			return fb.moveCursor(-wheelScroll), false
		case ev.Buttons()&tcell.WheelDown != 0:
			return fb.moveCursor(wheelScroll), false
		}
	}
	return false, false
}

// Draw pinta el listado en coordenadas propias desde (0,0): la entrada activa
// va resaltada a todo el ancho del panel y los directorios con el sufijo "/".
// Los nombres que no entran se recortan contra el ancho del panel (writeString
// avanza por grapheme cluster). Con el listado vacío o sin alto no hay nada
// que dibujar.
func (fb *FileBrowser) Draw(s Surface) {
	if len(fb.entries) == 0 || fb.height <= 0 {
		return
	}
	for row := 0; row < fb.height; row++ {
		idx := fb.top + row
		if idx >= len(fb.entries) {
			break
		}
		e := fb.entries[idx]

		// La fila se pinta entera con el estilo de la entrada: la barra de
		// selección de ancho completo hace legible la activa aunque el nombre
		// sea corto, y el repintado por fila limpia el resaltado viejo del
		// redibujo anterior.
		style := tcell.StyleDefault
		if idx == fb.cursor {
			style = tcell.StyleDefault.Reverse(true)
		}
		for x := 0; x < fb.width; x++ {
			s.SetContent(x, row, ' ', nil, style)
		}

		name := e.Name
		if e.IsDir {
			name += "/"
		}
		writeString(s, 0, row, name, style, fb.width)
	}
}
