package view

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// extManagerFixture devuelve un manager cargado con una fila por pestaña
// (más la de agregar proveedor) y dimensionado con las filas visibles que
// dejan las cuatro pestañas en pantalla.
func extManagerFixture(t *testing.T) *ExtManager {
	t.Helper()
	m := NewExtManager()
	m.SetItems(ExtTabInstalled, []ExtItem{
		{Kind: ExtItemRemove, Label: "Linter", Right: "v1.0.0", ID: "tcode.linter", Provider: "remoto", Ref: "remoto/tcode.linter"},
	})
	m.SetItems(ExtTabUpdatable, []ExtItem{
		{Kind: ExtItemUpdate, Label: "remoto/tcode.linter", Right: "1.0.0 → 1.1.0", Ref: "remoto/tcode.linter"},
	})
	m.SetItems(ExtTabAvailable, []ExtItem{
		{Kind: ExtItemInstall, Label: "Tema", Right: "v2.0.0", ID: "tcode.tema", Provider: "remoto", Ref: "remoto/tcode.tema"},
	})
	m.SetItems(ExtTabProviders, []ExtItem{
		{Kind: ExtItemInfo, Label: "remoto", Right: "(sin aprobar)"},
		{Kind: ExtItemAddProvider, Label: "+ Agregar proveedor"},
	})
	m.Resize(80, ExtManagerHeight())
	return m
}

// TestExtManagerSwitchesTabsWithLeftAndRight: Left/Right cambian de pestaña
// (con wrap en los extremos) y cada una RECUERDA su cursor: volver a la primera
// pestaña devuelve el cursor donde estaba. Up/Down mueven el cursor de la
// pestaña activa, no del global.
func TestExtManagerSwitchesTabsWithLeftAndRight(t *testing.T) {
	m := extManagerFixture(t)
	if got := m.Tab(); got != ExtTabInstalled {
		t.Fatalf("la pestaña inicial es %v, se esperaba ExtTabInstalled", got)
	}

	// Right: de Instaladas a Actualizables.
	if handled, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); !handled || intent.Kind != ExtIntentNone {
		t.Fatalf("Right devolvió (handled=%v, intent=%v), se esperaba (true, ExtIntentNone)", handled, intent.Kind)
	}
	if got := m.Tab(); got != ExtTabUpdatable {
		t.Fatalf("tras Right la pestaña es %v, se esperaba ExtTabUpdatable", got)
	}

	// Right otra vez: Disponibles, y Enter sobre la fila propone instalar.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if got := m.Tab(); got != ExtTabAvailable {
		t.Fatalf("tras el segundo Right la pestaña es %v, se esperaba ExtTabAvailable", got)
	}
	handled, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !handled || intent.Kind != ExtIntentAction {
		t.Fatalf("Enter en Disponibles devolvió (handled=%v, intent=%v), se esperaba (true, ExtIntentAction)", handled, intent.Kind)
	}
	if intent.Item.Kind != ExtItemInstall {
		t.Fatalf("la intención de la fila de Disponibles es de tipo %v, se esperaba ExtItemInstall", intent.Item.Kind)
	}
	if intent.Item.Ref != "remoto/tcode.tema" {
		t.Fatalf("la intención lleva la referencia %q, se esperaba \"remoto/tcode.tema\"", intent.Item.Ref)
	}

	// Right hasta Proveedores: la última fila es la de agregar proveedor.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if got := m.Tab(); got != ExtTabProviders {
		t.Fatalf("tras el tercer Right la pestaña es %v, se esperaba ExtTabProviders", got)
	}
	if items := m.Items(ExtTabProviders); len(items) != 2 {
		t.Fatalf("la pestaña de proveedores tiene %d filas, se esperaban 2", len(items))
	}
	// Sobre la fila informativa del proveedor, Enter NO propone acción.
	if handled, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); !handled || intent.Kind != ExtIntentNone {
		t.Fatalf("Enter sobre un proveedor devolvió (handled=%v, intent=%v), se esperaba (true, ExtIntentNone)", handled, intent.Kind)
	}
	// Down y Enter sobre "+ Agregar proveedor": la intención es agregar.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	handled, intent = m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !handled || intent.Kind != ExtIntentAddProvider {
		t.Fatalf("Enter sobre \"Agregar proveedor\" devolvió (handled=%v, intent=%v), se esperaba (true, ExtIntentAddProvider)", handled, intent.Kind)
	}

	// Cada pestaña RECUERDA su cursor: Installadas vuelve a la fila 0.
	for range 3 {
		m.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	}
	if got := m.Tab(); got != ExtTabInstalled {
		t.Fatalf("tras los Left la pestaña es %v, se esperaba ExtTabInstalled (wrap)", got)
	}
	if got := m.Cursor(); got != 0 {
		t.Fatalf("el cursor de Instaladas quedó en %d, se esperaba 0 (cursor por pestaña)", got)
	}
}

