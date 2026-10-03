package model

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/edsrzf/mmap-go"
)

// ErrNoPath se devuelve al intentar guardar un documento que no tiene archivo
// asociado.
var ErrNoPath = errors.New("el documento no tiene archivo asociado")

// Piece es un tramo contiguo del documento que vive entero en uno de los dos
// buffers. Una vez creada, una pieza nunca se muta: editar es crear y descartar
// piezas, lo que deja intactos los buffers y habilita el undo más adelante.
type Piece struct {
	Start int  // Offset dentro de su buffer
	Len   int  // Longitud en bytes
	IsNew bool // true: newBuffer (insertado); false: originalBuffer (mmap)
}

// PieceTable representa el documento como una secuencia ordenada de piezas.
//
// Hay dos sistemas de coordenadas que no hay que confundir:
//
//   - Offset de documento: posición lógica del byte dentro del texto actual.
//     Es el que usan la vista, los offsets de línea y las operaciones de edición.
//   - Offset de buffer: posición del byte dentro de originalBuffer o newBuffer.
//     Es interno a cada pieza.
//
// La suma de las longitudes de las piezas es siempre docLen.
type PieceTable struct {
	originalBuffer mmap.MMap // Mapeo de solo lectura del archivo original
	newBuffer      []byte    // Buffer append-only para el texto insertado
	pieces         []Piece   // Secuencia ordenada; recorre el documento completo
	lineOffsets    []int     // Inicios de línea, en coordenadas de documento
	docLen         int       // Longitud del documento en bytes

	file     *os.File    // Descriptor del archivo mapeado
	path     string      // Ruta asociada, tal como la dio el usuario
	mode     os.FileMode // Permisos a preservar al guardar
	modified bool        // Hay cambios sin guardar
}

// NewPieceTable inicializa una tabla vacía.
func NewPieceTable() *PieceTable {
	return &PieceTable{lineOffsets: []int{0}}
}

// Path devuelve la ruta del archivo asociado, o "" si no hay ninguno.
func (pt *PieceTable) Path() string { return pt.path }

// Modified indica si hay cambios sin guardar.
func (pt *PieceTable) Modified() bool { return pt.modified }

// LoadFile abre un archivo y lo mapea en memoria para lectura eficiente.
func (pt *PieceTable) LoadFile(path string) error {
	pt.release()
	pt.path = path
	if err := pt.openAndMap(); err != nil {
		return err
	}
	pt.modified = false
	return nil
}

// openAndMap abre y mapea pt.path dejando el documento en una sola pieza.
// Es la operación que comparten la carga inicial y la recarga posterior a guardar.
func (pt *PieceTable) openAndMap() error {
	f, err := os.OpenFile(pt.path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}

	pt.file = f
	pt.mode = info.Mode().Perm()
	pt.originalBuffer = nil
	pt.newBuffer = nil
	pt.pieces = nil
	pt.lineOffsets = []int{0}
	pt.docLen = 0

	// mmap no puede mapear 0 bytes: un archivo vacío es un documento vacío.
	if info.Size() == 0 {
		return nil
	}

	mmaped, err := mmap.Map(f, mmap.RDONLY, 0)
	if err != nil {
		f.Close()
		pt.file = nil
		return err
	}

	pt.originalBuffer = mmaped
	pt.pieces = []Piece{{Start: 0, Len: len(mmaped), IsNew: false}}
	pt.docLen = len(mmaped)
	pt.rebuildLineOffsets()
	return nil
}

// release desmapea y cierra el archivo actual sin tocar la ruta ni el documento.
func (pt *PieceTable) release() {
	if pt.originalBuffer != nil {
		pt.originalBuffer.Unmap()
		pt.originalBuffer = nil
	}
	if pt.file != nil {
		pt.file.Close()
		pt.file = nil
	}
}

// Len devuelve la longitud del documento en bytes.
func (pt *PieceTable) Len() int { return pt.docLen }

// LineStart devuelve el offset de documento donde empieza la línea indicada.
// Un índice fuera de rango se resuelve al final del documento.
func (pt *PieceTable) LineStart(line int) int {
	if line <= 0 {
		return 0
	}
	if line >= len(pt.lineOffsets) {
		return pt.docLen
	}
	return pt.lineOffsets[line]
}

