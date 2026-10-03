package controller

import (
	"path/filepath"
	"testing"
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

	if !app.dedupSaveAsConsolidates(bufA, pathB) {
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
	app, pathA, _ := newTwoBufferApp(t, "uno", "dos")
	bufA := app.ws.Active()
	destino := filepath.Join(t.TempDir(), "nuevo.txt")

	if app.dedupSaveAsConsolidates(bufA, destino) {
		t.Fatal("un destino nuevo no debe consolidar")
	}
	if app.ws.Len() != 2 {
		t.Fatalf("workspace = %d buffers, esperaba 2", app.ws.Len())
	}
	_ = pathA
}

// TestDedupSaveAsNeedsTargetActive: si el target del Save As ya no es la
// pestaña activa, el dedup declina (closeTab cerraría otra pestaña).
func TestDedupSaveAsNeedsTargetActive(t *testing.T) {
	app, pathA, pathB := newTwoBufferApp(t, "uno", "dos")

	// Cambiar a la pestaña b hace que a deje de ser el activo.
	app.ws.SetActive(1)
	bufA := app.bufferAtPath(pathA)
	if bufA == nil {
		t.Fatal("setup inválido")
	}
	if bufA == app.ws.Active() {
		t.Fatal("el setup debe dejar a a.txt sin el foco")
	}

	if app.dedupSaveAsConsolidates(bufA, pathB) {
		t.Fatal("el dedup debe declinar si el target no es la pestaña activa")
	}
	if app.ws.Len() != 2 {
		t.Fatalf("workspace = %d buffers, esperaba 2", app.ws.Len())
	}
}
