package ext

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxThemeBytes acota la lectura de un tema de extensión: una paleta por rol
// pesa unos pocos KiB; 256 KiB deja margen sin dejar que un archivo gigante
// frene el arranque.
const maxThemeBytes = 256 * 1024

// LoadedTheme es un tema aportado por una extensión, ya leído de disco: el id
// y la etiqueta del manifest, la paleta cruda (el JSON por rol que consume
// view.LoadTheme) y el id de la extensión que lo aporta (origen visible en la
// ventana de temas).
type LoadedTheme struct {
	ID    string
	Label string
	Data  []byte
	From  string
}

// LoadThemes lee los temas declarados por las extensiones: por cada
// contributes.themes resuelve File contra el Dir de la extensión (sin escape),
// lo lee acotado y verifica que sea un objeto JSON. Un tema roto no impide que
// los demás se carguen: se salta y su error se acumula nombrando extensión y
// tema. Sin temas declarados no hay errores.
func LoadThemes(exts []Extension) ([]LoadedTheme, []error) {
	var out []LoadedTheme
	var errs []error
	for _, e := range exts {
		for _, th := range e.Manifest.Contributes.Themes {
			data, err := readThemeFile(e.Dir, th.File)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s tema %q: %w", e.Manifest.ID, th.ID, err))
				continue
			}
			out = append(out, LoadedTheme{ID: th.ID, Label: th.Label, Data: data, From: e.Manifest.ID})
		}
	}
	return out, errs
}

// readThemeFile resuelve file contra dir, lee el archivo acotado y exige un
// objeto JSON (mapa rol→color, el formato de view.LoadTheme).
func readThemeFile(dir, file string) ([]byte, error) {
	if err := validateThemeFile(file); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, filepath.FromSlash(file))
	// El Join limpia, pero un file con `..` intermedio podría igual salirse:
	// se verifica contra el dir absoluto.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	absP, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	if absP != absDir && !strings.HasPrefix(absP, absDir+string(filepath.Separator)) {
		return nil, fmt.Errorf("file %q escapa de la extensión", file)
	}
	data, err := os.ReadFile(absP)
	if err != nil {
		return nil, err
	}
	if len(data) > maxThemeBytes {
		return nil, fmt.Errorf("file %q excede %d bytes", file, maxThemeBytes)
	}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("file %q: JSON inválido: %w", file, err)
	}
	return data, nil
}
