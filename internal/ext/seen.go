package ext

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Seen es el conjunto de novedades ya listadas: claves "proveedor/id@versión"
// que el arranque ya le mostró al usuario (en el aviso o en la ventana). Existe
// para no crear falsos positivos: una disponible no instalada que ya se listó
// no vuelve a anunciarse, pero un bump de versión SÍ (es realmente nueva y su
// clave cambia).
//
// El mapa va en memoria; la persistencia vive en ~/.tcode/extensions-seen.json
// (estado, como providers.json — no es ajuste del config.json).
type Seen map[string]bool

// SeenKey arma la clave de visto de una novedad: "proveedor/id@versión". La
// versión es parte de la clave a propósito: si el autor publica una versión
// nueva, la clave cambia y la novedad vuelve a avisar.
func SeenKey(provider, id, version string) string {
	return provider + "/" + id + "@" + version
}

// SeenKeyOf arma la clave de visto de una novedad concreta.
func SeenKeyOf(a AvailableExt) string {
	return SeenKey(a.Provider.Name, a.ID, a.Version)
}

// FilterUnseen devuelve solo las novedades nunca vistas: las que ya están en
// seen se asumen listadas y no vuelven a contar para el aviso. La ventana de
// Disponibles sigue mostrando TODO (sale de Available, sin filtro): el filtro
// es solo para no repetir el aviso de arranque.
//
// Un seen nil o vacío no filtra nada: todo es nuevo.
func FilterUnseen(available []AvailableExt, seen Seen) []AvailableExt {
	if len(seen) == 0 {
		return available
	}
	out := make([]AvailableExt, 0, len(available))
	for _, a := range available {
		if seen[SeenKeyOf(a)] {
			continue
		}
		out = append(out, a)
	}
	return out
}

// MarkSeen registra las novedades dadas como ya listadas. Crea el mapa si es
// nil para que el llamador no tenga que inicializarlo.
func MarkSeen(seen Seen, available []AvailableExt) Seen {
	if seen == nil {
		seen = make(Seen, len(available))
	}
	for _, a := range available {
		seen[SeenKeyOf(a)] = true
	}
	return seen
}

// PruneSeen saca del conjunto las claves de extensiones que ya están
// instaladas: una instalada nunca vuelve a ser novedad, así que guardar su
// clave es peso muerto. Las claves de catálogos ilegibles se CONSERVAN: si un
// proveedor cayó en esta lectura, sus vistos no se pierden y no re-avisan
// cuando vuelve (eso sería el falso positivo que esto evita).
func PruneSeen(seen Seen, installed []Info) Seen {
	if len(seen) == 0 {
		return seen
	}
	refs := make(map[string]bool, len(installed))
	for _, info := range installed {
		refs[info.Ref()] = true
	}
	for key := range seen {
		if refs[seenBase(key)] {
			delete(seen, key)
		}
	}
	return seen
}

// seenBase recorta la "@versión" de una clave de visto para comparar por
// referencia ("proveedor/id").
func seenBase(key string) string {
	if i := strings.LastIndex(key, "@"); i >= 0 {
		return key[:i]
	}
	return key
}

// UnseenAvailable deriva las novedades no vistas del snapshot: el mismo
// Available de siempre menos lo ya listado. Es el conteo que usa el aviso de
// arranque; la ventana sigue usando Available.
func (s Snapshot) UnseenAvailable(seen Seen) []AvailableExt {
	return FilterUnseen(s.Available(), seen)
}

// SeenFilePath devuelve la ruta del estado de vistos bajo home.
func SeenFilePath(home string) string {
	return filepath.Join(home, ".tcode", "extensions-seen.json")
}

// seenFile es el sobre de ~/.tcode/extensions-seen.json: la lista ordenada de
// claves ya listadas. Se guarda ordenada para que el archivo no cambie de forma
// por el orden aleatorio del mapa.
type seenFile struct {
	Seen []string `json:"seen"`
}

// LoadSeenFile lee el estado de vistos. Un archivo inexistente no es un error:
// todavía no se listó nada. Un archivo corrupto se reporta (nombrando el
// problema) y devuelve el conjunto vacío: perder los vistos re-avisa una vez,
// no rompe el arranque — quien llama decide si avisar o silenciar.
func LoadSeenFile(path string) (Seen, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(Seen), nil
		}
		return make(Seen), fmt.Errorf("leyendo %s: %w", path, err)
	}
	var file seenFile
	if err := json.Unmarshal(data, &file); err != nil {
		return make(Seen), fmt.Errorf("estado de novedades inválido (%s): %w", path, err)
	}
	seen := make(Seen, len(file.Seen))
	for _, key := range file.Seen {
		if key != "" {
			seen[key] = true
		}
	}
	return seen, nil
}

// SaveSeenFile escribe el estado de vistos. Un seen vacío escribe la lista
// vacía (no borra el archivo): así la próxima lectura sigue siendo estable.
func SaveSeenFile(path string, seen Seen) error {
	if path == "" {
		return fmt.Errorf("ruta de estado de novedades vacía")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creando %s: %w", filepath.Dir(path), err)
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	data, err := json.MarshalIndent(seenFile{Seen: keys}, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando novedades vistas: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("escribiendo %s: %w", path, err)
	}
	return nil
}
