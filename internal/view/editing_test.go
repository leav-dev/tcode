package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func typeRune(v *EditorView, r rune) bool {
	return v.HandleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
}

func pressKey(v *EditorView, key tcell.Key) bool {
	return v.HandleEvent(tcell.NewEventKey(key, 0, tcell.ModNone))
}

// typeString tipea s usando los eventos que un terminal real entrega para los
// saltos: Enter llega como KeyEnter y el tab como KeyTab, no como runas.
func typeString(v *EditorView, s string) {
	for _, r := range s {
		switch r {
		case '\n':
			pressKey(v, tcell.KeyEnter)
		case '\t':
			pressKey(v, tcell.KeyTab)
		default:
			typeRune(v, r)
		}
	}
}

func contentOf(t *testing.T, v *EditorView) string {
	t.Helper()
	return v.model.GetContent()
}

// --- inserción ---

func TestTypingInsertsAtCursor(t *testing.T) {
	v := newTestView(t, "uno", 20, 1)

	pressKey(v, tcell.KeyEnd)
	typeRune(v, 's')

	if got := contentOf(t, v); got != "unos" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "unos")
	}
	if v.cursor.ByteCol != 4 {
		t.Fatalf("ByteCol = %d, se esperaba 4: el cursor avanza con el texto", v.cursor.ByteCol)
	}
}

func TestTypingInTheMiddleOfALine(t *testing.T) {
	v := newTestView(t, "holamundo", 20, 1)

	pressKey(v, tcell.KeyEnd)
	for i := 0; i < 5; i++ {
		pressKey(v, tcell.KeyLeft)
	}
	typeRune(v, ' ')

	if got := contentOf(t, v); got != "hola mundo" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "hola mundo")
	}
	if v.cursor.ByteCol != 5 {
		t.Fatalf("ByteCol = %d, se esperaba 5", v.cursor.ByteCol)
	}
}

func TestTypingWideCharacterAdvancesByItsWidth(t *testing.T) {
	s := newTestScreen(t, 20, 1)
	v := newTestView(t, "ab", 20, 1)

	pressKey(v, tcell.KeyHome)
	typeRune(v, '日')

	if got := contentOf(t, v); got != "日ab" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "日ab")
	}
	if v.cursor.ByteCol != 3 {
		t.Fatalf("ByteCol = %d, se esperaba 3", v.cursor.ByteCol)
	}

	draw(v, s)
	// El cursor tiene que quedar en la columna 4, no en la 3: '日' ocupa dos
	// celdas y el texto arranca tras el gutter de 2 columnas.
	if x, _, _ := s.GetCursor(); x != 4 {
		t.Fatalf("cursor en columna %d, se esperaba 4", x)
	}
}

func TestEnterSplitsTheLineAndMovesCursorToTheNewOne(t *testing.T) {
	v := newTestView(t, "unodos", 20, 2)

	pressKey(v, tcell.KeyEnd)
	for i := 0; i < 3; i++ {
		pressKey(v, tcell.KeyLeft)
	}
	pressKey(v, tcell.KeyEnter)

	if got := contentOf(t, v); got != "uno\ndos" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "uno\ndos")
	}
	if v.cursor.Line != 1 || v.cursor.ByteCol != 0 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,0)", v.cursor.Line, v.cursor.ByteCol)
	}
	if got := v.model.LineCount(); got != 2 {
		t.Fatalf("LineCount() = %d, se esperaba 2", got)
	}
}

