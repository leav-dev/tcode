package view

import "strings"

// La configuración del editor vive en vars del paquete view, una por ajuste,
// con acceso exportado de solo lectura (ExplorerWidth, IndentSize,
// WordWrapEnabled) y setters que la ventana flotante de configuración (Ctrl+,)
// y la persistencia del controlador usan. Son el estado por defecto del
// arranque: el controlador los sobreescribe con ~/.tcode/config.json cuando
// existe, y mientras tanto los tests y la terminal conviven con estos valores.
//
// El rol de cada var: indentUnit es la unidad estándar de indentación (la que
// inserta Tab y el auto-indent), wordWrapEnabled decide si las líneas se
// envuelven por palabra, y explorerWidth es el ancho máximo del panel lateral
// del explorador —el editor conserva el resto de la terminal—.

// explorerWidth es el ancho máximo del panel lateral del explorador. Vive acá
// (y no en el controlador) porque la ventana de configuración la expone y la
// persiste; el controlador solo la consulta con ExplorerWidth.
var explorerWidth = 24

// ExplorerWidth reporta el ancho máximo actual del panel lateral.
func ExplorerWidth() int { return explorerWidth }

// SetExplorerWidth fija el ancho máximo del panel lateral.
func SetExplorerWidth(n int) { explorerWidth = n }

// IndentSize devuelve el tamaño actual de la indentación en espacios: la
// longitud de la unidad de indentación (indentUnit siempre es una hilera de
// espacios, así que la longitud en bytes es la cantidad de espacios).
func IndentSize() int { return len(indentUnit) }

// SetIndentSize fija el tamaño de la indentación a n espacios, con mínimo 1:
// un tab de 0 espacios no existe.
func SetIndentSize(n int) {
	if n < 1 {
		n = 1
	}
	indentUnit = strings.Repeat(" ", n)
}

// SetWordWrapEnabled fija la configuración del salto de palabra. ToggleWordWrap
// (Ctrl+Shift+W) sigue siendo quien la alterna en vivo; este setter es la
// puerta de la ventana de configuración y de la carga del archivo persistido.
func SetWordWrapEnabled(b bool) { wordWrapEnabled = b }
