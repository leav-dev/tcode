package ext

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// catalogRaw describe el comportamiento del endpoint raw de un subdir en el
// servidor falso del catálogo: body (con status 200 implícito), status (error
// HTTP) y delay (para provocar timeouts en el cliente).
type catalogRaw struct {
	body   string
	status int
	delay  time.Duration
}

// catalogServer arma un httptest server que emula las dos costuras del
// catálogo: el árbol recursivo del repo (API de GitHub) y los manifests raw
// por subdir. treeJSON es el JSON completo del árbol; raw mapea subdir (o ""
// para la raíz) a su comportamiento.
func catalogServer(t *testing.T, treeJSON string, raw map[string]catalogRaw) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repos/") {
			fmt.Fprint(w, treeJSON)
			return
		}
		// raw: /{CatalogRepo}/HEAD/{subdir}/extension.json
		rest := strings.TrimPrefix(r.URL.Path, "/"+CatalogRepo+"/HEAD/")
		subdir := strings.TrimSuffix(rest, "/extension.json")
		if subdir == rest {
			subdir = "" // manifest en la raíz del repo
		}
		behavior, ok := raw[subdir]
		if !ok {
			http.Error(w, "sin manifest", http.StatusNotFound)
			return
		}
		if behavior.delay > 0 {
			time.Sleep(behavior.delay)
		}
		if behavior.status != 0 {
			http.Error(w, "error", behavior.status)
			return
		}
		fmt.Fprint(w, behavior.body)
	}))
	t.Cleanup(server.Close)
	return server
}

// overrideCatalog apunta el repo y las bases del catálogo al servidor falso y
// devuelve la función que restaura las variables de paquete originales.
func overrideCatalog(serverURL, repo string) func() {
	origRepo, origAPI, origRaw, origHTTP := CatalogRepo, apiBase, rawBase, catalogHTTP
	CatalogRepo, apiBase, rawBase = repo, serverURL, serverURL
	return func() {
		CatalogRepo, apiBase, rawBase, catalogHTTP = origRepo, origAPI, origRaw, origHTTP
	}
}

// errorNames verifica que errs contiene un error que nombra a name.
func errorNames(errs []error, name string) bool {
	for _, err := range errs {
		if strings.Contains(err.Error(), name) {
			return true
		}
	}
	return false
}