// TestExtManagerEnterActsPerRow: Enter actúa según la fila de la pestaña:
// en Instaladas propone borrar (ExtItemRemove) y en Actualizables actualizar
// (ExtItemUpdate). Nunca se confunden los dos.
func TestExtManagerEnterActsPerRow(t *testing.T) {
	m := extManagerFixture(t)

	handled, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !handled || intent.Kind != ExtIntentAction {
		t.Fatalf("Enter en Instaladas devolvió (handled=%v, intent=%v), se esperaba (true, ExtIntentAction)", handled, intent.Kind)
	}
	if intent.Item.Kind != ExtItemRemove {
		t.Fatalf("Enter en Instaladas propone %v, se esperaba ExtItemRemove", intent.Item.Kind)
	}
	if intent.Tab != ExtTabInstalled {
		t.Fatalf("la intención dice la pestaña %v, se esperaba ExtTabInstalled", intent.Tab)
	}

	m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	handled, intent = m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !handled || intent.Kind != ExtIntentAction {
		t.Fatalf("Enter en Actualizables devolvió (handled=%v, intent=%v), se esperaba (true, ExtIntentAction)", handled, intent.Kind)
	}
	if intent.Item.Kind != ExtItemUpdate {
		t.Fatalf("Enter en Actualizables propone %v, se esperaba ExtItemUpdate", intent.Item.Kind)
	}
}

// TestExtManagerClosesOnForeignKeys: Escape, Ctrl+C y cualquier tecla ajena NO
// las maneja la ventana: devuelven (false, ExtIntent{}) para que el controlador
// la cierre descartando.
func TestExtManagerClosesOnForeignKeys(t *testing.T) {
	m := extManagerFixture(t)
	for _, ev := range []tcell.Event{
		tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone),
	} {
		if handled, intent := m.HandleEvent(ev); handled || intent.Kind != ExtIntentNone {
			t.Fatalf("%v devolvió (handled=%v, intent=%v), se esperaba (false, ExtIntentNone)", ev, handled, intent.Kind)
		}
	}
}

// TestExtManagerScrollsOnlyAsNeeded: con más filas que el alto, top se corre lo
// MÍNIMO para que la fila del cursor se vea: al bajar al final la última fila
// queda pegada al borde inferior del interior, y al volver arriba el scroll
// vuelve al principio.
func TestExtManagerScrollsOnlyAsNeeded(t *testing.T) {
	m := NewExtManager()
	var items []ExtItem
	for i := range 20 {
		items = append(items, ExtItem{Kind: ExtItemRemove, Label: "ext", Right: string(rune('0' + i))})
	}
	m.SetItems(ExtTabInstalled, items)
	m.Resize(40, 8) // 6 filas visibles

	// Con el cursor arriba, top es 0 y se ve la primera fila.
	if got := m.topFor(ExtTabInstalled); got != 0 {
		t.Fatalf("top inicial = %d, se esperaba 0", got)
	}
	// PageDown avanza una página completa (lo que cabe en el interior).
	if handled, _ := m.HandleEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)); !handled {
		t.Fatal("PageDown debe ser de la ventana")
	}
	if got := m.Cursor(); got != 6 {
		t.Fatalf("tras PageDown el cursor quedó en %d, se esperaba 6 (la página)", got)
	}
	// Un paso más del cursor: top se corre solo lo necesario (cursor = 7 con 6
	// visibles → top = 2, la fila pegada al borde inferior).
	m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if got := m.topFor(ExtTabInstalled); got != 2 {
		t.Fatalf("top = %d con el cursor en 7 y 6 filas visibles, se esperaba 2", got)
	}
	// End: la última fila queda visible (top = 14, la última de las 20).
	m.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if got := m.Cursor(); got != 19 {
		t.Fatalf("tras End el cursor quedó en %d, se esperaba 19", got)
	}
	if got := m.topFor(ExtTabInstalled); got != 14 {
		t.Fatalf("top = %d tras End, se esperaba 14", got)
	}
	// Home: el scroll vuelve al principio.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if got := m.topFor(ExtTabInstalled); got != 0 {
		t.Fatalf("top = %d tras Home, se esperaba 0", got)
	}
}

