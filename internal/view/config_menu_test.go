package view

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// resetConfigDefaults fija las vars globales de configuración del paquete a
// sus defaults y las restaura al final (t.Cleanup): las vars son GLOBALES de
// la sesión de tests y un test que las mute contagiaria al siguiente.
func resetConfigDefaults(t *testing.T) {
	t.Helper()
	SetIndentSize(4)
	SetWordWrapEnabled(true)
	SetExplorerWidth(24)
	SetActiveThemeID("")
	t.Cleanup(func() {
		SetIndentSize(4)
		SetWordWrapEnabled(true)
		SetExplorerWidth(24)
		SetActiveThemeID("")
	})
}

// drawConfigMenu dibuja la ventana y hace flush al front buffer: GetContents()
// lee el front, así que Show() es obligatorio.
func drawConfigMenu(m *ConfigMenu, s tcell.SimulationScreen) {
	m.Draw(s)
	s.Show()
}

// TestConfigMenuDrawsTheFloatingWindow: la ventana dibuja su marco con el
// borde superior (con el título centrado) y el inferior, la fila del cursor va
// con la barra de selección (TreeCursor) a todo el ancho interior y cada fila
// muestra su etiqueta con el valor alineado a la derecha: el entero de Tab
// size, "on"/"off" del Word wrap, el entero de Panel width y el nombre del
// tema del selector (con defaults, "Custom").
func TestConfigMenuDrawsTheFloatingWindow(t *testing.T) {
	resetConfigDefaults(t)
	m := NewConfigMenu()
	m.Resize(30, 6)

	s := newTestScreen(t, 30, 6)
	drawConfigMenu(m, s)

	// Marco: ┌ en (0,0) y └ en (0,5), con las esquinas derechas y la pared
	// lateral.
	if got := cellRuneAt(s, 0, 0); got != '┌' {
		t.Fatalf("(0,0) = %q, se esperaba la esquina del marco '┌'", got)
	}
	if got := cellRuneAt(s, 29, 0); got != '┐' {
		t.Fatalf("(29,0) = %q, se esperaba la esquina del marco '┐'", got)
	}
	if got := cellRuneAt(s, 0, 5); got != '└' {
		t.Fatalf("(0,5) = %q, se esperaba la esquina del marco '└'", got)
	}
	if got := cellRuneAt(s, 29, 5); got != '┘' {
		t.Fatalf("(29,5) = %q, se esperaba la esquina del marco '┘'", got)
	}
	if got := cellRuneAt(s, 0, 1); got != '│' {
		t.Fatalf("(0,1) = %q, se esperaba la pared lateral del marco '│'", got)
	}

	// El título va centrado sobre el borde superior.
	if line := screenLines(s)[0]; !strings.Contains(line, "Configuración") {
		t.Fatalf("borde superior = %q, debe contener el título centrado %q", line, "Configuración")
	}

	// La fila del cursor (0) va con la barra de selección a TODO el ancho
	// interior: el fondo explícito TreeCursor al que el texto se escribe encima.
	for x := 1; x < 29; x++ {
		if bg := cellBg(s, x, 1); bg != tcell.PaletteColor(24) {
			t.Fatalf("fondo de la fila del cursor en x=%d = %v, se esperaba %v (TreeCursor)", x, bg, tcell.PaletteColor(24))
		}
	}
	// Las demás filas van con el fondo del TEMA (th.Text lleva el fondo del
	// documento, ya no el de la terminal): sin la barra, pero pintadas.
	docBg := tcell.NewHexColor(0x1E1E1E) // el fondo del tema Dark+ por defecto
	for _, pos := range [][2]int{{5, 2}, {5, 3}, {5, 4}} {
		if bg := cellBg(s, pos[0], pos[1]); bg != docBg {
			t.Fatalf("las filas sin cursor deben llevar el fondo del tema: fondo en (%d,%d) = %v, esperaba %v", pos[0], pos[1], bg, docBg)
		}
	}

	// Cada fila: etiqueta a la izquierda y valor alineado a la derecha contra
	// la pared (el valor termina justo antes del │ derecho).
	lines := screenLines(s)
	want := []struct {
		label string
		val   string
	}{
		{"Tab size", "4"},
		{"Word wrap", "on"},
		{"Panel width", "24"},
		{"Theme", "Custom"},
	}
	for i, w := range want {
		line := lines[i+1]
		if !strings.Contains(line, w.label) {
			t.Fatalf("fila %d = %q, debe contener la etiqueta %q", i+1, line, w.label)
		}
		if !strings.HasSuffix(line, w.val+"│") {
			t.Fatalf("fila %d = %q, el valor %q debe quedar alineado a la derecha antes del marco", i+1, line, w.val)
		}
	}
}

