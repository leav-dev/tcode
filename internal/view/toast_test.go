package view

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func newToastScreen(t *testing.T, width int) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla: %v", err)
	}
	s.SetSize(width, 1)
	t.Cleanup(s.Fini)
	return s
}

// newStubbedToast devuelve un Toast con el reloj inyectado, para probar la
// expiración sin dormir el test.
func newStubbedToast(now time.Time) *Toast {
	toast := NewToast()
	toast.now = func() time.Time { return now }
	return toast
}

func TestToastStartsHidden(t *testing.T) {
	toast := newStubbedToast(time.Now())
	if toast.Visible() {
		t.Fatal("un toast recién creado no debe estar visible")
	}
}

func TestToastShowMakesItVisibleWithMessageAndKind(t *testing.T) {
	toast := newStubbedToast(time.Now())
	toast.Show("Guardado", ToastSuccess)

	if !toast.Visible() {
		t.Fatal("tras Show el toast debe estar visible")
	}
	if toast.Message() != "Guardado" {
		t.Fatalf("mensaje = %q, se esperaba %q", toast.Message(), "Guardado")
	}
	if toast.Kind() != ToastSuccess {
		t.Fatalf("kind = %d, se esperaba ToastSuccess", toast.Kind())
	}
}

func TestToastExpiresAtTheDeadline(t *testing.T) {
	now := time.Now()
	toast := NewToast()
	// El reloj se inyecta cerrando sobre la variable del test: reasignar now
	// mueve el reloj del toast (un parámetro no alcanzaba).
	toast.now = func() time.Time { return now }
	toast.Show("Guardado", ToastSuccess)

	now = now.Add(ToastDuration - time.Millisecond)
	if !toast.Visible() {
		t.Fatal("un milisegundo antes del plazo el toast sigue visible")
	}

	// El límite exacto ya cuenta como expirado.
	now = now.Add(time.Millisecond)
	if toast.Visible() {
		t.Fatal("en el instante de expiración el toast ya no está visible")
	}
}

func TestToastClearHidesIt(t *testing.T) {
	toast := newStubbedToast(time.Now())
	toast.Show("Guardado", ToastSuccess)
	toast.Clear()

	if toast.Visible() {
		t.Fatal("tras Clear el toast no debe estar visible")
	}
	if toast.Message() != "" {
		t.Fatalf("mensaje = %q, se esperaba vacío tras Clear", toast.Message())
	}
}

func TestToastDrawsRightAlignedWithPadding(t *testing.T) {
	s := newToastScreen(t, 40)
	toast := newStubbedToast(time.Now())
	toast.Show("Guardado", ToastSuccess)
	toast.Draw(s, 40)

	// Alineado a la derecha: la caja ocupa las columnas 25-39 (barra en la
	// 25, padding doble, glifo en la 28, mensaje 30-37, padding en 38-39).
	// screenLines recorta los espacios finales, así que la alineación exacta
	// se verifica celda por celda.
	if c, _, _, _ := s.GetContent(30, 0); c != 'G' {
		t.Fatalf("celda (30,0) = %q, se esperaba 'G' del mensaje alineado a la derecha", c)
	}
	if c, _, _, _ := s.GetContent(39, 0); c != ' ' {
		t.Fatalf("celda (39,0) = %q, se esperaba el padding del borde", c)
	}
	if got := statusRow(t, s); !strings.HasSuffix(got, "✓ Guardado") {
		t.Fatalf("fila = %q, se esperaba el mensaje con glifo al final", got)
	}
	if c, _, _, _ := s.GetContent(28, 0); c != '✓' {
		t.Fatalf("celda (28,0) = %q, se esperaba el glifo de éxito", c)
	}
	// La barra de acento abre la caja con el color fuerte del kind.
	_, _, barStyle, _ := s.GetContent(25, 0)
	_, _, bodyStyle, _ := s.GetContent(30, 0)
	if barStyle == bodyStyle {
		t.Fatal("la barra de acento debe pintar distinto al cuerpo del toast")
	}
}

func TestToastDrawsHiddenWhenNotVisible(t *testing.T) {
	s := newToastScreen(t, 40)
	toast := newStubbedToast(time.Now())
	toast.Draw(s, 40)

	if got := statusRow(t, s); strings.TrimSpace(got) != "" {
		t.Fatalf("fila = %q, se esperaba la fila vacía sin notificación", got)
	}
}