// TestExtManagerDrawsTabsAndRows: la ventana dibuja su marco con las pestañas
// en el borde superior —la activa, entre corchetes y con el estilo de pestaña
// activa— y las filas de la pestaña activa en el interior, con la del cursor
// resaltada a todo el ancho.
func TestExtManagerDrawsTabsAndRows(t *testing.T) {
	m := extManagerFixture(t)
	s := newTestScreen(t, 80, ExtManagerHeight())
	m.Draw(s)
	s.Show()

	if got := cellRuneAt(s, 0, 0); got != '┌' {
		t.Fatalf("(0,0) = %q, se esperaba la esquina del marco '┌'", got)
	}
	if got := cellRuneAt(s, 79, ExtManagerHeight()-1); got != '┘' {
		t.Fatalf("esquina inferior derecha = %q, se esperaba '┘'", got)
	}
	if got := cellRuneAt(s, 0, 1); got != '│' {
		t.Fatalf("(0,1) = %q, se esperaba la pared lateral del marco '│'", got)
	}

	// Las cuatro pestañas están en el borde superior; la activa va marcada.
	top := screenLines(s)[0]
	for _, name := range []string{"Instaladas", "Actualizables", "Disponibles", "Proveedores"} {
		if !strings.Contains(top, name) {
			t.Fatalf("borde superior = %q, debe contener la pestaña %q", top, name)
		}
	}
	if !strings.Contains(top, "[Instaladas (1)]") {
		t.Fatalf("borde superior = %q, la pestaña activa debe ir marcada entre corchetes con su conteo", top)
	}

	// La fila del cursor va con la barra de selección (TreeCursor) a todo el
	// ancho interior, y su texto muestra la etiqueta y el dato de la derecha.
	if bg := cellBg(s, 2, 1); bg != tcell.PaletteColor(24) {
		t.Fatalf("fondo de la fila del cursor = %v, se esperaba %v (TreeCursor)", bg, tcell.PaletteColor(24))
	}
	if line := screenLines(s)[1]; !strings.Contains(line, "Linter") || !strings.Contains(line, "v1.0.0") {
		t.Fatalf("fila del cursor = %q, debe mostrar la etiqueta y la versión", line)
	}

	// Cambiar de pestaña cambia lo dibujado.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	s.Show()
	m.Draw(s)
	s.Show()
	top = screenLines(s)[0]
	if !strings.Contains(top, "[Actualizables (1)]") {
		t.Fatalf("borde superior = %q tras cambiar de pestaña, se esperaba [Actualizables (1)]", top)
	}
	if line := screenLines(s)[1]; !strings.Contains(line, "1.0.0 → 1.1.0") {
		t.Fatalf("fila de Actualizables = %q, debe mostrar el salto de versión", line)
	}
}

// TestExtManagerEmptyTabShowsAPlaceholder: una pestaña sin filas no queda en
// blanco: dibuja un texto de estado ("sin …") para que la ventana explique por
// qué no hay nada.
func TestExtManagerEmptyTabShowsAPlaceholder(t *testing.T) {
	m := NewExtManager()
	m.SetItems(ExtTabInstalled, nil)
	m.SetItems(ExtTabAvailable, []ExtItem{{Kind: ExtItemInstall, Label: "Tema"}})
	m.Resize(80, ExtManagerHeight())
	s := newTestScreen(t, 80, ExtManagerHeight())
	m.Draw(s)
	s.Show()

	if line := screenLines(s)[1]; !strings.Contains(line, "sin extensiones") {
		t.Fatalf("la pestaña vacía dibuja %q, debe explicar que no hay nada instalado", line)
	}
	// Enter sobre la pestaña vacía no propone ninguna acción.
	if handled, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); !handled || intent.Kind != ExtIntentNone {
		t.Fatalf("Enter en una pestaña vacía devolvió (handled=%v, intent=%v), se esperaba (true, ExtIntentNone)", handled, intent.Kind)
	}
}

// TestExtManagerResizeReencuadra: un resize deja el cursor dentro del rango de
// filas y la fila del cursor visible, como el resto de las listas con scroll.
func TestExtManagerResizeReencuadra(t *testing.T) {
	m := NewExtManager()
	var items []ExtItem
	for range 10 {
		items = append(items, ExtItem{Kind: ExtItemRemove, Label: "ext"})
	}
	m.SetItems(ExtTabInstalled, items)
	m.Resize(40, 6)
	m.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	m.Resize(40, 30)
	if got := m.Cursor(); got != 9 {
		t.Fatalf("el cursor quedó en %d tras agrandar la ventana, se esperaba 9", got)
	}
	if got := m.topFor(ExtTabInstalled); got != 0 {
		t.Fatalf("top = %d con todo dentro, se esperaba 0", got)
	}
}