// TestConfigMenuNavigatesAndMutates: Left/Right mutan la fila del cursor con
// su paso (el entero se clampea al rango; el booleano alterna) devolviendo
// (true, true), Up/Down/Home/End navegan sin cambiar nada devolviendo
// (true, false), y Enter alterna el booleano.
func TestConfigMenuNavigatesAndMutates(t *testing.T) {
	resetConfigDefaults(t)
	m := NewConfigMenu()
	m.Resize(30, 5)

	// Right en "Tab size": 4 → 5, mutación (true, true).
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); !handled || !changed {
		t.Fatalf("Right devolvió (handled=%v, changed=%v), se esperaba (true, true)", handled, changed)
	}
	if got := IndentSize(); got != 5 {
		t.Fatalf("IndentSize() = %d tras Right, se esperaba 5", got)
	}

	// Left hasta el mínimo: 5 → 1 y el clamp la deja en 1.
	for range 5 {
		m.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	}
	if got := IndentSize(); got != 1 {
		t.Fatalf("IndentSize() = %d tras los Left, se esperaba 1 (clamp al mínimo)", got)
	}
	// En el mínimo, Left ya no cambia: (true, false), pero sigue siendo del menú.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); !handled || changed {
		t.Fatalf("Left en el mínimo devolvió (handled=%v, changed=%v), se esperaba (true, false)", handled, changed)
	}
	if got := IndentSize(); got != 1 {
		t.Fatalf("IndentSize() quedó %d, no puede bajar del mínimo 1", got)
	}

	// Down: navegación sin cambio de valor.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)); !handled || changed {
		t.Fatalf("Down devolvió (handled=%v, changed=%v), se esperaba (true, false)", handled, changed)
	}
	// Down de nuevo: el cursor llega a "Panel width".
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)); !handled || changed {
		t.Fatalf("Down devolvió (handled=%v, changed=%v), se esperaba (true, false)", handled, changed)
	}

	// Right en "Panel width": 24 → 26 (paso 2).
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); !handled || !changed {
		t.Fatalf("Right en Panel width devolvió (handled=%v, changed=%v), se esperaba (true, true)", handled, changed)
	}
	if got := ExplorerWidth(); got != 26 {
		t.Fatalf("ExplorerWidth() = %d tras Right, se esperaba 26", got)
	}
	// Left: 26 → 24.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); !handled || !changed {
		t.Fatalf("Left en Panel width devolvió (handled=%v, changed=%v), se esperaba (true, true)", handled, changed)
	}
	if got := ExplorerWidth(); got != 24 {
		t.Fatalf("ExplorerWidth() = %d tras Left, se esperaba 24", got)
	}

	// Up hasta "Word wrap".
	m.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	// Enter alterna el booleano: on → off.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); !handled || !changed {
		t.Fatalf("Enter en Word wrap devolvió (handled=%v, changed=%v), se esperaba (true, true)", handled, changed)
	}
	if WordWrapEnabled() {
		t.Fatal("Enter debe apagar el salto de palabra (alternar el booleano)")
	}
	// Enter de nuevo: off → on.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyLF, 0, tcell.ModNone)); !handled || !changed {
		t.Fatalf("KeyLF en Word wrap devolvió (handled=%v, changed=%v), se esperaba (true, true)", handled, changed)
	}
	if !WordWrapEnabled() {
		t.Fatal("KeyLF debe volver a encender el salto de palabra")
	}
}

// TestConfigMenuClosesOnForeignKeys: Escape, Ctrl+C y cualquier tecla ajena NO
// los maneja la ventana: devuelven (false, false) para que el controlador la
// cierre descartando, y nada se muta.
func TestConfigMenuClosesOnForeignKeys(t *testing.T) {
	resetConfigDefaults(t)
	m := NewConfigMenu()
	m.Resize(30, 5)

	for _, ev := range []tcell.Event{
		tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone),
	} {
		if handled, changed := m.HandleEvent(ev); handled || changed {
			t.Fatalf("%v devolvió (handled=%v, changed=%v), se esperaba (false, false)", ev, handled, changed)
		}
	}
	if IndentSize() != 4 || !WordWrapEnabled() || ExplorerWidth() != 24 {
		t.Fatal("las teclas ajenas no deben mutar ninguna configuración")
	}
}