// lineSpan devuelve los offsets de documento de la línea completa, salto incluido.
func (pt *PieceTable) lineSpan(line int) (start, end int) {
	start = pt.LineStart(line)
	end = pt.docLen
	if line+1 < len(pt.lineOffsets) {
		end = pt.lineOffsets[line+1]
	}
	return start, end
}

// LineBreakLen devuelve el largo del salto de línea: 1 para '\n', 2 para '\r\n'
// y 0 en la última línea cuando el documento no termina en salto.
func (pt *PieceTable) LineBreakLen(line int) int {
	start, end := pt.lineSpan(line)
	if raw := end - start; raw > 0 {
		return raw - len(pt.LineContent(line))
	}
	return 0
}

// LineContent devuelve el texto de la línea sin su salto de línea.
func (pt *PieceTable) LineContent(line int) []byte {
	start, end := pt.lineSpan(line)
	if end <= start {
		return nil
	}

	raw := pt.slice(start, end)
	// Descontar el salto: '\n' y, si está, el '\r' que lo precede.
	if n := len(raw); n > 0 && raw[n-1] == '\n' {
		raw = raw[:n-1]
		if n = len(raw); n > 0 && raw[n-1] == '\r' {
			raw = raw[:n-1]
		}
	}
	return raw
}

// LineAt devuelve el índice de la línea que contiene el offset de documento dado.
func (pt *PieceTable) LineAt(offset int) int {
	if offset <= 0 || len(pt.lineOffsets) == 0 {
		return 0
	}
	i := sort.SearchInts(pt.lineOffsets, offset+1) - 1
	if i < 0 {
		return 0
	}
	if i >= len(pt.lineOffsets) {
		return len(pt.lineOffsets) - 1
	}
	return i
}

// LineCount devuelve la cantidad de líneas direccionables del documento.
//
// Un documento que termina en '\n' tiene una línea vacía final, igual que en
// cualquier editor: es donde el cursor queda después de presionar Enter al final.
// Un documento vacío, en cambio, no tiene ninguna línea direccionable.
func (pt *PieceTable) LineCount() int {
	if pt.docLen == 0 {
		return 0
	}
	return len(pt.lineOffsets)
}

// buffer devuelve el buffer al que apunta una pieza.
func (pt *PieceTable) buffer(p Piece) []byte {
	if p.IsNew {
		return pt.newBuffer
	}
	return pt.originalBuffer
}

// rebuildLineOffsets reconstruye el índice de líneas escaneando el documento
// completo. Solo se usa al cargar; las ediciones lo mantienen incrementalmente.
func (pt *PieceTable) rebuildLineOffsets() {
	pt.lineOffsets = pt.lineOffsets[:0]
	pt.lineOffsets = append(pt.lineOffsets, 0)

	doc := 0
	for _, p := range pt.pieces {
		buf := pt.buffer(p)
		for i := 0; i < p.Len; i++ {
			if buf[p.Start+i] == '\n' {
				pt.lineOffsets = append(pt.lineOffsets, doc+i+1)
			}
		}
		doc += p.Len
	}
}

// locate devuelve el índice de la pieza que contiene offset y el desplazamiento
// dentro de esa pieza. Un offset igual a docLen se resuelve al final, con
// idx == len(pieces), que es la posición de inserción al final del documento.
func (pt *PieceTable) locate(offset int) (idx, within int) {
	doc := 0
	for i, p := range pt.pieces {
		if offset < doc+p.Len {
			return i, offset - doc
		}
		doc += p.Len
	}
	return len(pt.pieces), 0
}

// Insert inserta text en el offset de documento indicado.
func (pt *PieceTable) Insert(offset int, text string) error {
	if offset < 0 || offset > pt.docLen {
		return fmt.Errorf("offset %d fuera del documento (0..%d)", offset, pt.docLen)
	}
	if text == "" {
		return nil
	}

	// newBuffer es append-only: el texto nuevo va siempre al final y la pieza
	// lo referencia. Eso deja intactas las piezas ya existentes.
	start := len(pt.newBuffer)
	pt.newBuffer = append(pt.newBuffer, text...)
	piece := Piece{Start: start, Len: len(text), IsNew: true}

	idx, within := pt.locate(offset)
	switch {
	case idx == len(pt.pieces):
		// Inserción al final del documento.
		pt.pieces = append(pt.pieces, piece)

	case within == 0:
		// Cae justo en el borde de una pieza: no hay nada que partir.
		pt.pieces = append(pt.pieces, Piece{})
		copy(pt.pieces[idx+1:], pt.pieces[idx:])
		pt.pieces[idx] = piece

	default:
		// Cae adentro de una pieza: se parte en dos y la nueva va en el medio.
		p := pt.pieces[idx]
		left := Piece{Start: p.Start, Len: within, IsNew: p.IsNew}
		right := Piece{Start: p.Start + within, Len: p.Len - within, IsNew: p.IsNew}

		pt.pieces = append(pt.pieces, Piece{}, Piece{})
		copy(pt.pieces[idx+3:], pt.pieces[idx+1:])
		pt.pieces[idx] = left
		pt.pieces[idx+1] = piece
		pt.pieces[idx+2] = right
	}

	pt.docLen += len(text)
	pt.insertIntoLineOffsets(offset, text)
	pt.modified = true
	return nil
}

