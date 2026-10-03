package model

import (
	"os"
	"testing"
)

// TestSaveOverMappedFileWorks es el contrato que Windows exige: guardar sobre
// un archivo abierto (mapeado por el buffer) tiene que poder renombrar el
// temporal sobre el destino. Windows bloquea el rename mientras exista una
// sección mapeada del MISMO proceso sobre el destino (ERROR_USER_MAPPED_FILE →
// mensaje "Acceso denegado"); por eso el mapeo se libera antes del rename.
func TestSaveOverMappedFileWorks(t *testing.T) {
	path := newFileWithContent(t, "uno\n")
	pt := openTable(t, path)
	t.Cleanup(func() { pt.Close() })

	if err := pt.Insert(0, "X"); err != nil {
		t.Fatalf("Insert falló: %v", err)
	}
	if err := pt.SaveForce(); err != nil {
		t.Fatalf("guardar sobre un archivo mapeado falló: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer el archivo guardado: %v", err)
	}
	if string(got) != "Xuno\n" {
		t.Fatalf("disco = %q, esperaba %q", got, "Xuno\n")
	}
	if pt.Modified() {
		t.Fatal("tras guardar el buffer no debe quedar modificado")
	}
	if docText(pt) != "Xuno\n" {
		t.Fatalf("buffer = %q, esperaba %q", docText(pt), "Xuno\n")
	}
}

// TestSaveAsOverExistingMappedFileWorks: Save As a un destino que ya estaba
// abierto (también mapeado) guarda y sigue corrigiendo el buffer.
func TestSaveAsOverExistingMappedFileWorks(t *testing.T) {
	src := newFileWithContent(t, "origen\n")
	dst := newFileWithContent(t, "destino existente\n") + "" // el destino real
	dst = src + ".bak"
	if err := os.WriteFile(dst, []byte("destino existente\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pt := openTable(t, src)
	t.Cleanup(func() { pt.Close() })

	// El destino también está abierto/mapeado: es la ruta ya abierta del dedup.
	// El controlador desmapea ese buffer antes del Save As (Unmap) para
	// destrabar el rename de Windows; acá se reproduce esa mecánica del modelo.
	dstPt := openTable(t, dst)
	t.Cleanup(func() { dstPt.Close() })
	dstPt.Unmap()

	if err := pt.SaveAs(dst); err != nil {
		t.Fatalf("SaveAs sobre destino mapeado falló: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("no se pudo leer el destino: %v", err)
	}
	if string(got) != "origen\n" {
		t.Fatalf("destino = %q, esperaba %q", got, "origen\n")
	}
	if pt.Path() != dst {
		t.Fatalf("el buffer debe apuntar al destino nuevo, apunta a %q", pt.Path())
	}
}
