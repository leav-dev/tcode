package model

import (
	"os"
	"path/filepath"
	"testing"
)

func newFileWithContent(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	return path
}

func openTable(t *testing.T, path string) *PieceTable {
	t.Helper()
	pt := NewPieceTable()
	if err := pt.LoadFile(path); err != nil {
		t.Fatalf("LoadFile falló: %v", err)
	}
	t.Cleanup(func() { pt.Close() })
	return pt
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", path, err)
	}
	return string(b)
}

func TestSaveWritesEditsToDisk(t *testing.T) {
	path := newFileWithContent(t, "hola mundo")
	pt := openTable(t, path)

	if err := pt.Insert(4, " lindo"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if _, err := pt.Delete(0, 5); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}

	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}

	if got := readFile(t, path); got != "lindo mundo" {
		t.Fatalf("archivo en disco = %q, se esperaba %q", got, "lindo mundo")
	}
}

func TestSaveKeepsTheDocumentUsableAfterwards(t *testing.T) {
	path := newFileWithContent(t, "uno\ndos\n")
	pt := openTable(t, path)

	if err := pt.Insert(0, "cero\n"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}

	// Tras guardar, la tabla se recarga desde el archivo: el documento tiene que
	// seguir siendo el mismo y el índice de líneas tiene que ser coherente.
	if got := pt.GetContent(); got != "cero\nuno\ndos\n" {
		t.Fatalf("contenido = %q, se esperaba %q", got, "cero\nuno\ndos\n")
	}
	if got, want := pt.LineCount(), 4; got != want {
		t.Fatalf("LineCount() = %d, se esperaba %d", got, want)
	}
	if got := string(pt.LineContent(1)); got != "uno" {
		t.Fatalf("LineContent(1) = %q, se esperaba %q", got, "uno")
	}
	if got := string(pt.GetRange(2, 3)); got != "dos\n" {
		t.Fatalf("GetRange(2,3) = %q", got)
	}

	// Y se tiene que poder seguir editando y guardando.
	if err := pt.Insert(0, "// "); err != nil {
		t.Fatalf("Insert posterior al Save falló: %v", err)
	}
	if err := pt.Save(); err != nil {
		t.Fatalf("segundo Save falló: %v", err)
	}
	if got := readFile(t, path); got != "// cero\nuno\ndos\n" {
		t.Fatalf("archivo en disco = %q", got)
	}
}

func TestSavePreservesExactBytes(t *testing.T) {
	// Mezcla de no-ASCII, saltos de línea y tabulaciones, para comprobar que el
	// volcado por piezas no reinterpreta nada.
	const content = "café 日 ☕\n\ttabulado\nfin sin salto"
	path := newFileWithContent(t, content)
	pt := openTable(t, path)

	if err := pt.Insert(len(content), "!"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}

	if got := readFile(t, path); got != content+"!" {
		t.Fatalf("archivo en disco = %q, se esperaba %q", got, content+"!")
	}
}

func TestModifiedLifecycle(t *testing.T) {
	path := newFileWithContent(t, "uno\ndos")
	pt := openTable(t, path)

	if pt.Modified() {
		t.Fatal("un documento recién cargado no debe estar modificado")
	}

	if err := pt.Insert(0, "x"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if !pt.Modified() {
		t.Fatal("tras Insert el documento debe quedar modificado")
	}

	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}
	if pt.Modified() {
		t.Fatal("tras Save el documento no debe quedar modificado")
	}

	if _, err := pt.Delete(0, 1); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}
	if !pt.Modified() {
		t.Fatal("tras Delete el documento debe quedar modificado")
	}
}

func TestSaveWithoutChangesIsANoOp(t *testing.T) {
	path := newFileWithContent(t, "uno\ndos")
	pt := openTable(t, path)

	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat falló: %v", err)
	}

	if err := pt.Save(); err != nil {
		t.Fatalf("Save sin cambios falló: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat falló: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("guardar sin cambios no debe reescribir el archivo (mtime %v -> %v)",
			before.ModTime(), after.ModTime())
	}
	if got := readFile(t, path); got != "uno\ndos" {
		t.Fatalf("contenido = %q", got)
	}
}

