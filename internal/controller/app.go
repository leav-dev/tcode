package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/ext"
	"tcode/internal/model"
	"tcode/internal/view"
)

// statusHeight es la cantidad de filas que ocupa la barra de estado.
// tabBarHeight es la que ocupa la fila de pestañas. El ancho máximo del panel
// lateral del explorador vive en la configuración de view (ExplorerWidth, la
// ventana (Ctrl+P) la expone y el archivo de config la persiste): el editor
// conserva el resto.
const (
	statusHeight = 1
	tabBarHeight = 1
)

// panelWidth es el ancho del panel lateral para una pantalla de ancho
// columnas, con la fórmula literal min(ExplorerWidth(), max(1, ancho-16)): el
// editor conserva al menos 16 columnas, el panel nunca desaparece (mínimo 1) y
// el valor es determinista para los tests (80 → 24, 30 → 14, 20 → 4).
func panelWidth(ancho int) int {
	return min(view.ExplorerWidth(), max(1, ancho-16))
}

type App struct {
	screen    tcell.Screen
	ws        *model.Workspace
	statusBar *view.StatusBar
	tabBar    *view.TabBar

	// editorSurf es la superficie recortada con la que se componen los dos
	// panes del redibujo: el explorador y el editor activo. Se reusa entre
	// redibujos: el controlador la reencuadra con SetRegion (primero para el
	// panel, después para el editor) cuando cambia el tamaño de la terminal o
	// la visibilidad del panel, en lugar de alocar una superficie por tecla.
	editorSurf *view.OffsetSurface

	// explorer es el panel lateral de archivos: el árbol de la raíz de la
	// sesión, anclado al root y cargado por nivel. El controlador lee los
	// directorios (os.ReadDir) y deposita los listados en la vista con
	// SetRootEntries (primer nivel, al arrancar) y SetChildren (hijos de un
	// dir al expandir); la vista solo dibuja, navega y colapsa. El root es la
	// cima implícita y nunca cambia: el panel no tiene subida ni "..".
	explorer        *view.FileBrowser
	explorerVisible bool
	explorerFocused bool

	// editors guarda una vista por buffer. La clave es el puntero del buffer:
	// el workspace ya deduplica por ruta, así que el mapa no puede desincronizarse
	// de él como podría hacerlo un slice mantenido a mano. La entrada se borra
	// cuando el buffer se cierra: la vista vieja conservaría un puntero a un
	// PieceTable ya desmapeado.
	editors map[*model.PieceTable]*view.EditorView

	lastEscape time.Time // marca del último Escape (doble presión para salir)
	confirmQuit  bool
	confirmClose bool
	// confirmReload arma la confirmación no modal de Ctrl+R sobre un buffer
	// con ediciones: la primera Ctrl+R avisa, la segunda recarga.
	confirmReload bool

	// forceSave es el permiso de pisar cambios externos, por buffer: autorizar
	// sobrescribir UN archivo no tiene que valer para otro.
	forceSave map[*model.PieceTable]bool

	// promptActive, promptBuf, promptLabel y promptAction sostienen el pedido
	// de texto de la barra de estado. Mientras está activo, el teclado alimenta
	// el pedido y no el documento. El pedido es GENERALIZADO: label dice qué se
	// está pidiendo ("Guardar como: ", "Nuevo archivo: "…), prefill es el texto
	// con el que arranca (la ruta actual en Save As, vacío al crear) y action
	// es lo que corre al confirmar con Enter —el trabajo pesado vive en el
	// closure del controlador, y lo que se necesita para él (el buffer destino
	// de Save As, el directorio destino de una creación) queda capturado al
	// ABRIR el pedido: el texto que alguien escribe pertenece a la acción que
	// empezó, no a la que esté activa cuando aprieta Enter.
	promptActive bool
	promptBuf    string
	promptLabel  string
	promptAction func(path string) error

	// menu es el superpuesto transitorio de pestañas (Ctrl+T) y menuActive dice
	// si está abierto. Mientras está activo, el menú posee el teclado y el mouse
	// —como el pedido de Save As—: Enter activa la pestaña del cursor y cierra,
	// y toda otra tecla (Escape incluido) cierra descartando.
	menu       *view.TabMenu
	menuActive bool

	// configMenu es la ventana flotante de configuración (Ctrl+P) y
	// configActive dice si está abierta. Como el menú de pestañas, mientras
	// está activa posee el teclado y el mouse: las teclas que la ventana no
	// maneja (Escape incluido) la cierran descartando, Left/Right mutan la fila
	// del cursor y Enter alterna el booleano.
	configMenu   *view.ConfigMenu
	configActive bool

	// extManager es la ventana flotante de gestión de extensiones (la fila
	// "Extensiones" de la de configuración) y extActive dice si está abierta.
	// Como las demás overlays posee el teclado mientras está activa: Enter
	// devuelve la INTENCIÓN de la fila del cursor y el controlador la ejecuta
	// con su confirmación (openPrompt) y sus datos.
	extManager *view.ExtManager
	extActive  bool

	// Los datos de la ventana se cargan AL ABRIRLA y se recargan después de
	// cada acción: userRoot es la raíz de extensiones del usuario sobre la que
	//-Corren las acciones- y las tres listas son las que llenan las pestañas.
	// La vista no lee el disco: recibe filas (view.ExtItem) ya resueltas.
	extUserRoot  string
	extInstalled []ext.Info
	extUpdates   []ext.UpdateResult
	extAvailable []ext.AvailableExt
	extProviders []ext.Provider

	// sessionEnabled marca los modos con sesión persistida (directorio o sin
	// argumentos): el modo archivo explícito no guarda ni restaura. Se define
	// en el arranque y cubre tanto el guardado como la restauración.
	sessionEnabled bool

	// ext es el sistema de extensiones: registro de comandos compartido
	// (built-ins tcode.* más stubs declarados), keybindings y bus de hooks.
	// handleEvent resuelve las teclas de extensión después de los atajos del
	// núcleo; los hooks se emiten desde open/save/close.
	ext *ext.Manager

	// theme es la paleta por rol del editor, resuelta por applyTheme desde el
	// selector (ActiveThemeID) o el Custom; se aplica a las vistas (y a cada
	// editor, también a los ya abiertos).
	theme view.Theme

	// customTheme es el tema del usuario (~/.tcode/theme.json): el fallback del
	// selector cuando el id activo es "" (Custom). loadTheme lo lee al arranque
	// y themeFor lo devuelve mientras el selector no elija una paleta del
	// registry.
	customTheme view.Theme

	// extensionRoots son los directorios donde se buscan extensiones, en orden
	// de precedencia: el primero gana en caso de ids duplicados. Por defecto,
	// las del usuario y las del proyecto actual; los tests los reemplazan.
	extensionRoots []string

	// startupCheck es la revisión de extensiones del arranque —el inyector de
	// appStartupCheck— con su resultado: lo detectado (actualizaciones y
	// novedades) y lo que se aplicó. Vive en el App para que el arranque y sus
	// tests puedan afirmar qué pasó sin volver a tocar el disco.
	startupUpdates   []ext.UpdateResult
	startupAvailable []ext.AvailableExt
	startupApplied   bool
}

// NewApp inicializa la terminal y carga el archivo indicado (si path != "").
func NewApp(path string) (*App, error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	return NewAppWithScreen(s, path)
}

// NewAppWithScreen construye la aplicación sobre una pantalla ya provista. Es lo
// que permite testear el controlador con una pantalla simulada, sin terminal.
//
// El argumento decide el modo de arranque: sin argumento (o con un directorio)
// el explorador es el camino de entrada natural —raíz en cwd o en ese
// directorio, sin buffers y con el foco en el panel— y un archivo arranca el
// editor clásico con el explorador oculto y la raíz de la sesión en su
// directorio padre. El árbol queda anclado a esa raíz (SetRoot): la base es
// fija y la navegación solo expande y colapsa, sin cambiar de carpeta.
func NewAppWithScreen(s tcell.Screen, path string) (*App, error) {
	if err := s.Init(); err != nil {
		return nil, err
	}
	s.EnableMouse()

	ws := model.NewWorkspace()
	app := &App{
		screen:     s,
		ws:         ws,
		editors:    make(map[*model.PieceTable]*view.EditorView),
		forceSave:  make(map[*model.PieceTable]bool),
		statusBar:  view.NewStatusBar(),
		tabBar:     view.NewTabBar(),
		explorer:   view.NewFileBrowser(),
		menu:       view.NewTabMenu(),
		configMenu: view.NewConfigMenu(),
		extManager: view.NewExtManager(),
		ext:        ext.NewManager(),

		// La sesión es el estado de los modos explorador; el modo archivo la
		// apaga abajo.
		sessionEnabled: true,
	}

	if path == "" {
		// Sin argumentos: el explorador sobre el directorio de trabajo. Sin
		// buffers: el estado a propósito vacío que la feature declaró en U1;
		// el explorador (o Save As) le da un documento después.
		root, err := os.Getwd()
		if err != nil {
			s.Fini()
			return nil, err
		}
		ws.SetRoot(root)
		app.explorerVisible, app.explorerFocused = true, true
	} else if info, err := os.Stat(path); err != nil {
		s.Fini()
		return nil, err
	} else if info.IsDir() {
		// Directorio como argumento: el explorador sobre ese directorio, sin
		// buffers.
		ws.SetRoot(filepath.Clean(path))
		app.explorerVisible, app.explorerFocused = true, true
	} else {
		// Archivo como argumento: el arranque clásico de edición. El panel
		// queda oculto (abrir un archivo por línea de comandos es una acción
		// de edición, no de navegación) y la raíz de la sesión es su directorio
		// padre. Es una acción puntual, no una sesión: abrir <archivo> no
		// escribe .tcode/ en el directorio padre de cualquier archivo ajeno.
		app.sessionEnabled = false
		ws.SetRoot(filepath.Dir(path))
		if _, err := ws.Open(path); err != nil {
			s.Fini()
			return nil, err
		}
		app.emitEvent(ext.EventDidOpenBuffer)
	}

	width, height := s.Size()
	app.explorer.Resize(panelWidth(width), editorHeight(height))

	// El árbol se ancla al root de la sesión y su primer nivel se lee SIEMPRE
	// —también con el panel oculto—: eso hace que mostrarlo después con Ctrl+B
	// aparezca poblado, y el panel oculto no dibuja, así que la geometría de
	// los tests existentes no cambia. Los SUBdirectorios no se leen acá: su
	// carga es perezosa, al expandir (ActionExpand → SetChildren) —leer el
	// árbol completo al arrancar cargaría proyectos enteros sin estar en
	// pantalla—.
	app.explorer.SetRoot(app.ws.Root())
	if entries, err := readEntries(app.ws.Root()); err == nil {
		app.explorer.SetRootEntries(entries)
	} else {
		// Root inaccesible (permisos o borrado ajeno): árbol vacío, como el
		// relist de U3 sobre un directorio ilegible.
		app.explorer.SetRootEntries(nil)
	}

	// Las extensiones de disco se cargan ANTES de los built-ins: sus comandos
	// declarados no pueden piser a tcode.*.
	app.extensionRoots = defaultExtensionRoots(app.ws.Root())
	// El prompt de actualizaciones y novedades va ANTES de loadExtensions: al
	// aceptar, lo nuevo queda en disco y lo carga el mismo arranque. Después no
	// se recarga —AddExtensions no es idempotente—.
	app.checkExtensionsAndPrompt()
	app.loadExtensions()
	// El manager habla con el editor a través del App: los comandos con script
	// corren Lua con la API tcode.* cableada a App (ScriptAPI).
	app.ext.SetEditor(app)

	// El tema se aplica a todas las vistas en el arranque; los editores que se
	// creen bajo demanda lo reciben en activeEditor.
	app.loadTheme()

	// La configuración persistida (~/.tcode/config.json) se aplica sobre las
	// vars del paquete view ANTES de que se use cualquier geometría: panelWidth,
	// el indent y el wrap ya quedan con lo del usuario al primer redibujo.
	app.loadConfig()

	// Los built-ins tcode.* se registran después de armar el App completo: los
	// handlers cierran sobre el App ya construido.
	app.registerBuiltins()

	// El arranque activa las extensiones que lo declaran (onStartup/*). Con el
	// registro y el keymap ya listos, los hooks de las activas participan desde
	// el primer evento de buffer.
	app.ext.ActivateEvent(ext.ActivateStartup)

	// La sesión se restaura después de que el árbol quedó listo: SetRoot y el
	// primer nivel ya corrieron, y la restauración nunca es fatal (pestañas
	// muertas se saltan, JSON corrupto se descarta).
	app.loadSession()
	app.syncStatus()
	return app, nil
}

