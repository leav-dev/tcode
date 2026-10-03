package view

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// StatusBar es la Screen inferior: muestra el archivo abierto, si hay cambios sin
// guardar y el último mensaje.
type StatusBar struct {
	path     string
	modified bool
	message  string
	prompt   string
	style    tcell.Style
}

func NewStatusBar() *StatusBar {
	return &StatusBar{style: tcell.StyleDefault.Reverse(true)}
}

// SetFile actualiza el archivo mostrado y su estado de modificación.
func (s *StatusBar) SetFile(path string, modified bool) {
	s.path = path
	s.modified = modified
}

// SetMessage muestra un mensaje.
func (s *StatusBar) SetMessage(msg string) { s.message = msg }

// ClearMessage borra el mensaje.
func (s *StatusBar) ClearMessage() { s.message = "" }

// SetPrompt muestra un pedido de texto en la barra, en lugar de la etiqueta del
// archivo. Se usa para Save As.
func (s *StatusBar) SetPrompt(text string) { s.prompt = text }

// label es el texto de la izquierda: el pedido activo o el nombre del archivo con
// su marca de modificación.
func (s *StatusBar) label() string {
	if s.prompt != "" {
		return s.prompt
	}

	name := s.path
	if name == "" {
		name = "(sin archivo)"
	} else {
		name = filepath.Base(name)
	}
	if s.modified {
		// La marca de modificado es lo que evita perder trabajo sin darse cuenta.
		return name + " [+]"
	}
	return name + "    "
}

// Label expone la etiqueta que la barra esta mostrando. Pensado para tests.
func (s *StatusBar) Label() string { return s.label() }

// Message expone el mensaje transitorio actual. Pensado para tests.
func (s *StatusBar) Message() string { return s.message }

// Draw pinta la barra completa en la fila y.
func (s *StatusBar) Draw(sc tcell.Screen, y, width int) {
	if width <= 0 {
		return
	}

	// Fondo de la barra: se limpia la fila con su estilo.
	for x := 0; x < width; x++ {
		sc.SetContent(x, y, ' ', nil, s.style)
	}

	label := s.label()
	if s.message == "" {
		writeString(sc, 0, y, label, s.style, width)
		return
	}

	// El MENSAJE tiene prioridad sobre la etiqueta: si no entra completo a la
	// derecha, se recorta la etiqueta —y el mensaje mismo, con '…'— en vez de
	// desaparecer. Un aviso invisible por el ancho de la terminal es un aviso
	// perdido: la confirmación de «cambios sin guardar» tiene que verse siempre.
	msgWidth := displayWidth(s.message)
	if msgWidth >= width {
		writeString(sc, 0, y, s.message, s.style, width-1)
		sc.SetContent(width-1, y, '…', nil, s.style)
		return
	}

	start := width - msgWidth
	// La etiqueta cede: como mucho ocupa hasta la celda anterior al mensaje.
	writeString(sc, 0, y, label, s.style, start-1)
	writeString(sc, start, y, s.message, s.style, msgWidth)
}

// writeString escribe s en la fila y desde la columna x, respetando maxWidth
// columnas. Avanza por *grapheme cluster* para que los caracteres anchos no
// desalineen. Devuelve las columnas usadas.
//
// Recibe una Surface y no una tcell.Screen (aditivo, sin cambio de
// comportamiento: solo usa Put, que la interfaz expone; los callers pasan
// tcell.Screen, que ya la satisface). Es la misma evolución que EditorView.Draw
// hizo en U2b: el explorador (U3) la usa para dibujar sobre la superficie
// compuesta, sin enterarse del layout.
func writeString(sc Surface, x, y int, s string, style tcell.Style, maxWidth int) int {
	col := 0
	for s != "" && col < maxWidth {
		var width int
		s, width = sc.Put(x+col, y, s, style)
		if width == 0 {
			break
		}
		col += width
	}
	return col
}

// displayWidth devuelve el ancho en celdas de un texto ya formateado.
func displayWidth(s string) int {
	total := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		if w := g.Width(); w > 0 {
			total += w
		}
	}
	return total
}
