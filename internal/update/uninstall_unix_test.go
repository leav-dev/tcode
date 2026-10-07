//go:build unix

package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUninstallAcumulaErroresDeRc: un rc no escribible no impide limpiar el
// otro; el error se acumula y lo ya eliminado vuelve junto a él (progreso
// parcial visible, no un nil).
func TestUninstallAcumulaErroresDeRc(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignora los permisos: el chmod no bloquearía el write")
	}
	home, exe := uninstallFixture(t)
	locked := filepath.Join(home, ".bashrc")
	if err := os.Chmod(locked, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })

	removed, err := Uninstall(home, exe)
	if err == nil || !strings.Contains(err.Error(), ".bashrc") {
		t.Fatalf("err = %v, esperaba el error acumulado del rc", err)
	}
	data, _ := os.ReadFile(filepath.Join(home, ".zshrc"))
	if strings.Contains(string(data), "tcode") {
		t.Fatalf(".zshrc no se limpió:\n%s", data)
	}
	if len(removed) != 3 { // binario + dir + .zshrc
		t.Fatalf("eliminados = %v, esperaba el progreso parcial", removed)
	}
}
