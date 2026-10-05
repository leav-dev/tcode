package ext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// providerFixture crea un proveedor local (el "monorepo en disco"): una carpeta
// con una subcarpeta por extensión, cada una con su extension.json. Devuelve
// la raíz del proveedor.
func providerFixture(t *testing.T, exts map[string][3]string) string {
	t.Helper()
	root := t.TempDir()
	for dir, spec := range exts {
		id, name, version := spec[0], spec[1], spec[2]
		full := filepath.Join(root, dir)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
		toyExt(t, full, id, name, version)
	}
	return root
}

// TestDeriveNameDeGitAndLocal: la identidad del proveedor sale del último
// componente de la URL git y del nombre de la carpeta local. Es la misma regla
// que usa la resolución, así que se prueba con los casos que importan: URL con
// y sin .git, con barra final y forma scp.
func TestDeriveNameDeGitAndLocal(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{"https://github.com/leav-dev/tcode-extention", "tcode-extention"},
		{"https://github.com/leav-dev/tcode-extention.git", "tcode-extention"},
		{"https://github.com/leav-dev/tcode-extention/", "tcode-extention"},
		{"git@github.com:leav-dev/mis-extensions.git", "mis-extensions"},
	}
	for _, tc := range cases {
		got, err := DeriveName(tc.source)
		if err != nil {
			t.Errorf("DeriveName(%q): %v", tc.source, err)
			continue
		}
		if got != tc.want {
			t.Errorf("DeriveName(%q) = %q, esperaba %q", tc.source, got, tc.want)
		}
	}

	local := providerFixture(t, map[string][3]string{"a": {"tcode.a", "A", "0.1.0"}})
	got, err := DeriveName(local)
	if err != nil {
		t.Fatalf("DeriveName(carpeta): %v", err)
	}
	if got != filepath.Base(local) {
		t.Errorf("DeriveName(carpeta) = %q, esperaba %q", got, filepath.Base(local))
	}
}

// TestDeriveNameRejectsUnsafe: un nombre derivado con separadores o vacío no
// sirve como carpeta de instalación, así que se rechaza en el origen.
func TestDeriveNameRejectsUnsafe(t *testing.T) {
	if _, err := DeriveName("   "); err == nil {
		t.Error("DeriveName aceptó una fuente vacía")
	}
	if _, err := DeriveName("https://host/"); err == nil {
		t.Error("DeriveName aceptó una URL sin nombre")
	}
	if _, err := DeriveName("https://host/malo con espacios"); err == nil {
		t.Error("DeriveName aceptó un nombre con espacios")
	}
}

// TestLoadProvidersMissingFileIsEmpty: sin ~/.tcode/providers.json no hay
// proveedores agregados, y eso no es un error.
func TestLoadProvidersMissingFileIsEmpty(t *testing.T) {
	ps, err := LoadProviders(filepath.Join(t.TempDir(), "no-existe", "providers.json"))
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if len(ps) != 0 {
		t.Errorf("LoadProviders devolvió %d proveedores, esperaba 0", len(ps))
	}
}

// TestSaveLoadProvidersRoundTrip: lo guardado se relee igual, con el approval
// intacto y el orden de resolución preservado.
func TestSaveLoadProvidersRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".tcode", "providers.json")
	want := []Provider{
		{Name: "mis-extensions", Source: "/tmp/mis-extensions", Approved: false},
		{Name: "otro", Source: "https://example.com/otro", Approved: true},
	}
	if err := SaveProviders(path, want); err != nil {
		t.Fatalf("SaveProviders: %v", err)
	}
	got, err := LoadProviders(path)
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("LoadProviders devolvió %d proveedores, esperaba %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("proveedor %d = %+v, esperaba %+v", i, got[i], want[i])
		}
	}
}

// TestSaveProvidersRejectsEmptyPath: escribir un config sin ruta no puede
// terminar en un archivo en el cwd.
func TestSaveProvidersRejectsEmptyPath(t *testing.T) {
	if err := SaveProviders("", []Provider{}); err == nil {
		t.Error("SaveProviders aceptó una ruta vacía")
	}
}

