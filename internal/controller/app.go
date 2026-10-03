package controller

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/gdamore/tcell/v2"
	"tcode/internal/ext"
	"tcode/internal/model"
	"tcode/internal/view"
)

// statusHeight es la cantidad de filas que ocupa la barra de estado.
// tabBarHeight es la que ocupa la fila de pestañas. explorerWidth es el ancho
// máximo del panel lateral del explorador: el editor conserva el resto.
const (
	statusHeight  = 1
	tabBarHeight  = 1
	explorerWidth = 24
)

// panelWidth es el ancho del panel lateral para una pantalla de ancho
// columnas, con la fórmula literal min(explorerWidth, max(1, ancho-16)): el
// editor conserva al menos 16 columnas, el panel nunca desaparece (mínimo 1) y
// el valor es determinista para los tests (80 → 24, 30 → 14, 20 → 4).
func panelWidth(ancho int) int {
	return min(explorerWidth, max(1, ancho-16))
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

	confirmQuit  bool
	confirmClose bool

	// forceSave es el permiso de pisar cambios externos, por buffer: autorizar
	// sobrescribir UN archivo no tiene que valer para otro.
	forceSave map[*model.PieceTable]bool

	// promptActive, promptBuf y promptTarget sostienen el pedido de texto de
	// Save As. Mientras está activo, el teclado alimenta el pedido y no el
	// documento. El buffer destino se captura al ABRIR el pedido: el texto que
	// alguien escribe pertenece al documento que estaba mirando cuando empezó,
	// no al que esté activo cuando aprieta Enter.
	promptActive bool
	promptBuf    string
	promptTarget *model.PieceTable

	// menu es el superpuesto transitorio de pestañas (Ctrl+T) y menuActive dice
	// si está abierto. Mientras está activo, el menú posee el teclado y el mouse
	// —como el pedido de Save As—: Enter activa la pestaña del cursor y cierra,
	// y toda otra tecla (Escape incluido) cierra descartando.
	menu       *view.TabMenu
	menuActive bool

	// sessionEnabled marca los modos con sesión persistida (directorio o sin
	// argumentos): el modo archivo explícito no guarda ni restaura. Se define
	// en el arranque y cubre tanto el guardado como la restauración.
	sessionEnabled bool

	// ext es el sistema de extensiones: registro de comandos compartido
	// (built-ins tcode.* más stubs declarados), keybindings y bus de hooks.
	// handleEvent resuelve las teclas de extensión después de los atajos del
	// núcleo; los hooks se emiten desde open/save/close.
	ext *ext.Manager

	// extensionRoots son los directorios donde se buscan extensiones, en orden
	// de precedencia: el primero gana en caso de ids duplicados. Por defecto,
	// las del usuario y las del proyecto actual; los tests los reemplazan.
	extensionRoots []string
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
		screen:    s,
		ws:        ws,
		editors:   make(map[*model.PieceTable]*view.EditorView),
		forceSave: make(map[*model.PieceTable]bool),
		statusBar: view.NewStatusBar(),
		tabBar:    view.NewTabBar(),
		explorer:  view.NewFileBrowser(),
		menu:      view.NewTabMenu(),
		ext:       ext.NewManager(),

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
	app.loadExtensions()

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
		// Con un pedido activo el teclado es del pedido, no del documento.
		if a.promptActive {
			a.handlePromptKey(ev)
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
		// del dir del cursor (SetChildren es la única E/S de expansión). Lo no
		// consumido cae al flujo normal: atajos, documento y salida.
		if a.explorerVisible && a.explorerFocused {
			action, handled := a.explorer.HandleEvent(ev)
			if handled {
				// Navegar el panel es "seguir trabajando": desarma las
				// confirmaciones pendientes y el permiso de pisar, como
				// cualquier otra tecla del documento —si no, un Escape armado
				// por error seguiría activo tras navegar el árbol—.
				a.confirmQuit = false
				a.confirmClose = false
				a.clearForceSave()
				a.statusBar.ClearMessage()
				switch action {
				case view.ActionActivate:
					a.activateExplorerEntry()
				case view.ActionExpand:
					a.explorerExpand()
				}
				a.redraw()
				return false
			}
		}

		// Tab devuelve el foco al editor SOLO cuando el explorador lo tiene:
		// con el foco en el editor, Tab sigue insertando tabulación en el
		// documento —la edición no pierde su tecla más básica por tener el
		// panel a la vista—. Volver al panel desde el editor es con clic en
		// el panel o re-mostrándolo con Ctrl+B. Va antes del guard porque
		// mover el foco no toca ningún buffer: con el workspace vacío también
		// tiene que funcionar.
		if ev.Key() == tcell.KeyTab && a.explorerVisible && a.explorerFocused {
			a.explorerFocused = false
			a.confirmQuit = false
			a.confirmClose = false
			a.clearForceSave()
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

		// Workspace vacío: no hay nada que editar, guardar ni deshacer. Salir
		// sigue funcionando, y sin buffers no hay nada que perder.
		buf := a.activeBuffer()
		if buf == nil {
			return ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC
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

		case ev.Key() == tcell.KeyCtrlW:
			a.closeTab()
			return false

		case ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC:
			// Salir con cambios sin guardar en CUALQUIER buffer pide
			// confirmación: la primera vez solo se avisa, así una tecla de más
			// no tira el trabajo de ninguna pestaña.
			if !a.ws.AnyModified() || a.confirmQuit {
				return true
			}
			a.confirmQuit = true
			a.statusBar.SetMessage("Cambios sin guardar: Ctrl+S guarda, Escape de nuevo sale igual")
			a.redraw()
			return false
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
		// salida y la de cierre de pestaña— y el permiso de pisar que se haya
		// dado con un Ctrl+S previo.
		a.confirmQuit = false
		a.confirmClose = false
		a.clearForceSave()
		a.statusBar.ClearMessage()
		if ed := a.activeEditor(); ed != nil && ed.HandleEvent(ev) {
			a.redraw()
		}

	case *tcell.EventMouse:
		// Con el menú abierto el mouse es del menú como el teclado, y con un
		// pedido activo (Save As) el mouse es del pedido: se ignora por completo
		// —el controlador no traduce nada ni redibuja— y el clic no puede
		// cambiar de pestaña, seleccionar un archivo ni raspar el documento por
		// debajo de lo que el usuario está escribiendo.
		if a.menuActive || a.promptActive {
			return false
		}

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

// isUndoKey reconoce Ctrl+Z sin modificadores.
func isUndoKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyCtrlZ && ev.Modifiers()&tcell.ModShift == 0
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

// startPrompt abre el pedido de Save As, prellenado con la ruta actual para poder
// editarla en lugar de reescribirla entera. El buffer destino queda capturado
// acá: no depende de qué pestaña esté activa cuando se termine de tipear.
func (a *App) startPrompt() {
	a.confirmQuit = false
	a.confirmClose = false
	a.clearForceSave()
	a.promptActive = true
	a.promptBuf = a.activeBuffer().Path()
	a.promptTarget = a.activeBuffer()
	a.refreshPrompt()
	a.redraw()
}

func (a *App) refreshPrompt() {
	a.statusBar.SetPrompt("Guardar como: " + a.promptBuf)
}

func (a *App) endPrompt() {
	a.promptActive = false
	a.promptBuf = ""
	a.promptTarget = nil
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
// grapheme cluster: alcanza para rutas, que son ASCII en la práctica.
func (a *App) handlePromptKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		a.endPrompt()
		a.statusBar.SetMessage("Save As cancelado")
		a.redraw()
		return

	case tcell.KeyEnter:
		path := strings.TrimSpace(a.promptBuf)
		target := a.promptTarget
		a.endPrompt()
		a.saveAs(target, path)
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

	if err := target.SaveAs(path); err != nil {
		a.statusBar.SetMessage("Error al guardar como: " + err.Error())
	} else {
		a.confirmQuit = false
		a.clearForceSave()
		a.statusBar.SetMessage("Guardado en " + filepath.Base(path))
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

	a.statusBar.Draw(a.screen, height-statusHeight, width)
	a.screen.Show()
}

// toggleExplorer muestra u oculta el panel lateral. Mostrar enfoca el
// explorador; ocultar lo desenfoca (el foco queda en el editor). En ambos casos
// el ancho del editor cambia, así que todas las vistas reciben el mismo
// tratamiento que un resize —sin eso, la vista de otra pestaña dibujaría con
// el ancho viejo al volver— y se redibuja.
func (a *App) toggleExplorer() {
	a.explorerVisible = !a.explorerVisible
	a.explorerFocused = a.explorerVisible
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

// switchTab cambia de pestaña (adelante o atrás, con wrap) y deja la
// composición consistente: desarma las confirmaciones pendientes (la de salida
// muere al seguir trabajando; la de cierre no viaja a otra pestaña), revoca los
// permisos de pisar, reencuadra la fila de pestañas y redibuja. handleEvent ya
// filtró el workspace vacío, así que acá siempre hay un buffer al que ir.
func (a *App) switchTab(move func() *model.PieceTable) {
	move()
	a.confirmQuit = false
	a.confirmClose = false
	a.clearForceSave()
	a.syncStatus()
	a.tabBar.EnsureActive(a.ws, a.tabBarWidth())
	a.redraw()
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
	a.tabBar.EnsureActive(a.ws, a.tabBarWidth())
	a.syncStatus()
	a.redraw()
}
