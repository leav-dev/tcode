package controller

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDirFilesProvidesSiblingGoFiles: el proveedor expone los .go hermanos
// del mismo directorio (contexto multi-archivo del paquete), omite el buffer
// activo y ignora los no-Go.
func TestDirFilesProvidesSiblingGoFiles(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "main.go")
	if err := os.WriteFile(active, []byte("package p\nfunc f() { helper() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "helper.go"), []byte("package p\nfunc helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not source"), 0o644); err != nil {
		t.Fatal(err)
	}

	app, err := newTestAppOnDir(t, dir)
	if err != nil {
		t.Fatalf("newTestAppOnDir falló: %v", err)
	}
	// El archivo activo se abre como buffer (modo archivo / open del test).
	if _, err := app.ws.Open(active); err != nil {
		t.Fatalf("ws.Open: %v", err)
	}

	files, err := app.DirFiles()
	if err != nil {
		t.Fatalf("DirFiles: %v", err)
	}
	if len(files) != 1 || filepath.Base(files[0].Path) != "helper.go" {
		t.Fatalf("DirFiles = %+v, se esperaba solo helper.go", files)
	}
	if files[0].Content == "" {
		t.Fatal("el contenido del hermano debería leerse")
	}
}

// TestReadFileResolvesInsideDir: la lectura puntual resuelve contra el
// directorio del buffer activo y devuelve la ruta absoluta canónica.
func TestReadFileResolvesInsideDir(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "main.ts")
	if err := os.WriteFile(active, []byte("import './sub/util'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "util.ts"), []byte("export const x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app, err := newTestAppOnDir(t, dir)
	if err != nil {
		t.Fatalf("newTestAppOnDir falló: %v", err)
	}
	if _, err := app.ws.Open(active); err != nil {
		t.Fatalf("ws.Open: %v", err)
	}

	f, err := app.ReadFile("sub/util.ts")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if f.Path != filepath.Join(dir, "sub", "util.ts") {
		t.Fatalf("path = %q, se esperaba la absoluta canónica", f.Path)
	}
	if f.Content != "export const x = 1\n" {
		t.Fatalf("content = %q", f.Content)
	}
}

// TestReadFileRejectsEscapes: absolutos, .. que salen del directorio y rutas
// vacías se rechazan antes de tocar el disco.
func TestReadFileRejectsEscapes(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "main.ts")
	if err := os.WriteFile(active, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, err := newTestAppOnDir(t, dir)
	if err != nil {
		t.Fatalf("newTestAppOnDir falló: %v", err)
	}
	if _, err := app.ws.Open(active); err != nil {
		t.Fatalf("ws.Open: %v", err)
	}

	for _, rel := range []string{"", "/etc/hostname", "../afuera.ts", "sub/../../afuera.ts"} {
		if _, err := app.ReadFile(rel); err == nil {
			t.Errorf("ReadFile(%q) debe rechazar la ruta fuera del directorio", rel)
		}
	}
}

// TestReadFileBounds: extensiones no expuestas, directorios, inexistentes y
// archivos sobre la cota no se exponen.
func TestReadFileBounds(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "main.ts")
	if err := os.WriteFile(active, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notas.txt"), []byte("no fuente"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	big := make([]byte, maxReadFileBytes+1)
	if err := os.WriteFile(filepath.Join(dir, "grande.ts"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	app, err := newTestAppOnDir(t, dir)
	if err != nil {
		t.Fatalf("newTestAppOnDir falló: %v", err)
	}
	if _, err := app.ws.Open(active); err != nil {
		t.Fatalf("ws.Open: %v", err)
	}

	for _, rel := range []string{"notas.txt", "sub", "falta.ts", "grande.ts"} {
		if _, err := app.ReadFile(rel); err == nil {
			t.Errorf("ReadFile(%q) debe fallar por cota", rel)
		}
	}
}

// TestReadFileResolvesSymlinkEscape: un symlink dentro del directorio que
// apunta afuera se resuelve y se re-valida: no sale.
func TestReadFileResolvesSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	fuera := t.TempDir()
	active := filepath.Join(dir, "main.ts")
	if err := os.WriteFile(active, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fuera, "secreto.ts"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(fuera, "secreto.ts"), filepath.Join(dir, "link.ts")); err != nil {
		t.Skipf("sin symlinks en este entorno: %v", err)
	}
	app, err := newTestAppOnDir(t, dir)
	if err != nil {
		t.Fatalf("newTestAppOnDir falló: %v", err)
	}
	if _, err := app.ws.Open(active); err != nil {
		t.Fatalf("ws.Open: %v", err)
	}

	if _, err := app.ReadFile("link.ts"); err == nil {
		t.Error("ReadFile(link.ts) debe rechazar el symlink que apunta afuera")
	}
}

// TestReadFileWithoutActiveBuffer: sin buffer activo, error legible.
func TestReadFileWithoutActiveBuffer(t *testing.T) {
	app, _ := newTestAppOnDir(t, t.TempDir()) // el workspace no tiene buffers
	if _, err := app.ReadFile("x.ts"); err == nil {
		t.Fatal("sin buffer activo, ReadFile debe fallar con error legible")
	}
}

// TestDirFilesWithoutActiveBuffer: sin buffer activo, error legible.
func TestDirFilesWithoutActiveBuffer(t *testing.T) {
	app, _ := newTestAppOnDir(t, t.TempDir()) // el workspace no tiene buffers
	if _, err := app.DirFiles(); err == nil {
		t.Fatal("sin buffer activo, DirFiles debe fallar con error legible")
	}
}
