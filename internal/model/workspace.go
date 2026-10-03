package model

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Errores del workspace.
var (
	// ErrBufferModified se devuelve al cerrar un buffer con cambios sin guardar.
	// El cierre forzado (CloseForce) ignora este chequeo.
	ErrBufferModified = errors.New("el buffer tiene cambios sin guardar")

	// ErrIsDirectory se devuelve al abrir una ruta que existe pero es un directorio.
	ErrIsDirectory = errors.New("la ruta es un directorio")

	// ErrBufferOutOfRange se devuelve al direccionar un buffer por índice fuera
	// de rango.
	ErrBufferOutOfRange = errors.New("índice de buffer fuera de rango")

	// ErrEmptyOpenPath se devuelve al abrir con una ruta vacía o solo espacios.
	ErrEmptyOpenPath = errors.New("Open necesita una ruta")
)

// Workspace agrupa los buffers abiertos de la sesión, cual pestañas de un
// editor. Solo sabe de datos: no guarda cursor, viewport ni estado de terminal
// (eso vive en la vista y el controlador).
//
// El orden de apertura es el orden de las pestañas; el índice activo es la
// pestaña que se muestra. Abrir una ruta ya abierta no duplica el buffer, solo
// lo activa.
type Workspace struct {
	root     string        // Directorio raíz para el explorador de archivos.
	buffers  []*PieceTable // Pestañas en orden de apertura.
	activeAt int           // Índice del buffer activo, o -1 si está vacío.
}

// NewWorkspace crea un workspace vacío sin raíz asignada.
func NewWorkspace() *Workspace {
	return &Workspace{activeAt: -1}
}

// Root devuelve el directorio raíz registrado, o "" si no hay ninguno.
func (w *Workspace) Root() string { return w.root }

// SetRoot registra el directorio raíz que usa el explorador de archivos. Es
// estado asociado a la sesión, no una operación de buffer.
func (w *Workspace) SetRoot(dir string) error { w.root = dir; return nil }

// Len devuelve cuántos buffers hay abiertos.
func (w *Workspace) Len() int { return len(w.buffers) }

// Buffers devuelve una copia del slice de buffers en orden de pestañas.
// Mutarla no afecta al workspace.
func (w *Workspace) Buffers() []*PieceTable {
	out := make([]*PieceTable, len(w.buffers))
	copy(out, w.buffers)
	return out
}

// Active devuelve el buffer activo, o nil si el workspace está vacío.
func (w *Workspace) Active() *PieceTable {
	if w.activeAt < 0 || w.activeAt >= len(w.buffers) {
		return nil
	}
	return w.buffers[w.activeAt]
}

// ActiveIndex devuelve el índice del buffer activo, o -1 si está vacío.
func (w *Workspace) ActiveIndex() int { return w.activeAt }

// SetActive hace activo el buffer en el índice i.
func (w *Workspace) SetActive(i int) error {
	if i < 0 || i >= len(w.buffers) {
		return ErrBufferOutOfRange
	}
	w.activeAt = i
	return nil
}

// Next activa el siguiente buffer circularmente (con wrap-around) y lo devuelve.
// Sobre un workspace vacío devuelve nil sin cambiar nada.
func (w *Workspace) Next() *PieceTable {
	if len(w.buffers) == 0 {
		return nil
	}
	w.activeAt = (w.activeAt + 1) % len(w.buffers)
	return w.buffers[w.activeAt]
}

// Prev activa el buffer anterior circularmente (con wrap-around) y lo devuelve.
// Sobre un workspace vacío devuelve nil sin cambiar nada.
func (w *Workspace) Prev() *PieceTable {
	if len(w.buffers) == 0 {
		return nil
	}
	w.activeAt = (w.activeAt - 1 + len(w.buffers)) % len(w.buffers)
	return w.buffers[w.activeAt]
}

// AnyModified dice si ALGÚN buffer abierto tiene cambios sin guardar, no solo
// el activo.
func (w *Workspace) AnyModified() bool {
	for _, b := range w.buffers {
		if b.Modified() {
			return true
		}
	}
	return false
}

