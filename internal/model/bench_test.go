package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeLinesTable carga un archivo de n líneas. Se usa un archivo real y no una
// tabla sintética para medir el camino completo, mmap incluido.
func makeLinesTable(b *testing.B, n int) *PieceTable {
	b.Helper()

	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "linea %d con una cantidad razonable de texto\n", i)
	}

	path := filepath.Join(b.TempDir(), "grande.txt")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		b.Fatalf("no se pudo crear el archivo: %v", err)
	}

	pt := NewPieceTable()
	if err := pt.LoadFile(path); err != nil {
		b.Fatalf("LoadFile falló: %v", err)
	}
	b.Cleanup(func() { pt.Close() })
	return pt
}

// benchmarkTyping mide el costo por tecla de escribir una runa. Es la operación más
// frecuente del editor, así que su costo manda sobre la sensación de fluidez.
func benchmarkTyping(b *testing.B, lines int, position func(pt *PieceTable) int) {
	pt := makeLinesTable(b, lines)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt.BreakTypingGroup()
		if err := pt.Insert(position(pt), "x"); err != nil {
			b.Fatalf("Insert falló: %v", err)
		}
	}
}

func BenchmarkTypingAtEnd1kLines(b *testing.B) {
	benchmarkTyping(b, 1_000, func(pt *PieceTable) int { return pt.Len() })
}

func BenchmarkTypingAtEnd100kLines(b *testing.B) {
	benchmarkTyping(b, 100_000, func(pt *PieceTable) int { return pt.Len() })
}

// Escribir al principio es el peor caso del índice de líneas: hay que correr todos
// los inicios que vienen después.
func BenchmarkTypingAtStart1kLines(b *testing.B) {
	benchmarkTyping(b, 1_000, func(pt *PieceTable) int { return 0 })
}

func BenchmarkTypingAtStart100kLines(b *testing.B) {
	benchmarkTyping(b, 100_000, func(pt *PieceTable) int { return 0 })
}

// Escribir en el medio mide el caso intermedio: se corre solo la mitad de la cola.
func BenchmarkTypingAtMiddle100kLines(b *testing.B) {
	pt := makeLinesTable(b, 100_000)
	middle := pt.Len() / 2

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt.BreakTypingGroup()
		if err := pt.Insert(middle, "x"); err != nil {
			b.Fatalf("Insert falló: %v", err)
		}
		middle++
	}
}

// BenchmarkTypingNewline100kLines mide el caso que sí agrega inicios de línea, así
// que no puede evitar mover la cola ni insertar entradas.
func BenchmarkTypingNewline100kLines(b *testing.B) {
	pt := makeLinesTable(b, 100_000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt.BreakTypingGroup()
		if err := pt.Insert(pt.Len(), "\n"); err != nil {
			b.Fatalf("Insert falló: %v", err)
		}
	}
}

// BenchmarkBackspace100kLines mide el borrado de una runa, el otro camino caliente.
func BenchmarkBackspace100kLines(b *testing.B) {
	pt := makeLinesTable(b, 100_000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt.BreakTypingGroup()
		if err := pt.Insert(pt.Len(), "x"); err != nil {
			b.Fatalf("Insert falló: %v", err)
		}
		if _, err := pt.Delete(pt.Len()-1, pt.Len()); err != nil {
			b.Fatalf("Delete falló: %v", err)
		}
	}
}

// BenchmarkGetRangeVisibleWindow mide el camino del render: pedir una ventana de 50
// líneas. Debería ser barato y no depender del tamaño del archivo.
func BenchmarkGetRangeVisibleWindow(b *testing.B) {
	for _, lines := range []int{1_000, 100_000} {
		b.Run(fmt.Sprintf("%dLineas", lines), func(b *testing.B) {
			pt := makeLinesTable(b, lines)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := (i * 50) % (pt.LineCount() - 50)
				if got := pt.GetRange(start, start+50); len(got) == 0 {
					b.Fatal("GetRange devolvió vacío")
				}
			}
		})
	}
}

// BenchmarkLocate100kLines aísla el costo de resolver un offset de documento, que
// recorre las piezas linealmente.
func BenchmarkLocate100kLines(b *testing.B) {
	pt := makeLinesTable(b, 100_000)

	// Se fragmenta el documento para que haya muchas piezas: sin fragmentar,
	// locate siempre acierta en la primera.
	for i := 0; i < 200; i++ {
		pt.BreakTypingGroup()
		if err := pt.Insert(i*100, "y"); err != nil {
			b.Fatalf("Insert falló: %v", err)
		}
	}
	b.Logf("piezas: %d", len(pt.pieces))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt.locate(pt.docLen)
	}
}
