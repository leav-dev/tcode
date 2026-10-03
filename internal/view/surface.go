package view

import "github.com/gdamore/tcell/v2"

// Surface es la interfaz mínima que la vista usa para dibujar: escribir texto,
// depositar contenido en una celda y controlar el cursor. tcell.Screen ya la
// satisface, así que una vista dibuja igual sobre una terminal real que sobre
// un adaptador que la compone (OffsetSurface). Quien compone —el controlador—
// elige dónde arranca cada pane; la vista nunca se entera del layout.
type Surface interface {
	Put(x, y int, s string, style tcell.Style) (string, int)
	SetContent(x, y int, mainc rune, combc []rune, style tcell.Style)
	ShowCursor(x, y int)
	HideCursor()
}

// OffsetSurface envuelve una tcell.Screen, suma un origen (ox, oy) a todas las
// coordenadas y recorta contra una región de w por h columnas. Todo lo que cae
// fuera de la región se descarta; el cursor fuera de la región se esconde: no
// puede quedar flotando sobre otra pane.
//
// La instancia se reusa: el controlador la crea una vez y la reencuadra con
// SetRegion cuando cambia el tamaño de la terminal, en lugar de alocar una
// superficie por redibujo.
type OffsetSurface struct {
	sc     tcell.Screen
	ox, oy int
	w, h   int
}

func NewOffsetSurface(sc tcell.Screen) *OffsetSurface {
	return &OffsetSurface{sc: sc}
}

// SetRegion actualiza origen y región sin reasignar la instancia.
func (o *OffsetSurface) SetRegion(x, y, w, h int) {
	o.ox, o.oy = x, y
	o.w, o.h = w, h
}

// Put escribe s traducido al origen y recortado contra el borde derecho de la
// región: lo que excede vuelve como sobrante y no se escribe, igual que hace
// la pantalla con su propio borde. Fuera de la región devuelve (s, 0) sin
// escribir nada.
func (o *OffsetSurface) Put(x, y int, s string, style tcell.Style) (string, int) {
	if x < 0 || y < 0 || x >= o.w || y >= o.h || s == "" {
		return s, 0
	}
	maxWidth := o.w - x
	used := 0
	for s != "" && used < maxWidth {
		var w int
		s, w = o.sc.Put(x+o.ox+used, y+o.oy, s, style)
		if w == 0 {
			break
		}
		used += w
	}
	return s, used
}

// SetContent traduce la celda y descarta la que cae fuera de la región.
func (o *OffsetSurface) SetContent(x, y int, mainc rune, combc []rune, style tcell.Style) {
	if x < 0 || y < 0 || x >= o.w || y >= o.h {
		return
	}
	o.sc.SetContent(x+o.ox, y+o.oy, mainc, combc, style)
}

// ShowCursor traduce el cursor; si cae fuera de la región, lo esconde.
func (o *OffsetSurface) ShowCursor(x, y int) {
	if x < 0 || y < 0 || x >= o.w || y >= o.h {
		o.sc.HideCursor()
		return
	}
	o.sc.ShowCursor(x+o.ox, y+o.oy)
}

// HideCursor delega: no hay traducción que aplicar.
func (o *OffsetSurface) HideCursor() {
	o.sc.HideCursor()
}
