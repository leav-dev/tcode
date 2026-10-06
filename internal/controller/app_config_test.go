package controller

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/ext"
	"tcode/internal/view"
)

// resetConfigVars fija las vars globales de configuración de view a sus
// defaults al inicio de cada test y las restaura al final (t.Cleanup): las
// vars son GLOBALES y un test que las mute contagiaria al siguiente.
func resetConfigVars(t *testing.T) {
	t.Helper()
	view.SetIndentSize(4)
	view.SetWordWrapEnabled(true)
	view.SetExplorerWidth(24)
	view.SetActiveThemeID("")
	t.Cleanup(func() {
		view.SetIndentSize(4)
		view.SetWordWrapEnabled(true)
		view.SetExplorerWidth(24)
		view.SetActiveThemeID("")
	})
}

// tmpConfigFile remapea configFilePath a un archivo del TempDir del test y lo
// restaura al final, con el mismo patrón que themeFilePath en theme_test.go.
// Devuelve la ruta del archivo de configuración.
func tmpConfigFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := configFilePath
	configFilePath = func() string { return filepath.Join(dir, "config.json") }
	t.Cleanup(func() { configFilePath = old })
	return filepath.Join(dir, "config.json")
}

// TestCtrlPTogglesTheConfigWindow: Ctrl+P abre la ventana flotante de
// configuración (también con el workspace vacío, como el explorador), una
// tecla ajena la cierra descartando, y ninguna de las dos cierra el editor.
// La tecla original era Ctrl+, pero el terminal del usuario la intercepta.
func TestCtrlPTogglesTheConfigWindow(t *testing.T) {
	resetConfigVars(t)
	app, _ := newTestApp(t, "uno")

	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone)); quit {
		t.Fatal("Ctrl+P no debe cerrar el editor")
	}
	if !app.configActive {
		t.Fatal("Ctrl+P debe abrir la ventana de configuración")
	}

	// Una tecla ajena la cierra descartando, sin cerrar el editor.
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)); quit {
		t.Fatal("la tecla ajena con la ventana abierta no debe cerrar el editor")
	}
	if app.configActive {
		t.Fatal("una tecla ajena debe cerrar la ventana de configuración")
	}

	// Ctrl+P de nuevo la abre y Escape la cierra.
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone)); quit {
		t.Fatal("Ctrl+P no debe cerrar el editor")
	}
	if !app.configActive {
		t.Fatal("Ctrl+P debe volver a abrir la ventana de configuración")
	}
	pressed := app.handleEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if app.configActive {
		t.Fatal("Escape debe cerrar la ventana de configuración")
	}
	if pressed {
		t.Fatal("Escape consumido por la ventana no debe cerrar el editor")
	}

	// Con el workspace vacío (modo explorador) también abre: la configuración
	// existe sin buffers.
	app2 := newExplorerApp(t, t.TempDir())
	if quit := app2.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone)); quit {
		t.Fatal("Ctrl+P sin buffers no debe cerrar el editor")
	}
	if !app2.configActive {
		t.Fatal("Ctrl+P debe abrir la configuración también con el workspace vacío")
	}
}

// TestConfigChangePersists: mutar una fila de la ventana aplica el ajuste a la
// config viva del editor y escribe config.json en ~/.tcode; los demás ajustes
// quedan intactos.
func TestConfigChangePersists(t *testing.T) {
	resetConfigVars(t)
	path := tmpConfigFile(t)
	app, _ := newTestApp(t, "uno")

	// Abrir y mover: Right en "Tab size" → indent 5.
	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	if !app.configActive {
		t.Fatal("Ctrl+P debe abrir la ventana de configuración")
	}
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); quit {
		t.Fatal("Right dentro de la ventana no debe cerrar el editor")
	}
	if got := view.IndentSize(); got != 5 {
		t.Fatalf("IndentSize() = %d tras Right, se esperaba 5", got)
	}
	if !app.configActive {
		t.Fatal("mutar una fila no debe cerrar la ventana de configuración")
	}
	if !view.WordWrapEnabled() || view.ExplorerWidth() != 24 {
		t.Fatal("mutar Tab size no debe tocar los demás ajustes")
	}

	// El cambio se persiste en config.json.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config.json debe escribirse al mutar: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer config.json: %v", err)
	}
	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config.json no parsea: %v", err)
	}
	if cfg.IndentUnit != 5 {
		t.Fatalf("IndentUnit en disco = %d, se esperaba 5", cfg.IndentUnit)
	}
	if cfg.WordWrap == nil || *cfg.WordWrap != true {
		t.Fatal("WordWrap debe persistir como true (el valor actual)")
	}
	if cfg.ExplorerWidth != 24 {
		t.Fatalf("ExplorerWidth en disco = %d, se esperaba 24", cfg.ExplorerWidth)
	}
}