// normalizePath resuelve la ruta como clave de deduplicación: absoluta, limpia
// y con symlinks resueltos cuando el sistema lo permite. Dos formas de escribir
// la misma ruta terminan en la misma clave.
func normalizePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(abs)
	// Si la ruta no existe todavía, EvalSymlinks falla y queda la limpia.
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		return resolved, nil
	}
	return clean, nil
}

// Open carga path como buffer y lo activa. Si la ruta ya está abierta (comparada
// con symlinks resueltos) devuelve el buffer existente sin duplicarlo, sin
// cambiar el orden; solo lo activa.
func (w *Workspace) Open(path string) (*PieceTable, error) {
	if strings.TrimSpace(path) == "" {
		return nil, ErrEmptyOpenPath
	}

	key, err := normalizePath(path)
	if err != nil {
		return nil, err
	}

	// Ya está abierto: activarlo y devolverlo tal cual.
	for i, b := range w.buffers {
		if b.path == key {
			w.activeAt = i
			return b, nil
		}
	}

	// Existe pero es un directorio: no entra al workspace.
	if info, err := os.Stat(key); err == nil && info.IsDir() {
		return nil, ErrIsDirectory
	}

	// Archivo nuevo: crear la tabla, cargarla y solo entonces tocar el workspace.
	// Si la carga falla, el workspace tiene que quedar intacto.
	pt := NewPieceTable()
	if err := pt.LoadFile(key); err != nil {
		pt.Close() // Libera lo que LoadFile haya podido mapear antes de fallar.
		return nil, err
	}
	w.buffers = append(w.buffers, pt)
	w.activeAt = len(w.buffers) - 1
	return pt, nil
}

// Close cierra el buffer del índice i y libera sus recursos. Se niega si el
// buffer tiene cambios sin guardar (ErrBufferModified); usar CloseForce para
// ignorar ese chequeo.
//
// Cerrar una pestaña nunca cambia de archivo por sorpresa: la activa sigue
// siendo la misma siempre que siga abierta (corrida a la izquierda si se cerró
// una anterior). Si se cerró la activa, pasa al frente la que ocupó su lugar, o
// la nueva última si era la última. Con el workspace vacío, el índice es -1.
func (w *Workspace) Close(i int) error {
	return w.closeImpl(i, false)
}

// CloseForce cierra el buffer del índice i sin mirar el flag de modificación.
// Es la variante para cuando el humano ya confirmó descartar los cambios.
func (w *Workspace) CloseForce(i int) error {
	return w.closeImpl(i, true)
}

func (w *Workspace) closeImpl(i int, force bool) error {
	if i < 0 || i >= len(w.buffers) {
		return ErrBufferOutOfRange
	}
	if !force && w.buffers[i].Modified() {
		return ErrBufferModified
	}

	w.buffers[i].Close()
	w.buffers = append(w.buffers[:i], w.buffers[i+1:]...)

	switch {
	case len(w.buffers) == 0:
		w.activeAt = -1
	case i < w.activeAt:
		// Se cerró una pestaña anterior a la activa: el mismo archivo sigue
		// al frente, solo se corrió una posición a la izquierda.
		w.activeAt--
	case i == w.activeAt:
		// Se cerró la activa: pasa al frente la que quedó en ese índice, o la
		// nueva última si la activa era la última.
		w.activeAt = min(i, len(w.buffers)-1)
	}
	// i > activeAt: se cerró una pestaña posterior; la activa no se mueve.
	return nil
}

// CloseAll libera todos los buffers y vacía el workspace SIN mirar el flag de
// modificación: es el camino de apagado, para cuando el humano ya confirmó
// descartar los cambios de todos los buffers.
func (w *Workspace) CloseAll() {
	for _, b := range w.buffers {
		b.Close()
	}
	w.buffers = nil
	w.activeAt = -1
}
