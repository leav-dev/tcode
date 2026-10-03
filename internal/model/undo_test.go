package model

import (
	"strings"
	"testing"
)

func applyInsert(t *testing.T, pt *PieceTable, offset int, text string) {
	t.Helper()
	if err := pt.Insert(offset, text); err != nil {
		t.Fatalf("Insert(%d, %q) falló: %v", offset, text, err)
	}
}

func applyDelete(t *testing.T, pt *PieceTable, start, end int) {
	t.Helper()
	if _, err := pt.Delete(start, end); err != nil {
		t.Fatalf("Delete(%d, %d) falló: %v", start, end, err)
	}
}

func mustUndo(t *testing.T, pt *PieceTable) Change {
	t.Helper()
	c, ok, err := pt.Undo()
	if err != nil {
		t.Fatalf("Undo falló: %v", err)
	}
	if !ok {
		t.Fatal("Undo no tenía nada para deshacer")
	}
	return c
}

func mustRedo(t *testing.T, pt *PieceTable) Change {
	t.Helper()
	c, ok, err := pt.Redo()
	if err != nil {
		t.Fatalf("Redo falló: %v", err)
	}
	if !ok {
		t.Fatal("Redo no tenía nada para rehacer")
	}
	return c
}

// --- operaciones básicas ---

func TestUndoAfterInsert(t *testing.T) {
	pt := loadTable(t, "hola")

	applyInsert(t, pt, 4, " mundo")
	if got := pt.GetContent(); got != "hola mundo" {
		t.Fatalf("contenido = %q", got)
	}

	c := mustUndo(t, pt)

	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("tras deshacer, contenido = %q, se esperaba %q", got, "hola")
	}
	if c.Offset != 4 || c.Inserted != " mundo" || c.Removed != "" {
		t.Fatalf("cambio devuelto = %+v", c)
	}
	if pt.Len() != 4 {
		t.Fatalf("Len() = %d, se esperaba 4", pt.Len())
	}
}

func TestUndoAfterDelete(t *testing.T) {
	pt := loadTable(t, "hola mundo")

	applyDelete(t, pt, 4, 10)
	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("contenido = %q", got)
	}

	mustUndo(t, pt)

	if got := pt.GetContent(); got != "hola mundo" {
		t.Fatalf("tras deshacer, contenido = %q, se esperaba %q", got, "hola mundo")
	}
	if pt.Len() != 10 {
		t.Fatalf("Len() = %d, se esperaba 10", pt.Len())
	}
}

func TestRedoReappliesTheEdit(t *testing.T) {
	pt := loadTable(t, "hola")

	applyInsert(t, pt, 4, "!")
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("contenido = %q", got)
	}

	mustRedo(t, pt)

	if got := pt.GetContent(); got != "hola!" {
		t.Fatalf("tras rehacer, contenido = %q, se esperaba %q", got, "hola!")
	}
}

func TestUndoAndRedoAreLIFO(t *testing.T) {
	pt := loadTable(t, "")

	applyInsert(t, pt, 0, "uno")
	applyInsert(t, pt, 3, " dos")
	applyInsert(t, pt, 7, " tres")
	if got := pt.GetContent(); got != "uno dos tres" {
		t.Fatalf("contenido = %q", got)
	}

	mustUndo(t, pt)
	if got := pt.GetContent(); got != "uno dos" {
		t.Fatalf("tras un deshacer, %q", got)
	}
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "uno" {
		t.Fatalf("tras dos deshaceres, %q", got)
	}
	mustUndo(t, pt)
	if got := pt.GetContent(); got != "" {
		t.Fatalf("tras tres deshaceres, %q", got)
	}

	mustRedo(t, pt)
	if got := pt.GetContent(); got != "uno" {
		t.Fatalf("tras un rehacer, %q", got)
	}
	mustRedo(t, pt)
	mustRedo(t, pt)
	if got := pt.GetContent(); got != "uno dos tres" {
		t.Fatalf("tras tres rehaceres, %q", got)
	}
}

func TestUndoOnEmptyHistory(t *testing.T) {
	pt := loadTable(t, "hola")

	if _, ok, err := pt.Undo(); ok || err != nil {
		t.Fatalf("Undo sin historial = (ok=%v, err=%v), se esperaba (false, nil)", ok, err)
	}
	if pt.CanUndo() {
		t.Fatal("CanUndo() debe ser false sin historial")
	}
	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("el documento no debe cambiar: %q", got)
	}
}

func TestRedoOnEmptyHistory(t *testing.T) {
	pt := loadTable(t, "hola")

	if _, ok, err := pt.Redo(); ok || err != nil {
		t.Fatalf("Redo sin rama = (ok=%v, err=%v), se esperaba (false, nil)", ok, err)
	}
	if pt.CanRedo() {
		t.Fatal("CanRedo() debe ser false sin rama")
	}
}