// TestExtManagerSpaceTogglesInstalledRow: Space sobre una fila de Instaladas
// propone alternar el estado de esa extensión.
// TestExtManagerRefreshesWithR: la tecla r propone relanzar la validación
// en cualquier pestaña, sin cerrar la ventana. Con Ctrl no vale: es una r
// de la ventana, no un atajo del editor.
func TestExtManagerRefreshesWithR(t *testing.T) {
	m := extManagerFixture(t)

	handled, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))
	if !handled {
		t.Fatal("r es una tecla de la ventana: no debe caer al controlador")
	}
	if intent.Kind != ExtIntentRefresh {
		t.Fatalf("intent = %v, se esperaba ExtIntentRefresh", intent.Kind)
	}

	// En otra pestaña también vale.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if handled, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone)); !handled || intent.Kind != ExtIntentRefresh {
		t.Fatalf("(handled=%v, intent=%v), r debe refrescar en cualquier pestaña", handled, intent.Kind)
	}

	// Con Ctrl es otra tecla: la ventana no la reclama como refresh.
	if _, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModCtrl)); intent.Kind == ExtIntentRefresh {
		t.Fatal("Ctrl+r no es el refresh de la ventana")
	}
}

// TestExtManagerShowsTheRefreshHint: la pista de r vive en el borde inferior
// de TODAS las pestañas —la acción no puede vivir solo en la documentación—.
func TestExtManagerShowsTheRefreshHint(t *testing.T) {
	for tab := ExtTab(0); tab < extTabCount; tab++ {
		m := extManagerFixture(t)
		for i := ExtTab(0); i < tab; i++ {
			m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
		}
		s := newTestScreen(t, 80, ExtManagerHeight())
		m.Draw(s)
		s.Show()
		if got := screenLines(s)[ExtManagerHeight()-1]; !strings.Contains(got, "r: buscar actualizaciones") {
			t.Fatalf("pestaña %v: borde inferior = %q, se esperaba la pista de r", tab, got)
		}
	}
}

func TestExtManagerSpaceTogglesInstalledRow(t *testing.T) {
	m := extManagerFixture(t)

	handled, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if !handled {
		t.Fatal("Space es una tecla de la ventana: no debe caer al controlador")
	}
	if intent.Kind != ExtIntentToggle {
		t.Fatalf("intent = %v, se esperaba ExtIntentToggle", intent.Kind)
	}
	if intent.Item.ID != "tcode.linter" {
		t.Fatalf("item = %q, se esperaba la fila del cursor", intent.Item.ID)
	}
}

// TestExtManagerSpaceIsSilentOutsideInstalled: en otra pestaña Space se consume
// (la ventana no se cierra) pero no propone nada.
func TestExtManagerSpaceIsSilentOutsideInstalled(t *testing.T) {
	m := extManagerFixture(t)
	m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)) // Actualizables

	handled, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if !handled {
		t.Fatal("Space debe ser una tecla de la ventana en cualquier pestaña")
	}
	if intent.Kind != ExtIntentNone {
		t.Fatalf("intent = %v, se esperaba ExtIntentNone fuera de Instaladas", intent.Kind)
	}
}

// TestExtManagerSpaceOnEmptyInstalledDoesNothing: sin filas que alternar, Space
// se consume sin proponer nada.
func TestExtManagerSpaceOnEmptyInstalledDoesNothing(t *testing.T) {
	m := NewExtManager()
	m.Resize(60, ExtManagerHeight())

	handled, intent := m.HandleEvent(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if !handled || intent.Kind != ExtIntentNone {
		t.Fatalf("(handled=%v, intent=%v), se esperaba consumir sin intención", handled, intent.Kind)
	}
}

// TestExtManagerDrawsTheToggleHint: la pista de la acción vive en el borde
// inferior de la pestaña Instaladas —una acción de teclado invisible es una
// acción perdida— y no aparece en las demás pestañas.
func TestExtManagerDrawsTheToggleHint(t *testing.T) {
	s := newTestScreen(t, 60, ExtManagerHeight())
	m := extManagerFixture(t)
	m.Draw(s)
	s.Show()

	bottom := screenLines(s)[ExtManagerHeight()-1]
	if !strings.Contains(bottom, "espacio: activar/desactivar") {
		t.Fatalf("borde inferior = %q, se esperaba la pista del toggle", bottom)
	}

	// En otra pestaña no hay pista de toggle: el borde vuelve a ser una línea.
	m.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	s2 := newTestScreen(t, 60, ExtManagerHeight())
	m.Draw(s2)
	s2.Show()
	if got := screenLines(s2)[ExtManagerHeight()-1]; strings.Contains(got, "espacio") {
		t.Fatalf("borde inferior = %q, no debe haber pista fuera de Instaladas", got)
	}
}