// registerBuiltins expone las acciones existentes del controlador como
// comandos tcode.* en el registro del manager: es la primera contribución
// real que pueden invocar los keybindings y hooks de una extensión. Los que
// requieren un buffer abierto fallan con un error legible en lugar de tocar
// el documento; un registro fallido es un bug de programación, por eso pániquea.
func (a *App) registerBuiltins() {
	r := a.ext.Registry()
	register := func(id string, fn func() error) {
		if err := r.Register(id, fn); err != nil {
			panic(err)
		}
	}

	register("tcode.save", func() error {
		if _, err := a.requireBuffer(); err != nil {
			return err
		}
		a.save()
		return nil
	})
	register("tcode.saveAs", func() error {
		if _, err := a.requireBuffer(); err != nil {
			return err
		}
		a.startPrompt()
		return nil
	})
	register("tcode.closeTab", func() error {
		if _, err := a.requireBuffer(); err != nil {
			return err
		}
		a.closeTab()
		return nil
	})
	register("tcode.toggleExplorer", func() error {
		a.toggleExplorer()
		return nil
	})
	register("tcode.createFile", func() error {
		a.promptCreateEntry(false)
		return nil
	})
	register("tcode.createFolder", func() error {
		a.promptCreateEntry(true)
		return nil
	})
	register("tcode.deleteFile", func() error {
		a.promptDeleteEntry()
		return nil
	})
	register("tcode.undo", func() error {
		buf, err := a.requireBuffer()
		if err != nil {
			return err
		}
		a.applyHistory(buf.Undo, "Deshecho", "Nada que deshacer")
		return nil
	})
	register("tcode.redo", func() error {
		buf, err := a.requireBuffer()
		if err != nil {
			return err
		}
		a.applyHistory(buf.Redo, "Rehecho", "Nada que rehacer")
		return nil
	})
	register("tcode.switchTabNext", func() error {
		if _, err := a.requireBuffer(); err != nil {
			return err
		}
		a.switchTab(a.ws.Next)
		return nil
	})
	register("tcode.switchTabPrev", func() error {
		if _, err := a.requireBuffer(); err != nil {
			return err
		}
		a.switchTab(a.ws.Prev)
		return nil
	})
}

// themeFilePath resuelve el archivo de tema del usuario; es variable para que
// los tests lo apunten a un directorio temporal.
var themeFilePath = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".tcode", "theme.json")
}

// loadTheme lee ~/.tcode/theme.json al arranque como tema Custom (a.customTheme)
// y aplica la paleta resultante a todas las vistas. Si el archivo falta o el
// JSON está roto, el Custom es la default: el tema del usuario jamás rompe el
// editor.
func (a *App) loadTheme() {
	a.customTheme = view.DefaultTheme()
	if path := themeFilePath(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			a.customTheme = view.LoadTheme(data)
		}
	}
	a.applyTheme()
}

// themeFor resuelve el tema a aplicar: la paleta del registry si el selector
// tiene un id activo, o el Custom (a.customTheme) cuando el id es "".
func (a *App) themeFor() view.Theme {
	if id := view.ActiveThemeID(); id != "" {
		if th, ok := view.ThemeByID(id); ok {
			return th
		}
	}
	return a.customTheme
}

// applyTheme aplica el tema activo a TODAS las vistas, incluidos los editores
// YA abiertos: el selector en vivo (fila Theme de la ventana de configuración)
// necesita que un cambio de paleta se vea de inmediato, y hoy los editores
// solo recibían el tema al crearse (activeEditor).
func (a *App) applyTheme() {
	a.theme = a.themeFor()
	a.statusBar.SetTheme(a.theme)
	a.tabBar.SetTheme(a.theme)
	a.explorer.SetTheme(a.theme)
	a.menu.SetTheme(a.theme)
	a.configMenu.SetTheme(a.theme)
	a.extManager.SetTheme(a.theme)
	for _, ed := range a.editors {
		ed.SetTheme(a.theme)
	}
}

// quitEscapeWindow es la ventana de la doble presión de Escape: dos Escape
// dentro de este lapso cierran el editor; un segundo tardío reinicia el
// conteo. Variable para que los tests la ajusten.
var quitEscapeWindow = 500 * time.Millisecond

// clockNow es el reloj de la doble presión, inyectable para los tests: la
// ventana se mide con tiempo simulado en lugar de calcular sobre el real.
var clockNow = time.Now

// configFilePath resuelve el archivo de configuración del usuario; es variable
// para que los tests lo apunten a un directorio temporal.
var configFilePath = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".tcode", "config.json")
}

// configFile es el esquema persistido de la configuración: el tamaño de la
// tabulación, el salto de palabra (puntero: ausencia = default), el ancho
// máximo del panel lateral del explorador y el id del tema del selector ("" =
// Custom; un id desconocido se ignora al cargar). El tema de theme.json vive
// aparte: el config solo decide si el selector elige una paleta del registry.
type configFile struct {
	IndentUnit    int
	WordWrap      *bool
	ExplorerWidth int
	Theme         string
}

// loadConfig lee ~/.tcode/config.json al arranque y aplica la configuración a
// las vars del paquete view. Si el archivo falta o el JSON está roto, no hace
// nada y quedan los defaults: la configuración del usuario jamás rompe el
// editor, como el tema.
func (a *App) loadConfig() {
	if path := configFilePath(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			var cfg configFile
			if err := json.Unmarshal(data, &cfg); err != nil {
				return
			}
			if cfg.IndentUnit > 0 {
				view.SetIndentSize(cfg.IndentUnit)
			}
			if cfg.WordWrap != nil {
				view.SetWordWrapEnabled(*cfg.WordWrap)
			}
			if cfg.ExplorerWidth > 0 {
				view.SetExplorerWidth(cfg.ExplorerWidth)
			}
			// El id del tema persistido gana sobre el Custom de theme.json: el
			// config decidió que el selector elija una paleta del registry.
			if cfg.Theme != "" {
				if _, ok := view.ThemeByID(cfg.Theme); ok {
					view.SetActiveThemeID(cfg.Theme)
				}
			}
		}
	}
	// El tema del config puede diferir del aplicado por loadTheme (que solo
	// conocía el Custom): re-aplicar acá deja la paleta del id activo.
	a.applyTheme()
}

// saveConfig persiste la configuración actual de las vars del paquete view en
// ~/.tcode/config.json, incluido el id del tema activo del selector ("" =
// Custom). Un fallo de escritura no rompe la edición: se avisa en la barra de
// estado.
func (a *App) saveConfig() {
	wrap := view.WordWrapEnabled()
	cfg := configFile{
		IndentUnit:    view.IndentSize(),
		WordWrap:      &wrap,
		ExplorerWidth: view.ExplorerWidth(),
		Theme:         view.ActiveThemeID(),
	}
	data, err := json.MarshalIndent(&cfg, "", "  ")
	if err != nil {
		a.statusBar.SetMessage("No se pudo guardar la config: " + err.Error())
		return
	}
	if path := configFilePath(); path != "" {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			a.statusBar.SetMessage("No se pudo guardar la config: " + err.Error())
		}
	}
}

// defaultExtensionRoots devuelve los directorios de extensiones por defecto:
// las del proyecto (`.tcode/extensions` bajo la raíz de la sesión) y las del
// usuario (~/.tcode/extensions). El primer root gana ante ids duplicados, así
// que las del usuario preceden.
func defaultExtensionRoots(root string) []string {
	roots := []string{filepath.Join(root, ".tcode", "extensions")}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".tcode", "extensions"))
	}
	return roots
}

// extensionUserSources es la resolución de los proveedores registrados y la
// raíz de extensiones del USUARIO (~/.tcode/extensions): el alcance del chequeo
// de arranque, el mismo que cubría updateExtensionsAtStartup en main.go. Es una
// variable para que los tests apunten a un temporal, sin tocar el HOME real.
var extensionUserSources = func() ([]ext.Provider, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", err
	}
	providers, err := ext.AllProviders(ext.ProvidersFilePath(home))
	if err != nil {
		return nil, "", err
	}
	return providers, filepath.Join(home, ".tcode", "extensions"), nil
}

// startupExtFetch es el lector de los proveedores en el chequeo de arranque:
// nil usa el lector real (git). Variable por la misma razón que
// extensionUserSources: los tests corren sin git ni red. La ventana de
// extensiones lo reutiliza: listar proveedores e instalar usan el mismo lector.
var startupExtFetch ext.FetchFunc

// providersConfigPath resuelve el archivo de proveedores del usuario
// (~/.tcode/providers.json): lo usa la ventana de extensiones para AGREGAR una
// fuente. Es variable por la misma razón que extensionUserSources —los tests la
// apuntan a un temporal para no tocar el HOME real—.
var providersConfigPath = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return ext.ProvidersFilePath(home), nil
}

// appStartupCheck es la INYECCIÓN del chequeo de arranque: devuelve las
// actualizaciones disponibles y las novedades, sin tocar el disco. Vive en una
// variable para que los tests no dependan de la red ni del HOME real, y para que
// una falla de lectura sea un no-op en vez de un arranque roto.
var appStartupCheck = detectStartupExtensions

// detectStartupExtensions revisa el HOME del usuario: primero las
// actualizaciones de lo instalado (ext.CheckUpdates, que NO aplica nada) y
// después las novedades (ext.AvailableExtensions). Un error de resolución se
// devuelve como aviso: el arranque nunca falla por esto.
func detectStartupExtensions() ([]ext.UpdateResult, []ext.AvailableExt, []error) {
	providers, userRoot, err := extensionUserSources()
	if err != nil {
		return nil, nil, []error{err}
	}
	updates, errs := ext.CheckUpdates(providers, userRoot, startupExtFetch)
	// El diff de novedades va DESPUÉS de la detección de actualizaciones a
	// propósito: comparar contra el conjunto ya refrescado evita anunciar como
	// novedad algo que se acaba de actualizar.
	available, availErrs := ext.AvailableExtensions(providers, userRoot, startupExtFetch)
	return updates, available, append(errs, availErrs...)
}

// checkExtensionsAndPrompt es el chequeo de extensiones del arranque, ahora
// VISIBLE: detecta actualizaciones y novedades, y en vez de aplicarlas en
// silencio le pregunta al usuario.
//
// La pregunta cubre las dos cosas juntas ("Aplicar N actualizaciones y M
// novedades?"): revisá todo, decidí una vez. El default es NO —cualquier tecla
// que no sea un sí explícito omite— y denegar omite SOLO esta vez: el próximo
// arranque vuelve a preguntar, porque el estado de la decisión no se persiste.
//
// El prompt usa un loop anidado de eventos (como el config menu, pero antes del
// loop principal): al arrancar todavía no hay nada que editar, así que espera
// solo por su respuesta y devuelve el control al resto del arranque.
//
// Va antes de loadExtensions por diseño: al aceptar, lo aplicado ya está en
// disco y lo carga el mismo arranque. Después NO se recarga, porque
// AddExtensions no es idempotente (appendea a los estados del manager y el
// registro rechaza el segundo registro).
//
// Nunca impide arrancar: un error de revisión se avisa en la barra y el editor
// sigue con lo que ya está instalado.
func (a *App) checkExtensionsAndPrompt() {
	updates, available, errs := appStartupCheck()
	a.startupUpdates, a.startupAvailable = updates, available
	if len(errs) > 0 {
		a.statusBar.SetMessage("Aviso de extensiones: " + errs[len(errs)-1].Error())
	}
	if len(updates) == 0 && len(available) == 0 {
		return
	}

	if !a.confirmStartupExtensions(len(updates), len(available)) {
		a.statusBar.SetMessage("Actualizaciones y novedades omitidas: se vuelven a preguntar al próximo arranque")
		return
	}
	a.applyStartupExtensions(updates, available)
}

// confirmStartupExtensions muestra la pregunta de arranque y espera la
// respuesta con un loop anidado. Acepta con "s"/"y" (en mayúscula o no) y
// deniega con cualquier otra tecla, Enter incluido: el default de "[s/N]" es
// NO, así que un Escape impaciente o un Enter a ciegas nunca instalan nada.
//
// El prompt se dibuja sobre la barra de estado con statusBar.Draw, no con
// redraw: en este punto la composición todavía no está armada (el tema y la
// configuración se aplican después) y no hace falta pintar el editor para que
// la pregunta se lea.
func (a *App) confirmStartupExtensions(updates, available int) bool {
	a.statusBar.SetPrompt(startupPromptLabel(updates, available))
	width, height := a.screen.Size()
	a.screen.Clear()
	a.statusBar.Draw(a.screen, height-statusHeight, width)
	a.screen.Show()

	answer := false
	for {
		ev := a.screen.PollEvent()
		if ev == nil {
			break
		}
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyRune {
				switch ev.Rune() {
				case 's', 'S', 'y', 'Y':
					answer = true
				}
			}
			// Cualquier tecla cierra el pedido: solo un sí explícito acepta.
			a.statusBar.SetPrompt("")
			return answer
		case *tcell.EventResize:
			a.screen.Sync()
			width, height = a.screen.Size()
			a.statusBar.Draw(a.screen, height-statusHeight, width)
			a.screen.Show()
		}
	}
	a.statusBar.SetPrompt("")
	return answer
}

// startupPromptLabel arma la pregunta del arranque con los plurales correctos:
// el rótulo tiene que decir cuántos cambios se van a aplicar, porque es lo
// único que el usuario tiene para decidir.
func startupPromptLabel(updates, available int) string {
	parts := make([]string, 0, 2)
	if updates > 0 {
		parts = append(parts, plural(updates, "actualización", "actualizaciones"))
	}
	if available > 0 {
		parts = append(parts, plural(available, "novedad", "novedades"))
	}
	return "Aplicar " + strings.Join(parts, " y ") + "? [s/N] "
}

