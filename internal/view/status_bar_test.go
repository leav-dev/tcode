package view

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func newStatusScreen(t *testing.T, width int) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar la pantalla: %v", err)
	}
	s.SetSize(width, 1)
	t.Cleanup(s.Fini)
	return s
}

func statusRow(t *testing.T, s tcell.SimulationScreen) string {
	t.Helper()
	s.Show()
	lines := screenLines(s)
	if len(lines) == 0 {
		t.Fatal("la pantalla no tiene filas")
	}
	return lines[0]
}

func TestStatusBarShowsFileNameAndModifiedMark(t *testing.T) {
	s := newStatusScreen(t, 40)
	bar := NewStatusBar()
	bar.SetFile("/home/alguien/priv/tcode/main.go", false)
	bar.Draw(s, 0, 40)

	if got := statusRow(t, s); got != "main.go" {
		t.Fatalf("barra = %q, se esperaba solo el nombre base", got)
	}

	bar.SetFile("/home/alguien/priv/tcode/main.go", true)
	bar.Draw(s, 0, 40)

	if got := statusRow(t, s); !strings.Contains(got, "main.go [+]") {
		t.Fatalf("barra = %q, se esperaba la marca de modificado", got)
	}
}

func TestStatusBarWithoutFile(t *testing.T) {
	s := newStatusScreen(t, 40)
	bar := NewStatusBar()
	bar.Draw(s, 0, 40)

	if got := statusRow(t, s); !strings.Contains(got, "(sin archivo)") {
		t.Fatalf("barra = %q, se esperaba el marcador de sin archivo", got)
	}
}

func TestStatusBarShowsMessageRightAligned(t *testing.T) {
	s := newStatusScreen(t, 40)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)
	bar.SetMessage("Guardado")
	bar.Draw(s, 0, 40)

	got := statusRow(t, s)
	if !strings.HasPrefix(got, "main.go") {
		t.Fatalf("barra = %q, se esperaba que empiece con el archivo", got)
	}
	if !strings.HasSuffix(got, "Guardado") {
		t.Fatalf("barra = %q, se esperaba el mensaje al final", got)
	}
	if !strings.Contains(got, " Guardado") {
		t.Fatalf("barra = %q, se esperaba el cerrador del prompt antes del mensaje", got)
	}
}

func TestStatusBarMessageWinsOverTheLabelAndTruncates(t *testing.T) {
	// Ancho chico: el mensaje no entra ni recortando la etiqueta del todo, así
	// que se muestra recortado con '…' —un aviso no puede desaparecer por el
	// ancho de la terminal: un aviso invisible es un aviso perdido—.
	s := newStatusScreen(t, 12)
	bar := NewStatusBar()
	bar.SetFile("unarchivolargo.go", true)
	bar.SetMessage("Cambios sin guardar: Ctrl+S guarda, Escape de nuevo sale igual")
	bar.Draw(s, 0, 12)

	got := statusRow(t, s)
	if !strings.HasPrefix(got, " Cambios si") {
		t.Fatalf("barra = %q, se esperaba el mensaje con prioridad sobre la etiqueta", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("barra = %q, se esperaba la elipsis del mensaje recortado", got)
	}
}

func TestStatusBarTruncatesTheLabelToKeepTheWholeMessage(t *testing.T) {
	// El mensaje entra en la fila pero no al lado de la etiqueta completa: la
	// etiqueta es la que se recorta, y el mensaje se ve entero.
	s := newStatusScreen(t, 40)
	bar := NewStatusBar()
	bar.SetFile("unarchivolargo.go", true)
	bar.SetMessage("Guardado en copia.txt")
	bar.Draw(s, 0, 40)

	got := statusRow(t, s)
	if !strings.HasSuffix(got, "Guardado en copia.txt") {
		t.Fatalf("barra = %q, se esperaba el mensaje completo al final", got)
	}
	if !strings.HasPrefix(got, "unarchivolargo.go") {
		t.Fatalf("barra = %q, se esperaba el nombre recortado por el lugar del mensaje", got)
	}
}

func TestStatusBarClearMessage(t *testing.T) {
	s := newStatusScreen(t, 40)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)
	bar.SetMessage("Guardado")
	bar.ClearMessage()
	bar.Draw(s, 0, 40)

	if got := statusRow(t, s); strings.Contains(got, "Guardado") {
		t.Fatalf("barra = %q: el mensaje fue borrado", got)
	}
}

