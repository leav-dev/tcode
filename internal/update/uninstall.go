package update

import (
	"errors"
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
// Best-effort tras borrar el binario: un paso que falla se acumula y no
// corta a los demás, y lo ya eliminado se devuelve junto al error para que
// el llamador lo cuente (nunca un nil que esconda el progreso parcial).
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

	// Desinstalar es best-effort a partir de acá: el binario ya salió (o no
	// había nada que hacer) y cada paso siguiente suma lo suyo o acumula su
	// error sin cortar a los demás. El llamador recibe el progreso parcial
	// JUNTO al error para poder contarlo, no un nil que lo esconda.
	var removed []string
	if err := os.Remove(exe); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("el ejecutable %s ya no existe", exe)
		}
		return nil, fmt.Errorf("borrando %s: %w", exe, err)
	}
	removed = append(removed, exe)

	var errs []error
	// Si el ejecutable vivía en el dir de instalación (~/.tcode/bin, lo único
	// que el instalador escribe ahí), el directorio se va si quedó vacío.
	// Con archivos ajenos no es nuestro para vaciarlo a la fuerza: se deja
	// y se sigue (el binario, que es lo que importa, ya salió).
	// Cualquier otro dir (~/go/bin con más herramientas, /usr/local/bin, …)
	// no se toca: solo sale el binario propio.
	installDir := filepath.Join(home, ".tcode", "bin")
	if filepath.Dir(exe) == installDir {
		removeErr := os.Remove(installDir)
		switch {
		case removeErr == nil:
			removed = append(removed, installDir)
		case os.IsNotExist(removeErr):
			// Ya no estaba: nada que reportar.
		default:
			// Con archivos ajenos no es nuestro para vaciarlo a la fuerza:
			// se deja en silencio. Vacío pero inborrable (permiso, lock)
			// sí se acumula y se sigue con los rc.
			if entries, rerr := os.ReadDir(installDir); rerr == nil && len(entries) == 0 {
				errs = append(errs, fmt.Errorf("borrando %s: %w", installDir, removeErr))
			}
		}
	}

	// Bloques PATH que escribe el instalador (marcas >>> tcode >>>).
	for _, rc := range []string{".bashrc", ".zshrc"} {
		path := filepath.Join(home, rc)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // sin rc (o ilegible) no hay nada que limpiar
		}
		cleaned := stripMarkedBlock(string(data))
		if cleaned == string(data) {
			continue
		}
		if err := os.WriteFile(path, []byte(cleaned), 0o644); err != nil {
			errs = append(errs, fmt.Errorf("limpiando %s: %w", path, err))
			continue
		}
		removed = append(removed, path)
	}
	return removed, errors.Join(errs...)
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
