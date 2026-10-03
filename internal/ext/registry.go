package ext

import (
	"errors"
	"fmt"
	"sync"
)

// ErrUnknownCommand envuelve el error de Run cuando el id no está registrado.
// El controlador lo distingue con errors.Is para traducirlo a un mensaje de
// estado en lugar de un error fatal.
var ErrUnknownCommand = errors.New("comando desconocido")

// Registry es la tabla central de comandos. El controlador registra los
// built-ins tcode.* contra acciones existentes; las extensiones declaran
// comandos que un futuro backend de scripting registraría aquí, y un
// keybinding solo puede apuntar a un comando registrado. El registro es
// tolerante a escritura concurrente (RWMutex): el despacho corre en el bucle
// de eventos, pero un backend async no debe poder corromper el mapa.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]func() error
}

// NewRegistry crea un registro vacío.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]func() error)}
}

// Register asocia un id a su handler. Rechaza ids que el manifest no aceptaría
// (misma gramática) y se niega a pisar un id ya registrado: el primer
// registro gana y un re-registro accidental es un error.
func (r *Registry) Register(id string, fn func() error) error {
	if err := validateID(id); err != nil {
		return fmt.Errorf("registro: %w", err)
	}
	if fn == nil {
		return fmt.Errorf("registro: %s sin handler", id)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.handlers[id]; ok {
		return fmt.Errorf("registro: %s ya registrado", id)
	}
	r.handlers[id] = fn
	return nil
}

// Has pregunta si un comando está registrado, sin ejecutarlo.
func (r *Registry) Has(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.handlers[id]
	return ok
}

// Run ejecuta el handler del id. Si el id no existe devuelve un error que
// envuelve ErrUnknownCommand y nombra el comando; si el handler falla, su
// error llega intacto al llamador.
func (r *Registry) Run(id string) error {
	r.mu.RLock()
	fn, ok := r.handlers[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownCommand, id)
	}
	return fn()
}