// TestConfigMenuClampsValues: los enteros se clampean a su rango: Tab size a
// [1, 8] y Panel width a [16, 48] con el paso 2, y en el borde la tecla se
// consume sin cambiar el valor. Los setters defienden su invariante: el indent
// nunca baja de 1 espacio.
func TestConfigMenuClampsValues(t *testing.T) {
	resetConfigDefaults(t)
	m := NewConfigMenu()
	m.Resize(30, 5)

	// Tab size: Right hasta el máximo 8 y se queda.
	for range 5 {
		m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	if got := IndentSize(); got != 8 {
		t.Fatalf("IndentSize() = %d tras los Rights, se esperaba 8 (clamp al máximo)", got)
	}
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); !handled || changed {
		t.Fatalf("Right en el máximo devolvió (handled=%v, changed=%v), se esperaba (true, false)", handled, changed)
	}
	if got := IndentSize(); got != 8 {
		t.Fatalf("IndentSize() quedó %d, no puede pasar del máximo 8", got)
	}

	// Tab size: Left hasta el mínimo 1 y se queda.
	for range 8 {
		m.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	}
	if got := IndentSize(); got != 1 {
		t.Fatalf("IndentSize() = %d tras los Left, se esperaba 1", got)
	}

	// Bajar hasta "Panel width".
	m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	// Panel width: paso 2. 24 → 48 son 12 Rights, el 13° se clampa.
	for range 13 {
		m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	if got := ExplorerWidth(); got != 48 {
		t.Fatalf("ExplorerWidth() = %d tras los Rights, se esperaba 48 (clamp al máximo)", got)
	}
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); !handled || changed {
		t.Fatalf("Right en el máximo devolvió (handled=%v, changed=%v), se esperaba (true, false)", handled, changed)
	}
	if got := ExplorerWidth(); got != 48 {
		t.Fatalf("ExplorerWidth() quedó %d, no puede pasar del máximo 48", got)
	}

	// Setter directo: el indent mínimo es 1 (un tab de 0 espacios no existe).
	SetIndentSize(0)
	if got := IndentSize(); got != 1 {
		t.Fatalf("SetIndentSize(0) dejó IndentSize() = %d, se esperaba 1", got)
	}
}

// TestConfigMenuThemeCyclesThroughPalettes: la fila Theme recorre el registry
// con Right y el ciclo hace wrap: al salir por "Custom" (que limpia el id
// activo) se vuelve a "light". El valor dibujado de la fila muestra el nombre
// de la paleta, no un número.
func TestConfigMenuThemeCyclesThroughPalettes(t *testing.T) {
	resetConfigDefaults(t)
	m := NewConfigMenu()
	m.Resize(30, 6)

	if got := ActiveThemeID(); got != "" {
		t.Fatalf("con defaults el tema activo debe ser \"\" (Custom), got %q", got)
	}
	// Navegar hasta la fila Theme (índice 3).
	for range 3 {
		m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	// Right desde "Custom" (cola del ciclo): wrap hasta "light".
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); !handled || !changed {
		t.Fatalf("Right en Theme devolvió (handled=%v, changed=%v), se esperaba (true, true)", handled, changed)
	}
	if got := ActiveThemeID(); got != "light" {
		t.Fatalf("ActiveThemeID() = %q tras Right, se esperaba \"light\"", got)
	}

	// Opcional según el spec: el valor dibujado muestra el nombre ("Light").
	s := newTestScreen(t, 30, 6)
	drawConfigMenu(m, s)
	if line := screenLines(s)[4]; !strings.Contains(line, "Light") {
		t.Fatalf("fila Theme = %q, debe mostrar el nombre \"Light\"", line)
	}

	// Right seguido recorre el resto del registry hasta "dracula"...
	for _, id := range []string{"dark", "light-hc", "dark-hc", "tokyo-night", "dracula"} {
		m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
		if got := ActiveThemeID(); got != id {
			t.Fatalf("ActiveThemeID() = %q tras el ciclo, se esperaba %q", got, id)
		}
	}
	// ...y el siguiente Right entra por el otro extremo: "Custom" limpia el id.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); !handled || !changed {
		t.Fatalf("Right hacia \"Custom\" devolvió (handled=%v, changed=%v), se esperaba (true, true)", handled, changed)
	}
	if got := ActiveThemeID(); got != "" {
		t.Fatalf("ActiveThemeID() = %q al volver a Custom, se esperaba \"\"", got)
	}
}