// insertIntoLineOffsets ajusta el índice de líneas tras insertar text en offset.
func (pt *PieceTable) insertIntoLineOffsets(offset int, text string) {
	length := len(text)

	// Primer inicio de línea posterior al punto de inserción. Los offsets
	// anteriores no se mueven.
	i := sort.SearchInts(pt.lineOffsets, offset+1)

	// Cada '\n' insertado crea un inicio de línea. Todos caen en
	// (offset, offset+length], o sea antes que cualquier inicio desplazado,
	// que pasa a ser > offset+length. Por eso no hace falta intercalar.
	var inserted []int
	for j := 0; j < length; j++ {
		if text[j] == '\n' {
			inserted = append(inserted, offset+j+1)
		}
	}

	out := make([]int, 0, len(pt.lineOffsets)+len(inserted))
	out = append(out, pt.lineOffsets[:i]...)
	out = append(out, inserted...)
	for _, l := range pt.lineOffsets[i:] {
		out = append(out, l+length)
	}
	pt.lineOffsets = out
}

// Delete borra el rango de documento [start, end) y devuelve cuántos bytes quitó.
func (pt *PieceTable) Delete(start, end int) (int, error) {
	if start < 0 || end > pt.docLen || start > end {
		return 0, fmt.Errorf("rango [%d,%d) inválido en documento de %d bytes", start, end, pt.docLen)
	}
	if start == end {
		return 0, nil
	}

	kept := make([]Piece, 0, len(pt.pieces))
	doc := 0
	for _, p := range pt.pieces {
		pStart, pEnd := doc, doc+p.Len
		doc = pEnd

		// Pieza totalmente fuera del rango borrado.
		if pEnd <= start || pStart >= end {
			kept = append(kept, p)
			continue
		}
		// Sobrevive el tramo izquierdo.
		if pStart < start {
			kept = append(kept, Piece{Start: p.Start, Len: start - pStart, IsNew: p.IsNew})
		}
		// Sobrevive el tramo derecho.
		if pEnd > end {
			kept = append(kept, Piece{
				Start: p.Start + (end - pStart),
				Len:   pEnd - end,
				IsNew: p.IsNew,
			})
		}
	}

	removed := end - start
	pt.pieces = kept
	pt.docLen -= removed
	pt.deleteFromLineOffsets(start, end)
	pt.modified = true
	return removed, nil
}

// deleteFromLineOffsets ajusta el índice de líneas tras borrar [start, end).
func (pt *PieceTable) deleteFromLineOffsets(start, end int) {
	removed := end - start

	out := make([]int, 0, len(pt.lineOffsets))
	for _, l := range pt.lineOffsets {
		switch {
		case l <= start:
			// El texto anterior a start no se movió, así que el inicio sigue
			// siendo válido en la misma posición.
			out = append(out, l)

		case l > end:
			out = append(out, l-removed)

			// Los inicios en (start, end] desaparecen con su línea. El que caía
			// justo en end se mapearía a start, pero solo sería un inicio válido
			// si start ya lo fuera, y en ese caso ya se conservó arriba. Por eso
			// se descarta siempre y no hace falta deduplicar después.
		}
	}

	if len(out) == 0 {
		out = append(out, 0)
	}
	pt.lineOffsets = out
}

