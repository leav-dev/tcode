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

// TestDirFilesWithoutActiveBuffer: sin buffer activo, error legible.
func TestDirFilesWithoutActiveBuffer(t *testing.T) {
	app, _ := newTestAppOnDir(t, t.TempDir()) // el workspace no tiene buffers
	if _, err := app.DirFiles(); err == nil {
		t.Fatal("sin buffer activo, DirFiles debe fallar con error legible")
	}
}