// TestConfigMenuThemeCustomClearsTheID: Left desde "Custom" desanda el ciclo
// (Left y Right son reversibles): "" → dracula → tokyo-night, y el ciclo
// cierra por ambos extremos (Right desde Custom vuelve a light; Left desde
// light vuelve a Custom).
func TestConfigMenuThemeCustomClearsTheID(t *testing.T) {
	resetConfigDefaults(t)
	m := NewConfigMenu()
	m.Resize(30, 6)

	for range 3 {
		m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	// Desde "" (Custom, índice 6): Left → dracula (índice 5).
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); !handled || !changed {
		t.Fatalf("Left en Theme devolvió (handled=%v, changed=%v), se esperaba (true, true)", handled, changed)
	}
	if got := ActiveThemeID(); got != "dracula" {
		t.Fatalf("ActiveThemeID() = %q tras Left, se esperaba \"dracula\"", got)
	}
	// Left de nuevo → tokyo-night.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); !handled || !changed {
		t.Fatalf("el segundo Left devolvió (handled=%v, changed=%v), se esperaba (true, true)", handled, changed)
	}
	if got := ActiveThemeID(); got != "tokyo-night" {
		t.Fatalf("ActiveThemeID() = %q tras el segundo Left, se esperaba \"tokyo-night\"", got)
	}
	// Reversibilidad: Right devuelve a dracula.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); !handled || !changed {
		t.Fatalf("Right tras los Left devolvió (handled=%v, changed=%v), se esperaba (true, true)", handled, changed)
	}
	if got := ActiveThemeID(); got != "dracula" {
		t.Fatalf("ActiveThemeID() = %q tras Right, se esperaba \"dracula\"", got)
	}
	// Enter sobre el enum no hace nada (delta 0), como en los enteros, pero la
	// tecla sigue siendo de la ventana: (true, false) y el id no cambia.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); !handled || changed {
		t.Fatalf("Enter en Theme devolvió (handled=%v, changed=%v), se esperaba (true, false)", handled, changed)
	}
	if got := ActiveThemeID(); got != "dracula" {
		t.Fatalf("Enter debe dejar el tema intacto: ActiveThemeID() = %q, se esperaba \"dracula\"", got)
	}
}

// TestConfigMenuActionRowTriggers: la última fila (Extensiones) no muta un valor:
// Left/Right no hacen nada y Enter DISPARA la acción. Como HandleEvent solo
// devuelve (handled, changed), la acción queda en un campo interno y el
// controlador la lee con Activated(), que la devuelve y la limpia (se dispara
// una vez por pulsación).
func TestConfigMenuActionRowTriggers(t *testing.T) {
	resetConfigDefaults(t)
	m := NewConfigMenu()
	m.Resize(34, ConfigMenuHeight())

	// Bajar hasta la fila de acción (la última).
	items := configItems()
	for range len(items) - 1 {
		if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)); !handled || changed {
			t.Fatalf("Down devolvió (handled=%v, changed=%v), se esperaba (true, false)", handled, changed)
		}
	}

	// Left/Right en la fila de acción: la ventana las consume pero no mutan
	// nada, así que no hay acción pendiente.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); !handled || changed {
		t.Fatalf("Right en la fila de acción devolvió (handled=%v, changed=%v), se esperaba (true, false)", handled, changed)
	}
	if got := m.Activated(); got != "" {
		t.Fatalf("Activated() = %q antes de Enter, se esperaba \"\"", got)
	}

	// Enter dispara la acción y devuelve (true, false): la ventana NO se cierra
	// (cayó en el controlador), solo queda la acción pendiente.
	if handled, changed := m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); !handled || changed {
		t.Fatalf("Enter en la fila de acción devolvió (handled=%v, changed=%v), se esperaba (true, false)", handled, changed)
	}
	if got := m.Activated(); got != "extensions" {
		t.Fatalf("Activated() = %q tras Enter, se esperaba \"extensions\"", got)
	}
	// La acción se limpia al leerla: un Enter no arrastra la apertura.
	if got := m.Activated(); got != "" {
		t.Fatalf("Activated() = %q en la segunda lectura, se esperaba \"\" (se limpia)", got)
	}

	// En las filas de valor, Enter NO dispara ninguna acción.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if got := m.Activated(); got != "" {
		t.Fatalf("Enter en una fila de valor dispara %q, no debería disparar nada", got)
	}
}

// TestConfigMenuHasTheExtensionsRow: la ventana expone la fila Extensiones al
// final (con su altura contada) y la dibuja con la etiqueta y el valor de la
// acción.
func TestConfigMenuHasTheExtensionsRow(t *testing.T) {
	resetConfigDefaults(t)
	items := configItems()
	last := items[len(items)-1]
	if last.kind != ConfigAction {
		t.Fatalf("la última fila es de tipo %v, se esperaba ConfigAction", last.kind)
	}
	if last.label != "Extensiones" {
		t.Fatalf("la última fila es %q, se esperaba \"Extensiones\"", last.label)
	}
	if got := ConfigMenuHeight(); got != len(items)+2 {
		t.Fatalf("ConfigMenuHeight() = %d, se esperaba %d (marco + filas)", got, len(items)+2)
	}

	m := NewConfigMenu()
	m.Resize(34, ConfigMenuHeight())
	s := newTestScreen(t, 34, ConfigMenuHeight())
	drawConfigMenu(m, s)
	// La fila se dibuja en su línea: la última del interior.
	if line := screenLines(s)[ConfigMenuHeight()-2]; !strings.Contains(line, "Extensiones") {
		t.Fatalf("la última fila = %q, debe contener la etiqueta %q", line, "Extensiones")
	}
}