func TestNewEditClearsTheRedoBranch(t *testing.T) {
	pt := loadTable(t, "hola")

	applyInsert(t, pt, 4, " uno")
	mustUndo(t, pt)
	if !pt.CanRedo() {
		t.Fatal("debería haber algo para rehacer")
	}

	applyInsert(t, pt, 4, " dos")

	if pt.CanRedo() {
		t.Fatal("editar después de deshacer debe descartar la rama de rehacer")
	}
	if got := pt.GetContent(); got != "hola dos" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "hola dos")
	}
}

// --- interacción con el marcador de modificado ---

func TestModifiedFollowsTheHistory(t *testing.T) {
	path := newFileWithContent(t, "hola")
	pt := openTable(t, path)

	if pt.Modified() {
		t.Fatal("un documento recién cargado no está modificado")
	}

	applyInsert(t, pt, 4, "!")
	if !pt.Modified() {
		t.Fatal("tras editar el documento queda modificado")
	}

	mustUndo(t, pt)
	if pt.Modified() {
		t.Fatal("volver al estado inicial con deshacer debe dejarlo limpio")
	}

	mustRedo(t, pt)
	if !pt.Modified() {
		t.Fatal("rehacer el cambio vuelve a marcarlo como modificado")
	}
}

// TestUndoAfterSaveMarksTheDocumentDirtyAgain es el caso central de la
// integración entre guardado e historial: guardar no borra el historial, así que
// deshacer un cambio ya guardado deja el documento otra vez sucio.
func TestUndoAfterSaveMarksTheDocumentDirtyAgain(t *testing.T) {
	path := newFileWithContent(t, "hola")
	pt := openTable(t, path)

	applyInsert(t, pt, 4, " mundo")
	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}
	if pt.Modified() {
		t.Fatal("tras guardar el documento está limpio")
	}
	if got := readFile(t, path); got != "hola mundo" {
		t.Fatalf("archivo = %q", got)
	}

	mustUndo(t, pt)

	if !pt.Modified() {
		t.Fatal("deshacer un cambio guardado debe volver a marcar el documento como modificado")
	}
	if got := pt.GetContent(); got != "hola" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "hola")
	}
	// Y el archivo sigue con el contenido guardado hasta que se vuelva a guardar.
	if got := readFile(t, path); got != "hola mundo" {
		t.Fatalf("el archivo no debe cambiar con deshacer: %q", got)
	}

	// Rehacer hasta el punto guardado lo vuelve a dejar limpio.
	mustRedo(t, pt)
	if pt.Modified() {
		t.Fatal("rehacer hasta el punto guardado debe dejar el documento limpio")
	}
}

// TestBranchingAfterUndoKeepsTheDocumentDirty cubre un error sutil del marcador
// deducido del historial: si el punto de guardado queda en la rama de rehacer que
// se descarta, el documento NO puede quedar marcado como limpio aunque la
// longitud del historial coincida.
func TestBranchingAfterUndoKeepsTheDocumentDirty(t *testing.T) {
	path := newFileWithContent(t, "hola")
	pt := openTable(t, path)

	applyInsert(t, pt, 0, "A") // historial: 1
	applyInsert(t, pt, 1, "B") // historial: 2
	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}
	if pt.Modified() {
		t.Fatal("tras guardar está limpio")
	}

	mustUndo(t, pt) // historial: 1; el punto de guardado (2) queda en la rama de rehacer

	// Editar ahora descarta esa rama: el estado guardado queda inalcanzable.
	applyInsert(t, pt, 1, "C") // historial vuelve a 2

	if !pt.Modified() {
		t.Fatal("el documento NO está limpio: el estado guardado quedó en la rama descartada")
	}
	if got := pt.GetContent(); got != "AC"+"hola" {
		t.Fatalf("contenido = %q", got)
	}
	if got := readFile(t, path); got != "ABhola" {
		t.Fatalf("el archivo debe seguir con el contenido guardado: %q", got)
	}
}

// --- consistencia del índice de líneas ---

func TestUndoRestoresTheLineIndex(t *testing.T) {
	pt := loadTable(t, "uno\ndos")

	applyInsert(t, pt, 3, "\nX")
	if got := pt.GetContent(); got != "uno\nX\ndos" {
		t.Fatalf("contenido = %q", got)
	}
	if got := pt.LineCount(); got != 3 {
		t.Fatalf("LineCount() = %d, se esperaba 3", got)
	}

	mustUndo(t, pt)

	if got := pt.GetContent(); got != "uno\ndos" {
		t.Fatalf("contenido = %q", got)
	}
	if got := pt.LineCount(); got != 2 {
		t.Fatalf("LineCount() = %d, se esperaba 2", got)
	}
	if want := []int{0, 4}; !equalInts(pt.lineOffsets, want) {
		t.Fatalf("lineOffsets = %v, se esperaba %v", pt.lineOffsets, want)
	}
}

func TestUndoOfDeletedLineBreakRestoresTheLineCount(t *testing.T) {
	pt := loadTable(t, "uno\ndos")

	applyDelete(t, pt, 3, 4) // borra el salto
	if got := pt.LineCount(); got != 1 {
		t.Fatalf("LineCount() = %d, se esperaba 1", got)
	}

	mustUndo(t, pt)

	if got := pt.GetContent(); got != "uno\ndos" {
		t.Fatalf("contenido = %q", got)
	}
	if got := pt.LineCount(); got != 2 {
		t.Fatalf("LineCount() = %d, se esperaba 2", got)
	}
}

