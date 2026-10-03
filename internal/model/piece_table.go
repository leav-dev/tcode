package model

import (
	"os"

	"github.com/edsrzf/mmap-go"
)

// Piece representa un fragmento de texto (ya sea del archivo original o nuevo texto insertado).
type Piece struct {
	Start int  // Inicio del offset en el buffer
	Len   int  // Longitud de la pieza
	IsNew bool // Si es texto nuevo (insertado) o original
}

// PieceTable gestiona el documento como una colección de piezas.
type PieceTable struct {
	originalBuffer mmap.MMap // Mapeo de solo lectura del archivo original (zero-copy)
	newBuffer      []byte    // Buffer para texto nuevo
	pieces         []Piece   // Lista de piezas actuales
	lineOffsets    []int     // Índices de los saltos de línea
	file           *os.File
}

// NewPieceTable inicializa una tabla vacía.
func NewPieceTable() *PieceTable {
	return &PieceTable{
		pieces:      []Piece{{Start: 0, Len: 0, IsNew: false}},
		lineOffsets: []int{0},
	}
}

// LoadFile abre un archivo y lo mapea en memoria para lectura eficiente.
func (pt *PieceTable) LoadFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}

	// mmap no puede mapear 0 bytes: un archivo vacío es un documento vacío.
	if info.Size() == 0 {
		pt.file = f
		pt.originalBuffer = nil
		pt.lineOffsets = []int{0}
		pt.pieces = nil
		return nil
	}

	mmaped, err := mmap.Map(f, mmap.RDONLY, 0)
	if err != nil {
		f.Close()
		return err
	}

	pt.file = f
	pt.originalBuffer = mmaped

	// Mapear saltos de línea una sola vez por carga.
	pt.lineOffsets = []int{0}
	for i, b := range mmaped {
		if b == '\n' {
			pt.lineOffsets = append(pt.lineOffsets, i+1)
		}
	}

	pt.pieces = []Piece{{Start: 0, Len: len(mmaped), IsNew: false}}
	return nil
}

// LineCount devuelve la cantidad total de líneas del documento.
func (pt *PieceTable) LineCount() int {
	if len(pt.originalBuffer) == 0 {
		return 0
	}

	n := len(pt.lineOffsets)
	if n == 0 {
		return 0
	}
	// Si el archivo termina en '\n', el último offset es el inicio de una
	// línea fantasma vacía: no la contamos.
	if n > 1 && pt.lineOffsets[n-1] == len(pt.originalBuffer) {
		return n - 1
	}
	return n
}

// GetRange devuelve los bytes correspondientes al rango de líneas [startLine, endLine).
// El slice devuelto es una vista del mmap (zero-copy): NO debe mutarse.
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

	startByte := pt.lineOffsets[startLine]
	endByte := len(pt.originalBuffer)
	if endLine < len(pt.lineOffsets) {
		endByte = pt.lineOffsets[endLine]
	}

	return pt.originalBuffer[startByte:endByte]
}

// GetContent devuelve el contenido lógico del documento.
// NOTA: Para un editor real, no usar esto para imprimir todo el archivo,
// solo para obtener el rango visible del viewport.
func (pt *PieceTable) GetContent() string {
	var result []byte
	for _, p := range pt.pieces {
		if p.IsNew {
			result = append(result, pt.newBuffer[p.Start:p.Start+p.Len]...)
		} else {
			result = append(result, pt.originalBuffer[p.Start:p.Start+p.Len]...)
		}
	}
	return string(result)
}

func (pt *PieceTable) Close() error {
	if pt.originalBuffer != nil {
		if err := pt.originalBuffer.Unmap(); err != nil {
			return err
		}
	}
	if pt.file != nil {
		return pt.file.Close()
	}
	return nil
}