func TestSaveWithoutPathFails(t *testing.T) {
	pt := NewPieceTable()
	if err := pt.Insert(0, "texto"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := pt.Save(); err != ErrNoPath {
		t.Fatalf("Save sin ruta = %v, se esperaba ErrNoPath", err)
	}
	if pt.Path() != "" {
		t.Fatalf("Path() = %q, se esperaba vacío", pt.Path())
	}
}

func TestSavePreservesFilePermissions(t *testing.T) {
	path := newFileWithContent(t, "uno")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("Chmod falló: %v", err)
	}

	pt := openTable(t, path)
	if err := pt.Insert(0, "x"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat falló: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("permisos = %o, se esperaban 600: CreateTemp crea con 600 y hay que respetar el original", got)
	}
}

// TestSaveLeavesTheOriginalIntactOnFailure es la prueba de la propiedad de
// seguridad: si no se puede escribir el temporal, el archivo original no debe
// quedar tocado. Escribir en el lugar habría truncado el archivo.
func TestSaveLeavesTheOriginalIntactOnFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("corriendo como root: los permisos de solo lectura no aplican")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	const original = "contenido original\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	pt := openTable(t, path)
	if err := pt.Insert(0, "NO DEBE LLEGAR AL DISCO\n"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}

	// Directorio sin permiso de escritura: CreateTemp tiene que fallar.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Chmod falló: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	if err := pt.Save(); err == nil {
		t.Fatal("Save debería fallar en un directorio sin permiso de escritura")
	}

	if got := readFile(t, path); got != original {
		t.Fatalf("el archivo original cambió tras un Save fallido: %q", got)
	}
	if !pt.Modified() {
		t.Fatal("tras un Save fallido el documento debe seguir marcado como modificado")
	}
	// El documento en memoria tampoco debe haberse perdido.
	if got := pt.GetContent(); got != "NO DEBE LLEGAR AL DISCO\n"+original {
		t.Fatalf("el documento en memoria cambió: %q", got)
	}
}

// TestSaveFollowsSymlinks cubre un pie de banco real del guardado atómico:
// renombrar encima de un enlace simbólico lo reemplazaría por un archivo común.
func TestSaveFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.txt")
	link := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(real, []byte("uno"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo real: %v", err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("no se pudo crear el enlace: %v", err)
	}

	pt := openTable(t, link)
	if err := pt.Insert(3, " dos"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("Lstat falló: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("el enlace simbólico fue reemplazado por un archivo común")
	}
	if got := readFile(t, real); got != "uno dos" {
		t.Fatalf("el archivo real = %q, se esperaba %q", got, "uno dos")
	}
}

func TestSaveDoesNotLeaveTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(path, []byte("uno"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}

	pt := openTable(t, path)
	if err := pt.Insert(0, "x"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir falló: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("quedaron archivos en el directorio: %v", names)
	}
}

func TestSaveEmptyDocument(t *testing.T) {
	path := newFileWithContent(t, "contenido")
	pt := openTable(t, path)

	if _, err := pt.Delete(0, pt.Len()); err != nil {
		t.Fatalf("Delete falló: %v", err)
	}
	if err := pt.Save(); err != nil {
		t.Fatalf("Save falló: %v", err)
	}

	if got := readFile(t, path); got != "" {
		t.Fatalf("archivo en disco = %q, se esperaba vacío", got)
	}
	if got := pt.Len(); got != 0 {
		t.Fatalf("Len() = %d, se esperaba 0", got)
	}
	if got := pt.LineCount(); got != 0 {
		t.Fatalf("LineCount() = %d, se esperaba 0", got)
	}
	if pt.Modified() {
		t.Fatal("tras guardar no debe quedar modificado")
	}
}