// plural cuenta un sustantivo en español: 1 va en singular, el resto en
// plural.
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// applyStartupExtensions aplica lo que el usuario aceptó: las actualizaciones de
// las instaladas y las novedades de los proveedores APROBADOS (las de un
// proveedor sin aprobar se saltan en silencio, igual que siempre: instalarlas
// sin confianza repetiría por la puerta de atrás la confirmación que pide
// instalar por id).
//
// El resultado se resume en la barra, y las novedades que quedaron sin instalar
// se nombran agrupadas por proveedor con el comando para aprobarlo: el usuario
// acaba de decidir que las quiere y tiene que saber cuál no entró y por qué.
func (a *App) applyStartupExtensions(updates []ext.UpdateResult, available []ext.AvailableExt) {
	providers, userRoot, err := extensionUserSources()
	if err != nil {
		a.statusBar.SetMessage("Aviso de extensiones: " + err.Error())
		return
	}

	applied, errs := ext.UpdateAll(providers, userRoot, startupExtFetch)
	installed, installErrs := ext.InstallAvailable(available, userRoot, startupExtFetch)
	errs = append(errs, installErrs...)
	a.startupApplied = len(applied) > 0 || len(installed) > 0

	msg := plural(len(applied), "actualización aplicada", "actualizaciones aplicadas") +
		", " + plural(len(installed), "novedad instalada", "novedades instaladas")
	if pend := unapprovedRefs(available); pend != "" {
		msg += " — sin aprobar: " + pend + " (tcode --list-extensions para verlas)"
	}
	// El resumen es el mensaje; un fallo puntual se le cuelga al final para que
	// no se pierda (el editor siguió con lo instalado).
	if len(errs) > 0 {
		msg += " — con errores: " + errs[len(errs)-1].Error()
	}
	a.statusBar.SetMessage(msg)
}

// unapprovedRefs agrupa por proveedor las novedades que no se instalaron por no
// estar su proveedor aprobado, en orden alfabético para que el aviso sea
// estable entre corridas.
func unapprovedRefs(available []ext.AvailableExt) string {
	pending := map[string][]string{}
	for _, av := range available {
		if av.Approved() {
			continue
		}
		pending[av.Provider.Name] = append(pending[av.Provider.Name], av.ID)
	}
	names := make([]string, 0, len(pending))
	for name := range pending {
		names = append(names, name)
	}
	sort.Strings(names)
	groups := make([]string, 0, len(names))
	for _, name := range names {
		groups = append(groups, name+" ("+strings.Join(pending[name], ", ")+")")
	}
	return strings.Join(groups, "; ")
}

// loadExtensions descubre las extensiones de cada root y las agrega al
// manager. Cada extensión rota se avisa en la barra de estado una vez; el
// arranque nunca falla por una extensión quebrada.
func (a *App) loadExtensions() {
	var exts []ext.Extension
	for _, root := range a.extensionRoots {
		found, errs := ext.Discover(root)
		exts = append(exts, found...)
		for _, e := range errs {
			a.statusBar.SetMessage("Extensión ignorada: " + e.Error())
		}
	}
	a.ext.AddExtensions(exts)
}

// reloadActive recarga el buffer activo desde disco con la confirmación no
// modal del proyecto: sobre un buffer limpio recarga directo; con ediciones, la
// primera Ctrl+R avisa que se perderán y la segunda confirma. Después del
// re-mapeo el cursor se clampa al documento nuevo (pudo encogerse) y se
// redibuja.
func (a *App) reloadActive() {
	buf, err := a.requireBuffer()
	if err != nil {
		return
	}
	if buf.Modified() && !a.confirmReload {
		a.confirmReload = true
		a.statusBar.SetMessage("Recargar descarta los cambios sin guardar: Ctrl+R de nuevo recarga")
		a.redraw()
		return
	}
	a.confirmReload = false
	if err := buf.Reload(); err != nil {
		a.statusBar.SetMessage("Error al recargar: " + err.Error())
		a.redraw()
		return
	}
	if ed := a.activeEditor(); ed != nil {
		ed.ClampCursor()
	}
	a.statusBar.SetMessage("Recargado")
	a.syncStatus()
	a.redraw()
}

// checkExternalReloads mira los buffers abiertos en cada evento de actividad:
// si el archivo cambió en disco y el buffer está limpio, lo recarga solo —no
// hay nada que perder— y lo anuncia; un buffer con ediciones no se toca, ahí
// decide el Ctrl+R (confirmado) o el Ctrl+S de siempre. El stat por buffer es
// lo que Save ya paga; en montajes de red la cadencia por evento puede costar y
// es la limitación documentada de la detección automática.
func (a *App) checkExternalReloads() {
	for _, buf := range a.ws.Buffers() {
		if !buf.ChangedOnDisk() || buf.Modified() {
			continue
		}
		if err := buf.Reload(); err != nil {
			a.statusBar.SetMessage("Error al recargar: " + err.Error())
			continue
		}
		a.statusBar.SetMessage("Cambios externos recargados")
		if ed := a.editors[buf]; ed != nil {
			ed.ClampCursor()
		}
	}
}

// bufferAtPath devuelve el buffer abierto sobre path (rutas limpiadas), o nil.
func (a *App) bufferAtPath(path string) *model.PieceTable {
	clean := filepath.Clean(path)
	for _, b := range a.ws.Buffers() {
		if b.Path() != "" && filepath.Clean(b.Path()) == clean {
			return b
		}
	}
	return nil
}

// dedupSaveAsConsolidates resuelve el último pendiente de la feature de
// pestañas: tras un Save As exitoso a una ruta que ya estaba abierta en otra
// pestaña, no puede quedar otro buffer sobre el mismo archivo. other es el
// buffer que ya ocupaba la ruta, capturado ANTES del SaveAs (después, el target
// re-apunta su path al destino y una búsqueda por ruta sería ambigua). El
// buffer recién guardado —la pestaña activa del prompt— se cierra (quedó
// limpio: su documento ya está en disco) y el existente queda activo y
// recargado para ver lo escrito. Devuelve si consolidó.
func (a *App) dedupSaveAsConsolidates(other *model.PieceTable) bool {
	if other == nil {
		return false
	}
	a.closeTab()
	if err := other.Reload(); err != nil {
		return true // consolidado igual; el error del contenido es del llamador
	}
	if ed := a.editors[other]; ed != nil {
		ed.ClampCursor()
	}
	// El buffer existente queda activo (puede no ser el vecino de la activa).
	for i := 0; i < a.ws.Len(); i++ {
		if a.ws.BufferAt(i) == other {
			a.ws.SetActive(i)
			break
		}
	}
	a.tabBar.EnsureActive(a.ws, a.tabBarWidth())
	return true
}

// requireBuffer devuelve el buffer activo o un error legible cuando el
// workspace está vacío: los comandos que editan no pueden inventarse un
// documento.
func (a *App) requireBuffer() (*model.PieceTable, error) {
	buf := a.activeBuffer()
	if buf == nil {
		return nil, errors.New("sin buffer abierto")
	}
	return buf, nil
}

// runExtensionCommand ejecuta un comando resuelto por un keybinding de
// extensión y traduce su resultado a la barra de estado: un comando que corre
// limpio es "seguir trabajando" (desarma confirmaciones, como cualquier tecla
// del editor), y un error se muestra sin romper nada.
func (a *App) runExtensionCommand(cmd string) {
	if err := a.ext.RunCommand(cmd); err != nil {
		a.statusBar.SetMessage(err.Error())
	} else {
		a.confirmQuit = false
		a.confirmClose = false
		a.clearForceSave()
		a.statusBar.ClearMessage()
	}
	a.redraw()
}

// --- ScriptAPI: el puente que los scripts de extensión usan para tocar el ---
// --- editor (tcode.*), cableado al Manager con SetEditor en el arranque. ---

// RunCommand ejecuta un comando registrado por id (ScriptAPI). Los scripts la
// invocan vía tcode.command para llamar built-ins tcode.* u otros comandos.
func (a *App) RunCommand(id string) error { return a.ext.RunCommand(id) }

// ActiveBuffer devuelve la ruta y el contenido completo del buffer activo
// (ScriptAPI); sin buffer activo, ok=false. Límite del hito 1: devuelve el
// documento entero (GetContent); un backend maduro pediría rangos al Model
// para no copiar archivos grandes al host Lua.
func (a *App) ActiveBuffer() (path, content string, ok bool) {
	buf := a.activeBuffer()
	if buf == nil {
		return "", "", false
	}
	return buf.Path(), buf.GetContent(), true
}

// InsertAtCursor inserta text en la posición del cursor del editor activo
// (ScriptAPI). Sin buffer activo no hay cursor: error legible, igual que los
// comandos del núcleo que requieren buffer.
func (a *App) InsertAtCursor(text string) error {
	ed := a.activeEditor()
	if ed == nil {
		return errors.New("sin buffer activo")
	}
	return a.activeBuffer().Insert(ed.CursorOffset(), text)
}

// StatusMessage muestra msg en la barra de estado (ScriptAPI).
func (a *App) StatusMessage(msg string) { a.statusBar.SetMessage(msg) }

// LineCount devuelve la cantidad de líneas del buffer activo (ScriptAPI);
// sin buffer activo, ok=false, como ActiveBuffer.
func (a *App) LineCount() (int, bool) {
	buf := a.activeBuffer()
	if buf == nil {
		return 0, false
	}
	return buf.LineCount(), true
}

// Line devuelve el texto de la línea n (0-indexada) del buffer activo
// (ScriptAPI); sin buffer activo o con n fuera de [0, LineCount), ok=false.
// El modelo no copia la línea: LineContent la extrae del PieceTable.
func (a *App) Line(n int) (string, bool) {
	buf := a.activeBuffer()
	if buf == nil || n < 0 || n >= buf.LineCount() {
		return "", false
	}
	return string(buf.LineContent(n)), true
}

// SetDiagnostics reemplaza las anotaciones del buffer activo (ScriptAPI): el
// backend de scripting es el proveedor de diagnostics y deposita acá lo que el
// HITO A renderiza. Sin buffer activo, error legible como el resto de la API.
// Límite del hito: un hook de onDidSaveBuffer corre "en el contexto del
// activo", así que el buffer anotado es el activo al momento del set.
func (a *App) SetDiagnostics(source string, d []view.Diagnostic) error {
	ed := a.activeEditor()
	if ed == nil {
		return errors.New("sin buffer activo")
	}
	ed.SetDiagnostics(source, d)
	return nil
}

// emitEvent despacha un evento de buffer al manager de extensiones y muestra
// el último error de hook en la barra de estado. Un hook roto nunca rompe el
// editor: solo avisa.
func (a *App) emitEvent(event string) {
	errs := a.ext.Emit(event)
	if len(errs) > 0 {
		a.statusBar.SetMessage(errs[len(errs)-1].Error())
	}
}

// editorHeight es el alto disponible para el editor, descontando la fila de
// pestañas y la barra de estado. Nunca baja de 1: un área de cero filas no
// tendría dónde dibujar el cursor.
func editorHeight(height int) int {
	if h := height - statusHeight - tabBarHeight; h >= 1 {
		return h
	}
	return 1
}

// activeBuffer devuelve el buffer activo, o nil si el workspace está vacío.
func (a *App) activeBuffer() *model.PieceTable { return a.ws.Active() }

// activeEditor resuelve la vista del buffer activo, creándola bajo demanda con
// el tamaño ACTUAL de la pantalla. Devuelve nil si no hay buffer activo. La
// vista se crea con el ancho que el editor tiene SEGÚN el panel: con el panel
// visible, la columna del panel no es del documento.
func (a *App) activeEditor() *view.EditorView {
	buf := a.activeBuffer()
	if buf == nil {
		return nil
	}
	if ev, ok := a.editors[buf]; ok {
		return ev
	}
	width, height := a.screen.Size()
	ev := view.NewEditorView(buf, editorHeight(height), width-a.explorerColumn())
	ev.SetTheme(a.theme)
	a.editors[buf] = ev
	return ev
}

// explorerColumn devuelve el ancho del panel cuando está visible, o 0 cuando
// no: es la columna a la que arranca el editor.
func (a *App) explorerColumn() int {
	if !a.explorerVisible {
		return 0
	}
	width, _ := a.screen.Size()
	return panelWidth(width)
}

// tabBarWidth es el ancho de la fila de pestañas: el del área de trabajo según
// el panel. Las pestañas se renderizan sobre el editor, nunca sobre el árbol;
// el ancho disponible para ellas —y para su desplazamiento— es el del editor.
func (a *App) tabBarWidth() int {
	width, _ := a.screen.Size()
	return width - a.explorerColumn()
}

func (a *App) syncStatus() {
	buf := a.activeBuffer()
	if buf == nil {
		a.statusBar.SetFile("", false)
		return
	}
	a.statusBar.SetFile(buf.Path(), buf.Modified())
}

// Los diagnósticos se muestran inline, a la derecha de cada línea anotada
// (render del view), no en la barra de estado: así varias líneas con errores
// se ven a la vez. La barra solo lleva los mensajes transitorios (guardado,
// resúmenes de extensión, confirmaciones).

// Run ejecuta el loop de eventos hasta que el usuario cierra el editor.
func (a *App) Run() (err error) {
	// El crash paper del runtime (fatal, pánico en goroutine, stack overflow)
	// mata el proceso SIN desenrollar los defers: queda apuntado a crash.log
	// para que la evidencia sobreviva aunque la terminal quede destruida.
	setCrashPaper()

	defer a.screen.Fini()
	defer a.ws.CloseAll()
	// La sesión se guarda como ÚLTIMO defer: al desenrollar (LIFO) corre
	// PRIMERO, con los buffers todavía abiertos, antes de CloseAll y Fini. Es
	// el camino de salida, silencioso: un fallo de escritura no puede impedir
	// cerrar el editor.
	defer a.saveSession()

	// Guard de crash: un pánico en el despacho de un evento no deja el editor
	// muerto con la terminal en modo raw. Se vuelca el stack a crash.log y se
	// devuelve un error que main imprime —la pantalla ya quedó restaurada
	// cuando corre el defer de Fini, así el mensaje es legible en la consola—.
	defer func() {
		if r := recover(); r != nil {
			path := writeCrashLog(r)
			if path != "" {
				err = fmt.Errorf("crash interno (detalle en %s): %v", path, r)
			} else {
				err = fmt.Errorf("crash interno: %v", r)
			}
		}
	}()

	a.redraw()

	for {
		ev := a.screen.PollEvent()
		if ev == nil {
			return nil
		}
		if a.handleEvent(ev) {
			return nil
		}
	}
}