// TestLoadConfigAppliesAtStartup: un config.json existente se aplica al
// arrancar (antes de que se use cualquier vista); un JSON roto no rompe el
// arranque y todo queda en los defaults.
func TestLoadConfigAppliesAtStartup(t *testing.T) {
	resetConfigVars(t)
	path := tmpConfigFile(t)

	if err := os.WriteFile(path, []byte(`{"IndentUnit": 8, "WordWrap": false, "ExplorerWidth": 32}`), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _ := newTestApp(t, "uno")
	if got := view.IndentSize(); got != 8 {
		t.Fatalf("IndentSize() = %d al arrancar con config, se esperaba 8", got)
	}
	if view.WordWrapEnabled() {
		t.Fatal("WordWrap debe arrancar en false (config aplicada)")
	}
	if got := view.ExplorerWidth(); got != 32 {
		t.Fatalf("ExplorerWidth() = %d al arrancar con config, se esperaba 32", got)
	}
	_ = app

	// JSON roto: el arranque sigue y quedan los defaults (los globals se
	// restauran a mano porque el JSON corrupto no los toca).
	if err := os.WriteFile(path, []byte("{ roto"), 0o644); err != nil {
		t.Fatal(err)
	}
	view.SetIndentSize(4)
	view.SetWordWrapEnabled(true)
	view.SetExplorerWidth(24)
	app2, _ := newTestApp(t, "uno")
	if view.IndentSize() != 4 || !view.WordWrapEnabled() || view.ExplorerWidth() != 24 {
		t.Fatal("JSON roto: el arranque debe quedar con los defaults")
	}
	_ = app2
}

// TestConfigThemeAppliesAtStartup: un config.json con "Theme" se aplica al
// arrancar: el id queda en la var del selector y el tema aplicado al App es
// exactamente la paleta del registry (comparar con == vale: Theme solo lleva
// estilos y colores).
func TestConfigThemeAppliesAtStartup(t *testing.T) {
	resetConfigVars(t)
	path := tmpConfigFile(t)
	if err := os.WriteFile(path, []byte(`{"IndentUnit":4,"WordWrap":true,"ExplorerWidth":24,"Theme":"dracula"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _ := newTestApp(t, "uno")
	if got := view.ActiveThemeID(); got != "dracula" {
		t.Fatalf("ActiveThemeID() = %q al arrancar con config, se esperaba \"dracula\"", got)
	}
	if app.theme != view.DraculaTheme() {
		t.Fatal("el tema aplicado debe ser exactamente la paleta Dracula")
	}
}

// TestConfigThemePersistsWhenCycled: ciclar la fila Theme aplica la paleta en
// vivo y persiste el id en config.json (solo con "Theme": "light" porque el
// resto de las filas no se tocó).
func TestConfigThemePersistsWhenCycled(t *testing.T) {
	resetConfigVars(t)
	path := tmpConfigFile(t)
	app, _ := newTestApp(t, "uno")

	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	if !app.configActive {
		t.Fatal("Ctrl+P debe abrir la ventana de configuración")
	}
	// Navegar hasta la fila Theme (Down×3) y ciclar un paso.
	app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); quit {
		t.Fatal("Right dentro de la ventana no debe cerrar el editor")
	}
	if got := view.ActiveThemeID(); got != "light" {
		t.Fatalf("ActiveThemeID() = %q tras Right en Theme, se esperaba \"light\"", got)
	}
	if !app.configActive {
		t.Fatal("mutar la fila Theme no debe cerrar la ventana de configuración")
	}

	// El cambio se persiste en config.json, con el resto de los ajustes
	// intactos (los defaults de resetConfigVars).
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer config.json: %v", err)
	}
	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config.json no parsea: %v", err)
	}
	if cfg.Theme != "light" {
		t.Fatalf("Theme en disco = %q, se esperaba \"light\"", cfg.Theme)
	}
	if cfg.IndentUnit != 4 || cfg.ExplorerWidth != 24 {
		t.Fatalf("ciclar el tema no debe tocar los demás ajustes: indent=%d width=%d", cfg.IndentUnit, cfg.ExplorerWidth)
	}
}

// fakeCatalog es el fake de fetchCatalog: devuelve entradas y errores
// configurables y registra las llamadas (con mutex: lo llama una goroutine).
type fakeCatalog struct {
	mu      sync.Mutex
	entries []ext.CatalogEntry
	errs    []error
	calls   int
}

func (f *fakeCatalog) fetch(ctx context.Context, userRoot string) ([]ext.CatalogEntry, []error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.entries, f.errs
}

// fakeInstall es el fake de installExtension: registra los subdirs pedidos y
// devuelve un error configurable.
type fakeInstall struct {
	mu      sync.Mutex
	subdirs []string
	err     error
}

func (f *fakeInstall) install(url, subdir, userRoot string, cloner ext.CloneFunc) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subdirs = append(f.subdirs, subdir)
	return subdir, f.err
}

// swapCatalogFakes sustituye fetchCatalog e installExtension por los fakes
// dados (nil deja el original) y los restaura al final. Se llama ANTES de
// abrir el panel: la goroutine de consulta usa el fake.
func swapCatalogFakes(t *testing.T, cat *fakeCatalog, inst *fakeInstall) {
	t.Helper()
	oldFetch, oldInstall := fetchCatalog, installExtension
	if cat != nil {
		fetchCatalog = cat.fetch
	}
	if inst != nil {
		installExtension = inst.install
	}
	t.Cleanup(func() {
		fetchCatalog, installExtension = oldFetch, oldInstall
	})
}

// openExtensionsPanelFromMenu abre el panel desde la ventana de configuración
// (Ctrl+P → Down ×4 → Enter) y drena la consulta del catálogo.
func openExtensionsPanelFromMenu(t *testing.T, app *App) {
	t.Helper()
	app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone))
	if !app.configActive {
		t.Fatal("Ctrl+P debe abrir la ventana de configuración")
	}
	for range 4 {
		app.handleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter en Extensions no debe cerrar el editor")
	}
	if !app.extPanelActive {
		t.Fatal("Enter en Extensions debe abrir el panel")
	}
	drainCatalog(t, app)
}

// drainCatalog espera el EventInterrupt que posteó la goroutine de consulta y
// lo procesa (drena catalogCh y deposita el resultado en el panel). PollEvent
// bloquea hasta que la goroutine postee: el envío a catalogCh ocurre ANTES del
// post (orden del programa en la goroutine), así que al volver del Poll el
// resultado ya está en el canal — sin carrera.
func drainCatalog(t *testing.T, app *App) {
	t.Helper()
	evCh := make(chan tcell.Event, 1)
	go func() {
		evCh <- app.screen.PollEvent()
	}()
	var ev tcell.Event
	select {
	case ev = <-evCh:
	case <-time.After(5 * time.Second):
		t.Fatal("la goroutine de consulta no posteó el EventInterrupt a tiempo")
	}
	if ev == nil {
		t.Fatal("PollEvent devolvió nil: la goroutine no posteó el evento")
	}
	if _, ok := ev.(*tcell.EventInterrupt); !ok {
		t.Fatalf("PollEvent devolvió %T, se esperaba *tcell.EventInterrupt", ev)
	}
	app.handleEvent(ev)
}

// TestCtrlPOpensTheExtensionsPanel: Enter sobre la fila Extensions de la
// ventana de configuración abre el panel (extPanelActive, configActive false),
// la consulta del catálogo pobló la lista, y Escape sobre el panel lo cierra
// y vuelve a la ventana de configuración.
func TestCtrlPOpensTheExtensionsPanel(t *testing.T) {
	resetConfigVars(t)
	app, _ := newTestApp(t, "uno")

	cat := &fakeCatalog{entries: []ext.CatalogEntry{
		{ID: "alpha", Name: "Alpha", Version: "1.0.0", Subdir: "alpha"},
	}}
	swapCatalogFakes(t, cat, nil)

	openExtensionsPanelFromMenu(t, app)
	if app.configActive {
		t.Fatal("el panel abierto debe cerrar la ventana de configuración")
	}

	// La consulta del catálogo pobló la lista del panel.
	cat.mu.Lock()
	calls := cat.calls
	cat.mu.Unlock()
	if calls != 1 {
		t.Fatalf("fetchCatalog llamado %d veces, se esperaba 1", calls)
	}
	if got := len(app.extPanel.Entries()); got != 1 {
		t.Fatalf("el panel tiene %d entradas, se esperaba 1", got)
	}

	// Escape sobre el panel: cierra y vuelve a la ventana de configuración.
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); quit {
		t.Fatal("Escape en el panel no debe cerrar el editor")
	}
	if app.extPanelActive {
		t.Fatal("Escape debe cerrar el panel de extensiones")
	}
	if !app.configActive {
		t.Fatal("Escape en el panel debe volver a la ventana de configuración")
	}
}

// TestExtensionsPanelInstallsSelected: Espacio marca una extensión, Enter en
// el item final (bajar con End) instala la marcada —el fake de install
// registra la llamada con el subdir correcto— y tras instalar la entrada queda
// Installed.
func TestExtensionsPanelInstallsSelected(t *testing.T) {
	resetConfigVars(t)
	app, _ := newTestApp(t, "uno")

	cat := &fakeCatalog{entries: []ext.CatalogEntry{
		{ID: "alpha", Name: "Alpha", Version: "1.0.0", Subdir: "alpha"},
		{ID: "beta", Name: "Beta", Version: "2.0.0", Subdir: "beta"},
	}}
	inst := &fakeInstall{}
	swapCatalogFakes(t, cat, inst)

	openExtensionsPanelFromMenu(t, app)
	if got := len(app.extPanel.Entries()); got != 2 {
		t.Fatalf("el panel tiene %d entradas, se esperaban 2", got)
	}

	// Espacio marca la primera (alpha).
	app.extPanel.HandleEvent(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	// End: cursor al item final.
	app.extPanel.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	// Enter en el item final: instala la marcada (alpha).
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter en el panel no debe cerrar el editor")
	}

	// El fake de install registró la llamada con el subdir correcto.
	inst.mu.Lock()
	subdirs := append([]string(nil), inst.subdirs...)
	inst.mu.Unlock()
	if len(subdirs) != 1 || subdirs[0] != "alpha" {
		t.Fatalf("installExtension llamado con subdirs=%v, se esperaba [alpha]", subdirs)
	}

	// Tras instalar, la entrada queda Installed (y la otra no).
	installed := map[string]bool{}
	for _, e := range app.extPanel.Entries() {
		installed[e.ID] = e.Installed
	}
	if !installed["alpha"] {
		t.Fatal("alpha debe quedar instalada tras instalarla")
	}
	if installed["beta"] {
		t.Fatal("beta no debe estar instalada")
	}
}

// TestConfigToggleClosesTheExtensionsPanel: con el panel abierto, Ctrl+P lo
// cierra y vuelve a la ventana de configuración (no abre nada nuevo).
func TestConfigToggleClosesTheExtensionsPanel(t *testing.T) {
	resetConfigVars(t)
	app, _ := newTestApp(t, "uno")
	swapCatalogFakes(t, &fakeCatalog{}, nil)

	openExtensionsPanelFromMenu(t, app)

	// Ctrl+P con el panel abierto: cierra el panel y vuelve a la ventana.
	if quit := app.handleEvent(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone)); quit {
		t.Fatal("Ctrl+P no debe cerrar el editor")
	}
	if app.extPanelActive {
		t.Fatal("Ctrl+P debe cerrar el panel de extensiones")
	}
	if !app.configActive {
		t.Fatal("Ctrl+P con el panel abierto debe volver a la ventana de configuración")
	}
}

// TestConfigWithoutThemeKeepsCustom: un config.json sin "Theme" no toca el
// selector (queda "", Custom) y el tema aplicado sigue siendo el Custom de
// theme.json —el id del config solo decide si el selector elige una paleta del
// registry, y el fallback Custom sobrevive.
func TestConfigWithoutThemeKeepsCustom(t *testing.T) {
	resetConfigVars(t)
	cpath := tmpConfigFile(t)
	if err := os.WriteFile(cpath, []byte(`{"IndentUnit":4}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Un theme.json válido para el Custom, con el mismo patrón de remapeo que
	// los tests de tema.
	dir := t.TempDir()
	old := themeFilePath
	themeFilePath = func() string { return filepath.Join(dir, "theme.json") }
	defer func() { themeFilePath = old }()
	content := []byte(`{"keyword": "#123456"}`)
	if err := os.WriteFile(filepath.Join(dir, "theme.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	app, _ := newTestApp(t, "uno")
	if got := view.ActiveThemeID(); got != "" {
		t.Fatalf("ActiveThemeID() = %q, se esperaba \"\" (Custom sin config Theme)", got)
	}
	if app.theme != view.LoadTheme(content) {
		t.Fatal("sin Theme en el config, el tema aplicado debe ser el Custom de theme.json")
	}
}

// TestConfigRegionBoundsTheFloatingWindow: configRegion dimensiona la ventana
// flotante con los topes de geometría. Con un editor sano (80x25, panel
// oculto) la ventana queda en la base de ancho (el contenido no llega a 34)
// y con todas sus filas (5 + marco = 7, menos que el tope de 10), centrada
// sobre el área del editor. Con un editor angosto o bajo, la región se recorta
// al editor: nunca más ancha ni más alta que su área.
func TestConfigRegionBoundsTheFloatingWindow(t *testing.T) {
	resetConfigVars(t)

	// Editor sano: ancho base y todas las filas, centrada.
	app, _ := newTestApp(t, "uno")
	resizeApp(app, 80, 25)
	x, y, w, h := app.configRegion()
	if w != view.ConfigMenuBaseWidth {
		t.Fatalf("ancho = %d, se esperaba la base %d (el contenido no la supera)", w, view.ConfigMenuBaseWidth)
	}
	if h != view.ConfigMenuHeight() {
		t.Fatalf("alto = %d, se esperaba %d (todas las filas, menos que el tope)", h, view.ConfigMenuHeight())
	}
	// Centrada: x reparte el sobrante del editor, y parte de la fila de pestañas.
	editorW := 80 // panel oculto: el editor ocupa todo el ancho
	editorH := 25 - statusHeight - tabBarHeight
	if wantX := (editorW - w) / 2; x != wantX {
		t.Fatalf("x = %d, se esperaba %d (centrada)", x, wantX)
	}
	if wantY := tabBarHeight + (editorH-h)/2; y != wantY {
		t.Fatalf("y = %d, se esperaba %d (centrada tras las pestañas)", y, wantY)
	}

	// Editor angosto (30 de ancho): la ventana se recorta al editor.
	resizeApp(app, 30, 25)
	_, _, w, _ = app.configRegion()
	if w != 30 {
		t.Fatalf("ancho con editor de 30 = %d, se esperaba 30 (recortado al editor)", w)
	}

	// Editor bajo (8 de alto): la ventana se recorta al área del editor.
	resizeApp(app, 80, 8)
	_, _, _, h = app.configRegion()
	if want := 8 - statusHeight - tabBarHeight; h != want {
		t.Fatalf("alto con editor de 8 filas = %d, se esperaba %d (recortado al área)", h, want)
	}
}
