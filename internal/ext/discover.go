package ext

import (
	"fmt"
	"os"
	"path/filepath"
)

// Extension es una extensión descubierta: su manifest validado y el directorio
// donde vive (que conserva el nombre que la identifica en los mensajes).
type Extension struct {
	Manifest *Manifest
	Dir      string
}

// Discover recorre los subdirectorios de root, lee extension.json de cada uno
// y devuelve las extensiones válidas junto con los errores de las rotas. Es
// tolerante por diseño: una extensión quebrada jamás impide que las demás se
// carguen, y su error se acumula nombrando el directorio que la contiene. Los
// directorios sin extension.json se ignoran en silencio (son carpetas
// comunes), y un root inexistente no es un error: simplemente no hay nada que
// descubrir. El orden devuelto es lexicográfico (ReadDir ordena).
func Discover(root string) ([]Extension, []error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("extensions: %w", err)}
	}

	var exts []Extension
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, "extension.json"))
		if err != nil {
			continue // sin manifest, no es una extensión
		}
		m, err := Load(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		exts = append(exts, Extension{Manifest: m, Dir: dir})
	}
	return exts, errs
}