// crashLogPath resuelve la ruta del registro de crashes; es una variable de
// función para que los tests la reemplacen por un directorio temporal.
var crashLogPath = defaultCrashLogPath

func defaultCrashLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".tcode", "crash.log")
}

// writeCrashLog vuelca el valor y el stack de un pánico a crash.log (append),
// creando el directorio si hace falta, y devuelve la ruta (o "" si no se
// pudo). Un fallo de escritura jamás impide el resto del recovery.
func writeCrashLog(r any) string {
	path := crashLogPath()
	if path == "" {
		return ""
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return path
	}
	defer f.Close()
	fmt.Fprintf(f, "tcode crash: %v\n%s\n", r, debug.Stack())
	return path
}

// setCrashPaper redirige el crash paper del runtime al registro: los errores
// fatales no pasan por el guard de Run, y sin esta redirección la única
// evidencia quedaría enterrada en una terminal destruida. El archivo queda
// abierto toda la vida del proceso, como pide SetCrashOutput — por eso es una
// variable inyectable: los tests la reemplazan por un no-op (un archivo abierto
// de por vida rompería el cleanup de TempDir en Windows).
var setCrashPaper = func() {
	path := crashLogPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	debug.SetCrashOutput(f, debug.CrashOptions{})
}

// handleEvent procesa un evento y devuelve true si la aplicación debe terminar.
func (a *App) handleEvent(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		// Cada evento de actividad revisa los buffers abiertos: un archivo que
		// cambió por fuera y un buffer limpio se recargan solos. Con ediciones
		// sin guardar no se toca nada: ahí decide el Ctrl+R (confirmado).
		a.checkExternalReloads()
		// Con un pedido activo el teclado es del pedido, no del documento.
		if a.promptActive {
			a.handlePromptKey(ev)
			return false
		}

		// La ventana de extensiones abierta posee el teclado, como las demás
		// overlays. El pedido va ANTES (arriba) porque la confirmación de una
		// acción —instalar, actualizar, borrar— se superpone a la ventana: el
		// prompt tiene el teclado mientras está escribiendo. La intención de la
		// fila la ejecuta el controlador; (false, _) —Escape, Ctrl+C y
		// CUALQUIER otra tecla ajena— cierra la ventana descartando.
		if a.extActive {
			handled, intent := a.extManager.HandleEvent(ev)
			if !handled {
				a.extActive = false // cualquier tecla ajena —Escape incluido— cierra y descarta
			} else {
				a.handleExtIntent(intent)
			}
			a.redraw()
			return false
		}

		// La ventana de configuración abierta posee el teclado: (true, changed)
		// es una tecla suya (y changed dice si una fila se mutó, para persistir
		// y reencuadrar), y (false, false) —Escape, Ctrl+C y CUALQUIER otra
		// tecla ajena— la cierra descartando, sin dejar que la tecla caiga al
		// documento ni a los atajos. Una fila de acción (Extensiones) no muta
		// nada: dispara su acción, que se lee con Activated() y abre la
		// ventana de extensiones cerrando esta.
		if a.configActive {
			handled, changed := a.configMenu.HandleEvent(ev)
			if !handled {
				a.configActive = false // cualquier tecla ajena —Escape incluido— cierra y descarta
			} else if a.configMenu.Activated() != "" {
				a.configActive = false
				a.openExtManager()
			} else if changed {
				a.configChanged()
			}
			a.redraw()
			return false
		}

		// El menú de pestañas abierto posee el teclado. (true, true) es Enter:
		// activar la pestaña del cursor y cerrar. (false, false) —Escape, Ctrl+C
		// y CUALQUIER otra tecla ajena— cierra el menú descartando, sin dejar
		// que la tecla caiga al documento ni a los atajos.
		if a.menuActive {
			handled, activate := a.menu.HandleEvent(ev)
			if !handled {
				a.menuActive = false // cualquier tecla ajena —Escape incluido— cierra y descarta
			} else if activate {
				a.menuSwitchTab() // Enter: cambiar y cerrar
			}
			a.redraw()
			return false
		}

		// El explorador enfocado consume su teclado ANTES del guard del
		// workspace vacío: la navegación del árbol tiene que funcionar sin
		// ningún buffer abierto —el arranque sobre un directorio no abre
		// buffers y el foco ya está en el panel—. ActionMove solo redibuja;
		// ActionActivate abre el archivo del nodo; ActionExpand pide los hijos
		// del dir del cursor (SetChildren es la única E/S de expansión) y
		// ActionDelete pide confirmación para borrar el nodo del cursor. Lo no
		// consumido cae al flujo normal: atajos, documento y salida.
		if a.explorerVisible && a.explorerFocused {
			action, handled := a.explorer.HandleEvent(ev)
			if handled {
				// Navegar el panel es "seguir trabajando": desarma las
				// confirmaciones pendientes y el permiso de pisar, como
				// cualquier otra tecla del documento —si no, un Escape armado
				// por error seguiría activo tras navegar el árbol—. También
				// invalida el primer Escape de la doble presión.
				a.lastEscape = time.Time{}
				a.confirmQuit = false
				a.confirmClose = false
				a.clearForceSave()
				a.statusBar.ClearMessage()
				switch action {
				case view.ActionActivate:
					a.activateExplorerEntry()
				case view.ActionExpand:
					a.explorerExpand()
				case view.ActionDelete:
					a.promptDeleteEntry()
				}
				a.redraw()
				return false
			}
		}

		// Tab devuelve el foco al editor SOLO cuando el explorador lo tiene:
		// con el foco en el editor, Tab sigue insertando tabulación en el
		// documento —la edición no pierde su tecla más básica por tener el
		// panel a la vista—. Va antes del guard porque mover el foco no toca
		// ningún buffer: con el workspace vacío también tiene que funcionar.
		if ev.Key() == tcell.KeyTab && a.explorerVisible && a.explorerFocused {
			a.explorerFocused = false
			a.confirmQuit = false
			a.confirmClose = false
			a.clearForceSave()
			a.redraw()
			return false
		}

		// Shift+Tab (KeyBacktab) mueve el foco AL OTRO panel: con el explorador
		// a la vista, Shift+Tab alterna en los dos sentidos. Desde el EDITOR
		// entra al selector y REVELA el buffer activo —el árbol expande (con
		// E/S perezosa) el camino hasta el archivo que se está editando y deja
		// el cursor sobre él, para que el selector no muestre una selección
		// vieja—; desde el SELECCIONADO devuelve el foco al editor, igual que
		// Tab: quien llega al selector con Shift+Tab no queda atrapado, la
		// misma tecla lo saca. La tecla nunca llega a la edición y la
		// visibilidad del panel sigue siendo decisión de Ctrl+B. Va antes del
		// guard por la misma razón que Tab: solo mueve foco.
		if ev.Key() == tcell.KeyBacktab && a.explorerVisible {
			a.explorerFocused = !a.explorerFocused
			a.confirmQuit = false
			a.confirmClose = false
			a.clearForceSave()
			if a.explorerFocused {
				a.revealActiveInExplorer()
			}
			a.redraw()
			return false
		}

		// Ctrl+B muestra u oculta el panel —también con el workspace vacío:
		// el arranque sobre un directorio cae en el explorador y tiene que
		// poder ocultarse para ganar ancho sin abrir primero un archivo—.
		// Mostrar enfoca al explorador, ocultar lo desenfoca, y el ancho del
		// editor cambia: las vistas se redimensionan en el toggle.
		if ev.Key() == tcell.KeyCtrlB {
			a.toggleExplorer()
			return false
		}

		// Ctrl+P abre la ventana flotante de configuración —también con el
		// workspace vacío: la configuración existe sin buffers—. Arrancó como
		// Ctrl+, pero el terminal del usuario interceptaba la coma: Ctrl+P
		// (el byte 0x10, KeyCtrlP) pasa limpio en Windows Terminal y en casi
		// todo terminal estándar. Ctrl+Shift+P queda fuera a propósito: la
		// Shift del par no pide nadie.
		if ev.Key() == tcell.KeyCtrlP && ev.Modifiers()&tcell.ModShift == 0 {
			a.toggleConfig()
			return false
		}

		// Ctrl+N crea un archivo y Ctrl+Shift+N una carpeta —también con el
		// workspace vacío: crear es justamente la acción del arranque sin
		// buffers—, y ANTES del guard por la misma razón que Ctrl+B/Ctrl+P. El
		// destino es contextual (promptCreateEntry) y el prompt abre el
		// teclado; Enter hace el resto.
		if isCreateFileKey(ev) {
			a.promptCreateEntry(false)
			return false
		}

		if isCreateFolderKey(ev) {
			a.promptCreateEntry(true)
			return false
		}

		// Workspace vacío: no hay nada que editar, guardar ni deshacer. Salir
		// sigue funcionando, y sin buffers no hay nada que perder.
		buf := a.activeBuffer()
		if buf == nil {
			// Workspace vacío: tampoco se sale con un Escape solo (la única
			// salida es la doble presión); Ctrl+C ya no es la forma de cerrar.
			if ev.Key() == tcell.KeyEscape {
				return a.quitEscape()
			}
			return false
		}

		switch {
		case ev.Key() == tcell.KeyCtrlT:
			a.toggleMenu()
			return false

		case ev.Key() == tcell.KeyCtrlS:
			a.confirmQuit = false
			a.confirmClose = false
			a.save()
			return false

		case isSaveAsKey(ev):
			a.startPrompt()
			return false

		case isRedoKey(ev):
			a.confirmQuit = false
			a.confirmClose = false
			a.applyHistory(buf.Redo, "Rehecho", "Nada que rehacer")
			return false

		case isUndoKey(ev):
			a.confirmQuit = false
			a.confirmClose = false
			a.applyHistory(buf.Undo, "Deshecho", "Nada que deshacer")
			return false

		case ev.Key() == tcell.KeyPgDn && ev.Modifiers()&tcell.ModCtrl != 0:
			// Ctrl+PageDown/PageUp cambian de pestaña (con wrap). El scroll de
			// página dentro del documento es PgUp/PgDn sin Ctrl, que cae en la
			// vista.
			a.switchTab(a.ws.Next)
			return false

		case ev.Key() == tcell.KeyPgUp && ev.Modifiers()&tcell.ModCtrl != 0:
			a.switchTab(a.ws.Prev)
			return false

		case isSwitchTabNextKey(ev):
			a.switchTab(a.ws.Next)
			return false

		case isSwitchTabPrevKey(ev):
			a.switchTab(a.ws.Prev)
			return false

		case ev.Key() == tcell.KeyCtrlW:
			a.closeTab()
			return false

		case ev.Key() == tcell.KeyCtrlR && ev.Modifiers()&tcell.ModShift == 0:
			a.reloadActive()
			return false

		case isWrapToggleKey(ev):
			if view.ToggleWordWrap() {
				a.statusBar.SetMessage("Salto de palabra activado")
			} else {
				a.statusBar.SetMessage("Salto de palabra desactivado")
			}
			a.redraw()
			return false

		case ev.Key() == tcell.KeyEscape:
			return a.quitEscape()
		}

		// Las extensiones resuelven después de los atajos del núcleo —que ganan
		// siempre— y antes de que la tecla caiga al documento. Con el workspace
		// vacío el guard de arriba cortó antes: sin buffers no hay keybinding de
		// extensión (solo navegación y salida), regla documentada.
		if cmd := a.ext.Resolve(ev); cmd != "" {
			a.runExtensionCommand(cmd)
			return false
		}

		// Cualquier otra tecla cancela las confirmaciones pendientes —la de
		// salida, la de cierre de pestaña y la de recarga— y el permiso de
		// pisar que se haya dado con un Ctrl+S previo. También invalida el
		// primer Escape de la doble presión: dos Escape con una tecla en
		// medio no son una "doble presión limpia" y no cierran.
		a.lastEscape = time.Time{}
		a.confirmQuit = false
		a.confirmClose = false
		a.confirmReload = false
		a.clearForceSave()
		a.statusBar.ClearMessage()
		if ed := a.activeEditor(); ed != nil && ed.HandleEvent(ev) {
			a.redraw()
		}

	case *tcell.EventMouse:
		// Con el menú abierto el mouse es del menú como el teclado, con la
		// ventana de configuración abierta es de la ventana, y con un pedido
		// activo (Save As o creación) es del pedido: se ignora por completo —el controlador
		// no traduce nada ni redibuja— y el clic no puede cambiar de pestaña,
		// seleccionar un archivo ni raspar el documento por debajo de lo que el
		// usuario está escribiendo.
		if a.menuActive || a.promptActive || a.configActive || a.extActive {
			return false
		}
		a.checkExternalReloads()

		// La composición es dueña del layout: el mouse llega en coordenadas de
		// pantalla y cada pane traduce su propio origen. Con el panel visible,
		// el clic a la izquierda de su borde va al explorador (solo se resta la
		// fila de pestañas) y lo enfoca si lo manejó; lo que el panel no
		// consume cae al editor con la traducción completa —columna del panel
		// y fila de pestañas—, igual que el clic a la derecha (y, sin panel,
		// sin columna que restar).
		width, _ := a.screen.Size()
		x, y := ev.Position()

		// La fila de pestañas es del EDITOR —vive sobre su área, nunca sobre el
		// árbol—: un clic en una pestaña la activa y la rueda cambia de
		// pestaña. Se traduce la x a la región de la barra restando la columna
		// del panel, como todo lo demás de la composición.
		if y == 0 {
			if idx, handled := a.tabBar.HandleMouse(x-a.explorerColumn(), y, ev.Buttons(), a.ws, a.tabBarWidth()); handled {
				a.explorerFocused = false
				if idx >= 0 {
					a.activateTab(idx)
				}
				a.redraw()
				return false
			}
		}

		column := 0
		if a.explorerVisible {
			panelW := panelWidth(width)
			if x < panelW {
				if _, handled := a.explorer.HandleEvent(tcell.NewEventMouse(x, y-tabBarHeight, ev.Buttons(), ev.Modifiers())); handled {
					a.explorerFocused = true
					a.redraw()
					return false
				}
			}
			column = panelW
		}
		if ed := a.activeEditor(); ed != nil {
			viewEv := tcell.NewEventMouse(x-column, y-tabBarHeight, ev.Buttons(), ev.Modifiers())
			if ed.HandleEvent(viewEv) {
				a.redraw()
			}
		}

	case *tcell.EventResize:
		a.screen.Sync()
		width, height := a.screen.Size()
		// El resize cambia el viewport de TODOS los buffers abiertos, no solo
		// del activo: si no, la vista de otro buffer queda desactualizada y
		// dibuja mal al volver a esa pestaña. El ancho es el del editor según
		// el panel, como en el toggle.
		a.resizeEditors()
		// El ancho nuevo también reencuadra la fila de pestañas: entra más (o
		// menos) de ella, y la activa tiene que seguir visible.
		a.tabBar.EnsureActive(a.ws, a.tabBarWidth())
		a.explorer.Resize(panelWidth(width), editorHeight(height))
		a.redraw()
	}
	return false
}

