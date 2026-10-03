package ext

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeExtension crea dir/<name>/extension.json con src y devuelve la ruta del
// directorio de la extensión.
func writeExtension(t *testing.T, root, name, src string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile extension.json: %v", err)
	}
	return dir
}

// TestDiscoverFindsValidExtensions carga todas las extensiones válidas de un
// directorio con su Dir correcto, en orden lexicográfico (ReadDir ordena).
func TestDiscoverFindsValidExtensions(t *testing.T) {
	root := t.TempDir()
	writeExtension(t, root, "b-ext", validManifest)
	writeExtension(t, root, "a-ext", validManifest)

	exts, errs := Discover(root)
	if len(errs) != 0 {
		t.Fatalf("Discover reportó errores con extensiones válidas: %v", errs)
	}
	if len(exts) != 2 {
		t.Fatalf("Discover devolvió %d extensiones, esperaba 2", len(exts))
	}
	if exts[0].Manifest.ID != "tcode.demosaludo" || exts[1].Manifest.ID != "tcode.demosaludo" {
		t.Errorf("orden o contenido inesperado: %+v", exts)
	}
	if exts[0].Dir != filepath.Join(root, "a-ext") {
		t.Errorf("Dir = %q, esperaba %q", exts[0].Dir, filepath.Join(root, "a-ext"))
	}
}

// TestDiscoverSkipsBrokenExtensions es el corazón de la tolerancia: una
// extensión con JSON inválido o manifest que no valida no impide que las
// demás se carguen; su error se acumula y se nombra su directorio.
func TestDiscoverSkipsBrokenExtensions(t *testing.T) {
	root := t.TempDir()
	writeExtension(t, root, "rota", "{ no json")
	writeExtension(t, root, "invalida", `{"id": "", "version": "0.1.0"}`)
	writeExtension(t, root, "sana", validManifest)

	exts, errs := Discover(root)
	if len(exts) != 1 || exts[0].Dir != filepath.Join(root, "sana") {
		t.Fatalf("Discover = %+v, esperaba solo la extensión sana", exts)
	}
	if len(errs) != 2 {
		t.Fatalf("Discover acumuló %d errores, esperaba 2: %v", len(errs), errs)
	}
	for _, e := range errs {
		if !strings.Contains(e.Error(), "rota") && !strings.Contains(e.Error(), "invalida") {
			t.Errorf("error sin nombre de extensión: %q", e)
		}
	}
}

// TestDiscoverIgnoresPlainDirectories no se queja por carpetas que no son
// extensiones, y el extension.json de la raíz misma no cuenta como extensión.
func TestDiscoverIgnoresPlainDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extension.json"), []byte(validManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	exts, errs := Discover(root)
	if len(exts) != 0 || len(errs) != 0 {
		t.Fatalf("Discover = (%d ext, %d errs), esperaba (0, 0)", len(exts), len(errs))
	}
}

// TestDiscoverMissingRoot es el caso del arranque sin carpeta de extensiones:
// no es un error, no hay nada que descubrir.
func TestDiscoverMissingRoot(t *testing.T) {
	exts, errs := Discover(filepath.Join(t.TempDir(), "no-existe"))
	if len(exts) != 0 || len(errs) != 0 {
		t.Fatalf("Discover = (%d ext, %d errs), esperaba (0, 0)", len(exts), len(errs))
	}
}