// TestEnterAtTheEndCreatesAnAddressableEmptyLine cubre la corrección de la línea
// fantasma: tras el Enter, el cursor vive en una línea que antes no se contaba.
func TestEnterAtTheEndCreatesAnAddressableEmptyLine(t *testing.T) {
	v := newTestView(t, "uno", 20, 2)

	pressKey(v, tcell.KeyEnd)
	pressKey(v, tcell.KeyEnter)

	if got := contentOf(t, v); got != "uno\n" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "uno\n")
	}
	if v.cursor.Line != 1 || v.cursor.ByteCol != 0 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,0)", v.cursor.Line, v.cursor.ByteCol)
	}
	if got := v.model.LineCount(); got != 2 {
		t.Fatalf("LineCount() = %d, se esperaba 2: la línea vacía final es direccionable", got)
	}

	// Y la flecha abajo desde ahí no debe saltar hacia arriba.
	pressKey(v, tcell.KeyDown)
	if v.cursor.Line != 1 {
		t.Fatalf("cursor.Line = %d, se esperaba 1: ya es la última línea", v.cursor.Line)
	}

	// Se puede escribir en esa línea nueva.
	typeRune(v, 'x')
	if got := contentOf(t, v); got != "uno\nx" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "uno\nx")
	}
}

// TestTabInsertsIndentUnit es el contrato del feature de auto-indentación: la
// tecla Tab inserta la unidad estándar del editor (4 espacios por defecto), no
// un tab crudo — el nivel extra del Enter y la tecla coinciden.
func TestTabInsertsIndentUnit(t *testing.T) {
	s := newTestScreen(t, 20, 1)
	v := newTestView(t, "", 20, 1)

	typeRune(v, 'a')
	pressKey(v, tcell.KeyTab)
	typeRune(v, 'b')

	if got := contentOf(t, v); got != "a"+indentUnit+"b" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "a"+indentUnit+"b")
	}

	draw(v, s)
	// 'a' tras el gutter en la columna 0 del texto, los espacios de indentUnit,
	// 'b' al final de ellos.
	want := "1 " + "a" + indentUnit + "b"
	if got := screenLines(s)[0]; got != want {
		t.Fatalf("pantalla = %q, se esperaba %q", got, want)
	}
}

func TestCtrlCombinationsDoNotInsert(t *testing.T) {
	v := newTestView(t, "uno", 20, 1)

	pressKey(v, tcell.KeyEnd)
	if v.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModCtrl)) {
		t.Fatal("Ctrl+a no debe insertar: los modificadores quedan para atajos")
	}
	if got := contentOf(t, v); got != "uno" {
		t.Fatalf("contenido = %q, se esperaba sin cambios", got)
	}
}

// TestLineFeedAlsoInsertsANewline cubre los terminales que mandan LF crudo en vez
// de CR. tcell convierte una runa '\n' en KeyLF con ModCtrl, así que sin este
// alias el salto de línea se perdería en silencio.
func TestLineFeedAlsoInsertsANewline(t *testing.T) {
	v := newTestView(t, "uno", 20, 2)

	pressKey(v, tcell.KeyEnd)
	if !v.HandleEvent(tcell.NewEventKey(tcell.KeyRune, '\n', tcell.ModNone)) {
		t.Fatal("una runa '\\n' (KeyLF) debería insertar un salto de línea")
	}

	if got := contentOf(t, v); got != "uno\n" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "uno\n")
	}
	if v.cursor.Line != 1 || v.cursor.ByteCol != 0 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,0)", v.cursor.Line, v.cursor.ByteCol)
	}
}

// --- Backspace ---

func TestBackspaceRemovesPreviousCluster(t *testing.T) {
	v := newTestView(t, "uno", 20, 1)

	pressKey(v, tcell.KeyEnd)
	pressKey(v, tcell.KeyBackspace)

	if got := contentOf(t, v); got != "un" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "un")
	}
	if v.cursor.ByteCol != 2 {
		t.Fatalf("ByteCol = %d, se esperaba 2", v.cursor.ByteCol)
	}
}

func TestBackspaceRemovesWholeWideCharacter(t *testing.T) {
	v := newTestView(t, "a日", 20, 1)

	pressKey(v, tcell.KeyEnd)
	pressKey(v, tcell.KeyBackspace)

	if got := contentOf(t, v); got != "a" {
		t.Fatalf("contenido = %q, se esperaba %q: se borra el carácter ancho completo", got, "a")
	}
}

