package controller

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestDedupSaveAsConsolidates: un Save As a una ruta ya abierta en otra pestaña
// no puede dejar dos buffers sobre el mismo archivo: el recién guardado se
// cierra y el existente queda activo, recargado desde disco.
//
// El helper se prueba sin el SaveAs real: en este host Windows no se puede
// escribir sobre un archivo con una sección mapeada abierta (el mismo bloqueo
// de los tests ambientales), así que el test ejerce el reordenamiento del
// workspace —cierre, activación y recarga— que es la parte del dedup que el
// controlador aporta; el SaveAs (escribir y cambiar de ruta) ya está cubierto
// por los tests del modelo.
// TestDedupSaveAsConsolidates: target activo sobre a.txt, destino b.txt
// (abierta en otra pestaña) → una sola pestaña, b activo y recargado.
func TestDedupSaveAsConsolidates(t *testing.T) {
	app, pathA, pathB := newTwoBufferApp(t, "uno", "dos")

	bufA := app.ws.Active() // a.txt es el activo al arrancar
	if bufA.Path() != pathA {
		t.Fatalf("activo = %q, esperaba %q", bufA.Path(), pathA)
	}

	// El Save As ya se escribió en el modelo: el target tiene la ruta destino.
	bufB := app.bufferAtPath(pathB)
	if bufB == nil {
		t.Fatal("el setup requiere b.txt abierto")
	}

	if !app.dedupSaveAsConsolidates(bufB) {
		t.Fatal("el dedup debía consolidar (el destino ya estaba abierto)")
	}

	if app.ws.Len() != 1 {
		t.Fatalf("workspace = %d buffers, esperaba 1 tras el dedup", app.ws.Len())
	}
	active := app.ws.Active()
	if active == nil || active.Path() != pathB {
		path := "<nil>"
		if active != nil {
			path = active.Path()
		}
		t.Fatalf("activo = %q, esperaba el buffer de %q", path, pathB)
	}
	if active.Modified() {
		t.Fatal("el buffer consolidado debe quedar limpio (Reload del disco)")
	}
}

// TestDedupSaveAsDoesNothingForFreshPath: un destino nuevo (no abierto) no
// consolida nada y el workspace queda con sus dos pestañas.
func TestDedupSaveAsDoesNothingForFreshPath(t *testing.T) {
	app, _, _ := newTwoBufferApp(t, "uno", "dos")

	if app.dedupSaveAsConsolidates(nil) {
		t.Fatal("sin buffer previo en el destino no debe consolidar")
	}
	if app.ws.Len() != 2 {
		t.Fatalf("workspace = %d buffers, esperaba 2", app.ws.Len())
	}
}

// TestDedupSaveAsNeedsNoPriorUnmapDoesNotCrash: consolidar un buffer que
// todavía está mapeado (sin el Unmap que hace el flujo de saveAs) es igual
// válido: Reload lo re-mapea.
func TestDedupSaveAsNeedsNoPriorUnmapDoesNotCrash(t *testing.T) {
	app, _, pathB := newTwoBufferApp(t, "uno", "dos")
	bufB := app.bufferAtPath(pathB)
	if bufB == nil {
		t.Fatal("setup inválido")
	}

	if !app.dedupSaveAsConsolidates(bufB) {
		t.Fatal("el dedup debe consolidar sin Unmap previo")
	}
	if app.ws.Len() != 1 {
		t.Fatalf("workspace = %d buffers, esperaba 1", app.ws.Len())
	}
	if bufB.Modified() {
		t.Fatal("el buffer consolidado debe quedar limpio")
	}
}

// TestSaveAsToOpenPathWorks es el flujo real (prompt + Save As + dedup) sobre
// un destino ya abierto y mapeado: con el fix del rename de Windows (Unmap del
// buffer destino antes de escribir) debe funcionar en este host.
func TestSaveAsToOpenPathWorks(t *testing.T) {
	app, pathA, pathB := newTwoBufferApp(t, "uno\n", "dos\n")

	// El buffer a (activo) se edita y se guarda AS con la ruta de b: el prompt
	// se alimenta y el Enter dispara el saveAs (flujo real de handlePromptKey).
	typeRune(app, 'X')
	app.startPrompt()
	app.promptBuf = pathB
	hk := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	app.handlePromptKey(hk)
	t.Logf("mensaje: %q", app.statusBar.Message())
	t.Logf("activo=%q other(pathB)=%v target(pathA)=%v", app.ws.Active().Path(), app.bufferAtPath(pathB) != nil, app.bufferAtPath(pathA) != nil)
	if app.ws.Active() == nil {
		t.Fatal("workspace vacío tras el dedup")
	}
	if app.ws.Len() != 1 {
		t.Fatalf("Len = %d, esperaba 1 (dedup)", app.ws.Len())
	}
	if app.ws.Active().Path() != pathB {
		t.Fatalf("activo = %q, esperaba %q", app.ws.Active().Path(), pathB)
	}
	if got := app.ws.Active().GetContent(); got != "Xuno\n" {
		t.Fatalf("contenido del buffer consolidado = %q, esperaba el doc guardado", got)
	}
	_ = pathA
}
