package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAsWritesToTheNewPath(t *testing.T) {
	original := newFileWithContent(t, "uno")
	pt := openTable(t, original)

	if err := pt.Insert(0, "propio "); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "copia.txt")

	if err := pt.SaveAs(dest); err != nil {
		t.Fatalf("SaveAs falló: %v", err)
	}

	if got := readFile(t, dest); got != "propio uno" {
		t.Fatalf("destino = %q", got)
	}
	// El archivo original queda como estaba: Save As copia, no mueve.
	if got := readFile(t, original); got != "uno" {
		t.Fatalf("el original cambió: %q", got)
	}
}

func TestSaveAsSwitchesTheWorkingPath(t *testing.T) {
	original := newFileWithContent(t, "uno")
	pt := openTable(t, original)

	dir := t.TempDir()
	dest := filepath.Join(dir, "destino.txt")

	if err := pt.Insert(0, "A"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := pt.SaveAs(dest); err != nil {
		t.Fatalf("SaveAs falló: %v", err)
	}

	if got := pt.Path(); got != dest {
		t.Fatalf("Path() = %q, se esperaba %q", got, dest)
	}
	if pt.Modified() {
		t.Fatal("tras Save As el documento queda limpio")
	}

	// El guardado siguiente va al destino nuevo, no al original.
	if err := pt.Insert(0, "B"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}

	if got := readFile(t, dest); got != "BAuno" {
		t.Fatalf("destino = %q, se esperaba %q", got, "BAuno")
	}
	if got := readFile(t, original); got != "uno" {
		t.Fatalf("el original no debía tocarse: %q", got)
	}
}

// TestSaveAsWritesEvenWithoutChanges: elegir una ruta es una decisión explícita, así
// que tiene que escribir aunque el documento esté limpio.
func TestSaveAsWritesEvenWithoutChanges(t *testing.T) {
	path := newFileWithContent(t, "contenido")
	pt := openTable(t, path)

	if pt.Modified() {
		t.Fatal("el documento recién cargado está limpio")
	}

	dest := filepath.Join(t.TempDir(), "copia.txt")
	if err := pt.SaveAs(dest); err != nil {
		t.Fatalf("SaveAs falló: %v", err)
	}

	if got := readFile(t, dest); got != "contenido" {
		t.Fatalf("destino = %q, se esperaba %q", got, "contenido")
	}
}

// TestSaveAsWorksForADocumentWithoutPath cubre el caso que motivó la unidad: un
// documento nuevo no tenía forma de llegar al disco.
func TestSaveAsWorksForADocumentWithoutPath(t *testing.T) {
	pt := NewPieceTable()

	if err := pt.Insert(0, "contenido nuevo"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	// Guardar sin ruta sigue fallando: primero hay que elegir una.
	if err := pt.Save(); err != ErrNoPath {
		t.Fatalf("Save = %v, se esperaba ErrNoPath", err)
	}

	dest := filepath.Join(t.TempDir(), "nuevo.txt")
	if err := pt.SaveAs(dest); err != nil {
		t.Fatalf("SaveAs falló: %v", err)
	}

	if got := readFile(t, dest); got != "contenido nuevo" {
		t.Fatalf("destino = %q", got)
	}
	if pt.Modified() {
		t.Fatal("tras Save As el documento queda limpio")
	}
	if got := pt.Path(); got != dest {
		t.Fatalf("Path() = %q", got)
	}
}

func TestSaveAsRejectsAnEmptyPath(t *testing.T) {
	pt := openTable(t, newFileWithContent(t, "uno"))

	if err := pt.SaveAs(""); err != ErrEmptyPath {
		t.Fatalf("SaveAs(\"\") = %v, se esperaba ErrEmptyPath", err)
	}
}

func TestSaveAsOverwritesAnExistingDestination(t *testing.T) {
	pt := openTable(t, newFileWithContent(t, "origen"))

	dest := filepath.Join(t.TempDir(), "existente.txt")
	if err := os.WriteFile(dest, []byte("viejo"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el destino: %v", err)
	}

	if err := pt.SaveAs(dest); err != nil {
		t.Fatalf("SaveAs falló: %v", err)
	}

	if got := readFile(t, dest); got != "origen" {
		t.Fatalf("destino = %q, se esperaba %q", got, "origen")
	}
}

func TestSaveAsRespectsTheDestinationPermissions(t *testing.T) {
	pt := openTable(t, newFileWithContent(t, "origen"))

	dest := filepath.Join(t.TempDir(), "ejecutable.sh")
	if err := os.WriteFile(dest, []byte("viejo"), 0o700); err != nil {
		t.Fatalf("no se pudo crear el destino: %v", err)
	}

	if err := pt.SaveAs(dest); err != nil {
		t.Fatalf("SaveAs falló: %v", err)
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("Stat falló: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("permisos = %o, se esperaban 700", got)
	}
}

// TestSaveAsCreatesWithReadablePermissions: un archivo nuevo no puede quedar con
// los 0600 de CreateTemp.
func TestSaveAsCreatesWithReadablePermissions(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("como root el umask no aplica igual")
	}

	pt := openTable(t, newFileWithContent(t, "origen"))
	dest := filepath.Join(t.TempDir(), "nuevo.txt")

	if err := pt.SaveAs(dest); err != nil {
		t.Fatalf("SaveAs falló: %v", err)
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("Stat falló: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Fatalf("permisos = %o, se esperaban 644", perm)
	}
}

// TestSaveAsClearsTheExternalChangeWarning: la marca de disco pertenecía a la ruta
// vieja, así que después de Save As el destino no puede reportarse como cambiado.
func TestSaveAsClearsTheExternalChangeWarning(t *testing.T) {
	original := newFileWithContent(t, "uno")
	pt := openTable(t, original)

	writeExternally(t, original, "ajeno")

	if !pt.ChangedOnDisk() {
		t.Fatal("el original cambió en disco")
	}

	dest := filepath.Join(t.TempDir(), "copia.txt")
	if err := pt.SaveAs(dest); err != nil {
		t.Fatalf("SaveAs falló: %v", err)
	}

	if pt.ChangedOnDisk() {
		t.Fatal("tras Save As el destino coincide con el editor")
	}
	if err := pt.Save(); err != nil {
		t.Fatalf("Save tras Save As falló: %v", err)
	}
}

// TestSaveAsKeepsTheHistoryUsable: después de Save As se tiene que poder seguir
// deshaciendo, igual que después de un guardado normal.
func TestSaveAsKeepsTheHistoryUsable(t *testing.T) {
	pt := openTable(t, newFileWithContent(t, "base"))

	typing(pt, 4, " uno")
	dest := filepath.Join(t.TempDir(), "copia.txt")
	if err := pt.SaveAs(dest); err != nil {
		t.Fatalf("SaveAs falló: %v", err)
	}

	mustUndo(t, pt)

	if got := pt.GetContent(); got != "base" {
		t.Fatalf("tras deshacer, %q, se esperaba %q", got, "base")
	}
	if !pt.Modified() {
		t.Fatal("deshacer tras Save As deja el documento modificado")
	}
	if got := readFile(t, dest); got != "base uno" {
		t.Fatalf("el destino no debe cambiar con deshacer: %q", got)
	}

	mustRedo(t, pt)
	if pt.Modified() {
		t.Fatal("rehacer hasta el punto guardado deja el documento limpio")
	}
}