// GetRange devuelve el texto de las líneas [startLine, endLine).
//
// Si el documento no tiene ediciones, el resultado es una vista directa del mmap
// (cero copia). Con ediciones, el rango puede abarcar varios buffers y se
// materializa en un slice nuevo. Quien llame no debe asumir cuál de los dos casos
// le tocó: el slice solo es válido para lectura mientras la tabla no se edite.
func (pt *PieceTable) GetRange(startLine, endLine int) []byte {
	total := pt.LineCount()
	if total == 0 || startLine < 0 || startLine >= total {
		return nil
	}
	if endLine > total {
		endLine = total
	}
	if endLine <= startLine {
		return nil
	}

	from := pt.lineOffsets[startLine]
	to := pt.docLen
	if endLine < len(pt.lineOffsets) {
		to = pt.lineOffsets[endLine]
	}
	if to <= from {
		return nil
	}
	return pt.slice(from, to)
}

// slice devuelve los bytes del documento en [from, to) recorriendo las piezas.
func (pt *PieceTable) slice(from, to int) []byte {
	// Camino rápido: sin ediciones el documento es una sola pieza del mmap, así
	// que el rango es una vista sin copia.
	if len(pt.pieces) == 1 && !pt.pieces[0].IsNew {
		p := pt.pieces[0]
		return pt.originalBuffer[p.Start+from : p.Start+to]
	}

	out := make([]byte, 0, to-from)
	doc := 0
	for _, p := range pt.pieces {
		pStart, pEnd := doc, doc+p.Len
		doc = pEnd

		if pEnd <= from || pStart >= to {
			continue
		}

		lo := p.Start
		if from > pStart {
			lo += from - pStart
		}
		hi := p.Start + p.Len
		if to < pEnd {
			hi -= pEnd - to
		}
		out = append(out, pt.buffer(p)[lo:hi]...)
	}
	return out
}

// GetContent devuelve el documento completo. Pensado para tests y usos no
// interactivos: en el render se usa GetRange con el rango visible.
func (pt *PieceTable) GetContent() string {
	if pt.docLen == 0 {
		return ""
	}
	return string(pt.slice(0, pt.docLen))
}

// Save escribe el documento a disco de forma atómica.
//
// Se escribe a un archivo temporal en el mismo directorio y recién entonces se
// renombra sobre el original. Un renombre dentro del mismo sistema de archivos es
// atómico, así que en disco siempre queda el contenido viejo o el nuevo, nunca uno
// a medias. Escribir en el lugar sería destructivo por dos motivos: un fallo a
// mitad de camino deja el archivo truncado, y además el archivo está mapeado en
// memoria, así que sobrescribirlo invalidaría las páginas que estamos leyendo.
func (pt *PieceTable) Save() error {
	if pt.path == "" {
		return ErrNoPath
	}
	if !pt.modified {
		return nil
	}

	// Si la ruta es un enlace simbólico hay que escribir sobre el destino y no
	// sobre el enlace: renombrar encima del enlace lo reemplazaría por un archivo
	// común y el enlace se perdería.
	target := pt.path
	if resolved, err := filepath.EvalSymlinks(pt.path); err == nil {
		target = resolved
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".tcode-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	discard := func() {
		tmp.Close()
		os.Remove(tmpName)
	}

	// El contenido puede abarcar varios buffers, así que se recorre pieza por
	// pieza con un buffer de escritura en lugar de una syscall por pieza.
	w := bufio.NewWriterSize(tmp, 64*1024)
	if err := pt.writeContent(w); err != nil {
		discard()
		return err
	}
	if err := w.Flush(); err != nil {
		discard()
		return err
	}
	if err := tmp.Sync(); err != nil {
		discard()
		return err
	}
	// CreateTemp crea con 0600: hay que devolverle los permisos del original.
	if err := tmp.Chmod(pt.mode); err != nil {
		discard()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return err
	}

	// El contenido ya está en disco de forma duradera.
	pt.modified = false

	// El mapeo y el descriptor siguen apuntando al inodo viejo, que quedó
	// reemplazado: hay que reabrir para que las piezas vuelvan a apoyarse en algo
	// válido y el documento quede en una sola pieza.
	pt.release()
	if err := pt.openAndMap(); err != nil {
		return fmt.Errorf("guardado en disco, pero falló la recarga: %w", err)
	}
	return nil
}

// writeContent vuelca el documento completo en w recorriendo las piezas.
func (pt *PieceTable) writeContent(w io.Writer) error {
	for _, p := range pt.pieces {
		if p.Len == 0 {
			continue
		}
		if _, err := w.Write(pt.buffer(p)[p.Start : p.Start+p.Len]); err != nil {
			return err
		}
	}
	return nil
}

func (pt *PieceTable) Close() error {
	pt.release()
	return nil
}
