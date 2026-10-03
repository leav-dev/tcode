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

// label es el texto de la izquierda: nombre del archivo y marca de modificación.
func (s *StatusBar) label() string {
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
	used := writeString(sc, 0, y, label, s.style, width)

	if s.message == "" {
		return
	}
	msgWidth := displayWidth(s.message)
	// Solo se muestra si entra sin pisar la etiqueta.
	if start := width - msgWidth; start > used {
		writeString(sc, start, y, s.message, s.style, width-start)
	}
}

// writeString escribe s en la fila y desde la columna x, respetando maxWidth
// columnas. Avanza por *grapheme cluster* para que los caracteres anchos no
// desalineen. Devuelve las columnas usadas.
func writeString(sc tcell.Screen, x, y int, s string, style tcell.Style, maxWidth int) int {
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
