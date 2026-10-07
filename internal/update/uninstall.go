package update

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Uninstall borra tcode de la máquina: el ejecutable en uso, el directorio
// de instalación si es el que lo contenía (~/.tcode/bin) y los bloques PATH
// marcados en ~/.bashrc y ~/.zshrc. Es el espejo en Go de
// `scripts/install.sh --uninstall`: mismos efectos, mismo alcance.
//
// Lo que NO toca a propósito: config.json, theme.json, extensiones ni
// providers (~/.tcode/ sigue intacto para una futura instalación).
//
// En Unix, borrar el binario en ejecución es seguro (el inodo sobrevive hasta
// que el proceso sale). En Windows el .exe en uso está lockeado y no se puede
// borrar a sí mismo: ahí devuelve un error que dice cómo sacarlo.
//
// home es el home del usuario y exe la ruta del ejecutable en uso
// (os.Executable en producción); ambos se inyectan para tests. Devuelve la
// lista de lo eliminado, en orden, para el resumen al usuario.
func Uninstall(home, exe string) ([]string, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("cerrá tcode y borrá %s a mano (Windows no deja borrar el .exe en uso)", exe)
	}
	if home == "" {
		return nil, fmt.Errorf("home del usuario vacío")
	}
	if exe == "" {
		return nil, fmt.Errorf("ruta del ejecutable vacía")
	}

	var removed []string
	if err := os.Remove(exe); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("el ejecutable %s ya no existe", exe)
		}
		return nil, fmt.Errorf("borrando %s: %w", exe, err)
	}
	removed = append(removed, exe)

	// Si el ejecutable vivía en el dir de instalación (~/.tcode/bin, lo único
	// que el instalador escribe ahí), el directorio queda vacío: se va entero.
	// Cualquier otro dir (~/go/bin con más herramientas, /usr/local/bin, …)
	// no se toca: solo sale el binario propio.
	installDir := filepath.Join(home, ".tcode", "bin")
	if filepath.Dir(exe) == installDir {
		if err := os.Remove(installDir); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("borrando %s: %w", installDir, err)
		}
		removed = append(removed, installDir)
	}

	// Bloques PATH que escribe el instalador (marcas >>> tcode >>>).
	for _, rc := range []string{".bashrc", ".zshrc"} {
		path := filepath.Join(home, rc)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // sin rc no hay nada que limpiar
		}
		cleaned := stripMarkedBlock(string(data))
		if cleaned == string(data) {
			continue
		}
		if err := os.WriteFile(path, []byte(cleaned), 0o644); err != nil {
			return nil, fmt.Errorf("limpiando %s: %w", path, err)
		}
		removed = append(removed, path)
	}
	return removed, nil
}

// stripMarkedBlock saca de un rc el bloque entre las marcas del instalador,
// inclusive. Sin marcas devuelve el texto intacto.
func stripMarkedBlock(data string) string {
	const start, end = "# >>> tcode >>>", "# <<< tcode <<<"
	var out []string
	inside := false
	for _, line := range strings.Split(data, "\n") {
		switch strings.TrimSpace(line) {
		case start:
			inside = true
			continue
		case end:
			inside = false
			continue
		}
		if !inside {
			out = append(out, line)
		}
	}
	cleaned := strings.Join(out, "\n")
	// Sin marcas, el join/split es identidad: no se reescribe nada.
	if cleaned == data {
		return data
	}
	return strings.TrimRight(cleaned, "\n") + "\n"
}