func TestStatusBarDrawsSectionsBetweenLabelAndMessage(t *testing.T) {
	s := newStatusScreen(t, 60)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)
	bar.SetMessage("Recargado")
	bar.SetSections(map[string]string{"tcode.gitchanges": "Git: 3 files"})
	bar.Draw(s, 0, 60)

	got := statusRow(t, s)
	if !strings.Contains(got, "main.go") {
		t.Fatalf("barra = %q, se esperaba el archivo a la izquierda", got)
	}
	if !strings.Contains(got, "Git: 3 files") {
		t.Fatalf("barra = %q, se esperaba la sección de la extensión", got)
	}
	if !strings.HasSuffix(got, "Recargado") {
		t.Fatalf("barra = %q, se esperaba el mensaje a la derecha", got)
	}
}

func TestStatusBarDrawsSectionsSortedByID(t *testing.T) {
	// El orden de id es el orden de la barra: no puede cambiar entre
	// redibujos.
	s := newStatusScreen(t, 60)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)
	bar.SetSections(map[string]string{
		"tcode.zzz": "tercera",
		"tcode.aaa": "primera",
		"tcode.mmm": "segunda",
	})
	bar.Draw(s, 0, 60)

	got := statusRow(t, s)
	if !strings.Contains(got, "primera segunda tercera") {
		t.Fatalf("barra = %q, se esperaba las secciones en orden de id como segmentos", got)
	}
}

func TestStatusBarDrawsSectionsWithoutMessage(t *testing.T) {
	s := newStatusScreen(t, 60)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)
	bar.SetSections(map[string]string{"tcode.gitchanges": "Git: 3 files"})
	bar.Draw(s, 0, 60)

	got := statusRow(t, s)
	if !strings.Contains(got, "main.go") || !strings.Contains(got, "Git: 3 files") {
		t.Fatalf("barra = %q, se esperaba etiqueta y sección sin mensaje", got)
	}
}

func TestStatusBarSectionsPinnedRight(t *testing.T) {
	// Sin mensaje, la sección de git va fija al borde derecho, no pegada
	// a la etiqueta.
	s := newStatusScreen(t, 60)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)
	bar.SetSections(map[string]string{"tcode.gitchanges": "Git: 3 files"})
	bar.Draw(s, 0, 60)

	got := statusRow(t, s)
	if !strings.HasSuffix(got, "Git: 3 files") {
		t.Fatalf("barra = %q, se esperaba la sección al borde derecho", got)
	}
	if !strings.HasPrefix(got, "main.go") {
		t.Fatalf("barra = %q, se esperaba la etiqueta a la izquierda", got)
	}
}

func TestStatusBarSectionsPinnedRightOfMessage(t *testing.T) {
	// Con mensaje, las secciones van fijas a la derecha del hueco,
	// pegadas al mensaje de la derecha.
	s := newStatusScreen(t, 60)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)
	bar.SetMessage("Recargado")
	bar.SetSections(map[string]string{"tcode.gitchanges": "Git: 3 files"})
	bar.Draw(s, 0, 60)

	got := statusRow(t, s)
	if !strings.HasSuffix(got, "Git: 3 files Recargado") {
		t.Fatalf("barra = %q, se esperaba sección pegada al mensaje a la derecha", got)
	}
}

func TestStatusBarSectionsTruncateBeforeTheLabel(t *testing.T) {
	// Con mensaje y ancho choco: el mensaje gana, la etiqueta conserva
	// su lugar y las secciones se recortan primero.
	s := newStatusScreen(t, 30)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)
	bar.SetMessage("Recargado")
	bar.SetSections(map[string]string{"ext": "una sección larguísima"})
	bar.Draw(s, 0, 30)

	got := statusRow(t, s)
	if !strings.Contains(got, "main.go") {
		t.Fatalf("barra = %q, la etiqueta gana sobre las secciones", got)
	}
	if !strings.HasSuffix(got, "Recargado") {
		t.Fatalf("barra = %q, el mensaje tiene prioridad", got)
	}
	if w := displayWidth(got); w > 30 {
		t.Fatalf("barra = %q (%d columnas), se esperaba recortada a 30", got, w)
	}
}

