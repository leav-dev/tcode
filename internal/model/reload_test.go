package model

import (
	"os"
	"strings"
	"testing"
)

// docText lee el documento completo como texto, para las aserciones de recarga.
func docText(pt *PieceTable) string {
	return string(pt.GetRange(0, pt.Len()))
}

// TestReloadReplacesContentAndDiscardsEdits es el corazón de la recarga: el
// documento vuelve al estado del disco, las ediciones sin guardar desaparecen,
// el historial arranca vacío y la marca de cambio externo se refresca.
func TestReloadReplacesContentAndDiscardsEdits(t *testing.T) {
	path := newFileWithContent(t, "uno\ndos\ntres\n")
	pt := openTable(t, path)

	if err := pt.Insert(len("uno\n"), "X"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if !pt.Modified() {
		t.Fatal("el test requiere el buffer sucio")
	}

	// Windows no permite tocar (renombrar/truncar/escribir) un archivo con una
	// sección mapeada abierta en el MISMO proceso (ERROR_USER_MAPPED_FILE); un
	// cambio de otro proceso sí se puede. El test desmapea primero y escribe el
	// cambio como lo haría ese otro proceso; el contrato bajo prueba —Reload
	// trae el disco y descarta las ediciones— queda intacto.
	pt.release()
	writeExternally(t, path, "nuevo\ncontenido\n")

	if err := pt.Reload(); err != nil {
		t.Fatalf("Reload falló: %v", err)
	}
	if got := docText(pt); got != "nuevo\ncontenido\n" {
		t.Fatalf("contenido = %q, esperaba el estado del disco", got)
	}
	if pt.Modified() {
		t.Fatal("tras recargar el documento no tiene ediciones")
	}
	if pt.CanUndo() || pt.CanRedo() {
		t.Fatal("tras recargar el historial debe estar vacío")
	}
	if pt.ChangedOnDisk() {
		t.Fatal("la marca de disco debe refrescarse tras recargar")
	}
}

// TestReloadShrunkenFileDoesNotSigBus recarga un archivo que quedó MUCHO más
// corto que el mapeo viejo. Leer el mapeo viejo tras el cambio levantaría
// SIGBUS; Reload desmapea antes de leer y el documento queda con lo nuevo.
func TestReloadShrunkenFileDoesNotSigBus(t *testing.T) {
	big := strings.Repeat("línea de relleno con 日本語\n", 5000)
	path := newFileWithContent(t, big)
	pt := openTable(t, path)

	if err := pt.Insert(pt.Len(), "cola"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}

	pt.release() // ver nota de Windows en el test anterior
	writeExternally(t, path, "x\n")

	if err := pt.Reload(); err != nil {
		t.Fatalf("Reload falló: %v", err)
	}
	if got := docText(pt); got != "x\n" {
		t.Fatalf("contenido = %q, esperaba el archivo encogido", got)
	}
}

// TestReloadMissingFileReportsAndLeavesEmptyBuffer: si el archivo desapareció
// no hay nada que recargar; el error se reporta y el buffer queda desmapeado,
// no roto.
func TestReloadMissingFileReportsAndLeavesEmptyBuffer(t *testing.T) {
	path := newFileWithContent(t, "contenido")
	pt := openTable(t, path)

	pt.release() // ver nota de Windows en el test anterior: sin mapeo el borrado es legal
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove falló: %v", err)
	}

	if err := pt.Reload(); err == nil {
		t.Fatal("recargar un archivo borrado debe fallar")
	}
	if pt.Len() != 0 {
		t.Fatalf("len = %d, el buffer debe quedar vacío tras el fallo", pt.Len())
	}
	if pt.Modified() {
		t.Fatal("un buffer sin archivo no puede reportar ediciones")
	}
}

// TestReloadWithoutPathReturnsError: una tabla sin ruta no puede recargar.
func TestReloadWithoutPathReturnsError(t *testing.T) {
	pt := NewPieceTable()
	if err := pt.Reload(); err != ErrNoPath {
		t.Fatalf("Reload = %v, esperaba ErrNoPath", err)
	}
}
