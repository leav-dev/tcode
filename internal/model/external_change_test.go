package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeExternally reescribe el archivo como lo hace otro editor: un archivo temporal
// y un renombre. El inodo cambia, así que el mmap que tenemos abierto sigue viendo
// el contenido viejo y el documento en memoria no se altera.
//
// La fecha se fuerza distinta porque la granularidad del sistema de archivos puede
// hacer que dos escrituras seguidas compartan el mismo timestamp.
func writeExternally(t *testing.T, path, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".externo-*")
	if err != nil {
		t.Fatalf("no se pudo crear el temporal: %v", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("no se pudo escribir el temporal: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("no se pudo cerrar el temporal: %v", err)
	}

	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(tmp.Name(), future, future); err != nil {
		t.Fatalf("no se pudo cambiar la fecha: %v", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		t.Fatalf("no se pudo renombrar: %v", err)
	}
}

func TestChangedOnDiskIsFalseRightAfterLoad(t *testing.T) {
	path := newFileWithContent(t, "uno")
	pt := openTable(t, path)

	if pt.ChangedOnDisk() {
		t.Fatal("recién cargado no puede estar cambiado")
	}
}

func TestChangedOnDiskDetectsAnExternalWrite(t *testing.T) {
	path := newFileWithContent(t, "uno")
	pt := openTable(t, path)

	writeExternally(t, path, "uno modificado por otro proceso")

	if !pt.ChangedOnDisk() {
		t.Fatal("debería detectar la escritura externa")
	}
}

func TestChangedOnDiskDetectsADeletion(t *testing.T) {
	path := newFileWithContent(t, "uno")
	pt := openTable(t, path)

	if err := os.Remove(path); err != nil {
		t.Fatalf("no se pudo borrar el archivo: %v", err)
	}

	if !pt.ChangedOnDisk() {
		t.Fatal("un archivo borrado debe reportarse como cambiado")
	}
}

// TestSaveRefusesWhenTheFileChangedExternally es la propiedad central: el guardado
// no debe pisar en silencio el trabajo de otro proceso.
func TestSaveRefusesWhenTheFileChangedExternally(t *testing.T) {
	path := newFileWithContent(t, "uno")
	pt := openTable(t, path)

	if err := pt.Insert(0, "propio "); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}

	const externo = "escrito por otro proceso"
	writeExternally(t, path, externo)

	err := pt.Save()
	if err != ErrFileChangedExternally {
		t.Fatalf("Save = %v, se esperaba ErrFileChangedExternally", err)
	}

	if got := readFile(t, path); got != externo {
		t.Fatalf("el archivo fue pisado: %q", got)
	}
	// Con un renombre el inodo viejo sigue vivo, así que el documento tampoco cambió.
	if got := pt.GetContent(); got != "propio uno" {
		t.Fatalf("el documento en memoria cambió: %q", got)
	}
	if !pt.Modified() {
		t.Fatal("tras un guardado rechazado el documento sigue modificado")
	}
}

func TestSaveForceOverwritesExternalChanges(t *testing.T) {
	path := newFileWithContent(t, "uno")
	pt := openTable(t, path)

	if err := pt.Insert(0, "propio "); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	writeExternally(t, path, "escrito por otro proceso")

	if err := pt.SaveForce(); err != nil {
		t.Fatalf("SaveForce falló: %v", err)
	}

	if got := readFile(t, path); got != "propio uno" {
		t.Fatalf("tras forzar, archivo = %q", got)
	}
	if pt.Modified() {
		t.Fatal("tras forzar el guardado el documento queda limpio")
	}
}

// TestInPlaceExternalWriteChangesTheMappedContent documenta un peligro real del
// diseño, no una funcionalidad: el mmap es una **vista viva** del archivo.
//
// Una escritura EN EL LUGAR —truncar y reescribir, que es lo que hace os.WriteFile
// y varias herramientas— modifica el mismo inodo que tenemos mapeado. Como las
// piezas del documento apuntan a ese mapeo, el contenido del editor cambia por
// debajo sin que nadie lo pida.
//
// Es la razón por la que la detección de cambios externos no es solo una comodidad
// para no pisar al otro: es la única señal de que el documento en memoria dejó de
// ser confiable. Y es peor de lo que muestra este test: si la escritura deja el
// archivo **más corto** que el mapeo, leer las páginas que quedaron fuera del
// archivo levanta SIGBUS y mata el proceso —el mismo mecanismo que el guardado en
// el lugar de la unidad de Save.
func TestInPlaceExternalWriteChangesTheMappedContent(t *testing.T) {
	path := newFileWithContent(t, "uno")
	pt := openTable(t, path)

	if got := pt.GetContent(); got != "uno" {
		t.Fatalf("contenido inicial = %q", got)
	}

	// Escritura en el lugar, sobre el mismo inodo.
	if err := os.WriteFile(path, []byte("EXTERNO"), 0o644); err != nil {
		t.Fatalf("no se pudo escribir en el lugar: %v", err)
	}

	if !pt.ChangedOnDisk() {
		t.Fatal("debería detectar el cambio")
	}

	// El documento ya no dice lo que decía: lee las páginas del mapeo, que ahora
	// reflejan el archivo reescrito. Solo los primeros len(original) bytes.
	if got := pt.GetContent(); got != "EXT" {
		t.Fatalf("contenido tras la escritura en el lugar = %q, se esperaba %q: "+
			"el mmap refleja el archivo", got, "EXT")
	}
}

// TestSaveAfterForceClearsTheChangedState: una vez forzado el guardado, el archivo
// en disco es el del editor, así que la detección tiene que volver a dar limpio.
func TestSaveAfterForceClearsTheChangedState(t *testing.T) {
	path := newFileWithContent(t, "uno")
	pt := openTable(t, path)

	if err := pt.Insert(0, "propio "); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	writeExternally(t, path, "ajeno")

	if err := pt.SaveForce(); err != nil {
		t.Fatalf("SaveForce falló: %v", err)
	}
	if pt.ChangedOnDisk() {
		t.Fatal("tras guardar, el disco coincide con el editor")
	}

	// Y un guardado normal vuelve a funcionar sin forzar.
	if err := pt.Insert(0, "más "); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := pt.Save(); err != nil {
		t.Fatalf("Save tras el forzado falló: %v", err)
	}
	if got := readFile(t, path); got != "más propio uno" {
		t.Fatalf("archivo = %q", got)
	}
}

// TestSaveWithoutChangesRefreshesTheDiskState: si no hay nada que escribir, un
// cambio externo no puede perderse, así que se deja de reportarlo.
func TestSaveWithoutChangesRefreshesTheDiskState(t *testing.T) {
	path := newFileWithContent(t, "uno")
	pt := openTable(t, path)

	writeExternally(t, path, "ajeno")

	if err := pt.Save(); err != nil {
		t.Fatalf("Save sin cambios falló: %v", err)
	}
	if pt.ChangedOnDisk() {
		t.Fatal("sin cambios locales no tiene sentido seguir avisando")
	}
	if got := readFile(t, path); got != "ajeno" {
		t.Fatalf("archivo = %q, no debía tocarse", got)
	}
}

func TestChangedOnDiskIsFalseWithoutPath(t *testing.T) {
	pt := NewPieceTable()

	if pt.ChangedOnDisk() {
		t.Fatal("un documento sin archivo no puede haber cambiado en disco")
	}
}

func TestSaveRefusesAfterDeletingTheFileExternally(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(path, []byte("uno"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	pt := openTable(t, path)
	if err := pt.Insert(0, "propio "); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("no se pudo borrar: %v", err)
	}

	if err := pt.Save(); err != ErrFileChangedExternally {
		t.Fatalf("Save = %v, se esperaba ErrFileChangedExternally", err)
	}
	// Forzar recrea el archivo: es una decisión explícita de quien llama.
	if err := pt.SaveForce(); err != nil {
		t.Fatalf("SaveForce falló: %v", err)
	}
	if got := readFile(t, path); got != "propio uno" {
		t.Fatalf("archivo = %q", got)
	}
}