// isSwitchTabNextKey reconoce Ctrl+K sin Shift: pestaña siguiente. Ctrl+J no
// existe como par: en la terminal es el byte LF —el Enter que tcode ya trata
// como activar/insertar salto de línea—, así que el atajo quedó en K y L.
func isSwitchTabNextKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyCtrlK && ev.Modifiers()&tcell.ModShift == 0
}

// isSwitchTabPrevKey reconoce Ctrl+L sin Shift: pestaña anterior. El par final
// es Ctrl+K (siguiente) / Ctrl+L (anterior); Ctrl+Shift+K quedó descartado.
// Nota de terminal: en xterm y consolas Unix el form feed (Ctrl+L) limpia la
// pantalla y no llega a la app; en Windows Terminal, el entorno objetivo,
// llega limpio.
func isSwitchTabPrevKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyCtrlL && ev.Modifiers()&tcell.ModShift == 0
}

// isUndoKey reconoce Ctrl+Z sin modificadores.
func isUndoKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyCtrlZ && ev.Modifiers()&tcell.ModShift == 0
}

// isWrapToggleKey reconoce Ctrl+Shift+W: como Ctrl+Shift+Z, tcell reporta
// la combinación con Shift como KeyRune con ModCtrl y ModShift.
func isWrapToggleKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyRune &&
		ev.Modifiers()&tcell.ModCtrl != 0 &&
		ev.Modifiers()&tcell.ModShift != 0 &&
		(ev.Rune() == 'w' || ev.Rune() == 'W')
}

// isRedoKey reconoce Ctrl+Y y también Ctrl+Shift+Z.
//
// tcell solo reporta los códigos KeyCtrl* cuando no hay modificadores: con Shift
// presente, Ctrl+Shift+Z llega como KeyRune con ModCtrl y ModShift.
func isRedoKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyCtrlY {
		return true
	}
	return ev.Key() == tcell.KeyRune &&
		ev.Modifiers()&tcell.ModCtrl != 0 &&
		ev.Modifiers()&tcell.ModShift != 0 &&
		(ev.Rune() == 'z' || ev.Rune() == 'Z')
}

// applyHistory ejecuta deshacer o rehacer sobre el buffer activo y refleja el
// resultado en la barra.
func (a *App) applyHistory(op func() (model.Change, bool, error), done, empty string) {
	change, ok, err := op()
	switch {
	case err != nil:
		a.statusBar.SetMessage("Error: " + err.Error())
	case !ok:
		a.statusBar.SetMessage(empty)
	default:
		if ed := a.activeEditor(); ed != nil {
			ed.MoveCursorToOffset(change.Offset)
		}
		a.statusBar.SetMessage(done)
	}
	a.syncStatus()
	a.redraw()
}

// isSaveAsKey reconoce Ctrl+Shift+S. Como en Ctrl+Shift+Z, tcell entrega los
// control con Shift como KeyRune en lugar del código KeyCtrl*.
func isSaveAsKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyRune &&
		ev.Modifiers()&tcell.ModCtrl != 0 &&
		ev.Modifiers()&tcell.ModShift != 0 &&
		(ev.Rune() == 's' || ev.Rune() == 'S')
}

// startPrompt abre el pedido de Save As, prellenado con la ruta actual para
// poder editarla en lugar de reescribirla entera. Sin buffer activo (workspace
// vacío) el pedido igual puede abrirse —lo que se pide es una ruta nueva, no el
// documento activo—, con el prefill vacío.
func (a *App) startPrompt() {
	buf := a.activeBuffer()
	prefill := ""
	if buf != nil {
		prefill = buf.Path()
	}
	// El buffer destino se captura ACÁ, en el closure, no al confirmar: el
	// texto que alguien escribe pertenece al documento que estaba mirando
	// cuando abrió el pedido, no al que esté activo cuando aprieta Enter. Sin
	// buffer activo el closure responde con el error legible de requireBuffer en
	// vez de desreferenciar nil: crear es justamente la acción del workspace
	// vacío, y el pedido tiene que poder abrirse sin él.
	a.openPrompt("Guardar como: ", prefill, func(path string) error {
		if buf == nil {
			return errors.New("sin buffer abierto")
		}
		a.saveAs(buf, path)
		return nil
	})
}

// openPrompt arma el pedido generalizado: label + prefill + action. Es el
// núcleo que startPrompt (Save As) y promptCreateEntry (archivo/carpeta)
// comparten: arma el estado, refresca la barra y redibuja. Ningún llamador
// necesita un buffer activo —crear un archivo es justamente lo que se hace sin
// buffers—, así que esta función no toca el workspace.
func (a *App) openPrompt(label, prefill string, action func(path string) error) {
	a.confirmQuit = false
	a.confirmClose = false
	a.clearForceSave()
	a.promptActive = true
	a.promptBuf = prefill
	a.promptLabel = label
	a.promptAction = action
	a.refreshPrompt()
	a.redraw()
}

func (a *App) refreshPrompt() {
	a.statusBar.SetPrompt(a.promptLabel + a.promptBuf)
}

func (a *App) endPrompt() {
	a.promptActive = false
	a.promptBuf = ""
	a.promptLabel = ""
	a.promptAction = nil
	a.statusBar.SetPrompt("")
}

// clearForceSave revoca el permiso de pisar de todos los buffers. Reusa el mapa
// en lugar de asignar uno nuevo: esto corre en el camino caliente del tipeo
// (cualquier tecla que no sea un atajo lo llama) y alocar un mapa por tecla
// sería un costo por nada.
func (a *App) clearForceSave() { clear(a.forceSave) }

// activeForceSave devuelve el permiso de pisar del buffer activo.
func (a *App) activeForceSave() bool { return a.forceSave[a.activeBuffer()] }

// saveAsFor autoriza a un buffer concreto a pisar cambios externos.
func (a *App) saveAsFor(buf *model.PieceTable) { a.forceSave[buf] = true }

// handlePromptKey alimenta el pedido de texto. El borrado va por runa y no por
// grapheme cluster: alcanza para rutas, que son ASCII en la práctica. Al
// confirmar (Enter) el pedido se cierra PRIMERO y después corre la acción
// captured: la acción ya no necesita el estado del pedido, y un error suyo se
// muestra en la barra como mensaje normal.
func (a *App) handlePromptKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		a.endPrompt()
		a.statusBar.SetMessage("Cancelado")
		a.redraw()
		return

	case tcell.KeyEnter:
		path := strings.TrimSpace(a.promptBuf)
		action := a.promptAction
		a.endPrompt()
		if action == nil {
			a.redraw()
			return
		}
		if err := action(path); err != nil {
			a.statusBar.SetMessage(err.Error())
		}
		a.redraw()
		return

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if r := []rune(a.promptBuf); len(r) > 0 {
			a.promptBuf = string(r[:len(r)-1])
		}

	case tcell.KeyRune:
		if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0 {
			if r := ev.Rune(); r != 0 && r != '\n' && r != '\t' {
				a.promptBuf += string(r)
			}
		}
	}

	a.refreshPrompt()
	a.redraw()
}

// quitEscape cierra el editor SOLO con la doble presión de Escape: dos Escape
// dentro de quitEscapeWindow. Un solo Escape nunca cierra —con cambios sin
// guardar avisa y arma la confirmación, sin ellos da el feedback de "de nuevo
// rápido"—; un segundo Escape tardío reinicia el conteo. Ctrl+C dejó de ser
// una forma de cerrar: esta es la única salida.
func (a *App) quitEscape() bool {
	now := clockNow()
	if now.Sub(a.lastEscape) <= quitEscapeWindow {
		a.lastEscape = time.Time{}
		return true
	}
	a.lastEscape = now
	if a.ws.AnyModified() && !a.confirmQuit {
		a.confirmQuit = true
		a.statusBar.SetMessage("Cambios sin guardar: Ctrl+S guarda, Escape dos veces rápido sale")
	} else {
		a.statusBar.SetMessage("Escape de nuevo rápido para salir")
	}
	a.redraw()
	return false
}

// saveAs guarda el buffer capturado al abrir el pedido en la ruta elegida y pasa
// a trabajar sobre ella.
func (a *App) saveAs(target *model.PieceTable, path string) {
	if path == "" {
		a.statusBar.SetMessage("Save As cancelado")
		a.redraw()
		return
	}

	// El buffer capturado puede haber salido del workspace entre medio: no hay
	// que escribir en un documento que ya no está abierto.
	inWorkspace := false
	for _, b := range a.ws.Buffers() {
		if b == target {
			inWorkspace = true
			break
		}
	}
	if !inWorkspace {
		a.statusBar.SetMessage("El buffer de destino ya no está abierto")
		a.redraw()
		return
	}

	// Windows no deja renombrar sobre un archivo con una sección mapeada abierta
	// por el MISMO proceso (ERROR_USER_MAPPED_FILE → "Acceso denegado"): si la
	// ruta destino ya está abierta en otra pestaña, ese buffer mantiene el mmap
	// y el rename del Save As fallaría. Se desmapea antes de escribir; el dedup
	// posterior lo recarga con lo recién escrito. El otro buffer se CAPTURA acá:
	// después del SaveAs el target re-apunta su path al destino y una búsqueda
	// por ruta encontraría al target primero (no al existente).
	var other *model.PieceTable
	if b := a.bufferAtPath(path); b != nil && b != target {
		b.Unmap()
		other = b
	}

	if err := target.SaveAs(path); err != nil {
		a.statusBar.SetMessage("Error al guardar como: " + err.Error())
	} else {
		a.confirmQuit = false
		a.clearForceSave()
		if a.dedupSaveAsConsolidates(other) {
			// La ruta ya estaba abierta en otra pestaña: se consolidó en una sola.
			a.statusBar.SetMessage("Guardado en " + filepath.Base(path) + " — ruta ya abierta: una sola pestaña")
		} else {
			a.statusBar.SetMessage("Guardado en " + filepath.Base(path))
		}
		a.emitEvent(ext.EventDidSaveBuffer)
	}

	a.syncStatus()
	a.redraw()
}

// save escribe el buffer activo y refleja el resultado en la barra de estado.
//
// Si el archivo cambió en disco se avisa en lugar de pisarlo; un segundo Ctrl+S
// seguido fuerza la escritura. La decisión de perder esos cambios queda así en
// manos de quien usa el editor y no de un valor por defecto.
func (a *App) save() {
	buf := a.activeBuffer()
	var err error
	if a.activeForceSave() {
		a.clearForceSave()
		err = buf.SaveForce()
	} else {
		err = buf.Save()
	}

	switch {
	case errors.Is(err, model.ErrFileChangedExternally):
		a.saveAsFor(buf)
		a.statusBar.SetMessage("El archivo cambió en disco: Ctrl+S de nuevo pisa esos cambios")

	case err != nil:
		a.statusBar.SetMessage("Error al guardar: " + err.Error())

	default:
		a.confirmQuit = false
		a.statusBar.SetMessage("Guardado")
		a.emitEvent(ext.EventDidSaveBuffer)
	}

	a.syncStatus()
	a.redraw()
}

