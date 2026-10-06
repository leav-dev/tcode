package ext

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CatalogRepo es el monorepo de GitHub que actúa como catálogo de
// extensiones. Es una variable (no una constante) para que los tests puedan
// apuntar FetchCatalog a otro repositorio.
var CatalogRepo = "leav-dev/tcode-extention"

// apiBase y rawBase son las bases de la API de GitHub y de su contenido raw.
// Variables para que los tests las redirijan a un servidor httptest.
var (
	apiBase = "https://api.github.com"
	rawBase = "https://raw.githubusercontent.com"
)

// catalogHTTP es el cliente HTTP del catálogo: timeout de 15s para que una
// red caída no cuelgue la consulta indefinidamente. Es una variable para
// poder sustituirlo en tests (p. ej. con un timeout más corto).
var catalogHTTP = &http.Client{Timeout: 15 * time.Second}

// CatalogEntry es una extensión disponible en el catálogo remoto.
type CatalogEntry struct {
	ID      string
	Name    string
	Version string
	// Subdir es la carpeta dentro del monorepo donde vive la extensión;
	// vacío es el manifest en la raíz del repo.
	Subdir string
	// Installed marca si la extensión ya está instalada en userRoot.
	Installed bool
}

// gitTree es la respuesta de la API de GitHub al pedir el árbol del repo:
// una lista de paths con su tipo (blob = archivo, tree = directorio).
type gitTree struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
}

// FetchCatalog deriva el catálogo de extensiones desde el monorepo remoto.
//
// El repo remoto NO mantiene un índice: el catálogo se deriva consultando el
// árbol recursivo del repo (API de GitHub) y leyendo el extension.json de
// cada carpeta desde raw. Es tolerante, como Discover: una carpeta con
// manifest inválido o inalcanzable se acumula en errs nombrando su subdir y
// el resto del catálogo sigue adelante; ambos resultados pueden no estar
// vacíos a la vez.
//
// userRoot es la raíz de extensiones instaladas: Installed usa el mismo
// criterio que la instalación (existe la carpeta <id> bajo userRoot).
func FetchCatalog(ctx context.Context, userRoot string) ([]CatalogEntry, []error) {
	subdirs, err := fetchCatalogSubdirs(ctx)
	if err != nil {
		return nil, []error{err}
	}

	var entries []CatalogEntry
	var errs []error
	for _, subdir := range subdirs {
		m, err := fetchCatalogManifest(ctx, subdir)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", catalogSubdirName(subdir), err))
			continue
		}
		entries = append(entries, CatalogEntry{
			ID:        m.ID,
			Name:      m.Name,
			Version:   m.Version,
			Subdir:    subdir,
			Installed: isInstalled(userRoot, m.ID),
		})
	}
	return entries, errs
}

// fetchCatalogSubdirs consulta el árbol recursivo del repo y devuelve las
// carpetas que contienen un extension.json (type "blob"), en orden
// alfabético estable. El manifest de raíz (path "extension.json", sin "/")
// también vale: su subdir es "".
func fetchCatalogSubdirs(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/repos/%s/git/trees/HEAD?recursive=1", apiBase, CatalogRepo)
	var tree gitTree
	if err := catalogGet(ctx, url, &tree); err != nil {
		return nil, fmt.Errorf("consultando el árbol de %s: %w", CatalogRepo, err)
	}

	var subdirs []string
	for _, e := range tree.Tree {
		if e.Type != "blob" {
			continue
		}
		if e.Path != "extension.json" && !strings.HasSuffix(e.Path, "/extension.json") {
			continue
		}
		subdirs = append(subdirs, subdirOf(e.Path))
	}
	sort.Strings(subdirs)
	return subdirs, nil
}

// fetchCatalogManifest lee y valida el extension.json de un subdir desde raw.
// Un manifest inválido o un error de red se devuelve como error: FetchCatalog
// lo acumula nombrando el subdir y sigue con el resto.
func fetchCatalogManifest(ctx context.Context, subdir string) (*Manifest, error) {
	manifestPath := "extension.json"
	if subdir != "" {
		manifestPath = subdir + "/extension.json"
	}
	url := fmt.Sprintf("%s/%s/HEAD/%s", rawBase, CatalogRepo, manifestPath)

	data, err := catalogGetBytes(ctx, url)
	if err != nil {
		return nil, err
	}
	m, err := Load(data)
	if err != nil {
		return nil, fmt.Errorf("manifest inválido: %w", err)
	}
	return m, nil
}

// catalogGet hace GET de url con ctx y decodifica el JSON de la respuesta en
// out.
func catalogGet(ctx context.Context, url string, out any) error {
	data, err := catalogGetBytes(ctx, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("respuesta JSON inesperada: %w", err)
	}
	return nil
}

// catalogGetBytes hace GET de url con ctx y devuelve el cuerpo. El
// User-Agent es "tcode": la API de GitHub lo exige.
func catalogGetBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "tcode")

	resp, err := catalogHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// subdirOf devuelve la carpeta que contiene un path del árbol: el subdir
// dentro del monorepo. Un path de raíz (sin "/") es la raíz del repo ("").
func subdirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return ""
}

// catalogSubdirName nombra un subdir para los mensajes de error: la raíz del
// repo se llama "(raíz)" para que el error no quede con un nombre vacío.
func catalogSubdirName(subdir string) string {
	if subdir == "" {
		return "(raíz)"
	}
	return subdir
}

// isInstalled aplica el mismo criterio que la instalación: la extensión está
// instalada si existe la carpeta <id> bajo userRoot.
func isInstalled(userRoot, id string) bool {
	if userRoot == "" || id == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(userRoot, id))
	return err == nil
}