func TestBackspaceRemovesWholeCombiningCluster(t *testing.T) {
	v := newTestView(t, "e\u0301x", 20, 1)

	pressKey(v, tcell.KeyEnd)
	pressKey(v, tcell.KeyBackspace)

	if got := contentOf(t, v); got != "e\u0301" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "e\u0301")
	}

	// El siguiente borrado se lleva la 'e' junto con su acento, en un solo paso.
	pressKey(v, tcell.KeyBackspace)
	if got := contentOf(t, v); got != "" {
		t.Fatalf("contenido = %q, se esperaba vacío", got)
	}
}

func TestBackspaceAtLineStartMergesLines(t *testing.T) {
	v := newTestView(t, "uno\ndos", 20, 2)

	pressKey(v, tcell.KeyDown)
	if v.cursor.Line != 1 || v.cursor.ByteCol != 0 {
		t.Fatalf("cursor = (%d,%d), se esperaba (1,0)", v.cursor.Line, v.cursor.ByteCol)
	}

	pressKey(v, tcell.KeyBackspace)

	if got := contentOf(t, v); got != "unodos" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "unodos")
	}
	if v.cursor.Line != 0 || v.cursor.ByteCol != 3 {
		t.Fatalf("cursor = (%d,%d), se esperaba (0,3): queda en la unión", v.cursor.Line, v.cursor.ByteCol)
	}
}

func TestBackspaceAtDocumentStartIsNoOp(t *testing.T) {
	v := newTestView(t, "uno", 20, 1)

	if pressKey(v, tcell.KeyBackspace) {
		t.Fatal("Backspace al inicio del documento no debería cambiar nada")
	}
	if got := contentOf(t, v); got != "uno" {
		t.Fatalf("contenido = %q, se esperaba sin cambios", got)
	}
}

func TestBackspaceOnEmptyDocumentIsNoOp(t *testing.T) {
	v := newTestView(t, "", 20, 1)

	if pressKey(v, tcell.KeyBackspace) {
		t.Fatal("Backspace en documento vacío no debería cambiar nada")
	}
}

// --- Delete ---

func TestDeleteRemovesClusterAtCursor(t *testing.T) {
	v := newTestView(t, "uno", 20, 1)

	pressKey(v, tcell.KeyHome)
	pressKey(v, tcell.KeyDelete)

	if got := contentOf(t, v); got != "no" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "no")
	}
	if v.cursor.ByteCol != 0 {
		t.Fatalf("ByteCol = %d, se esperaba 0: el cursor no se mueve", v.cursor.ByteCol)
	}
}

func TestDeleteAtLineEndMergesWithNextLine(t *testing.T) {
	v := newTestView(t, "uno\ndos", 20, 2)

	pressKey(v, tcell.KeyEnd)
	pressKey(v, tcell.KeyDelete)

	if got := contentOf(t, v); got != "unodos" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "unodos")
	}
	if v.cursor.Line != 0 || v.cursor.ByteCol != 3 {
		t.Fatalf("cursor = (%d,%d), se esperaba (0,3)", v.cursor.Line, v.cursor.ByteCol)
	}
}

func TestDeleteAtDocumentEndIsNoOp(t *testing.T) {
	v := newTestView(t, "uno", 20, 1)

	pressKey(v, tcell.KeyEnd)
	if pressKey(v, tcell.KeyDelete) {
		t.Fatal("Delete al final del documento no debería cambiar nada")
	}
	if got := contentOf(t, v); got != "uno" {
		t.Fatalf("contenido = %q, se esperaba sin cambios", got)
	}
}

func TestDeleteOnEmptyDocumentIsNoOp(t *testing.T) {
	v := newTestView(t, "", 20, 1)

	if pressKey(v, tcell.KeyDelete) {
		t.Fatal("Delete en documento vacío no debería cambiar nada")
	}
}

// --- interacción con el viewport ---