// redraw limpia y vuelve a dibujar toda la composición: pestañas en la fila 0,
// el panel del explorador (si está visible) en la columna izquierda y el editor
// activo en la región que queda, ambas desde la fila 1, y la barra de estado al
// final. La vista de cada pane dibuja desde (0,0) de SU región —la composición
// es dueña del layout— y la superficie del editor se reusa: se crea la primera
// vez y después se reencuadra con SetRegion (primero para el panel, después
// para el editor), en lugar de alocar una por redibujo.
//
// Con el workspace vacío no hay editor: solo pestañas (vacías), el panel si
// está visible, la barra y el cursor escondido —un cursor huérfano de una
// pestaña cerrada quedaría flotando sobre la barra.
func (a *App) redraw() {
	width, height := a.screen.Size()
	a.screen.Clear()

	if a.editorSurf == nil {
		a.editorSurf = view.NewOffsetSurface(a.screen)
	}

	// La fila de pestañas vive SOBRE el área de trabajo —la del editor—, no
	// sobre el panel del árbol: arranca en la columna del editor, y con el
	// panel oculto es la columna 0 (el layout de siempre). Es el mismo uso de
	// la costura que para el editor: la superficie se reencuadra por pane.
	tabW := a.tabBarWidth()
	a.editorSurf.SetRegion(a.explorerColumn(), 0, tabW, tabBarHeight)
	a.tabBar.Draw(a.editorSurf, a.ws, tabW)

	panelW := 0
	if a.explorerVisible {
		panelW = panelWidth(width)
		a.editorSurf.SetRegion(0, tabBarHeight, panelW, editorHeight(height))
		a.explorer.Draw(a.editorSurf)
	}

	if ed := a.activeEditor(); ed != nil {
		a.editorSurf.SetRegion(panelW, tabBarHeight, width-panelW, editorHeight(height))
		ed.Draw(a.editorSurf)
	} else {
		a.screen.HideCursor()
	}

	// El menú de pestañas es un overlay de la región del editor: se compone
	// DESPUÉS del editor (tapa el documento, sin tocar pestañas ni barra) con
	// la misma superficie recortada al área de trabajo, con el alto de las
	// pestañas listadas (nunca más que el editor). No dibuja con el workspace
	// vacío: redraw lo llama siempre, también sin pestañas.
	if a.menuActive && a.ws.Len() > 0 {
		if a.editorSurf == nil {
			a.editorSurf = view.NewOffsetSurface(a.screen)
		}
		editorW := width - a.explorerColumn()
		a.editorSurf.SetRegion(a.explorerColumn(), tabBarHeight, editorW, min(a.ws.Len(), editorHeight(height)))
		a.menu.Draw(a.editorSurf, a.ws, editorW)
	}

	// La ventana de configuración flota centrada sobre el área del editor,
	// como el menú de pestañas: se compone DESPUÉS del editor (tapa el
	// documento, sin tocar pestañas ni barra) con la misma superficie recortada
	// a su región (configRegion). toggleConfig jamás la abre con un editor de
	// ancho 0, así que acá siempre hay espacio.
	if a.configActive {
		if a.editorSurf == nil {
			a.editorSurf = view.NewOffsetSurface(a.screen)
		}
		x, y, w, h := a.configRegion()
		a.editorSurf.SetRegion(x, y, w, h)
		a.configMenu.Draw(a.editorSurf)
	}

	// La ventana de extensiones flota sobre el editor como la de configuración,
	// encima de ella si alguna vez coincidieran: se compone DESPUÉS con la
	// misma superficie recortada a su región (extRegion). El prompt de
	// confirmación no se dibuja acá —vive en la barra de estado—.
	if a.extActive {
		x, y, w, h := a.extRegion()
		a.editorSurf.SetRegion(x, y, w, h)
		a.extManager.Draw(a.editorSurf)
	}

	a.statusBar.Draw(a.screen, height-statusHeight, width)
	a.screen.Show()
}

// toggleExplorer muestra u oculta el panel lateral. Mostrar enfoca el
// explorador y REVELA el buffer activo —otra puerta de entrada del foco, como
// Shift+Tab—: el selector nunca vuelve a una selección vieja al reaparecer.
// Ocultar lo desenfoca (el foco queda en el editor). En ambos casos el ancho
// del editor cambia, así que todas las vistas reciben el mismo tratamiento que
// un resize —sin eso, la vista de otra pestaña dibujaría con el ancho viejo al
// volver— y se redibuja.
func (a *App) toggleExplorer() {
	a.explorerVisible = !a.explorerVisible
	a.explorerFocused = a.explorerVisible
	if a.explorerVisible {
		a.revealActiveInExplorer()
	}
	a.resizeEditors()
	a.redraw()
}

// resizeEditors aplica el ancho del editor SEGÚN el panel a todas las vistas
// abiertas. Es el mismo tratamiento que un resize: mostrar u ocultar el panel
// cambia el ancho disponible, y una vista no activa quedaría desactualizada.
func (a *App) resizeEditors() {
	width, height := a.screen.Size()
	for _, ed := range a.editors {
		ed.Resize(width-a.explorerColumn(), editorHeight(height))
	}
}

// readEntries lee un directorio con las reglas del árbol: directorios primero
// y luego archivos, ambos alfabéticos (os.ReadDir ya ordena), ocultos
// incluidos; lo que no es directorio ni archivo regular queda fuera. Sin la
// entrada sintética "..": la base del árbol es la cima fija de la sesión y
// nunca se sube. El error se devuelve para que el llamador decida —el arranque
// muestra el primer nivel vacío, la expansión es un no-op silencioso—.
func readEntries(dir string) ([]view.Entry, error) {
	infos, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var dirs, files []view.Entry
	for _, de := range infos {
		// Los dotfiles (nombres que arrancan con ".") no entran al árbol: .git,
		// .tcode y el resto son estado de la herramienta, no código. El filtro
		// vive acá, en la ÚNICA puerta de datos del disco a la vista: cubre el
		// nivel raíz y toda expansión de subdirectorio con la misma regla.
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			continue
		}
		e := view.Entry{Name: de.Name(), Path: filepath.Join(dir, de.Name()), IsDir: info.IsDir()}
		if info.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	return append(dirs, files...), nil
}

// activateExplorerEntry abre el archivo del nodo activo del árbol —la rama
// ActionActivate—. Ya NO hace stat ni desciende: el nodo sabe si es un
// directorio (la expansión la dispara ActionExpand) y la carga perezosa evitó
// leerlo. Un archivo se abre en el workspace (con la dedupe por ruta
// normalizada de Open) y el foco vuelve al editor. El caller redibuja.
func (a *App) activateExplorerEntry() {
	path := a.explorer.CursorPath()
	if path == "" {
		return
	}

	if _, err := a.ws.Open(path); err != nil {
		a.statusBar.SetMessage("Error al abrir: " + err.Error())
		return
	}
	a.emitEvent(ext.EventDidOpenBuffer)
	a.explorerFocused = false
	a.syncStatus()
}

// revealActiveInExplorer acerca el selector al buffer activo: el bucle revela
// un nivel por pasada —Reveal dice qué dir colapsado falta → readEntries lo
// lee → ExpandDir lo deposita— hasta que el archivo queda visible y
// seleccionado, o hasta demostrar que no está en el árbol (y el cursor queda
// intacto). Sin buffer activo no hay nada que revelar. La expansión es
// perezosa: solo lee los directorios colapsados del camino; un árbol ya
// desplegado no re-lee nada. Un error de lectura es un no-op silencioso, como
// en explorerExpand.
func (a *App) revealActiveInExplorer() {
	buf := a.activeBuffer()
	if buf == nil {
		return
	}
	target := buf.Path()
	for {
		done, dir := a.explorer.Reveal(target)
		if done {
			return
		}
		entries, err := readEntries(dir)
		if err != nil {
			return
		}
		a.explorer.ExpandDir(dir, entries)
	}
}

// explorerExpand responde a ActionExpand: lee el directorio del nodo activo
// (el controlador es quien toca el filesystem: el modelo es PieceTable y
// texto, y la vista solo dibuja) y deposita sus hijos en el árbol con
// SetChildren, que marca el nodo expandido y re-aplana. Un error de lectura
// —directorio borrado entre el listado y la expansión, o sin permisos— es un
// no-op silencioso: el árbol queda como estaba y el usuario puede colapsar.
func (a *App) explorerExpand() {
	path := a.explorer.CursorPath()
	if path == "" {
		return
	}

	entries, err := readEntries(path)
	if err != nil {
		return
	}
	a.explorer.SetChildren(entries)
}

// promptDeleteEntry pide confirmación para borrar el nodo bajo el cursor del
// explorador —la rama ActionDelete y el command tcode.deleteFile—. El pedido
// es el mismo de siempre (openPrompt), con el rótulo "¿Borrar <nombre>? [s/N]"
// y la respuesta por omisión es NO: Enter sin escribir nada cancela, solo una
// "s" confirma. La ruta se captura acá, antes de que se responda: el prompt no
// depende del estado del árbol mientras se escribe.
//
// Sin nodo bajo el cursor (árbol vacío) no hay nada que borrar: no-op silencioso.
func (a *App) promptDeleteEntry() {
	path := a.explorer.CursorPath()
	if path == "" {
		return
	}
	name := filepath.Base(path)
	a.openPrompt("¿Borrar "+name+"? [s/N] ", "", func(answer string) error {
		if !isYesAnswer(answer) {
			a.statusBar.SetMessage("Cancelado")
			return nil
		}
		return a.deletePath(path)
	})
}

// isYesAnswer interpreta la respuesta del pedido de borrado: solo "s" (o "S")
// confirma —el rótulo anuncia [s/N], con N por omisión—. Cualquier otra cosa,
// incluido el Enter vacío, cancela.
func isYesAnswer(answer string) bool {
	s := strings.TrimSpace(answer)
	return s == "s" || s == "S"
}

// deletePath borra la ruta de forma PERMANENTE (os.RemoveAll: una carpeta se va
// con todo su contenido, no hay papelera) y deja la composición consistente:
//
//  1. borra en disco; si falla, el árbol y los buffers quedan como estaban;
//  2. cierra con CloseForce los buffers que apuntan a esa ruta o a algo dentro
//     de ella —un buffer sobre un archivo que ya no existe no debe quedar vivo:
//     guardarlo lo recrearía—;
//  3. saca el nodo del árbol con RemoveNode, para que desaparezca sin re-leer
//     el directorio.
//
// Es la acción pública detrás del prompt (command tcode.deleteFile): no depende
// del teclado, solo de que la ruta exista.
func (a *App) deletePath(path string) error {
	if path == "" {
		return errors.New("no hay nada que borrar")
	}
	// Las claves se calculan ANTES de borrar: después la ruta ya no existe y
	// resolver symlinks falla.
	keys := pathKeys(path)
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	a.closeBuffersUnder(keys)
	a.explorer.RemoveNode(path)
	a.tabBar.EnsureActive(a.ws, a.tabBarWidth())
	if a.ws.Len() == 0 {
		a.explorerVisible = true
		a.explorerFocused = true
	}
	a.syncStatus()
	a.statusBar.SetMessage("Borrado: " + filepath.Base(path))
	return nil
}

// pathKeys devuelve las formas normalizadas de una ruta para compararla contra
// las de los buffers —que llegan absolutas, limpias y con symlinks resueltos
// desde Workspace.Open—: la limpia y, si el sistema la puede resolver, la con
// symlinks. Comparar contra las dos cubre los dos casos sin depender de dónde
// se armó cada ruta.
func pathKeys(path string) []string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	keys := []string{filepath.Clean(abs)}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		keys = append(keys, resolved)
	}
	return keys
}

// closeBuffersUnder cierra con CloseForce toda pestaña cuya ruta es una de keys
// o está dentro de una de ellas. Se recorre de atrás hacia adelante porque cada
// cierre corre la lista hacia la izquierda, y el cierre es forzado a propósito:
// el humano ya confirmó borrar el archivo, así que sus cambios sin guardar no
// sobreviven al borrado —guardar recrearía el archivo que acaba de desaparecer—.
func (a *App) closeBuffersUnder(keys []string) {
	for i := a.ws.Len() - 1; i >= 0; i-- {
		buf := a.ws.BufferAt(i)
		if buf == nil || !matchesAnyPath(keys, buf.Path()) {
			continue
		}
		if err := a.ws.CloseForce(i); err != nil {
			continue
		}
		delete(a.editors, buf)
		delete(a.forceSave, buf)
		a.emitEvent(ext.EventDidCloseBuffer)
	}
}

// matchesAnyPath dice si la ruta de un buffer es alguna de las claves o está
// dentro de ella. El prefijo a secas no alcanza: con dir "/a", "/ab" no es
// descendiente —el carácter tras el prefijo tiene que ser un separador—.
func matchesAnyPath(keys []string, path string) bool {
	if path == "" {
		return false
	}
	for _, key := range keys {
		if path == key || isPathUnder(key, path) {
			return true
		}
	}
	return false
}

// isPathUnder dice si child está estrictamente dentro de dir.
func isPathUnder(dir, child string) bool {
	if !strings.HasPrefix(child, dir) || len(child) <= len(dir) {
		return false
	}
	switch child[len(dir)] {
	case '/', '\\':
		return true
	}
	return false
}

// isCreateFileKey reconoce Ctrl+N sin Shift: nuevo archivo. Ctrl+N llega como
// código KeyCtrl* cuando no hay modificadores, como el resto de los Ctrl.
func isCreateFileKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyCtrlN && ev.Modifiers()&tcell.ModShift == 0
}

// isCreateFolderKey reconoce Ctrl+Shift+N: nueva carpeta. Como en Ctrl+Shift+S
// y Ctrl+Shift+Z, tcell entrega los control con Shift como KeyRune en lugar del
// código KeyCtrl*.
func isCreateFolderKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyRune &&
		ev.Modifiers()&tcell.ModCtrl != 0 &&
		ev.Modifiers()&tcell.ModShift != 0 &&
		(ev.Rune() == 'n' || ev.Rune() == 'N')
}