func TestStatusBarEmptySectionsLeaveTheBarAsBefore(t *testing.T) {
	s := newStatusScreen(t, 40)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)
	bar.SetMessage("Guardado")
	bar.SetSections(map[string]string{})
	bar.Draw(s, 0, 40)

	got := statusRow(t, s)
	if !strings.HasPrefix(got, "main.go") || !strings.HasSuffix(got, "Guardado") {
		t.Fatalf("barra = %q, se esperaba el comportamiento de siempre sin secciones", got)
	}
}

func TestStatusBarWithWideCharactersStaysAligned(t *testing.T) {
	s := newStatusScreen(t, 40)
	bar := NewStatusBar()
	bar.SetFile("/tmp/日本語.go", false)
	bar.SetMessage("村")
	bar.Draw(s, 0, 40)

	// El mensaje es un carácter ancho: debe quedar en la última columna útil, no
	// desbordar la pantalla.
	if got := statusRow(t, s); !strings.Contains(got, "村") {
		t.Fatalf("barra = %q, se esperaba el carácter ancho", got)
	}
}

func TestStatusBarIgnoresZeroWidth(t *testing.T) {
	s := newStatusScreen(t, 40)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)

	// No debe entrar en pánico ni escribir nada.
	bar.Draw(s, 0, 0)
}

func TestStatusBarTruncatesLongFileName(t *testing.T) {
	s := newStatusScreen(t, 5)
	bar := NewStatusBar()
	bar.SetFile("/tmp/nombrelarguisimo.go", true)
	bar.Draw(s, 0, 5)

	got := statusRow(t, s)
	if displayWidth(got) > 5 {
		t.Fatalf("barra = %q, se esperaba recortada a 5 columnas", got)
	}
}

// TestWriteStringAdvancesByDisplayWidth verifica el avance por ancho real. Se
// comprueba celda por celda y no sobre la línea reconstruida, porque screenLines
// escribe un espacio en la celda de continuación de un carácter ancho.
func TestWriteStringAdvancesByDisplayWidth(t *testing.T) {
	s := newStatusScreen(t, 10)
	// "日" ocupa dos celdas, así que la 'b' debe quedar en la columna 2.
	used := writeString(s, 0, 0, "日b", tcell.StyleDefault, 10)

	if used != 3 {
		t.Fatalf("writeString usó %d columnas, se esperaban 3", used)
	}

	s.Show()
	if c, _, _, w := s.GetContent(0, 0); c != '日' || w != 2 {
		t.Fatalf("celda (0,0) = %q width=%d, se esperaba '日' width=2", c, w)
	}
	if c, _, _, _ := s.GetContent(2, 0); c != 'b' {
		t.Fatalf("celda (2,0) = %q, se esperaba 'b'", c)
	}
}

// TestStatusBarPromptStandsOut: el pedido activo se distingue del nombre del
// archivo con el prefijo » y el estilo de mensaje.
func TestStatusBarPromptStandsOut(t *testing.T) {
	s := newStatusScreen(t, 40)
	bar := NewStatusBar()
	bar.SetFile("main.go", false)
	bar.SetPrompt("¿Borrar a.txt? [s/N] ")
	bar.Draw(s, 0, 40)

	if got := statusRow(t, s); !strings.HasPrefix(got, "» ¿Borrar a.txt? [s/N]") {
		t.Fatalf("barra = %q, se esperaba el pedido con prefijo »", got)
	}
	_, _, promptStyle, _ := s.GetContent(0, 0)
	plain := NewStatusBar()
	plain.SetFile("main.go", false)
	s2 := newStatusScreen(t, 40)
	plain.Draw(s2, 0, 40)
	_, _, fileStyle, _ := s2.GetContent(0, 0)
	if promptStyle == fileStyle {
		t.Fatal("el pedido debe pintar con un estilo distinto al del nombre del archivo")
	}
}