func TestToastDrawsTruncatedWithEllipsis(t *testing.T) {
	// Ancho chico: el mensaje no entra ni con padding, así que se recorta con
	// '…' —un aviso cortado se lee; un aviso que no entra, no—.
	s := newToastScreen(t, 12)
	toast := newStubbedToast(time.Now())
	toast.Show("Cambios sin guardar: Ctrl+S guarda", ToastError)
	toast.Draw(s, 12)

	got := statusRow(t, s)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("fila = %q, se esperaba la elipsis del mensaje recortado", got)
	}
	if w := displayWidth(got); w > 12 {
		t.Fatalf("fila = %q (%d columnas), se esperaba recortada a 12", got, w)
	}
}

func TestToastDrawsWithWideCharactersStaysInside(t *testing.T) {
	s := newToastScreen(t, 40)
	toast := newStubbedToast(time.Now())
	toast.Show("村", ToastSuccess)
	toast.Draw(s, 40)

	// El carácter ancho ocupa dos celdas: debe quedar dentro de la fila, con su
	// celda de continuación en blanco, no desbordar la pantalla.
	if got := statusRow(t, s); !strings.Contains(got, "村") {
		t.Fatalf("fila = %q, se esperaba el carácter ancho", got)
	}
	if c, _, _, w := s.GetContent(39, 0); c != ' ' || w != 1 {
		t.Fatalf("celda (39,0) = %q width=%d, se esperaba el padding final", c, w)
	}
}

func TestToastDrawsNothingOnTinyWidth(t *testing.T) {
	s := newToastScreen(t, 2)
	toast := newStubbedToast(time.Now())
	toast.Show("Guardado", ToastSuccess)
	toast.Draw(s, 2)

	if got := statusRow(t, s); strings.TrimSpace(got) != "" {
		t.Fatalf("fila = %q, se esperaba que no dibuje con ancho 2", got)
	}
}

func TestToastErrorKindUsesTheErrorStyle(t *testing.T) {
	th := Theme{
		ToastSuccess: tcell.StyleDefault.Foreground(tcell.PaletteColor(42)),
		ToastError:   tcell.StyleDefault.Foreground(tcell.PaletteColor(208)),
	}
	s := newToastScreen(t, 40)
	toast := newStubbedToast(time.Now())
	toast.SetTheme(th)
	toast.Show("Error al guardar: disco lleno", ToastError)
	toast.Draw(s, 40)

	// El kind define el estilo: el error y el éxito no pueden pintar igual.
	_, _, errStyle, _ := s.GetContent(40-displayWidth("Error al guardar: disco lleno"), 0)

	ok := newStubbedToast(time.Now())
	ok.SetTheme(th)
	ok.Show("Guardado", ToastSuccess)
	s2 := newToastScreen(t, 40)
	ok.Draw(s2, 40)
	_, _, okStyle, _ := s2.GetContent(40-displayWidth("Guardado"), 0)

	if errStyle == okStyle {
		t.Fatal("el kind error y el kind success deben pintar con estilos distintos")
	}
}

func TestToastInfoKindUsesItsOwnStyle(t *testing.T) {
	th := Theme{
		ToastSuccess: tcell.StyleDefault.Foreground(tcell.PaletteColor(42)),
		ToastError:   tcell.StyleDefault.Foreground(tcell.PaletteColor(208)),
		ToastInfo:    tcell.StyleDefault.Foreground(tcell.PaletteColor(39)),
	}
	s := newToastScreen(t, 40)
	toast := newStubbedToast(time.Now())
	toast.SetTheme(th)
	toast.Show("Instalando remoto/tcode.tema…", ToastInfo)
	toast.Draw(s, 40)

	_, _, infoStyle, _ := s.GetContent(40-displayWidth("Instalando remoto/tcode.tema…"), 0)

	ok := newStubbedToast(time.Now())
	ok.SetTheme(th)
	ok.Show("Guardado", ToastSuccess)
	s2 := newToastScreen(t, 40)
	ok.Draw(s2, 40)
	_, _, okStyle, _ := s2.GetContent(40-displayWidth("Guardado"), 0)

	if infoStyle == okStyle {
		t.Fatal("el kind info y el kind success deben pintar con estilos distintos")
	}
}