// promptCreateEntry abre el pedido de nombre para crear un archivo (folder
// false) o una carpeta (folder true) en el destino CONTEXTUAL: el directorio
// del cursor del explorador si está sobre una carpeta, el directorio que
// contiene el archivo del cursor, o la raíz de la sesión si el árbol no tiene
// nodos. El destino se resuelve y captura acá, antes de que se escriba el
// nombre: el prompt no depende del estado del árbol mientras se tipea.
//
// El pedido arranca vacío (no hay un nombre que prellenar) y no necesita buffer
// activo: crear es justamente la acción del workspace vacío.
func (a *App) promptCreateEntry(folder bool) {
	dir := a.explorer.CursorDir()
	if dir == "" {
		dir = a.ws.Root()
	}
	label := "Nuevo archivo: "
	if folder {
		label = "Nueva carpeta: "
	}
	a.openPrompt(label, "", func(name string) error {
		return a.createEntry(dir, name, folder)
	})
}

// validateNewName aplica las reglas de nombre de la creación contextual: no
// vacío, sin separadores de ruta y sin "..". El nombre va dentro de la
// carpeta del cursor o de la raíz de la sesión —no a una ruta arbitraria—, así
// que un separador o un ".." no significan "subcarpeta" sino una salida del
// árbol.
func validateNewName(name string) error {
	switch {
	case strings.TrimSpace(name) == "":
		return errors.New("el nombre no puede estar vacío")
	case strings.ContainsAny(name, `/\`):
		return errors.New("el nombre no puede contener / ni \\")
	case strings.Contains(name, ".."):
		return errors.New("el nombre no puede contener ..")
	}
	return nil
}

// createEntry crea el archivo o la carpeta pedido en dir y lo refleja en la
// composición: el nodo nuevo entra al árbol (AddChild) y, si es un archivo,
// queda abierto y activo en el editor. El prompt ya pasó la validación del
// nombre; acá se valida de nuevo porque la acción es pública (command
// tcode.createFile) y puede llegar sin pasar por el teclado.
//
// No pisa nada existente: si la ruta ya está en disco, error legible y el árbol
// y el workspace quedan como estaban. El nodo del árbol se inserta solo si el
// padre está visible; si no (un nivel que nunca se expandió), el archivo existe
// igual y aparecerá al expandir —el contenido en disco es la fuente de verdad—.
func (a *App) createEntry(dir, name string, folder bool) error {
	if err := validateNewName(name); err != nil {
		return err
	}
	if dir == "" {
		return errors.New("no hay directorio destino")
	}
	path := filepath.Join(dir, name)
	if _, err := os.Lstat(path); err == nil {
		return errors.New("ya existe: " + name)
	} else if !os.IsNotExist(err) {
		return err
	}

	if folder {
		if err := os.Mkdir(path, 0o755); err != nil {
			return err
		}
		a.explorer.AddChild(dir, view.Entry{Name: name, Path: path, IsDir: true})
		a.statusBar.SetMessage("Carpeta creada: " + name)
		return nil
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	f.Close()

	// El archivo se abre primero: si el workspace lo rechaza, el árbol no
	// muestra un nodo que no se puede editar. Con el buffer abierto, el foco
	// pasa al editor (como al activar un archivo del árbol) y el nuevo nodo
	// entra con el orden de lectura de un relist.
	if _, err := a.ws.Open(path); err != nil {
		return err
	}
	a.emitEvent(ext.EventDidOpenBuffer)
	a.explorerFocused = false
	a.explorer.AddChild(dir, view.Entry{Name: name, Path: path})
	a.syncStatus()
	a.statusBar.SetMessage("Archivo creado: " + name)
	return nil
}

// switchTab cambia de pestaña (adelante o atrás, con wrap) y deja la
// composición consistente: desarma las confirmaciones pendientes (la de salida
// muere al seguir trabajando; la de cierre no viaja a otra pestaña), revoca los
// permisos de pisar, reencuadra la fila de pestañas y redibuja. handleEvent ya
// filtró el workspace vacío, así que acá siempre hay un buffer al que ir.
func (a *App) switchTab(move func() *model.PieceTable) {
	move()
	a.confirmQuit = false
	a.confirmClose = false
	a.confirmReload = false
	a.clearForceSave()
	a.syncStatus()
	a.tabBar.EnsureActive(a.ws, a.tabBarWidth())
	a.redraw()
}

// configRegion devuelve la región de la ventana flotante de configuración:
// 34x(ConfigMenuHeight) centrada en el área del editor (columna según el
// panel, fila tras la de pestañas), recortada si la terminal es chica (nunca
// más ancha que el editor ni más alta que su área). El alto lo decide la
// ventana (marco + todas sus filas), no un número fijo: cada fila nueva la
// agranda sola. Comparte la geometría entre toggleConfig (que solo usa el
// tamaño para Resize) y redraw (que reencuadra la superficie con la posición).
func (a *App) configRegion() (x, y, w, h int) {
	width, height := a.screen.Size()
	editorW := width - a.explorerColumn()
	menuW := 34
	if menuW > editorW {
		menuW = editorW
	}
	menuH := view.ConfigMenuHeight()
	if menuH > editorHeight(height) {
		menuH = editorHeight(height)
	}
	x = a.explorerColumn() + (editorW-menuW)/2
	y = tabBarHeight + (editorHeight(height)-menuH)/2
	w, h = menuW, menuH
	return
}

// toggleConfig abre o cierra la ventana flotante de configuración. Abrir la
// dimensiona a la región del editor (configRegion) y coloca el cursor donde
// quedó; cerrar solo apaga el flag. Sin espacio para el editor no abre: una
// ventana de ancho 0 no tendría dónde dibujarse. El caller redibuja al cerrar.
func (a *App) toggleConfig() {
	a.configActive = !a.configActive
	if a.configActive {
		width, _ := a.screen.Size()
		if width-a.explorerColumn() <= 0 {
			a.configActive = false
			return
		}
		_, _, w, h := a.configRegion()
		a.configMenu.Resize(w, h)
	}
	a.redraw()
}

// configChanged aplica en vivo un cambio de la ventana de configuración: el
// cambio pudo venir de la fila Theme, así que PRIMERO se aplica el tema con el
// id nuevo (y se re-themean los editores abiertos) y después se persiste y se
// reencuadra la composición —el tamaño del indent y el salto de palabra
// afectan a todas las vistas, y el ancho del panel cambia el del editor— con
// el mismo tratamiento que un resize. Si el cambio no fue del tema, re-aplicar
// es inofensivo. El caller redibuja.
func (a *App) configChanged() {
	a.applyTheme()
	a.saveConfig()
	a.resizeEditors()
	width, height := a.screen.Size()
	a.explorer.Resize(panelWidth(width), editorHeight(height))
	a.tabBar.EnsureActive(a.ws, a.tabBarWidth())
}

// extManagerWidth es el ancho con el que se dimensiona la ventana de
// extensiones: es más ancha que la de configuración porque las filas llevan
// referencia, versión y salto de versión. La región (extRegion) la recorta al
// ancho real del editor si la terminal es chica.
const extManagerWidth = 64

// extRegion devuelve la región de la ventana de extensiones: 64x(ExtManagerHeight)
// centrada en el área del editor (columna según el panel, fila tras la de
// pestañas), recortada si la terminal es chica. Comparte la geometría entre
// openExtManager (que solo usa el tamaño para el Resize) y redraw (que reencuadra
// la superficie con la posición), como configRegion con la de configuración.
func (a *App) extRegion() (x, y, w, h int) {
	width, height := a.screen.Size()
	editorW := width - a.explorerColumn()
	winW := min(extManagerWidth, editorW)
	winH := min(view.ExtManagerHeight(), editorHeight(height))
	return a.explorerColumn() + (editorW-winW)/2, tabBarHeight + (editorHeight(height)-winH)/2, winW, winH
}

// openExtManager abre la ventana de extensiones: la dimensiona a la región del
// editor (extRegion) y CARGA los datos recién abiertos —instaladas, actualizables,
// disponibles y proveedores—, porque leer los proveedores es una E/S con git que
// no puede correr en cada tecla. Sin espacio para el editor no abre. El caller
// redibuja (la ventana de configuración que la abre, o quien la invoque).
func (a *App) openExtManager() {
	width, _ := a.screen.Size()
	if width-a.explorerColumn() <= 0 {
		return
	}
	a.extActive = true
	// La ventana nace en su estado inicial: la pestaña donde quedó la sesión
	// anterior no debe decidir dónde empieza la próxima.
	a.extManager.Reset()
	_, _, w, h := a.extRegion()
	a.extManager.Resize(w, h)
	a.loadExtManagerData()
}

// loadExtManagerData relee las tres listas de la ventana con la misma maquinaria
// de la CLI (ext.List, ext.CheckUpdates, ext.AvailableExtensions, ext.AllProviders
// a través de extensionUserSources) y las traduce a filas de la vista. Los
// errores son TOLERANTES igual que en el arranque —un proveedor caído no impide
// ver el resto— y el último se avisa en la barra: la ventana nunca queda rota
// por una fuente caída.
//
// Los datos se releen al abrir y después de cada acción, no en cada tecla.
func (a *App) loadExtManagerData() {
	providers, userRoot, err := extensionUserSources()
	if err != nil {
		a.statusBar.SetMessage("Aviso de extensiones: " + err.Error())
		return
	}
	a.extProviders, a.extUserRoot = providers, userRoot

	var listErrs, updateErrs, availErrs []error
	a.extInstalled, listErrs = ext.List(userRoot)
	a.extUpdates, updateErrs = ext.CheckUpdates(providers, userRoot, startupExtFetch)
	// El diff de novedades va DESPUÉS de la detección de actualizaciones, como
	// en el arranque: comparar contra el conjunto ya refrescado evita ofrecer
	// como novedad algo que el proveedor acaba de actualizar.
	a.extAvailable, availErrs = ext.AvailableExtensions(providers, userRoot, startupExtFetch)

	var errs []error
	errs = append(errs, listErrs...)
	errs = append(errs, updateErrs...)
	errs = append(errs, availErrs...)
	if len(errs) > 0 {
		a.statusBar.SetMessage("Aviso de extensiones: " + errs[len(errs)-1].Error())
	}
	a.buildExtItems()
}

// buildExtItems traduce las listas cargadas a las filas de las cuatro pestañas.
// Los datos de la derecha son los que hacen legible la lista sin abrir nada: la
// versión de lo instalado, el salto "vieja → nueva" de lo actualizable y la
// marca de "(sin aprobar)" tanto en las novedades como en los proveedores —el
// modelo de confianza hecho visible en la fila—.
func (a *App) buildExtItems() {
	var installed, updatable, available, providers []view.ExtItem

	for _, i := range a.extInstalled {
		label := i.Name
		if label == "" {
			label = i.ID
		}
		installed = append(installed, view.ExtItem{
			Kind: view.ExtItemRemove, Label: label, Right: versionText(i.Version),
			ID: i.ID, Provider: i.Provider, Ref: i.Ref(),
		})
	}
	for _, u := range a.extUpdates {
		right := u.OldVer + " → " + u.NewVer
		if u.OldVer == "" {
			right = "→ " + u.NewVer
		}
		updatable = append(updatable, view.ExtItem{
			Kind: view.ExtItemUpdate, Label: u.Ref, Right: right, Ref: u.Ref,
		})
	}
	for _, av := range a.extAvailable {
		label := av.Name
		if label == "" {
			label = av.ID
		}
		right := versionText(av.Version)
		if !av.Approved() {
			right = strings.TrimSpace(right + " (sin aprobar)")
		}
		available = append(available, view.ExtItem{
			Kind: view.ExtItemInstall, Label: label, Right: right,
			ID: av.ID, Provider: av.Provider.Name, Ref: av.Ref(),
		})
	}
	for _, p := range a.extProviders {
		right := ""
		if !p.Approved {
			right = "(sin aprobar)"
		}
		providers = append(providers, view.ExtItem{Kind: view.ExtItemInfo, Label: p.Name, Right: right})
	}
	// Agregar proveedor es la ÚLTIMA fila de la pestaña, siempre presente: la
	// ventana tiene que poder sumar una fuente sin salir a la terminal.
	providers = append(providers, view.ExtItem{Kind: view.ExtItemAddProvider, Label: "+ Agregar proveedor"})

	a.extManager.SetItems(view.ExtTabInstalled, installed)
	a.extManager.SetItems(view.ExtTabUpdatable, updatable)
	a.extManager.SetItems(view.ExtTabAvailable, available)
	a.extManager.SetItems(view.ExtTabProviders, providers)
}

// versionText rotula una versión en las filas; una extensión sin versión en el
// manifest no inventa un "v" suelto.
func versionText(version string) string {
	if version == "" {
		return ""
	}
	return "v" + version
}

// handleExtIntent ejecuta la intención de la ventana: agregar proveedor o la
// acción de la fila del cursor, con la confirmación que corresponda. La ventana
// NO se cierra: la acción recarga los datos y la vista queda donde estaba. El
// caller redibuja (o el pedido que se abre ya redibuja).
func (a *App) handleExtIntent(intent view.ExtIntent) {
	switch intent.Kind {
	case view.ExtIntentAddProvider:
		a.promptAddProvider()
	case view.ExtIntentAction:
		switch intent.Tab {
		case view.ExtTabAvailable:
			a.promptInstallExtension(intent.Item)
		case view.ExtTabUpdatable:
			a.promptUpdateExtension(intent.Item)
		case view.ExtTabInstalled:
			a.promptRemoveExtension(intent.Item)
		}
	}
}

// promptInstallExtension pide confirmación antes de instalar la novedad de la
// fila. Un proveedor SIN aprobar no llega al pedido: instalar desde ahí es una
// decisión de confianza que la ventana no toma —el aviso dice cómo hacerlo por la
// CLI—, igual que el arranque no auto-instala novedades sin aprobar.
//
// La confirmación es el pedido Generalized (openPrompt) con el default NO, como
// el borrado de archivos del explorador. El id se captura ACÁ: lo que alguien
// escribe pertenece a la fila que estaba mirando.
func (a *App) promptInstallExtension(item view.ExtItem) {
	if !a.providerApproved(item.Provider) {
		a.statusBar.SetMessage("El proveedor " + item.Provider + " no está aprobado: tcode --approve-provider " + item.Provider)
		return
	}
	a.openPrompt("¿Instalar "+item.Ref+"? [s/N] ", "", func(answer string) error {
		if !isYesAnswer(answer) {
			a.statusBar.SetMessage("Cancelado")
			return nil
		}
		return a.installExtension(item)
	})
}

// installExtension instala la extensión por id con la MISMA ruta que la CLI
// (ext.InstallByID, no una reinvención): resolución por orden de proveedores y
// validación del manifest antes de tocar el destino. Los datos de la ventana se
// recargan pase lo que pase —instalar agrega o quita filas— y el error, si lo
// hay, lo muestra el pedido.
//
// Como en la CLI, la extensión recién instalada entra en la sesión SIGUIENTE:
// recargar las extensiones en caliente no es idempotente.
func (a *App) installExtension(item view.ExtItem) error {
	providers, userRoot, err := extensionUserSources()
	if err != nil {
		return err
	}
	res, err := ext.InstallByID(item.ID, providers, userRoot, startupExtFetch, nil)
	a.loadExtManagerData()
	if err != nil {
		return err
	}
	a.statusBar.SetMessage("Instalada: " + res.Ref() + " — disponible en la próxima sesión")
	return nil
}

// promptUpdateExtension pide confirmación antes de actualizar. La aplicación
// reusa ext.UpdateAll, que es el camino de la CLI: actualiza las extensiones
// cuyas versiones difieren, que —con los datos recién cargados— son exactamente
// las de la pestaña. Actualizar no vuelve a pedir confianza en el proveedor: la
// extensión instalada salió de ahí, continuarla no es agregar una fuente.
func (a *App) promptUpdateExtension(item view.ExtItem) {
	a.openPrompt("¿Actualizar "+item.Ref+"? [s/N] ", "", func(answer string) error {
		if !isYesAnswer(answer) {
			a.statusBar.SetMessage("Cancelado")
			return nil
		}
		providers, userRoot, err := extensionUserSources()
		if err != nil {
			return err
		}
		applied, errs := ext.UpdateAll(providers, userRoot, startupExtFetch)
		a.loadExtManagerData()
		msg := plural(len(applied), "actualización aplicada", "actualizaciones aplicadas")
		if len(applied) == 0 {
			msg = item.Ref + " ya estaba al día"
		}
		if len(errs) > 0 {
			msg += " — con errores: " + errs[len(errs)-1].Error()
		}
		a.statusBar.SetMessage(msg)
		return nil
	})
}

// promptRemoveExtension pide confirmación antes de borrar la extensión
// instalada de la fila. El borrado usa el camino de la CLI: RemoveNamespaced para
// lo namespaced (proveedor + id, que es como se instaló) y Remove para la
// instalación plana heredada, que no tiene proveedor.
func (a *App) promptRemoveExtension(item view.ExtItem) {
	a.openPrompt("¿Eliminar "+item.Ref+"? [s/N] ", "", func(answer string) error {
		if !isYesAnswer(answer) {
			a.statusBar.SetMessage("Cancelado")
			return nil
		}
		var err error
		if item.Provider != "" {
			err = ext.RemoveNamespaced(a.extUserRoot, item.Provider, item.ID)
		} else {
			err = ext.Remove(a.extUserRoot, item.ID)
		}
		a.loadExtManagerData()
		if err != nil {
			return err
		}
		a.statusBar.SetMessage("Eliminada: " + item.Ref)
		return nil
	})
}

// providerApproved dice si el proveedor de la fila es de confianza. Con el
// nombre vacío no hay de dónde instalar: la instalación plana heredada no se
// reinstala desde un proveedor, así que la fila no ofrece instalar.
func (a *App) providerApproved(name string) bool {
	if name == "" {
		return false
	}
	for _, p := range a.extProviders {
		if p.Name == name {
			return p.Approved
		}
	}
	return false
}

// promptAddProvider abre el pedido de texto para agregar una fuente (URL de git
// o ruta de carpeta), con el mismo openPrompt que el resto de los pedidos.
func (a *App) promptAddProvider() {
	a.openPrompt("Agregar proveedor (URL o carpeta): ", "", func(source string) error {
		return a.addProviderSource(source)
	})
}

// addProviderSource registra la fuente escrita en el pedido. Es el mismo camino
// que --add-provider de la CLI, con sus mismas reglas: tiene que ser una URL o
// una carpeta existente (si no, es un error de tipeo y se dice), el nombre se
// deriva de la fuente y no puede duplicar uno ya registrado (el nombre es la
// identidad del namespacing), y se guarda SIN aprobar: agregar una fuente no es
// confiar en ella. La validación de la forma del proveedor es la misma de la CLI
// (ValidateProviderSource, solo manifests), así que agregar no valida una ilusión
// que después no se sostiene.
func (a *App) addProviderSource(source string) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return errors.New("la fuente del proveedor no puede estar vacía")
	}
	if !ext.IsLocalSource(source) && !ext.IsRemoteSource(source) {
		return fmt.Errorf("%q no es una URL de git ni una carpeta existente", source)
	}
	path, err := providersConfigPath()
	if err != nil {
		return err
	}
	name, err := ext.DeriveName(source)
	if err != nil {
		return err
	}
	canonical, err := ext.CanonicalSource(source)
	if err != nil {
		return err
	}
	p := ext.Provider{Name: name, Source: canonical, Approved: false}

	stored, err := ext.LoadProviders(path)
	if err != nil {
		return err
	}
	for _, s := range stored {
		if s.Name == name {
			return fmt.Errorf("ya está registrado el proveedor %q", name)
		}
	}
	if p.Name == ext.DefaultProvider().Name {
		return fmt.Errorf("%q ya es el proveedor por defecto", name)
	}
	if _, err := ext.ValidateProviderSource(p, startupExtFetch); err != nil {
		return err
	}
	if err := ext.SaveProviders(path, append(stored, p)); err != nil {
		return err
	}
	// La ventana se recarga para que el proveedor nuevo aparezca en su lista,
	// marcado como sin aprobar.
	a.loadExtManagerData()
	a.statusBar.SetMessage("Proveedor agregado (sin aprobar): " + name)
	return nil
}

// toggleMenu abre o cierra el menú de pestañas. Abrir lo dimensiona a la
// región del editor (ancho según el panel, alto = el de las pestañas listadas,
// nunca más que el editor) y coloca el cursor sobre la activa; cerrar solo
// apaga el flag. handleEvent ya filtró el workspace vacío, así que acá siempre
// hay algo que listar cuando se abre.
func (a *App) toggleMenu() {
	a.menuActive = !a.menuActive
	if a.menuActive {
		width, height := a.screen.Size()
		a.menu.Resize(width-a.explorerColumn(), min(a.ws.Len(), editorHeight(height)))
		a.menu.Open(a.ws)
	}
	a.redraw()
}

// menuSwitchTab cambia a la pestaña elegida del menú (Enter) y deja la
// composición consistente como switchTab: desarma las confirmaciones y el
// permiso de pisar, reencuadra la fila de pestañas y cierra el menú. Elegir la
// pestaña ya activa es no-op salvo por el cierre.
func (a *App) menuSwitchTab() {
	a.menuActive = false
	a.activateTab(a.menu.Selected())
}

// activateTab hace activa la pestaña del índice idx y deja la composición
// consistente: desarma las confirmaciones (cambiar de pestaña es "seguir
// trabajando"), revoca el permiso de pisar, reencuadra el strip y sincroniza la
// barra. Es el camino compartido del menú (Ctrl+T), del clic en una pestaña y
// de la rueda sobre la fila de pestañas.
func (a *App) activateTab(idx int) {
	if idx < 0 || idx >= a.ws.Len() {
		return
	}
	if idx != a.ws.ActiveIndex() {
		if err := a.ws.SetActive(idx); err != nil {
			return
		}
	}
	a.confirmQuit = false
	a.confirmClose = false
	a.clearForceSave()
	a.tabBar.EnsureActive(a.ws, a.tabBarWidth())
	a.syncStatus()
}

// sessionPath es la ubicación fija de la sesión de un root de trabajo.
func sessionPath(root string) string {
	return filepath.Join(root, ".tcode", "session.json")
}

// loadSession restaura la sesión del root al arrancar (solo en los modos con
// sesión habilitada). Nunca es fatal: sin archivo o con JSON corrupto no hace
// nada y el estado por defecto —explorador sin buffers— queda como respaldo.
// Las pestañas que ya no existen se saltan (ws.Open falla → seguir) y la
// activa se resuelve por ruta DESPUÉS de restaurar, sobre las que quedaron;
// si su ruta no está entre ellas, queda la última abierta.
func (a *App) loadSession() {
	if !a.sessionEnabled {
		return
	}
	s, err := model.LoadSession(sessionPath(a.ws.Root()))
	if err != nil {
		// Sin sesión previa (os.IsNotExist) o JSON corrupto: estado por defecto.
		return
	}
	for _, p := range s.Tabs {
		a.ws.Open(p)
	}
	if s.Active != "" {
		for i, b := range a.ws.Buffers() {
			if b.Path() == s.Active {
				a.ws.SetActive(i)
				break
			}
		}
	}
}

// saveSession persiste la sesión al salir (solo en los modos con sesión
// habilitada). Persiste las rutas de los buffers con archivo, en orden, y la
// activa como ruta (o "" si no tiene). Sin pestañas persistibles el archivo se
// BORRA: guardar {tabs: []} dejaría un estado mentiroso, y el por defecto de un
// directorio es sin sesión. Corre en el camino de salida (defer de Run): un
// error se ignora —no vale impedir cerrar el editor por no poder escribir la
// sesión—.
func (a *App) saveSession() {
	if !a.sessionEnabled {
		return
	}
	tabs := make([]string, 0, a.ws.Len())
	for _, b := range a.ws.Buffers() {
		if p := b.Path(); p != "" {
			tabs = append(tabs, p)
		}
	}
	active := ""
	if b := a.ws.Active(); b != nil {
		active = b.Path()
	}
	path := sessionPath(a.ws.Root())
	if len(tabs) == 0 {
		os.Remove(path)
		return
	}
	model.SaveSession(path, model.Session{
		Version: 1,
		Root:    a.ws.Root(),
		Tabs:    tabs,
		Active:  active,
	})
}

// closeTab cierra la pestaña activa con la misma confirmación no modal que la
// salida: sobre una pestaña con cambios sin guardar, la primera vez solo avisa
// y arma la confirmación; la segunda cierra sin guardar. Cualquier otra tecla o
// cambio de pestaña la desarma. El modelo sigue siendo la única fuente de
// verdad sobre el estado sucio: se consulta Close y solo se usa CloseForce
// después de que el humano confirmó.
func (a *App) closeTab() {
	buf := a.activeBuffer()
	err := a.ws.Close(a.ws.ActiveIndex())
	switch {
	case errors.Is(err, model.ErrBufferModified) && !a.confirmClose:
		a.confirmClose = true
		a.statusBar.SetMessage("Cambios sin guardar: Ctrl+S guarda, Ctrl+W de nuevo cierra esta pestaña")
		a.redraw()
		return
	case errors.Is(err, model.ErrBufferModified):
		err = a.ws.CloseForce(a.ws.ActiveIndex())
	}
	if err != nil {
		a.statusBar.SetMessage("Error al cerrar: " + err.Error())
		a.redraw()
		return
	}

	// El buffer cerró: su vista y su permiso de pisar dejan de existir. La
	// entrada vieja del mapa apuntaría a un PieceTable ya desmapeado, y el
	// permiso autorizó a un archivo que ya no está abierto.
	delete(a.editors, buf)
	delete(a.forceSave, buf)
	a.emitEvent(ext.EventDidCloseBuffer)
	a.confirmClose = false
	a.statusBar.ClearMessage()
	// Cerrar la ÚLTIMA pestaña deja el workspace en el estado a propósito
	// vacío: el foco pasa al explorador —visible aunque estuviera oculto—,
	// como en el arranque sobre un directorio. Sin buffers no hay documento
	// que editar, y el árbol es el destino natural del teclado para dirigirse
	// a otro archivo; si el foco quedara en el editor vacío, el guard de
	// workspace vacío dejaría las teclas muertas salvo salir. Cerrar una
	// pestaña que no es la última no toca el foco ni la visibilidad.
	if a.ws.Len() == 0 {
		a.explorerVisible = true
		a.explorerFocused = true
	}
	a.tabBar.EnsureActive(a.ws, a.tabBarWidth())
	a.syncStatus()
	a.redraw()
}
