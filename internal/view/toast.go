package view

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// ToastDuration es cuánto permanece visible la notificación antes de borrarse
// sola. Es una variable (no una constante) para que los tests la acorten.
var ToastDuration = 2500 * time.Millisecond

// ToastKind clasifica la notificación: define el color con el que se pinta.
type ToastKind int

const (
	// ToastSuccess confirma que algo salió bien («Guardado»).
	ToastSuccess ToastKind = iota
	// ToastError avisa un fallo («Error al guardar: …»).
	ToastError
	// ToastInfo informa un estado neutro («Cancelado», «Instalando…»): ni
	// éxito ni fallo, con su propio color.
	ToastInfo
)

// Toast es la notificación transitoria de la esquina superior derecha: un
// mensaje con fondo propio que se borra solo a los segundos. Vive en la fila
// 0, sobre pestañas y editor —es un overlay, como las ventanas flotantes— y
// el controlador la compone DESPUÉS de todo lo demás en redraw.
type Toast struct {
	message string
	kind    ToastKind
	expires time.Time
	now     func() time.Time
	theme   Theme
}

// NewToast crea la notificación vacía.
func NewToast() *Toast {
	return &Toast{now: time.Now}
}

// SetTheme reemplaza la paleta del componente.
func (t *Toast) SetTheme(th Theme) { t.theme = th }

// Show muestra un mensaje del kind dado y arma su expiración. Mostrar de nuevo
// renueva mensaje y expiración: el controlador es dueño del timer.
func (t *Toast) Show(msg string, kind ToastKind) {
	t.message = msg
	t.kind = kind
	t.expires = t.now().Add(ToastDuration)
}

// Clear oculta la notificación.
func (t *Toast) Clear() {
	t.message = ""
	t.expires = time.Time{}
}

// Visible dice si la notificación sigue vigente: expira cuando ahora llega a
// expires (el límite exacto ya cuenta como expirado).
func (t *Toast) Visible() bool {
	return t.message != "" && t.now().Before(t.expires)
}

// Message expone el mensaje actual. Pensado para tests.
func (t *Toast) Message() string { return t.message }

// Kind expone el kind actual. Pensado para tests.
func (t *Toast) Kind() ToastKind { return t.kind }

// Draw pinta la notificación en la fila 0, alineada a la derecha, con un
// espacio de padding a cada lado y el fondo del estilo del kind. Si el mensaje
// no entra en el ancho disponible se recorta con '…': un aviso cortado se
// lee; un aviso que no entra, no. Con ancho de sobra para el padding ni el
// mensaje no dibuja nada.
func (t *Toast) Draw(sc tcell.Screen, width int) {
	if !t.Visible() || width < 3 {
		return
	}

	th := themeOr(t.theme)
	style := th.ToastSuccess
	switch t.kind {
	case ToastError:
		style = th.ToastError
	case ToastInfo:
		style = th.ToastInfo
	}

	msg := t.message
	if w := displayWidth(msg); w > width-2 {
		msg = truncateWidth(msg, width-3) + "…"
	}

	// La caja ocupa el mensaje más un padding de un lado: arranca en la
	// columna que la deja pegada al borde derecho.
	start := width - displayWidth(msg) - 2
	for x := start; x < width; x++ {
		sc.SetContent(x, 0, ' ', nil, style)
	}
	writeString(sc, start+1, 0, msg, style, displayWidth(msg))
}

// truncateWidth corta s a lo sumo max columnas de ancho de display, sin
// partir caracteres anchos: un grapheme que no entra se descarta completo.
func truncateWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	var b strings.Builder
	w := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		cw := g.Width()
		if cw <= 0 {
			continue
		}
		if w+cw > max {
			break
		}
		b.WriteString(g.Str())
		w += cw
	}
	return b.String()
}
