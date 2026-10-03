package ext

import (
	"fmt"
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
	// emitting es el guard de reentrancia de Emit: el bucle de eventos es una
	// sola goroutine, y un hook cuyo comando vuelve a emitir el mismo evento
	// (p. ej. tcode.closeTab en onDidCloseBuffer) debe cortarse, no recurrir
	// hasta el stack overflow.
	emitting bool
}

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
	return &Manager{registry: NewRegistry()}
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
			cmdID := c.ID
			// El stub activa la extensión si ella lo declara, luego informa la
			// ausencia de backend: es la verdad observable de este milestone.
			m.registry.Register(cmdID, func() error {
				m.activateIfDeclared(mid, ActivateCommand+cmdID)
				return fmt.Errorf("%s: comando declarado sin implementación (roadmap: backend de scripting)", cmdID)
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