// TestFetchCatalogDerivesEntries verifica la derivación del catálogo: el árbol
// del repo se filtra a carpetas con extension.json (incluida la raíz), los
// manifests se leen desde raw en orden alfabético de subdir, las carpetas sin
// manifest se ignoran, las rotas se acumulan nombrando el subdir, e Installed
// marca la carpeta <id> preexistente en userRoot.
func TestFetchCatalogDerivesEntries(t *testing.T) {
	userRoot := t.TempDir()
	// Pre-instalada: la instalación escribe la carpeta <id> bajo userRoot.
	if err := os.MkdirAll(filepath.Join(userRoot, "tcode.vim-lite"), 0o755); err != nil {
		t.Fatalf("MkdirAll instalada: %v", err)
	}

	const treeJSON = `{"tree":[
		{"path":"vim-lite","type":"tree"},
		{"path":"vim-lite/extension.json","type":"blob"},
		{"path":"vim-lite/lua/init.lua","type":"blob"},
		{"path":"emacs-lite/extension.json","type":"blob"},
		{"path":"error-detector/extension.json","type":"blob"},
		{"path":"sinmanifest/readme.md","type":"blob"},
		{"path":"rota/extension.json","type":"blob"},
		{"path":"error500/extension.json","type":"blob"},
		{"path":"sinraw/extension.json","type":"blob"},
		{"path":"README.md","type":"blob"},
		{"path":"extension.json","type":"blob"}
	]}`

	server := catalogServer(t, treeJSON, map[string]catalogRaw{
		"":               {body: `{"id":"tcode.raiz","name":"Raiz","version":"1.0.0"}`},
		"vim-lite":       {body: `{"id":"tcode.vim-lite","name":"Vim Lite","version":"1.0.0"}`},
		"emacs-lite":     {body: `{"id":"tcode.emacs-lite","name":"Emacs Lite","version":"0.2.1"}`},
		"error-detector": {body: `{"id":"tcode.error-detector","name":"Error Detector","version":"1.1.0"}`},
		"rota":           {body: `{"id":"tcode.rota"`}, // JSON roto
		"error500":       {status: http.StatusInternalServerError},
		// sinraw: sin entrada en el mapa → 404.
	})
	defer overrideCatalog(server.URL, "ejemplo/monorepo")()

	entries, errs := FetchCatalog(context.Background(), userRoot)

	want := []CatalogEntry{
		{ID: "tcode.raiz", Name: "Raiz", Version: "1.0.0"},
		{ID: "tcode.emacs-lite", Name: "Emacs Lite", Version: "0.2.1", Subdir: "emacs-lite"},
		{ID: "tcode.error-detector", Name: "Error Detector", Version: "1.1.0", Subdir: "error-detector"},
		{ID: "tcode.vim-lite", Name: "Vim Lite", Version: "1.0.0", Subdir: "vim-lite", Installed: true},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %+v, esperaba %+v", entries, want)
	}

	// Tres subdirs rotos: manifest inválido, HTTP 500 y 404. Cada error
	// nombra su subdir; el orden sigue el de los subdirs (alfabético).
	if len(errs) != 3 {
		t.Fatalf("errs = %v, esperaba 3 errores", errs)
	}
	for _, sub := range []string{"error500", "rota", "sinraw"} {
		if !errorNames(errs, sub) {
			t.Errorf("errs no nombra el subdir roto %q: %v", sub, errs)
		}
	}
}

// TestFetchCatalogToleratesTimeout verifica que un subdir cuyo endpoint raw
// no responde a tiempo se acumula como error sin tumbar el resto del
// catálogo: el cliente del catálogo se sustituye con uno de timeout corto.
func TestFetchCatalogToleratesTimeout(t *testing.T) {
	const treeJSON = `{"tree":[
		{"path":"rapida/extension.json","type":"blob"},
		{"path":"lenta/extension.json","type":"blob"}
	]}`
	server := catalogServer(t, treeJSON, map[string]catalogRaw{
		"rapida": {body: `{"id":"tcode.rapida","name":"Rapida","version":"1.0.0"}`},
		"lenta":  {delay: 500 * time.Millisecond, body: `{"id":"tcode.lenta","name":"Lenta","version":"1.0.0"}`},
	})
	defer overrideCatalog(server.URL, "ejemplo/monorepo")()
	// Timeout corto: lenta (500ms de delay) debe caer; rapida, no.
	catalogHTTP = &http.Client{Timeout: 100 * time.Millisecond}

	entries, errs := FetchCatalog(context.Background(), t.TempDir())

	if len(entries) != 1 || entries[0].ID != "tcode.rapida" {
		t.Errorf("entries = %+v, esperaba solo tcode.rapida", entries)
	}
	if len(errs) != 1 || !errorNames(errs, "lenta") {
		t.Errorf("errs = %v, esperaba un error nombrando lenta", errs)
	}
}

// TestFetchCatalogReportsTreeFailure verifica que un árbol que no se puede
// consultar (HTTP 500) es un error del catálogo completo, no un catálogo
// vacío silencioso.
func TestFetchCatalogReportsTreeFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	defer overrideCatalog(server.URL, "ejemplo/monorepo")()

	entries, errs := FetchCatalog(context.Background(), t.TempDir())
	if len(entries) != 0 {
		t.Errorf("entries = %+v, esperaba ninguna", entries)
	}
	if len(errs) != 1 {
		t.Fatalf("errs = %v, esperaba 1 error", errs)
	}
	if !errorNames(errs, "ejemplo/monorepo") {
		t.Errorf("el error no nombra el repo: %v", errs)
	}
}
