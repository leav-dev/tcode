package view

import (
	"testing"
	"unsafe"

	"github.com/gdamore/tcell/v2"
)

// recordingSurface captura los strings que Draw pone, para inspeccionar sus
// punteros sin pasar por el CellBuffer de tcell (que los retiene).
type recordingSurface struct {
	puts []string
}

func (r *recordingSurface) Put(x, y int, s string, style tcell.Style) (string, int) {
	r.puts = append(r.puts, s)
	return "", len(s)
}
func (r *recordingSurface) SetContent(x, y int, mainc rune, combc []rune, style tcell.Style) {}
func (r *recordingSurface) ShowCursor(x, y int)                                              {}
func (r *recordingSurface) HideCursor()                                                      {}

// TestDrawDoesNotAliasTheMmap fija la invariante de memoria del dibujo: los
// strings que Draw entrega a la Surface (y que tcell retiene en su buffer de
// celdas) NUNCA pueden apuntar dentro del mmap del archivo. Si aliasearan el
// mmap, cerrar la pestaña (que desmapea el archivo) dejaría celdas apuntando a
// memoria liberada: el siguiente redraw compararía ese string y segfaultaría.
func TestDrawDoesNotAliasTheMmap(t *testing.T) {
	v := newTestView(t, "hola mundo\nsegunda línea con 日本語\n", 20, 2)

	// El slice que el modelo devuelve apunta DENTRO del mmap: es la garantía
	// cero copia del proyecto y el rango exacto que el dibujo no debe retener.
	content := v.model.GetRange(0, 2)
	start := uintptr(unsafe.Pointer(&content[0]))
	end := start + uintptr(len(content))

	var r recordingSurface
	v.Draw(&r)

	if len(r.puts) == 0 {
		t.Fatal("el Draw no puso ningún cluster: el test no verifica nada")
	}
	for _, s := range r.puts {
		if len(s) == 0 {
			continue
		}
		p := uintptr(unsafe.Pointer(unsafe.StringData(s)))
		if p >= start && p < end {
			t.Fatalf("el cluster %q que se dibuja aliasea el mmap: cerrar el buffer lo dejaría colgando (UAF)", s)
		}
	}
}