func TestTypingKeepsCursorVisibleAtTheBottom(t *testing.T) {
	v := newTestView(t, "1\n2\n3\n4\n5", 20, 2)

	pressKey(v, tcell.KeyEnd)
	pressKey(v, tcell.KeyEnd)

	// Escribir varias veces al final debe empujar el viewport para que el cursor
	// siga en pantalla.
	typeRune(v, 'x')
	pressKey(v, tcell.KeyEnter)
	pressKey(v, tcell.KeyDown)
	typeRune(v, 'y')

	if v.cursor.Line < v.viewport.TopLine || v.cursor.Line >= v.viewport.TopLine+v.viewport.Height {
		t.Fatalf("el cursor quedó fuera de pantalla: línea %d, viewport [%d,%d)",
			v.cursor.Line, v.viewport.TopLine, v.viewport.TopLine+v.viewport.Height)
	}
}

func TestBackspaceToEmptyShrinksViewport(t *testing.T) {
	v := newTestView(t, "1\n2\n3\n4\n5\n6", 20, 3)

	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))
	if v.cursor.Line != 5 {
		t.Fatalf("cursor.Line = %d, se esperaba 5", v.cursor.Line)
	}
	if v.viewport.TopLine == 0 {
		t.Fatal("el test necesita que el viewport se haya desplazado")
	}

	for v.model.Len() > 0 {
		pressKey(v, tcell.KeyBackspace)
	}

	if got := contentOf(t, v); got != "" {
		t.Fatalf("contenido = %q, se esperaba vacío", got)
	}
	if v.viewport.TopLine != 0 {
		t.Fatalf("TopLine = %d, se esperaba 0 con el documento vacío", v.viewport.TopLine)
	}
	if got := v.model.LineCount(); got != 0 {
		t.Fatalf("LineCount() = %d, se esperaba 0", got)
	}
}

// --- ida y vuelta ---

// TestTypingThenBackspacingIsARoundTrip escribe texto letra por letra y lo borra
// letra por letra, comprobando que el documento y el cursor vuelvan al inicio.
func TestTypingThenBackspacingIsARoundTrip(t *testing.T) {
	const original = "linea uno\nlinea dos"
	v := newTestView(t, original, 40, 4)

	// Ctrl+End lleva al final del documento; End solo va al final de la línea.
	v.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))

	typed := "hola 日 mundo\nnueva línea"
	typeString(v, typed)
	if got := contentOf(t, v); got != original+typed {
		t.Fatalf("tras escribir, contenido = %q, se esperaba %q", got, original+typed)
	}

	for range []rune(typed) {
		pressKey(v, tcell.KeyBackspace)
	}
	if got := contentOf(t, v); got != original {
		t.Fatalf("tras borrar todo, contenido = %q, se esperaba %q", got, original)
	}
	if got := v.model.LineCount(); got != 2 {
		t.Fatalf("LineCount() = %d, se esperaba 2", got)
	}
}

// TestEditingMatchesModelDirectly comprueba que la vista no se desincronice del
// modelo: aplica la misma secuencia de teclas y compara con editar el modelo a mano.
func TestEditingMatchesModelDirectly(t *testing.T) {
	v := newTestView(t, "abcdef", 20, 2)

	// Escribe texto en el cursor y luego borra hacia atrás dos veces.
	pressKey(v, tcell.KeyEnd)
	typeRune(v, 'X')
	typeRune(v, 'Y')
	pressKey(v, tcell.KeyBackspace)

	if got := contentOf(t, v); got != "abcdefX" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "abcdefX")
	}
	if v.cursor.ByteCol != 7 {
		t.Fatalf("ByteCol = %d, se esperaba 7", v.cursor.ByteCol)
	}

	// El offset del cursor tiene que coincidir con el del modelo.
	if got, want := v.cursorOffset(), 7; got != want {
		t.Fatalf("cursorOffset() = %d, se esperaba %d", got, want)
	}
	if got, want := v.model.Len(), len("abcdefX"); got != want {
		t.Fatalf("Len() = %d, se esperaba %d", got, want)
	}
}
