package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// uninstallFixture arma un home falso: binario en ~/.tcode/bin y rcs con el
// bloque marcado más contenido propio que debe sobrevivir.
func uninstallFixture(t *testing.T) (home, exe string) {
	t.Helper()
	home = t.TempDir()
	bin := filepath.Join(home, ".tcode", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	exe = filepath.Join(bin, "tcode")
	if err := os.WriteFile(exe, []byte("binario"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc := "export PATH=\"$HOME/bin:$PATH\"\n# >>> tcode >>>\nexport PATH=\"${HOME}/.tcode/bin:${PATH}\"\n# <<< tcode <<<\nalias ll='ls -l'\n"
	for _, name := range []string{".bashrc", ".zshrc"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte(rc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home, exe
}

// TestUninstallBorraBinarioDirYPATH: el flujo completo deja home sin binario
// ni marcas, pero conserva el resto de los rc.
func TestUninstallBorraBinarioDirYPATH(t *testing.T) {
	home, exe := uninstallFixture(t)

	removed, err := Uninstall(home, exe)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if len(removed) != 4 {
		t.Fatalf("eliminados = %v, esperaba binario, dir y 2 rcs", removed)
	}
	if _, err := os.Stat(exe); !os.IsNotExist(err) {
		t.Fatalf("el binario sigue existiendo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".tcode", "bin")); !os.IsNotExist(err) {
		t.Fatalf("el dir de instalación sigue existiendo: %v", err)
	}
	for _, name := range []string{".bashrc", ".zshrc"} {
		data, _ := os.ReadFile(filepath.Join(home, name))
		if strings.Contains(string(data), "tcode") {
			t.Fatalf("%s aún menciona tcode:\n%s", name, data)
		}
		if !strings.Contains(string(data), "alias ll=") {
			t.Fatalf("%s perdió contenido propio:\n%s", name, data)
		}
	}
}

// TestUninstallFueraDelDirDeInstalacion: un binario en otro dir (go/bin, …)
// se borra solo él; el dir ajeno no se toca.
func TestUninstallFueraDelDirDeInstalacion(t *testing.T) {
	home := t.TempDir()
	other := filepath.Join(home, "go", "bin")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(other, "tcode")
	tool := filepath.Join(other, "otra-herramienta")
	if err := os.WriteFile(exe, []byte("binario"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, []byte("otra"), 0o755); err != nil {
		t.Fatal(err)
	}

	removed, err := Uninstall(home, exe)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if len(removed) != 1 || removed[0] != exe {
		t.Fatalf("eliminados = %v, esperaba solo el binario", removed)
	}
	if _, err := os.Stat(tool); err != nil {
		t.Fatalf("tocó herramientas ajenas: %v", err)
	}
}

// TestUninstallSinRcs: sin archivos rc no hay nada que limpiar y no falla.
func TestUninstallSinRcs(t *testing.T) {
	home := t.TempDir()
	exe := filepath.Join(home, "tcode")
	if err := os.WriteFile(exe, []byte("binario"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(home, exe); err != nil {
		t.Fatalf("Uninstall sin rcs: %v", err)
	}
}

// TestUninstallSegundaVezFalla: ya sin binario, avisa en vez de decir que
// borró algo.
func TestUninstallSegundaVezFalla(t *testing.T) {
	home, exe := uninstallFixture(t)
	if _, err := Uninstall(home, exe); err != nil {
		t.Fatalf("primer Uninstall: %v", err)
	}
	if _, err := Uninstall(home, exe); err == nil {
		t.Fatal("el segundo Uninstall debió fallar")
	}
}

// TestStripMarkedBlock: saca el bloque marcado y deja el resto; sin marcas
// devuelve el texto intacto para no reescribir.
func TestStripMarkedBlock(t *testing.T) {
	with := "a\n# >>> tcode >>>\nx\n# <<< tcode <<<\nb\n"
	if got := stripMarkedBlock(with); got != "a\nb\n" {
		t.Fatalf("con marcas = %q, esperaba %q", got, "a\nb\n")
	}
	without := "a\nb\n"
	if got := stripMarkedBlock(without); got != without {
		t.Fatalf("sin marcas = %q, esperaba intacto", got)
	}
}