// --- test diferencial ---

// TestUndoRedoMatchesReference aplica una secuencia determinista de ediciones
// guardando el estado del texto después de cada una, después deshace todo
// comparando hacia atrás y rehace todo comparando hacia adelante. Es el test que
// atrapa errores de inversión: si una inversa no es exacta, el contenido diverge.
func TestUndoRedoMatchesReference(t *testing.T) {
	const initial = "linea uno\nlinea dos\nlinea tres\n"
	pt := loadTable(t, initial)

	states := []string{initial}

	seed := uint64(0x9E3779B97F4A7C15)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		if n <= 0 {
			return 0
		}
		return int((seed >> 33) % uint64(n))
	}
	fragments := []string{"a", "b", " ", "\n", "é", "日", "XY", "\n\n", "Z"}

	// 120 ediciones, la mitad borrados.
	for i := 0; i < 120; i++ {
		ref := states[len(states)-1]

		if next(2) == 0 && len(ref) > 0 {
			start := next(len(ref))
			// El rango se fuerza no vacío a propósito: un borrado de longitud cero
			// no es una edición, no queda en el historial, y por lo tanto tampoco
			// debe generar un estado de referencia. Si se colara, las pilas de
			// deshacer y de estados quedarían desfasadas.
			end := start + 1 + next(len(ref)-start-1)
			applyDelete(t, pt, start, end)
			states = append(states, ref[:start]+ref[end:])
		} else {
			off := next(len(ref) + 1)
			text := fragments[next(len(fragments))]
			applyInsert(t, pt, off, text)
			states = append(states, ref[:off]+text+ref[off:])
		}

		if got, want := pt.GetContent(), states[len(states)-1]; got != want {
			t.Fatalf("iter %d: contenido divergente\ngot:  %q\nwant: %q", i, got, want)
		}
	}

	// Deshacer todo, comparando hacia atrás.
	for i := len(states) - 1; i > 0; i-- {
		change := mustUndo(t, pt)

		want := states[i-1]
		if got := pt.GetContent(); got != want {
			t.Fatalf("deshacer %d: contenido divergente\ngot:  %q\nwant: %q", i, got, want)
		}
		if got, want := pt.Len(), len(want); got != want {
			t.Fatalf("deshacer %d: Len() = %d, se esperaba %d", i, got, want)
		}
		if got, want := pt.LineCount(), len(expectedLines(want)); got != want {
			t.Fatalf("deshacer %d: LineCount() = %d, se esperaba %d (contenido %q)", i, got, want, want)
		}
		checkLineOffsetsInvariant(t, i, pt, want)
		if change.Offset < 0 || change.Offset > pt.Len() {
			t.Fatalf("deshacer %d: offset devuelto %d fuera del documento (%d bytes)", i, change.Offset, pt.Len())
		}
	}

	if got := pt.GetContent(); got != initial {
		t.Fatalf("tras deshacer todo, contenido = %q, se esperaba %q", got, initial)
	}
	if !pt.CanRedo() {
		t.Fatal("debería haber rama para rehacer")
	}

	// Rehacer todo, comparando hacia adelante.
	for i := 1; i < len(states); i++ {
		mustRedo(t, pt)

		want := states[i]
		if got := pt.GetContent(); got != want {
			t.Fatalf("rehacer %d: contenido divergente\ngot:  %q\nwant: %q", i, got, want)
		}
		if got, want := pt.LineCount(), len(expectedLines(want)); got != want {
			t.Fatalf("rehacer %d: LineCount() = %d, se esperaba %d", i, got, want)
		}
		checkLineOffsetsInvariant(t, i, pt, want)
	}

	if got, want := pt.GetContent(), states[len(states)-1]; got != want {
		t.Fatalf("tras rehacer todo, contenido divergente\ngot:  %q\nwant: %q", got, want)
	}
}

// TestUndoRedoRoundTripKeepsHistoryConsistent comprueba que deshacer y rehacer
// repetidamente no acumule ni pierda entradas.
func TestUndoRedoRoundTripKeepsHistoryConsistent(t *testing.T) {
	pt := loadTable(t, "base")

	for i := 0; i < 10; i++ {
		applyInsert(t, pt, pt.Len(), "x")
	}
	edits := 10

	for round := 0; round < 3; round++ {
		for i := 0; i < edits; i++ {
			if !pt.CanUndo() {
				t.Fatalf("ronda %d: se acabó el historial antes de tiempo", round)
			}
			mustUndo(t, pt)
		}
		if got := pt.GetContent(); got != "base" {
			t.Fatalf("ronda %d: tras deshacer todo, %q", round, got)
		}
		for i := 0; i < edits; i++ {
			mustRedo(t, pt)
		}
		if got := pt.GetContent(); got != "base"+strings.Repeat("x", edits) {
			t.Fatalf("ronda %d: tras rehacer todo, %q", round, got)
		}
	}
}
