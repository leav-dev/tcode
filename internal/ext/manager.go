package ext

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Manager es el punto de entrada del sistema de extensiones desde el
// controlador. Une las piezas: registra los stubs de comandos declarados en el
// Registry (que el controlador comparte con sus built-ins tcode.*), congela
// los keybindings de todas las extensiones en un Keymap estático (como VSCode,
// las contribuciones de teclado resuelven aunque la extensión no se haya
// activado), activa extensiones por evento y despacha los hooks evento→comando
// si la extensión está activa.
type Manager struct {
	registry *Registry
	keymap   *Keymap
	states   []*extState

	// editor es el puente hacia el editor (ScriptAPI): lo inyecta el
	// controlador con SetEditor al arrancar. Sin editor, los comandos con
	// script fallan con un error claro en lugar de tocar un nil.
	editor ScriptAPI
	// scriptHosts cachea un ScriptHost por script de extensión (key =
	// e.Dir + "::" + cmd.Script): el estado Lua sobrevive entre invocaciones
	// del comando y el archivo se lee de disco una sola vez.
	scriptHosts map[string]*ScriptHost
	// scriptDepth es el guard de reentrancia del scripting: un script que
	// invoca tcode.command sobre un comando con script no puede recurrir
	// hasta el stack overflow. Como el dispatch es de una sola goroutine, no
	// necesita sincronización (igual que emitting).
	scriptDepth int

	// emitting es el guard de reentrancia de Emit: el bucle de eventos es una
	// sola goroutine, y un hook cuyo comando vuelve a emitir el mismo evento
	// (p. ej. tcode.closeTab en onDidCloseBuffer) debe cortarse, no recurrir
	// hasta el stack overflow.
	emitting bool
}

// SetEditor inyecta el puente hacia el editor (ScriptAPI) que los scripts de
// extensión usan vía tcode.*. El controlador lo llama con el App al arrancar;
// hasta entonces, un comando con script falla con "backend de scripting sin
// editor" en lugar de tocar un nil.
func (m *Manager) SetEditor(api ScriptAPI) { m.editor = api }

// extState es una extensión descubierta con su estado de activación. En el
// milestone declarativo, activarse habilita sus hooks; es también la costura
// donde el futuro backend de scripting cargaría el módulo de la extensión.
type extState struct {
	ext    Extension
	active bool
}

// NewManager crea un Manager con su registro y keymap vacíos. El controlador
// registra los built-ins vía Registry(), agrega las extensiones descubiertas
// con AddExtensions y cierra el arranque con ActivateEvent(onStartup).
func NewManager() *Manager {
	return &Manager{registry: NewRegistry(), scriptHosts: make(map[string]*ScriptHost)}
}

// Registry expone el registro de comandos compartido.
func (m *Manager) Registry() *Registry { return m.registry }

// AddExtensions registra las extensiones: sus comandos declarados se vuelven
// stubs en el registro (ejecutarlos activa la extensión si declara
// onCommand:<id> y avisa que falta el backend de scripting), y sus
// keybindings se congelan en el keymap. Los duplicados conservan el primero.
func (m *Manager) AddExtensions(exts []Extension) {
	var bindings []Keybinding
	for i := range exts {
		e := &extState{ext: exts[i]}
		m.states = append(m.states, e)
		mid := e.ext.Manifest.ID
		for _, c := range e.ext.Manifest.Contributes.Commands {
			cmd := c // copia local: el closure no captura la variable del rango
			if cmd.Script != "" {
				// Con scripting, el comando delega en la función Lua del
				// script de la extensión (el host se cachea por script).
				m.registry.Register(cmd.ID, func() error {
					m.activateIfDeclared(mid, ActivateCommand+cmd.ID)
					return m.runScriptCommand(e.ext, cmd)
				})
				continue
			}
			// El stub activa la extensión si ella lo declara, luego informa la
			// ausencia de backend: es la verdad observable de este milestone.
			m.registry.Register(cmd.ID, func() error {
				m.activateIfDeclared(mid, ActivateCommand+cmd.ID)
				return fmt.Errorf("%s: comando declarado sin implementación (roadmap: backend de scripting)", cmd.ID)
			})
		}
		bindings = append(bindings, e.ext.Manifest.Contributes.Keybindings...)
	}
	m.keymap = buildKeymap(bindings)
}

// Resolve clasifica un evento de teclado contra los keybindings declarados por
// todas las extensiones y devuelve el comando a ejecutar, o "".
func (m *Manager) Resolve(ev *tcell.EventKey) string {
	if m.keymap == nil {
		return ""
	}
	return m.keymap.Resolve(ev)
}

