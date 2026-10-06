package view

import (
	"github.com/atotto/clipboard"
)

// readClipboard y writeClipboard son la interfaz al portapapeles del SISTEMA,
// inyectables para los tests (el host de tests no puede tocar el portapapeles
// real). El paquete view es el único que habla con él.
var (
	readClipboard  = clipboard.ReadAll
	writeClipboard = clipboard.WriteAll
)

// CopySelection copia el texto marcado al portapapeles del sistema. Sin
// selección no hace nada. El controlador la llama cuando Ctrl+C llega con una
// selección activa (y no sale del editor).
func (v *EditorView) CopySelection() error {
	if !v.SelectionActive() {
		return nil
	}
	return writeClipboard(v.SelectionText())
}

// PasteClipboard pega el contenido del portapapeles en el cursor (reemplazando
// la selección si la hay). Devuelve false si el portapapeles no se pudo leer.
func (v *EditorView) PasteClipboard() bool {
	text, err := readClipboard()
	if err != nil {
		return false
	}
	return v.insertText(text)
}
