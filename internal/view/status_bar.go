package view

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// StatusBar es la Screen inferior: muestra el archivo abierto, si hay cambios sin
// guardar, las secciones de las extensiones y el último mensaje.
type StatusBar struct {
	path     string
	modified bool
	message  string
	prompt   string
	theme    Theme
	sections map[string]string
}

func NewStatusBar() *StatusBar { return &StatusBar{} }

// SetTheme reemplaza la paleta del componente.
func (s *StatusBar) SetTheme(th Theme) { s.theme = th }

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

// SetSections reemplaza las secciones de las extensiones (id → texto) del
// buffer activo. El controlador las empuja en syncStatus; la vista solo dibuja.
// Se copia el mapa: la vista es dueña de su snapshot.
func (s *StatusBar) SetSections(sections map[string]string) {
	s.sections = make(map[string]string, len(sections))
	for id, text := range sections {
		s.sections[id] = text
	}
}

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

// Sections expone las secciones del buffer activo (id → texto). Pensado para
// tests.
func (s *StatusBar) Sections() map[string]string { return s.sections }

// sectionsText devuelve las secciones del buffer activo en orden de id,
// separadas por un filete vertical. Orden estable: la barra no puede cambiar
// entre redibujos.
func (s *StatusBar) sectionsText() string {
	if len(s.sections) == 0 {
		return ""
	}
	ids := make([]string, 0, len(s.sections))
	for id := range s.sections {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, s.sections[id])
	}
	return strings.Join(parts, " │ ")
}

// Draw pinta la barra completa en la fila y.
func (s *StatusBar) Draw(sc tcell.Screen, y, width int) {
	if width <= 0 {
		return
	}

	th := themeOr(s.theme)
	status := th.Status
	message := th.Message
	section := th.Section

	// Fondo de la barra: se limpia la fila con su estilo.
	for x := 0; x < width; x++ {
		sc.SetContent(x, y, ' ', nil, status)
	}

	label := s.label()
	labelStyle := status
	sections := s.sectionsText()
	if s.prompt != "" {
		// El pedido se distingue del nombre del archivo: prefijo y estilo
		// de mensaje para que se vea que espera una respuesta.
		label = "» " + label
		labelStyle = message
	}

	if s.message == "" {
		// Sin mensaje: la etiqueta y las secciones ocupan la barra.
		col := writeString(sc, 0, y, label, labelStyle, width)
		if sections != "" && col < width {
			writeString(sc, col, y, " │ "+sections, section, width-col)
		}
		return
	}

	// El MENSAJE tiene prioridad sobre la etiqueta: si no entra completo a la
	// derecha, se recorta la etiqueta —y el mensaje mismo, con '…'— en vez de
	// desaparecer. Un aviso invisible por el ancho de la terminal es un aviso
	// perdido: la confirmación de «cambios sin guardar» tiene que verse siempre.
	msgWidth := displayWidth(s.message)
	if msgWidth >= width {
		writeString(sc, 0, y, s.message, message, width-1)
		sc.SetContent(width-1, y, '…', nil, message)
		return
	}

	start := width - msgWidth
	// La etiqueta y las secciones ceden: como mucho ocupan hasta la celda
	// anterior al mensaje. Entre ellas, la etiqueta gana: las secciones se
	// recortan antes que el nombre del archivo.
	left := start - 1
	col := writeString(sc, 0, y, label, labelStyle, left)
	if sections != "" && col < left {
		writeString(sc, col, y, " │ "+sections, section, left-col)
	}
	writeString(sc, start, y, s.message, message, msgWidth)
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