// RunCommand ejecuta un comando por id, activando primero la extensión dueña si
// declara onCommand:<id>. Un comando desconocido devuelve ErrUnknownCommand.
func (m *Manager) RunCommand(id string) error {
	for _, s := range m.states {
		m.activateIfDeclared(s.ext.Manifest.ID, ActivateCommand+id)
		if s.active {
			break
		}
	}
	return m.registry.Run(id)
}

// runScriptCommand ejecuta un comando declarado con scripting: consigue (o
// crea, cacheando) el ScriptHost del script de la extensión y llama a la
// función Lua que implementa el comando. El guard de profundidad corta la
// recursión de un script que se invoca vía tcode.command; un host solo se
// construye con el editor presente (el puente al App), y todo error se
// envuelve nombrando el comando.
func (m *Manager) runScriptCommand(e Extension, cmd Command) error {
	if m.scriptDepth >= 8 {
		return fmt.Errorf("script: recursión excedida ejecutando %s (límite 8)", cmd.ID)
	}
	m.scriptDepth++
	defer func() { m.scriptDepth-- }()

	key := e.Dir + "::" + cmd.Script
	h, ok := m.scriptHosts[key]
	if !ok {
		if m.editor == nil {
			return fmt.Errorf("%s: backend de scripting sin editor", cmd.ID)
		}
		code, err := os.ReadFile(filepath.Join(e.Dir, cmd.Script))
		if err != nil {
			return fmt.Errorf("%s: script: %w", cmd.ID, err)
		}
		h, err = NewScriptHost(string(code), m.editor, key)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd.ID, err)
		}
		m.scriptHosts[key] = h
	}
	if err := h.Call(cmd.Fn); err != nil {
		return fmt.Errorf("%s: %v", cmd.ID, err)
	}
	return nil
}

// ActivateEvent activa todas las extensiones inactivas que declaran el evento
// (o "*") y devuelve cuántas activó. El controlador lo llama con onStartup al
// arranque y con onDidOpenBuffer al abrir un buffer.
func (m *Manager) ActivateEvent(event string) int {
	n := 0
	for _, s := range m.states {
		if s.active {
			continue
		}
		if declares(s.ext.Manifest, event) {
			s.active = true
			n++
		}
	}
	return n
}

// Emit despacha un evento de buffer: primero activa las extensiones que lo
// declaran (abrir un buffer despierta a las perezosas), después corre los
// hooks evento→comando de las extensiones activas en orden de descubrimiento.
// Los errores de los hooks se acumulan y devuelven para que el controlador los
// muestre; un hook roto no corta a los demás.
//
// Reentrancia: un hook cuya comando emite de nuevo el mismo evento (un
// onDidCloseBuffer que invoca tcode.closeTab) no debe recurrir. El guard
// corta el Emit anidado y devuelve sin efecto; el dispatch es de una sola
// goroutine, así que el guard no necesita sincronización.
func (m *Manager) Emit(event string) []error {
	if m.emitting {
		return nil
	}
	m.emitting = true
	defer func() { m.emitting = false }()

	m.ActivateEvent(event)
	var errs []error
	for _, s := range m.states {
		if !s.active {
			continue
		}
		for _, h := range s.ext.Manifest.Contributes.Hooks {
			if h.Event != event {
				continue
			}
			if err := m.RunCommand(h.Command); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s.ext.Manifest.ID, err))
			}
		}
	}
	return errs
}

// Active pregunta si una extensión (por id de manifest) está activa.
func (m *Manager) Active(id string) bool {
	for _, s := range m.states {
		if s.ext.Manifest.ID == id {
			return s.active
		}
	}
	return false
}

// ActiveCount reporta cuántas extensiones están activas (para tests y estado).
func (m *Manager) ActiveCount() int {
	n := 0
	for _, s := range m.states {
		if s.active {
			n++
		}
	}
	return n
}

// activateIfDeclared activa la extensión si declara event o "*".
func (m *Manager) activateIfDeclared(id, event string) {
	for _, s := range m.states {
		if s.ext.Manifest.ID == id && !s.active && declares(s.ext.Manifest, event) {
			s.active = true
			return
		}
	}
}

// declares dice si el manifest activa con el evento dado o con "*".
func declares(mf *Manifest, event string) bool {
	for _, ev := range mf.Activation {
		if ev == ActivateAlways || ev == event {
			return true
		}
		if strings.HasPrefix(ev, ActivateCommand) && ev == event {
			return true
		}
	}
	return false
}