// TestLoadProvidersRejectsCorrupt: un config roto se reporta, no se ignora en
// silencio (perdería aprobaciones en silencio).
func TestLoadProvidersRejectsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	if err := os.WriteFile(path, []byte(`{"providers": [`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadProviders(path); err == nil {
		t.Error("LoadProviders aceptó un JSON roto")
	}

	// Nombre fuera de la gramática: sería una carpeta de instalación insegura.
	if err := os.WriteFile(path, []byte(`{"providers":[{"name":"../mal","source":"/tmp/x"}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadProviders(path); err == nil {
		t.Error("LoadProviders aceptó un nombre con ..")
	}
}

// TestAllProvidersDefaultFirst: el proveedor por defecto va primero, siempre
// aprobado y sin estar en el config; uno guardado con su nombre se descarta
// (gana el por defecto ante la colisión).
func TestAllProvidersDefaultFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	def := DefaultProvider()
	stored := []Provider{
		{Name: def.Name, Source: "https://otro.example/falso", Approved: true},
		{Name: "mis-extensions", Source: "/tmp/mis-extensions", Approved: false},
	}
	if err := SaveProviders(path, stored); err != nil {
		t.Fatalf("SaveProviders: %v", err)
	}
	ps, err := AllProviders(path)
	if err != nil {
		t.Fatalf("AllProviders: %v", err)
	}
	if len(ps) != 2 {
		t.Fatalf("AllProviders devolvió %d, esperaba 2: %+v", len(ps), ps)
	}
	if ps[0].Source != DefaultProviderSource {
		t.Errorf("el primero debería ser el proveedor por defecto, es %q", ps[0].Source)
	}
	if !ps[0].Approved {
		t.Error("el proveedor por defecto debe estar aprobado")
	}
	if ps[1].Name != "mis-extensions" {
		t.Errorf("el segundo proveedor = %q, esperaba mis-extensions", ps[1].Name)
	}
}

// TestIsLocalSource: con esquema o forma scp nunca es local, aunque exista un
// directorio con ese nombre.
func TestIsLocalSource(t *testing.T) {
	dir := t.TempDir()
	if !IsLocalSource(dir) {
		t.Errorf("IsLocalSource(%q) = false, esperaba true", dir)
	}
	if IsLocalSource(DefaultProviderSource) {
		t.Error("IsLocalSource vio la URL del proveedor por defecto como local")
	}
	if IsLocalSource("git@github.com:leav-dev/tcode-extention") {
		t.Error("IsLocalSource vio una fuente scp como local")
	}
	if IsLocalSource(filepath.Join(dir, "no-existe")) {
		t.Error("IsLocalSource vio un directorio inexistente como local")
	}
}

// TestIsRemoteSource: separa la URL (o la forma scp) de una ruta que no
// existe, que es un error de tipeo y no una fuente.
func TestIsRemoteSource(t *testing.T) {
	for _, src := range []string{
		DefaultProviderSource,
		"git@github.com:leav-dev/tcode-extention",
		"file:///tmp/repo",
		"  https://example.com/x  ",
	} {
		if !IsRemoteSource(src) {
			t.Errorf("IsRemoteSource(%q) = false, esperaba true", src)
		}
	}
	dir := t.TempDir()
	for _, src := range []string{dir, filepath.Join(dir, "no-existe"), "   ", ""} {
		if IsRemoteSource(src) {
			t.Errorf("IsRemoteSource(%q) = true, esperaba false", src)
		}
	}
}

// TestListExtensionsLocal: un proveedor local se escanea directo, devolviendo
// id, nombre, versión y el subdirectorio relativo (no una ruta con el temp del
// sistema). Las carpetas sin manifest se ignoran; las rotas se acumulan.
func TestListExtensionsLocal(t *testing.T) {
	root := providerFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.2.3"},
		"roto":   {"", "", ""}, // se pisa abajo con un manifest inválido
	})
	if err := os.WriteFile(filepath.Join(root, "roto", "extension.json"), []byte(`{"id":"tcode.roto"`), 0o644); err != nil {
		t.Fatalf("WriteFile roto: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "comun"), 0o755); err != nil {
		t.Fatalf("MkdirAll comun: %v", err)
	}

	exts, err := ListExtensions(Provider{Name: "local", Source: root}, nil)
	if len(exts) != 1 {
		t.Fatalf("ListExtensions devolvió %d, esperaba 1: %+v", len(exts), exts)
	}
	got := exts[0]
	if got.ID != "tcode.linter" || got.Name != "Linter" || got.Version != "1.2.3" || got.Subdir != "linter" {
		t.Errorf("ProviderExt inesperada: %+v", got)
	}
	if err == nil {
		t.Error("ListExtensions debería reportar el manifest roto")
	}
}

// TestListExtensionsGitReadsOnlyManifests: un proveedor git se lee con un clon
// PARCIAL que pide solo los manifests: el patrón del listado es exactamente el
// de los extension.json, así que la fuente tiene .lua que el listado nunca
// baja. Es la garantía central del diseño de proveedores.
func TestListExtensionsGitReadsOnlyManifests(t *testing.T) {
	src := providerFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.2.3"},
		"tema":   {"tcode.tema", "Tema", "0.4.0"},
	})
	// La fuente "remota" trae .lua que el listado no debe ni mirar.
	for _, sub := range []string{"linter", "tema"} {
		if err := os.WriteFile(filepath.Join(src, sub, "main.lua"), []byte("return {}"), 0o644); err != nil {
			t.Fatalf("WriteFile main.lua: %v", err)
		}
	}

	var patrones []string
	record := func(_ string, dest, pattern string) error {
		patrones = append(patrones, pattern)
		return fetchFake(src)(src, dest, pattern)
	}

	exts, err := ListExtensions(Provider{Name: "remoto", Source: "https://example.com/remoto.git"}, record)
	if err != nil {
		t.Fatalf("ListExtensions: %v", err)
	}
	if len(exts) != 2 {
		t.Fatalf("ListExtensions devolvió %d, esperaba 2: %+v", len(exts), exts)
	}
	if exts[0].ID != "tcode.linter" || exts[0].Subdir != "linter" {
		t.Errorf("extensión inesperada: %+v", exts[0])
	}
	if len(patrones) != 1 || patrones[0] != "*/extension.json" {
		t.Errorf("el listado pidió %v, esperaba solo \"*/extension.json\"", patrones)
	}
}

// TestFetchFakeEmulaSparseCheckout: el fake de los tests copia lo mismo que
// copiaría el sparse checkout de git, porque los tests de "no se baja el .lua"
// solo valen si el fake es fiel.
func TestFetchFakeEmulaSparseCheckout(t *testing.T) {
	src := providerFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.2.3"},
		"tema":   {"tcode.tema", "Tema", "0.4.0"},
	})
	for _, sub := range []string{"linter", "tema"} {
		if err := os.WriteFile(filepath.Join(src, sub, "main.lua"), []byte("return {}"), 0o644); err != nil {
			t.Fatalf("WriteFile main.lua: %v", err)
		}
	}

	// Patrón de listado: los manifests y nada más.
	listing := t.TempDir()
	if err := fetchFake(src)(src, listing, "*/extension.json"); err != nil {
		t.Fatalf("fetchFake listado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(listing, "linter", "extension.json")); err != nil {
		t.Errorf("el listado no trajo el manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(listing, "linter", "main.lua")); !os.IsNotExist(err) {
		t.Error("el listado bajó un .lua: el listado no debe descargar código")
	}

	// Patrón de instalación: la extensión elegida, con su .lua.
	install := t.TempDir()
	if err := fetchFake(src)(src, install, "linter"); err != nil {
		t.Fatalf("fetchFake instalación: %v", err)
	}
	if _, err := os.Stat(filepath.Join(install, "linter", "main.lua")); err != nil {
		t.Errorf("la instalación no trajo el .lua de la extensión: %v", err)
	}
	if _, err := os.Stat(filepath.Join(install, "tema")); !os.IsNotExist(err) {
		t.Error("la instalación trajo otra extensión del proveedor")
	}
}

// TestListExtensionsCloneError: una lectura fallida se reporta con la fuente.
func TestListExtensionsCloneError(t *testing.T) {
	_, err := ListExtensions(Provider{Name: "roto", Source: "https://example.com/roto.git"},
		func(_, _, _ string) error { return os.ErrPermission })
	if err == nil {
		t.Fatal("ListExtensions aceptó una lectura fallida")
	}
	if !strings.Contains(err.Error(), "https://example.com/roto.git") {
		t.Errorf("el error no nombra la fuente: %v", err)
	}
}

// TestValidateProviderSourceRejectsEmpty: agregar una fuente sin extensiones no
// es error de la CLI posterior sino de la validación de la fuente.
func TestValidateProviderSourceRejectsEmpty(t *testing.T) {
	vacia := t.TempDir()
	if _, err := ValidateProviderSource(Provider{Name: "vacia", Source: vacia}, nil); err == nil {
		t.Error("ValidateProviderSource aceptó una carpeta vacía")
	} else if !strings.Contains(err.Error(), "no tiene subcarpetas con extension.json") {
		t.Errorf("error poco claro: %v", err)
	}

	// Una fuente remota inalcanzable tampoco: la lectura se hace de verdad.
	_, err := ValidateProviderSource(Provider{Name: "malo", Source: "https://example.com/malo.git"}, nil)
	if err == nil {
		t.Error("ValidateProviderSource aceptó un repo inalcanzable")
	}
}

// TestValidateProviderSourceAcceptsLocal: una fuente local válida se acepta y
// reporta cuántas extensiones ofrece.
func TestValidateProviderSourceAcceptsLocal(t *testing.T) {
	root := providerFixture(t, map[string][3]string{
		"a": {"tcode.a", "A", "0.1.0"},
		"b": {"tcode.b", "B", "0.1.0"},
	})
	exts, err := ValidateProviderSource(Provider{Name: "ok", Source: root}, nil)
	if err != nil {
		t.Fatalf("ValidateProviderSource: %v", err)
	}
	if len(exts) != 2 {
		t.Errorf("ValidateProviderSource reportó %d extensiones, esperaba 2", len(exts))
	}
}

// TestCanonicalSourceExpandsLocal: la fuente local se persiste absoluta para
// que el config no dependa del directorio desde el que se agregó.
func TestCanonicalSourceExpandsLocal(t *testing.T) {
	root := providerFixture(t, map[string][3]string{"a": {"tcode.a", "A", "0.1.0"}})
	got, err := CanonicalSource(root)
	if err != nil {
		t.Fatalf("CanonicalSource: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("CanonicalSource devolvió una ruta relativa: %q", got)
	}
	remote, err := CanonicalSource("  https://example.com/remoto.git  ")
	if err != nil {
		t.Fatalf("CanonicalSource remoto: %v", err)
	}
	if remote != "https://example.com/remoto.git" {
		t.Errorf("CanonicalSource remoto = %q", remote)
	}
	if _, err := CanonicalSource(""); err == nil {
		t.Error("CanonicalSource aceptó una fuente vacía")
	}
}

// TestProvidersFilePath: el config vive bajo ~/.tcode, junto al resto del
// estado del editor.
func TestProvidersFilePath(t *testing.T) {
	got := ProvidersFilePath("/home/tcode")
	want := filepath.Join("/home/tcode", ".tcode", "providers.json")
	if got != want {
		t.Errorf("ProvidersFilePath = %q, esperaba %q", got, want)
	}
}

// TestDefaultProviderMatchesSource: el proveedor por defecto es una constante
// de código y su nombre derivado de ella; no aparece en el config del usuario.
func TestDefaultProviderMatchesSource(t *testing.T) {
	def := DefaultProvider()
	if def.Source != DefaultProviderSource {
		t.Errorf("Source = %q, esperaba %q", def.Source, DefaultProviderSource)
	}
	if def.Name != "tcode-extention" {
		t.Errorf("Name = %q, esperaba tcode-extention", def.Name)
	}
	if !def.Approved {
		t.Error("el proveedor por defecto debe estar aprobado")
	}
	// Sin config en absoluto, el por defecto sigue siendo el único proveedor.
	ps, err := AllProviders(filepath.Join(t.TempDir(), "providers.json"))
	if err != nil {
		t.Fatalf("AllProviders: %v", err)
	}
	if len(ps) != 1 || ps[0].Source != DefaultProviderSource {
		t.Errorf("AllProviders sin config = %+v", ps)
	}
}

// TestSaveProvidersWritesProvidersKey: el sobre en disco usa la clave
// "providers" documentada (si cambia, el archivo deja de leerse).
func TestSaveProvidersWritesProvidersKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	if err := SaveProviders(path, []Provider{{Name: "a", Source: "/tmp/a"}}); err != nil {
		t.Fatalf("SaveProviders: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := raw["providers"]; !ok {
		t.Errorf("el config no trae la clave providers: %s", data)
	}
}
