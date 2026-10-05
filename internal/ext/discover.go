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

// Discover recorre root buscando extension.json a DOS profundidades:
// <root>/<id>/extension.json (layout plano: raíz de proyecto) y
// <root>/<proveedor>/<id>/extension.json (layout namespaced: raíz de usuario
// instalada por proveedor). Es tolerante por diseño: una extensión quebrada
// jamás impide que las demás se carguen, y su error se acumula nombrando el
// directorio que la contiene. Los directorios sin extension.json se ignoran en
// silencio, y un root inexistente no es un error: simplemente no hay nada que
// descubrir. El orden es lexicográfico por nivel (ReadDir ordena en cada uno).
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
		// Profundidad 1: <root>/<id>/extension.json
		if m, ok, err := loadManifest(dir); ok {
			exts = append(exts, Extension{Manifest: m, Dir: dir})
		} else if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
		}
		// Profundidad 2: <root>/<proveedor>/<id>/extension.json
		subs, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, s := range subs {
			if !s.IsDir() {
				continue
			}
			subdir := filepath.Join(dir, s.Name())
			m, ok, err := loadManifest(subdir)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s/%s: %w", e.Name(), s.Name(), err))
				continue
			}
			if ok {
				exts = append(exts, Extension{Manifest: m, Dir: subdir})
			}
		}
	}
	return exts, errs
}

// loadManifest lee y valida extension.json de dir. ok=false con err=nil
// significa "no hay manifest" (no es una extensión); ok=false con err!=nil
// significa "manifest roto" (es una extensión quebrada y hay que avisarlo).
func loadManifest(dir string) (m *Manifest, ok bool, err error) {
	data, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	m, err = Load(data)
	if err != nil {
		return nil, false, err
	}
	return m, true, nil
}
