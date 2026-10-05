package ext

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// toyExt crea en dir una extensión de juguete: extension.json con id, nombre y
// versión dados. name "" produce un manifest sin nombre útil, para verificar
// que List reporta nombre vacío cuando el manifest no lo declara.
func toyExt(t *testing.T, dir, id, name, version string) string {
	t.Helper()
	data, err := json.Marshal(Manifest{ID: id, Name: name, Version: version})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile extension.json: %v", err)
	}
	return dir
}

// installFixture crea un "repositorio" simulado en un directorio temporal:
// extension.json válido, un archivo auxiliar y una subcarpeta .git falsa (que
// la instalación debe excluir). Devuelve la raíz del repositorio.
func installFixture(t *testing.T, id, name, version string) string {
	t.Helper()
	dir := t.TempDir()
	toyExt(t, dir, id, name, version)
	if err := os.WriteFile(filepath.Join(dir, "info.txt"), []byte("extra"), 0o644); err != nil {
		t.Fatalf("WriteFile info.txt: %v", err)
	}
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0o644); err != nil {
		t.Fatalf("WriteFile .git/HEAD: %v", err)
	}
	return dir
}

// cloneFake es una CloneFunc que copia src a dest con un walk simple, sin
// depender de git: emula lo mínimo de un clone para los tests.
func cloneFake(src string) CloneFunc {
	return func(_ string, dest string) error {
		return walkCopy(src, dest, func(string) bool { return true })
	}
}

// fetchFake es una FetchFunc que emula el sparse checkout de git sobre un
// "repositorio" en disco: deja en dest SOLO lo que casa con pattern. Cubre las
// dos formas que usa el código real —"*/extension.json", que trae los manifests
// del proveedor, y el nombre de una subcarpeta, que trae esa extensión
// entera— para que los tests puedan afirmar qué se baja y qué no.
func fetchFake(src string) FetchFunc {
	return func(_ string, dest, pattern string) error {
		return walkCopy(src, dest, func(rel string) bool { return sparseMatch(pattern, rel) })
	}
}

// sparseMatch decide si un archivo del clon entra en el sparse checkout. Un
// patrón con "/" es de archivo (path.Match); los demás son de carpeta y
// matchean todo lo que hay debajo, igual que un gitignore.
func sparseMatch(pattern, rel string) bool {
	if strings.Contains(pattern, "/") {
		matched, err := path.Match(pattern, rel)
		return err == nil && matched
	}
	return rel == pattern || strings.HasPrefix(rel, pattern+"/")
}

// walkCopy copia de src a dest los archivos (con rutas relativas) que accept
// devuelve verdadero, creando las carpetas hechas falta. Es la base de los
// fakes de clonación: sin git, sin red y sin ruido de .git.
func walkCopy(src, dest string, accept func(rel string) bool) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dest, 0o755)
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			// No se crean carpetas vacías: el sparse checkout de git tampoco
			// materializa un directorio sin archivos.
			if !sparseHasFilesBelow(src, rel, accept) {
				return nil
			}
			return os.MkdirAll(filepath.Join(dest, filepath.FromSlash(rel)), 0o755)
		}
		if !d.Type().IsRegular() || !accept(rel) {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		to := filepath.Join(dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		return os.WriteFile(to, data, 0o644)
	})
}

// sparseHasFilesBelow dice si hay algún archivo aceptado bajo rel, para no
// materializar carpetas vacías en el clon simulado.
func sparseHasFilesBelow(src, rel string, accept func(string) bool) bool {
	entries, err := os.ReadDir(filepath.Join(src, filepath.FromSlash(rel)))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			if sparseHasFilesBelow(src, rel+"/"+e.Name(), accept) {
				return true
			}
			continue
		}
		if accept(rel + "/" + e.Name()) {
			return true
		}
	}
	return false
}

// gitMonorepoFixture crea un "monorepo" con un repo git real en un temporal:
// una subcarpeta por extensión, cada una con su extension.json y un main.lua
// (el archivo de código que el listado liviano NO debe bajar). Devuelve la ruta
// del repo, servible con file://.
func gitMonorepoFixture(t *testing.T, exts map[string][3]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible")
	}
	repo := t.TempDir()
	for dir, spec := range exts {
		full := filepath.Join(repo, dir)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
		toyExt(t, full, spec[0], spec[1], spec[2])
		if err := os.WriteFile(filepath.Join(full, "main.lua"), []byte("return {}\n"), 0o644); err != nil {
			t.Fatalf("WriteFile main.lua: %v", err)
		}
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("-c", "user.name=tcode-test", "-c", "user.email=tcode-test@example.com", "add", "-A")
	run("-c", "user.name=tcode-test", "-c", "user.email=tcode-test@example.com", "commit", "-m", "init")
	return repo
}

// localProvider es un proveedor de carpeta local (el "monorepo en disco") con
// la subcarpeta por extensión que pide exts.
func localProvider(t *testing.T, name string, exts map[string][3]string) Provider {
	t.Helper()
	return Provider{Name: name, Source: providerFixture(t, exts), Approved: true}
}

// TestInstallByIDResolvesAcrossProviders instala por id y verifica el destino
// namespaced <root>/<proveedor>/<id> con los archivos de la extensión.
func TestInstallByIDResolvesAcrossProviders(t *testing.T) {
	proveedores := []Provider{
		localProvider(t, "primero", map[string][3]string{"nada": {"tcode.nada", "Nada", "0.1.0"}}),
		localProvider(t, "segundo", map[string][3]string{"linter": {"tcode.linter", "Linter", "1.2.3"}}),
	}
	userRoot := filepath.Join(t.TempDir(), ".tcode", "extensions")

	res, err := InstallByID("tcode.linter", proveedores, userRoot, nil, nil)
	if err != nil {
		t.Fatalf("InstallByID: %v", err)
	}
	if res.Ref() != "segundo/tcode.linter" {
		t.Errorf("Ref = %q, esperaba segundo/tcode.linter", res.Ref())
	}
	dest := filepath.Join(userRoot, "segundo", "tcode.linter")
	if _, err := os.Stat(filepath.Join(dest, "extension.json")); err != nil {
		t.Errorf("extension.json no instalado: %v", err)
	}
	// Nada se instala en la raíz plana ni en el otro proveedor.
	if _, err := os.Stat(filepath.Join(userRoot, "tcode.linter")); !os.IsNotExist(err) {
		t.Error("la extensión se instaló sin namespacer por proveedor")
	}
	if _, err := os.Stat(filepath.Join(userRoot, "primero", "tcode.nada")); !os.IsNotExist(err) {
		t.Error("instaló una extensión que el usuario no pidió")
	}
}

// TestInstallByIDFirstProviderWins: ante ids repetidos entre proveedores gana el
// primero de la lista, así que el proveedor por defecto resuelve la colisión.
func TestInstallByIDFirstProviderWins(t *testing.T) {
	proveedores := []Provider{
		localProvider(t, "gana", map[string][3]string{"dup": {"tcode.dup", "Del primero", "1.0.0"}}),
		localProvider(t, "pierde", map[string][3]string{"dup": {"tcode.dup", "Del segundo", "9.9.9"}}),
	}
	userRoot := t.TempDir()
	res, err := InstallByID("tcode.dup", proveedores, userRoot, nil, nil)
	if err != nil {
		t.Fatalf("InstallByID: %v", err)
	}
	if res.Provider != "gana" {
		t.Errorf("Provider = %q, esperaba gana (el primero en orden)", res.Provider)
	}
	data, err := os.ReadFile(filepath.Join(userRoot, "gana", "tcode.dup", "extension.json"))
	if err != nil {
		t.Fatalf("leyendo el manifest instalado: %v", err)
	}
	if !strings.Contains(string(data), "1.0.0") {
		t.Errorf("se instaló la versión del segundo proveedor: %s", data)
	}
}

// TestInstallByIDUnapprovedProviderNeedsTrust: instalar desde un proveedor sin
// aprobar exige confirmación explícita; sin confirm (no interactivo) el error
// dice cómo aprobarlo, y un "no" cancela sin instalar.
func TestInstallByIDUnapprovedProviderNeedsTrust(t *testing.T) {
	nueva := func() ([]Provider, string) {
		p := localProvider(t, "sospechoso", map[string][3]string{"x": {"tcode.x", "X", "1.0.0"}})
		p.Approved = false
		return []Provider{p}, t.TempDir()
	}

	proveedores, userRoot := nueva()
	_, err := InstallByID("tcode.x", proveedores, userRoot, nil, nil)
	if err == nil {
		t.Fatal("InstallByID instaló sin aprobar el proveedor")
	}
	if !strings.Contains(err.Error(), "--approve-provider") {
		t.Errorf("el error no dice cómo aprobar: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(userRoot, "sospechoso", "tcode.x")); !os.IsNotExist(statErr) {
		t.Error("un proveedor rechazado no debe dejar nada instalado")
	}

	// Un "no" explícito cancela.
	proveedores, userRoot = nueva()
	_, err = InstallByID("tcode.x", proveedores, userRoot, nil, func(Provider) bool { return false })
	if err == nil || !strings.Contains(err.Error(), "cancelada") {
		t.Errorf("una cancelación debería cortar la instalación, err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(userRoot, "sospechoso", "tcode.x")); !os.IsNotExist(statErr) {
		t.Error("una cancelación no debe dejar nada instalado")
	}

	// Un "sí" instala, y el prompt se hace con el proveedor sin aprobar.
	proveedores, userRoot = nueva()
	var preguntado Provider
	res, err := InstallByID("tcode.x", proveedores, userRoot, nil, func(p Provider) bool {
		preguntado = p
		return true
	})
	if err != nil {
		t.Fatalf("InstallByID con confirmación: %v", err)
	}
	if preguntado.Name != "sospechoso" {
		t.Errorf("se preguntó por %q, esperaba sospechoso", preguntado.Name)
	}
	if res.Ref() != "sospechoso/tcode.x" {
		t.Errorf("Ref = %q, esperaba sospechoso/tcode.x", res.Ref())
	}
}

// TestInstallByIDFetchesOnlyTheChosenExtension: la instalación pide la
// subcarpeta de la extensión (y solo ella), mientras la resolución pide solo
// manifests. Es el reparto de descargas del diseño liviano.
func TestInstallByIDFetchesOnlyTheChosenExtension(t *testing.T) {
	src := providerFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.2.3"},
		"tema":   {"tcode.tema", "Tema", "0.4.0"},
	})
	for _, sub := range []string{"linter", "tema"} {
		if err := os.WriteFile(filepath.Join(src, sub, "main.lua"), []byte("return {}\n"), 0o644); err != nil {
			t.Fatalf("WriteFile main.lua: %v", err)
		}
	}
	// Carpeta local: el patrón no aplica, pero el destino sigue siendo el
	// namespaced y con los archivos de la extensión elegida.
	proveedores := []Provider{{Name: "local", Source: src, Approved: true}}
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.linter", proveedores, userRoot, nil, nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}
	dest := filepath.Join(userRoot, "local", "tcode.linter")
	if _, err := os.Stat(filepath.Join(dest, "main.lua")); err != nil {
		t.Errorf("la extensión instalada no trae su código: %v", err)
	}
	if _, err := os.Stat(filepath.Join(userRoot, "local", "tcode.tema")); !os.IsNotExist(err) {
		t.Error("se instaló otra extensión del mismo proveedor")
	}

	// Proveedor git: los patrones pedidos son manifests primero y subcarpeta
	// después, nunca el repo entero.
	var patrones []string
	fetch := func(_ string, dest, pattern string) error {
		patrones = append(patrones, pattern)
		return fetchFake(src)(src, dest, pattern)
	}
	userRoot = t.TempDir()
	_, err := InstallByID("tcode.linter", []Provider{{Name: "remoto", Source: "https://example.com/r.git", Approved: true}}, userRoot, fetch, nil)
	if err != nil {
		t.Fatalf("InstallByID git: %v", err)
	}
	want := []string{"*/extension.json", "linter"}
	if len(patrones) != len(want) || patrones[0] != want[0] || patrones[1] != want[1] {
		t.Errorf("patrones pedidos = %v, esperaba %v", patrones, want)
	}
	if _, err := os.Stat(filepath.Join(userRoot, "remoto", "tcode.linter", "main.lua")); err != nil {
		t.Errorf("el clon acotado no trajo el código de la extensión: %v", err)
	}
}

// TestInstallByIDRejectsUnknownID: un id que nadie ofrece da un error que
// lista lo disponible, y nada se instala.
func TestInstallByIDRejectsUnknownID(t *testing.T) {
	proveedores := []Provider{localProvider(t, "local", map[string][3]string{"a": {"tcode.a", "A", "0.1.0"}})}
	userRoot := t.TempDir()
	_, err := InstallByID("tcode.inexistente", proveedores, userRoot, nil, nil)
	if err == nil {
		t.Fatal("InstallByID aceptó un id inexistente")
	}
	if !strings.Contains(err.Error(), "local/tcode.a") {
		t.Errorf("el error no lista lo disponible: %v", err)
	}
	entries, readErr := os.ReadDir(userRoot)
	if readErr == nil && len(entries) > 0 {
		t.Errorf("un id inexistente no debe instalar nada, hay %d entradas", len(entries))
	}
}

// TestInstallByIDRejectsUnsafeID: un id con separadores nunca puede escapar de
// la raíz de usuario.
func TestInstallByIDRejectsUnsafeID(t *testing.T) {
	if _, err := InstallByID("../mal", nil, t.TempDir(), nil, nil); err == nil {
		t.Error("InstallByID aceptó un id con ..")
	}
}

// TestInstallByIDInvalidProviderNameIsSkipped: un proveedor guardado con un
// nombre fuera de la gramática no puede ser carpeta de instalación, así que se
// ignora en vez de escribir fuera del root.
func TestInstallByIDInvalidProviderNameIsSkipped(t *testing.T) {
	bueno := localProvider(t, "ok", map[string][3]string{"a": {"tcode.a", "A", "0.1.0"}})
	proveedores := []Provider{{Name: "../mal", Source: bueno.Source, Approved: true}, bueno}
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.a", proveedores, userRoot, nil, nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}
	if _, err := os.Stat(filepath.Join(userRoot, "ok", "tcode.a")); err != nil {
		t.Errorf("no se instaló desde el proveedor válido: %v", err)
	}
}

// TestInstallByIDPropagatesFetchFailure: si la extensión se resuelve pero su
// descarga falla, el error se propaga: no se cae al siguiente proveedor (que
// sería OTRA extensión con el mismo id) ni se reporta como "no encontrada".
func TestInstallByIDPropagatesFetchFailure(t *testing.T) {
	p := localProvider(t, "ok", map[string][3]string{"a": {"tcode.a", "A", "1.0.0"}})
	_, err := InstallByID("tcode.a", []Provider{{Name: "remoto", Source: "https://example.com/r.git", Approved: true}, p}, t.TempDir(),
		func(_, dest, pattern string) error {
			if pattern == manifestsPattern {
				return fetchFake(p.Source)(p.Source, dest, pattern)
			}
			return os.ErrPermission
		}, nil)
	if err == nil {
		t.Fatal("InstallByID ignoró el fallo de descarga de la extensión")
	}
	if !strings.Contains(err.Error(), "descarga") && !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("el error no explica el fallo de descarga: %v", err)
	}
}

// TestResolveExtensionReportsAndPropagates: resolver devuelve el proveedor que
// ofrece el id; si no está, el error junta lo disponible con los problemas de
// los proveedores que no se pudieron leer.
func TestResolveExtensionReportsAndPropagates(t *testing.T) {
	ok := localProvider(t, "ok", map[string][3]string{"a": {"tcode.a", "A", "0.1.0"}})
	res, err := ResolveExtension("tcode.a", []Provider{ok}, nil)
	if err != nil {
		t.Fatalf("ResolveExtension: %v", err)
	}
	if res.Provider.Name != "ok" || res.Ext.ID != "tcode.a" {
		t.Errorf("resolución inesperada: %+v", res)
	}

	caido := Provider{Name: "caido", Source: "https://example.com/caido.git"}
	_, err = ResolveExtension("tcode.b", []Provider{ok, caido},
		func(_, dest, pattern string) error {
			if dest == "" {
				return os.ErrPermission
			}
			return fetchFake(ok.Source)(ok.Source, dest, pattern)
		})
	if err == nil {
		t.Fatal("ResolveExtension aceptó un id inexistente")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ok/tcode.a") || !strings.Contains(msg, "caido") {
		t.Errorf("el error no junta disponibilidad y fallos: %v", err)
	}
}

// TestInstallByIDWithRealGit es la integración del camino completo con git
// real sobre un monorepo: resolución liviana (solo manifests) y clon acotado de
// la extensión elegida.
func TestInstallByIDWithRealGit(t *testing.T) {
	repo := gitMonorepoFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.2.3"},
		"tema":   {"tcode.tema", "Tema", "0.4.0"},
	})
	proveedores := []Provider{{
		Name:     "monorepo",
		Source:   "file://" + filepath.ToSlash(repo),
		Approved: true,
	}}

	exts, err := ListExtensions(proveedores[0], nil)
	if err != nil {
		t.Fatalf("ListExtensions con git real: %v", err)
	}
	if len(exts) != 2 {
		t.Fatalf("ListExtensions devolvió %d, esperaba 2: %+v", len(exts), exts)
	}

	userRoot := t.TempDir()
	res, err := InstallByID("tcode.linter", proveedores, userRoot, nil, nil)
	if err != nil {
		t.Fatalf("InstallByID con git real: %v", err)
	}
	if res.Ref() != "monorepo/tcode.linter" {
		t.Errorf("Ref = %q, esperaba monorepo/tcode.linter", res.Ref())
	}
	dest := filepath.Join(userRoot, "monorepo", "tcode.linter")
	if _, err := os.Stat(filepath.Join(dest, "extension.json")); err != nil {
		t.Errorf("extension.json no instalado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "main.lua")); err != nil {
		t.Errorf("el .lua de la extensión no bajó: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git no debió copiarse: %v", err)
	}
	// La otra extensión del proveedor no se bajó: la instalación es acotada.
	if _, err := os.Stat(filepath.Join(userRoot, "monorepo", "tcode.tema")); !os.IsNotExist(err) {
		t.Error("se instaló una extensión que el usuario no pidió")
	}
}

// TestRemoveNamespacedValidates: ni el nombre del proveedor ni el id pueden
// escapar de la raíz de usuario.
func TestRemoveNamespacedValidates(t *testing.T) {
	userRoot := t.TempDir()
	if err := RemoveNamespaced(userRoot, "../mal", "tcode.a"); err == nil {
		t.Error("RemoveNamespaced aceptó un proveedor con ..")
	}
	if err := RemoveNamespaced(userRoot, "ok", "../mal"); err == nil {
		t.Error("RemoveNamespaced aceptó un id con ..")
	}
	if err := RemoveNamespaced("", "ok", "tcode.a"); err == nil {
		t.Error("RemoveNamespaced aceptó una raíz vacía")
	}
	err := RemoveNamespaced(userRoot, "ok", "tcode.a")
	if err == nil || !strings.Contains(err.Error(), "no está instalada") {
		t.Errorf("una extensión ausente debería dar un error claro: %v", err)
	}
}

// TestRemoveRefAcceptsProviderAndSearch: la forma "proveedor:id" borra
// exactamente esa, y un id suelto se busca en todos los proveedores (gana el
// primero por orden de carpetas). Una instalación plana heredada sigue
// borrándose por id.
func TestRemoveRefAcceptsProviderAndSearch(t *testing.T) {
	proveedores := []Provider{
		localProvider(t, "uno", map[string][3]string{"dup": {"tcode.dup", "Uno", "1.0.0"}}),
		localProvider(t, "dos", map[string][3]string{"dup": {"tcode.dup", "Dos", "1.0.0"}, "otro": {"tcode.otro", "Otro", "1.0.0"}}),
	}
	userRoot := t.TempDir()
	for _, p := range proveedores {
		exts, err := ListExtensions(p, nil)
		if err != nil {
			t.Fatalf("ListExtensions: %v", err)
		}
		for _, e := range exts {
			if _, err := InstallByID(e.ID, []Provider{p}, userRoot, nil, nil); err != nil {
				t.Fatalf("preparando %s/%s: %v", p.Name, e.ID, err)
			}
		}
	}
	// Heredada plana, sin proveedor.
	plana := filepath.Join(userRoot, "tcode.plana")
	if err := os.MkdirAll(plana, 0o755); err != nil {
		t.Fatalf("MkdirAll plana: %v", err)
	}
	toyExt(t, plana, "tcode.plana", "Plana", "0.1.0")

	// Id suelto: gana el primer proveedor por orden de carpeta (ReadDir
	// ordena: "dos" antes que "uno").
	ref, err := RemoveRef(userRoot, "tcode.dup")
	if err != nil {
		t.Fatalf("RemoveRef por id: %v", err)
	}
	if ref != "dos:tcode.dup" {
		t.Errorf("RemoveRef = %q, esperaba dos:tcode.dup", ref)
	}
	if _, err := os.Stat(filepath.Join(userRoot, "uno", "tcode.dup")); err != nil {
		t.Errorf("el otro proveedor no debía tocarse: %v", err)
	}

	// Forma explícita "proveedor:id".
	ref, err = RemoveRef(userRoot, "dos:tcode.otro")
	if err != nil {
		t.Fatalf("RemoveRef por referencia: %v", err)
	}
	if ref != "dos:tcode.otro" {
		t.Errorf("RemoveRef = %q, esperaba dos:tcode.otro", ref)
	}

	// Heredada plana.
	ref, err = RemoveRef(userRoot, "tcode.plana")
	if err != nil {
		t.Fatalf("RemoveRef de la plana: %v", err)
	}
	if ref != "tcode.plana" {
		t.Errorf("RemoveRef = %q, esperaba tcode.plana", ref)
	}

	if _, err := RemoveRef(userRoot, "tcode.inexistente"); err == nil {
		t.Error("RemoveRef aceptó una extensión no instalada")
	}
	if _, err := RemoveRef(userRoot, "../mal"); err == nil {
		t.Error("RemoveRef aceptó un id con ..")
	}
	if _, err := RemoveRef("", "tcode.a"); err == nil {
		t.Error("RemoveRef aceptó una raíz vacía")
	}
}

// providerFirstID devuelve el único id que ofrece el proveedor (las fixtures de
// remove usan un id compartido entre proveedores, no uno por nombre de carpeta).
func providerFirstID(t *testing.T, p Provider) string {
	t.Helper()
	exts, err := ListExtensions(p, nil)
	if err != nil {
		t.Fatalf("ListExtensions: %v", err)
	}
	if len(exts) == 0 {
		t.Fatal("el proveedor no ofrece extensiones")
	}
	return exts[0].ID
}

// TestListIgnoresEmptyProviderFolder: al borrar la última extensión de un
// proveedor queda su carpeta vacía, y eso no es una extensión rota que avisar.
func TestListIgnoresEmptyProviderFolder(t *testing.T) {
	userRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(userRoot, "vacio"), 0o755); err != nil {
		t.Fatalf("MkdirAll vacio: %v", err)
	}
	infos, errs := List(userRoot)
	if len(errs) != 0 {
		t.Errorf("una carpeta vacía no debería registrarse como error: %v", errs)
	}
	if len(infos) != 0 {
		t.Errorf("List devolvió %d extensiones, esperaba 0: %+v", len(infos), infos)
	}
}

// TestListNamespacedAndFlatLayouts: List reporta el layout namespaced con su
// proveedor y sigue reportando las instalaciones planas heredadas con el
// proveedor vacío.
func TestListNamespacedAndFlatLayouts(t *testing.T) {
	proveedores := []Provider{
		localProvider(t, "uno", map[string][3]string{"a": {"tcode.a", "A", "0.1.0"}}),
		localProvider(t, "dos", map[string][3]string{"b": {"tcode.b", "B", "0.2.0"}}),
	}
	userRoot := t.TempDir()
	for _, p := range proveedores {
		if _, err := InstallByID(providerFirstID(t, p), []Provider{p}, userRoot, nil, nil); err != nil {
			t.Fatalf("instalando desde %s: %v", p.Name, err)
		}
	}
	plana := filepath.Join(userRoot, "tcode.plana")
	if err := os.MkdirAll(plana, 0o755); err != nil {
		t.Fatalf("MkdirAll plana: %v", err)
	}
	toyExt(t, plana, "tcode.plana", "Plana", "0.1.0")

	infos, errs := List(userRoot)
	if len(errs) != 0 {
		t.Errorf("List reportó errores inesperados: %v", errs)
	}
	if len(infos) != 3 {
		t.Fatalf("List devolvió %d, esperaba 3: %+v", len(infos), infos)
	}
	vistos := map[string]Info{}
	for _, i := range infos {
		vistos[i.Ref()] = i
	}
	if got := vistos["uno/tcode.a"]; got.Provider != "uno" || got.Version != "0.1.0" {
		t.Errorf("extensión namespaced inesperada: %+v", got)
	}
	if got := vistos["dos/tcode.b"]; got.Provider != "dos" || got.Name != "B" {
		t.Errorf("extensión namespaced inesperada: %+v", got)
	}
	if got := vistos["tcode.plana"]; got.Provider != "" {
		t.Errorf("la instalación heredada no debería tener proveedor: %+v", got)
	}
}

// TestInstallFromGitClonesAndValidates instala desde un "repositorio"
// simulado y verifica: el id devuelto, el árbol copiado (manifest + auxiliar),
// la exclusión de .git y la creación de la raíz de usuario.
func TestInstallFromGitClonesAndValidates(t *testing.T) {
	src := installFixture(t, "tcode.demoplug", "Demo Plug", "0.1.0")
	// userRoot anidado: la instalación debe crear la raíz si no existe.
	userRoot := filepath.Join(t.TempDir(), "usuario", ".tcode", "extensions")

	id, err := InstallFromGit("file:///repo-demo", userRoot, cloneFake(src))
	if err != nil {
		t.Fatalf("InstallFromGit: %v", err)
	}
	if id != "tcode.demoplug" {
		t.Errorf("id = %q, esperaba tcode.demoplug", id)
	}

	dest := filepath.Join(userRoot, id)
	data, err := os.ReadFile(filepath.Join(dest, "extension.json"))
	if err != nil {
		t.Fatalf("extension.json instalado: %v", err)
	}
	if !strings.Contains(string(data), `"version":"0.1.0"`) {
		t.Errorf("extension.json sin la versión instalada: %s", data)
	}
	if _, err := os.Stat(filepath.Join(dest, "info.txt")); err != nil {
		t.Errorf("archivo auxiliar no copiado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git debería estar excluido, err = %v", err)
	}
	if _, err := os.Stat(userRoot); err != nil {
		t.Errorf("la raíz de usuario no existe tras instalar: %v", err)
	}
}

// TestInstallFromGitRejectsBrokenManifest verifica que un manifest que no
// valida (sin version o con JSON roto) aborta sin tocar el destino.
func TestInstallFromGitRejectsBrokenManifest(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"sin version", `{"id": "tcode.roto", "name": "Roto"}`},
		{"json roto", `{"id": "tcode.roto", "version": `},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := t.TempDir()
			if err := os.WriteFile(filepath.Join(src, "extension.json"), []byte(tc.src), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			userRoot := t.TempDir()
			id, err := InstallFromGit("file:///repo-roto", userRoot, cloneFake(src))
			if err == nil {
				t.Fatalf("InstallFromGit aceptó un manifest inválido (id %q)", id)
			}
			if !strings.Contains(err.Error(), "manifiesto inválido") {
				t.Errorf("error sin contexto de manifest: %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(userRoot, "tcode.roto")); !os.IsNotExist(statErr) {
				t.Errorf("el destino no debió crearse: %v", statErr)
			}
		})
	}
}

// TestInstallFromGitRejectRepositoryWithoutManifest verifica el error claro
// cuando el repositorio clonado no trae extension.json.
func TestInstallFromGitRejectRepositoryWithoutManifest(t *testing.T) {
	src := t.TempDir() // vacío: sin extension.json
	_, err := InstallFromGit("file:///repo-vacio", t.TempDir(), cloneFake(src))
	if err == nil {
		t.Fatal("InstallFromGit aceptó un repositorio sin extension.json")
	}
	if !strings.Contains(err.Error(), "el repositorio no tiene extension.json en su raíz") {
		t.Errorf("error inesperado: %v", err)
	}
}

// TestInstallFromGitReplacesExisting instala dos veces la misma extensión con
// versiones distintas: la segunda debe reemplazar el árbol anterior.
func TestInstallFromGitReplacesExisting(t *testing.T) {
	userRoot := t.TempDir()
	src1 := installFixture(t, "tcode.demoplug", "Demo Plug", "0.1.0")
	if _, err := InstallFromGit("file:///v1", userRoot, cloneFake(src1)); err != nil {
		t.Fatalf("primera instalación: %v", err)
	}
	src2 := installFixture(t, "tcode.demoplug", "Demo Plug", "0.2.0")
	if _, err := InstallFromGit("file:///v2", userRoot, cloneFake(src2)); err != nil {
		t.Fatalf("reinstalación: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(userRoot, "tcode.demoplug", "extension.json"))
	if err != nil {
		t.Fatalf("extension.json tras reinstalar: %v", err)
	}
	if !strings.Contains(string(data), `"version":"0.2.0"`) {
		t.Errorf("la reinstalación no reemplazó la versión: %s", data)
	}
}

// TestRemoveValidatesTheID verifica que Remove rechaza ids peligrosos o vacíos
// y raíces vacías, y que una extensión ausente da un error claro.
func TestRemoveValidatesTheID(t *testing.T) {
	if err := Remove(t.TempDir(), "../mal"); err == nil {
		t.Error("Remove aceptó ../mal")
	}
	if err := Remove(t.TempDir(), ""); err == nil {
		t.Error("Remove aceptó un id vacío")
	}
	if err := Remove("", "tcode.algo"); err == nil {
		t.Error("Remove aceptó una raíz de usuario vacía")
	}
	err := Remove(t.TempDir(), "inexistente")
	if err == nil {
		t.Error("Remove aceptó una extensión no instalada")
	} else if !strings.Contains(err.Error(), "no está instalada") {
		t.Errorf("error poco claro: %v", err)
	}
}

// TestRemoveRemovesOnlyTheTarget elimina una extensión y verifica que la otra
// queda intacta.
func TestRemoveRemovesOnlyTheTarget(t *testing.T) {
	userRoot := t.TempDir()
	srcA := installFixture(t, "tcode.aaa", "A", "0.1.0")
	srcB := installFixture(t, "tcode.bbb", "B", "0.1.0")
	if _, err := InstallFromGit("file:///a", userRoot, cloneFake(srcA)); err != nil {
		t.Fatalf("instalando tcode.aaa: %v", err)
	}
	if _, err := InstallFromGit("file:///b", userRoot, cloneFake(srcB)); err != nil {
		t.Fatalf("instalando tcode.bbb: %v", err)
	}

	if err := Remove(userRoot, "tcode.aaa"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(userRoot, "tcode.aaa")); !os.IsNotExist(err) {
		t.Errorf("tcode.aaa debería estar eliminada: %v", err)
	}
	if _, err := os.Stat(filepath.Join(userRoot, "tcode.bbb")); err != nil {
		t.Errorf("tcode.bbb no debería tocarse: %v", err)
	}
}

// TestListSkipsBrokenFolders mezcla una válida (sin name), una carpeta sin
// manifest y otra con manifest roto: List debe devolver 1 Info, 2 errores y un
// nombre vacío para la que no lo declaró.
func TestListSkipsBrokenFolders(t *testing.T) {
	userRoot := t.TempDir()
	good := filepath.Join(userRoot, "buena")
	if err := os.MkdirAll(good, 0o755); err != nil {
		t.Fatalf("MkdirAll buena: %v", err)
	}
	toyExt(t, good, "tcode.buena", "", "0.1.0")
	if err := os.MkdirAll(filepath.Join(userRoot, "sinmanifest"), 0o755); err != nil {
		t.Fatalf("MkdirAll sinmanifest: %v", err)
	}
	// Con contenido: una carpeta suelta que no es extensión se reporta. Las
	// carpetas VACÍAS se ignoran (TestListIgnoresEmptyProviderFolder).
	if err := os.WriteFile(filepath.Join(userRoot, "sinmanifest", "notas.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile notas.md: %v", err)
	}
	broken := filepath.Join(userRoot, "rota")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatalf("MkdirAll rota: %v", err)
	}
	if err := os.WriteFile(filepath.Join(broken, "extension.json"), []byte(`{"id": "tcode.rota"`), 0o644); err != nil {
		t.Fatalf("WriteFile rota: %v", err)
	}

	infos, errs := List(userRoot)
	if len(infos) != 1 {
		t.Errorf("List devolvió %d válidas, esperaba 1: %+v", len(infos), infos)
	}
	if len(errs) != 2 {
		t.Errorf("List devolvió %d errores, esperaba 2: %v", len(errs), errs)
	}
	if len(infos) == 1 {
		got := infos[0]
		if got.ID != "tcode.buena" || got.Name != "" || got.Version != "0.1.0" {
			t.Errorf("Info inesperada: %+v", got)
		}
	}
}

// TestInstallFromGitWithRealGit es la prueba de integración: crea un repo git
// local con extension.json, lo instala con el clonador REAL (clone) vía file://
// y verifica que la extensión queda instalada sin .git.
func TestInstallFromGitWithRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible")
	}
	repo := t.TempDir()
	toyExt(t, repo, "tcode.gitplug", "Git Plug", "1.0.0")

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("-c", "user.name=tcode-test", "-c", "user.email=tcode-test@example.com", "add", ".")
	run("-c", "user.name=tcode-test", "-c", "user.email=tcode-test@example.com", "commit", "-m", "init")

	userRoot := t.TempDir()
	id, err := InstallFromGit("file://"+filepath.ToSlash(repo), userRoot, clone)
	if err != nil {
		t.Fatalf("InstallFromGit con git real: %v", err)
	}
	if id != "tcode.gitplug" {
		t.Errorf("id = %q, esperaba tcode.gitplug", id)
	}
	if _, err := os.Stat(filepath.Join(userRoot, id, "extension.json")); err != nil {
		t.Errorf("extension.json no instalado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(userRoot, id, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git real no debió copiarse: %v", err)
	}
}

// remoteProviderForUpdate crea un proveedor remoto (URL) backed por un
// "monorepo" en disco: el fetchFake emula el sparse checkout, así el test
// ejercita el camino de lectura liviana (solo manifests) y el acotado a la
// subcarpeta de la extensión.
func remoteProviderForUpdate(name string) Provider {
	return Provider{Name: name, Source: "https://example.com/" + name + ".git", Approved: true}
}

// bumpVersion reescribe el manifest de la extensión id en el "monorepo" src con
// una versión nueva: simula al autor publicando un cambio. Devuelve el archivo
// auxiliar con contenido nuevo, para poder afirmar que los archivos cambiaron.
func bumpVersion(t *testing.T, src, subdir, id, name, version, marker string) {
	t.Helper()
	toyExt(t, filepath.Join(src, subdir), id, name, version)
	if err := os.WriteFile(filepath.Join(src, subdir, "main.lua"), []byte(marker), 0o644); err != nil {
		t.Fatalf("WriteFile main.lua: %v", err)
	}
}

// TestUpdateAllSkipsWhenVersionIsTheSame: con la misma versión instalada y
// declarada no hay nada que hacer: ni se actualiza ni se reporta un error.
func TestUpdateAllSkipsWhenVersionIsTheSame(t *testing.T) {
	src := providerFixture(t, map[string][3]string{"linter": {"tcode.linter", "Linter", "1.2.3"}})
	if err := os.WriteFile(filepath.Join(src, "linter", "main.lua"), []byte("return {v=1}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main.lua: %v", err)
	}
	p := remoteProviderForUpdate("remoto")
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.linter", []Provider{p}, userRoot, fetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}

	updates, errs := UpdateAll([]Provider{p}, userRoot, fetchFake(src))
	if len(errs) != 0 {
		t.Errorf("UpdateAll reportó errores inesperados: %v", errs)
	}
	if len(updates) != 0 {
		t.Errorf("UpdateAll actualizó sin cambio de versión: %+v", updates)
	}
	// La instalación queda intacta: el archivo conserva su contenido.
	data, err := os.ReadFile(filepath.Join(userRoot, "remoto", "tcode.linter", "main.lua"))
	if err != nil {
		t.Fatalf("leyendo el archivo instalado: %v", err)
	}
	if string(data) != "return {v=1}\n" {
		t.Errorf("la instalación fue tocada sin cambio de versión: %q", data)
	}
}

// TestUpdateAllUpdatesWhenVersionChanges: si el proveedor sube la versión, la
// instalación se reemplaza con la nueva (manifest y archivos) y se reporta el
// salto de versión.
func TestUpdateAllUpdatesWhenVersionChanges(t *testing.T) {
	src := providerFixture(t, map[string][3]string{"linter": {"tcode.linter", "Linter", "1.2.3"}})
	if err := os.WriteFile(filepath.Join(src, "linter", "main.lua"), []byte("viejo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main.lua: %v", err)
	}
	p := remoteProviderForUpdate("remoto")
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.linter", []Provider{p}, userRoot, fetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}

	bumpVersion(t, src, "linter", "tcode.linter", "Linter", "2.0.0", "nuevo\n")
	updates, errs := UpdateAll([]Provider{p}, userRoot, fetchFake(src))
	if len(errs) != 0 {
		t.Fatalf("UpdateAll reportó errores: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("UpdateAll devolvió %d actualizaciones, esperaba 1: %+v", len(updates), updates)
	}
	got := updates[0]
	if got.Ref != "remoto/tcode.linter" || got.OldVer != "1.2.3" || got.NewVer != "2.0.0" {
		t.Errorf("UpdateResult inesperado: %+v", got)
	}
	dest := filepath.Join(userRoot, "remoto", "tcode.linter")
	manifest, err := os.ReadFile(filepath.Join(dest, "extension.json"))
	if err != nil {
		t.Fatalf("leyendo el manifest actualizado: %v", err)
	}
	if !strings.Contains(string(manifest), "2.0.0") {
		t.Errorf("el manifest en disco sigue viejo: %s", manifest)
	}
	code, err := os.ReadFile(filepath.Join(dest, "main.lua"))
	if err != nil {
		t.Fatalf("leyendo el archivo actualizado: %v", err)
	}
	if string(code) != "nuevo\n" {
		t.Errorf("el archivo en disco no se reemplazó: %q", code)
	}
}

// TestUpdateAllReportsUnknownProvider: una instalación cuyo proveedor ya no
// está en la lista vigente no se puede comparar: se reporta y no se corta la
// revisión de las demás.
func TestUpdateAllReportsUnknownProvider(t *testing.T) {
	src := providerFixture(t, map[string][3]string{"linter": {"tcode.linter", "Linter", "1.0.0"}})
	viejo := remoteProviderForUpdate("viejo")
	otro := remoteProviderForUpdate("otro")
	otroSrc := providerFixture(t, map[string][3]string{"tema": {"tcode.tema", "Tema", "3.0.0"}})
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.linter", []Provider{viejo}, userRoot, fetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}
	if _, err := InstallByID("tcode.tema", []Provider{otro}, userRoot, fetchFake(otroSrc), nil); err != nil {
		t.Fatalf("InstallByID tema: %v", err)
	}

	fetch := func(url, dest, pattern string) error {
		if strings.Contains(url, "viejo") {
			return fetchFake(src)(url, dest, pattern)
		}
		return fetchFake(otroSrc)(url, dest, pattern)
	}
	updates, errs := UpdateAll([]Provider{otro}, userRoot, fetch)
	if len(updates) != 0 {
		t.Errorf("no debía actualizar nada, pero actualizó: %+v", updates)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "viejo") {
		t.Fatalf("errores inesperados: %v", errs)
	}
}

// TestUpdateAllReportsExtensionMissingFromProvider: si el proveedor ya no
// ofrece la extensión instalada, es un error reportado (no un borrado ni una
// falla silenciosa).
func TestUpdateAllReportsExtensionMissingFromProvider(t *testing.T) {
	src := providerFixture(t, map[string][3]string{"linter": {"tcode.linter", "Linter", "1.0.0"}})
	p := remoteProviderForUpdate("remoto")
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.linter", []Provider{p}, userRoot, fetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}

	// El autor saca la extensión del monorepo.
	if err := os.RemoveAll(filepath.Join(src, "linter")); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	updates, errs := UpdateAll([]Provider{p}, userRoot, fetchFake(src))
	if len(updates) != 0 {
		t.Errorf("no debía actualizar nada: %+v", updates)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "tcode.linter") {
		t.Fatalf("errores inesperados: %v", errs)
	}
}

// TestUpdateAllUpdatesOnlyChangedAcrossProviders: con varias instaladas, solo
// cambian las que el proveedor reporta con otra versión, y una caída no impide
// revisar el resto.
func TestUpdateAllUpdatesOnlyChangedAcrossProviders(t *testing.T) {
	src := providerFixture(t, map[string][3]string{
		"linter":  {"tcode.linter", "Linter", "1.0.0"},
		"tema":    {"tcode.tema", "Tema", "2.0.0"},
		"formato": {"tcode.formato", "Formato", "5.0.0"},
	})
	p := remoteProviderForUpdate("remoto")
	caido := remoteProviderForUpdate("caido")
	caidoSrc := providerFixture(t, map[string][3]string{"x": {"tcode.x", "X", "1.0.0"}})
	userRoot := t.TempDir()
	fetch := func(url, dest, pattern string) error {
		if strings.Contains(url, "caido") {
			return fetchFake(caidoSrc)(url, dest, pattern)
		}
		return fetchFake(src)(url, dest, pattern)
	}
	for _, id := range []string{"tcode.linter", "tcode.tema", "tcode.formato"} {
		if _, err := InstallByID(id, []Provider{p}, userRoot, fetch, nil); err != nil {
			t.Fatalf("InstallByID %s: %v", id, err)
		}
	}
	if _, err := InstallByID("tcode.x", []Provider{caido}, userRoot, fetch, nil); err != nil {
		t.Fatalf("InstallByID tcode.x: %v", err)
	}

	bumpVersion(t, src, "tema", "tcode.tema", "Tema", "2.1.0", "tema nuevo\n")

	var caidas int
	caidoAbajo := func(url, dest, pattern string) error {
		if strings.Contains(url, "caido") {
			caidas++
			return errors.New("repo inalcanzable")
		}
		return fetchFake(src)(url, dest, pattern)
	}
	updates, errs := UpdateAll([]Provider{p, caido}, userRoot, caidoAbajo)
	if len(updates) != 1 || updates[0].Ref != "remoto/tcode.tema" || updates[0].NewVer != "2.1.0" {
		t.Fatalf("solo debía actualizarse tcode.tema: %+v", updates)
	}
	// La caída del segundo proveedor se reporta, sin cortar la actualización.
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "caido") {
		t.Fatalf("errores inesperados: %v", errs)
	}
	if caidas == 0 {
		t.Error("el proveedor caído no llegó a leerse")
	}
	// Las que no cambiaron siguen en su versión anterior.
	for id, want := range map[string]string{"tcode.linter": "1.0.0", "tcode.formato": "5.0.0"} {
		data, err := os.ReadFile(filepath.Join(userRoot, "remoto", id, "extension.json"))
		if err != nil {
			t.Fatalf("leyendo %s: %v", id, err)
		}
		if !strings.Contains(string(data), want) {
			t.Errorf("%s cambió sin que el proveedor cambiara su versión: %s", id, data)
		}
	}
}

// TestAvailableExtensionsDetectsOnlyTheMissing: el catálogo del proveedor se
// diff contra lo instalado, así que una extensión que NO está en userRoot se
// reporta como novedad y la que ya está instalada no aparece.
func TestAvailableExtensionsDetectsOnlyTheMissing(t *testing.T) {
	src := providerFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.0.0"},
		"tema":   {"tcode.tema", "Tema", "2.0.0"},
	})
	p := remoteProviderForUpdate("remoto")
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.linter", []Provider{p}, userRoot, fetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}

	available, errs := AvailableExtensions([]Provider{p}, userRoot, fetchFake(src))
	if len(errs) != 0 {
		t.Fatalf("AvailableExtensions reportó errores: %v", errs)
	}
	if len(available) != 1 {
		t.Fatalf("AvailableExtensions devolvió %d novedades, esperaba 1: %+v", len(available), available)
	}
	got := available[0]
	if got.ID != "tcode.tema" || got.Provider.Name != "remoto" || got.Version != "2.0.0" || got.Subdir != "tema" {
		t.Errorf("AvailableExt inesperado: %+v", got)
	}
	if got.Ref() != "remoto/tcode.tema" {
		t.Errorf("Ref inesperado: %q", got.Ref())
	}
	// Instalada también la que faltaba, no queda ninguna novedad.
	if _, err := InstallByID("tcode.tema", []Provider{p}, userRoot, fetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID tema: %v", err)
	}
	available, errs = AvailableExtensions([]Provider{p}, userRoot, fetchFake(src))
	if len(errs) != 0 {
		t.Fatalf("AvailableExtensions reportó errores: %v", errs)
	}
	if len(available) != 0 {
		t.Errorf("no debía reportar novedades con todo instalado: %+v", available)
	}
}

// TestAvailableExtensionsIgnoresBrokenProviderAndDedupes: un proveedor con
// nombre inválido se reporta y se ignora, un repo caído no corta el resto, y
// el mismo id en dos proveedores aparece UNA vez (gana el primero, como al
// resolver e instalar).
func TestAvailableExtensionsIgnoresBrokenProviderAndDedupes(t *testing.T) {
	primero := providerFixture(t, map[string][3]string{"linter": {"tcode.linter", "Linter", "1.0.0"}})
	segundo := providerFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "9.9.9"},
		"tema":   {"tcode.tema", "Tema", "2.0.0"},
	})
	caido := "https://example.com/caido.git"
	fetch := func(url, dest, pattern string) error {
		switch {
		case strings.Contains(url, "primero"):
			return fetchFake(primero)(url, dest, pattern)
		case strings.Contains(url, "segundo"):
			return fetchFake(segundo)(url, dest, pattern)
		}
		return errors.New("repo inalcanzable")
	}
	providers := []Provider{
		{Name: "../escape", Source: primero, Approved: true},
		{Name: "primero", Source: "https://example.com/primero.git", Approved: true},
		{Name: "segundo", Source: "https://example.com/segundo.git", Approved: true},
		{Name: "caido", Source: caido, Approved: true},
	}

	available, errs := AvailableExtensions(providers, t.TempDir(), fetch)
	// Los tres problemas se acumulan: nombre inválido y repo caído.
	if len(errs) != 2 {
		t.Fatalf("errores inesperados: %v", errs)
	}
	if len(available) != 2 {
		t.Fatalf("novedades inesperadas: %+v", available)
	}
	// El id repetido se reporta solo del primer proveedor, en su versión.
	if available[0].Ref() != "primero/tcode.linter" || available[0].Version != "1.0.0" {
		t.Errorf("la dedupe no favoreció al primer proveedor: %+v", available[0])
	}
	if available[1].Ref() != "segundo/tcode.tema" {
		t.Errorf("novedad inesperada: %+v", available[1])
	}
}

// TestInstallAvailableInstallsApprovedAndSkipsUnapproved: las novedades de un
// proveedor aprobado se instalan en <root>/<proveedor>/<id> con sus archivos;
// las de un proveedor sin aprobar se saltan (quedan para que las reporte quien
// llama) y no dejan nada en disco.
func TestInstallAvailableInstallsApprovedAndSkipsUnapproved(t *testing.T) {
	src := providerFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.0.0"},
		"tema":   {"tcode.tema", "Tema", "2.0.0"},
	})
	if err := os.WriteFile(filepath.Join(src, "tema", "main.lua"), []byte("return {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main.lua: %v", err)
	}
	aprobado := remoteProviderForUpdate("aprobado")
	sinAprobar := Provider{Name: "sinaprobar", Source: "https://example.com/sinaprobar.git"}
	available := []AvailableExt{
		{Provider: aprobado, ID: "tcode.linter", Name: "Linter", Version: "1.0.0", Subdir: "linter"},
		{Provider: sinAprobar, ID: "tcode.tema", Name: "Tema", Version: "2.0.0", Subdir: "tema"},
	}
	userRoot := t.TempDir()
	fetch := func(url, dest, pattern string) error {
		if strings.Contains(url, "sinaprobar") {
			return fetchFake(src)(url, dest, pattern)
		}
		return fetchFake(src)(url, dest, pattern)
	}

	installed, errs := InstallAvailable(available, userRoot, fetch)
	if len(errs) != 0 {
		t.Fatalf("InstallAvailable reportó errores: %v", errs)
	}
	if len(installed) != 1 || installed[0].Ref() != "aprobado/tcode.linter" {
		t.Fatalf("solo debía instalarse la aprobada: %+v", installed)
	}
	if _, err := os.Stat(filepath.Join(userRoot, "aprobado", "tcode.linter", "extension.json")); err != nil {
		t.Fatalf("la extensión aprobada no quedó instalada: %v", err)
	}
	if _, err := os.Stat(filepath.Join(userRoot, "sinaprobar")); !os.IsNotExist(err) {
		t.Errorf("la extensión sin aprobar no debía tocarse el disco: %v", err)
	}
}

// TestInstallAvailableReportsFailureWithoutStopping: una extensión que no se
// puede instalar se reporta y no impide instalar las siguientes.
func TestInstallAvailableReportsFailureWithoutStopping(t *testing.T) {
	src := providerFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.0.0"},
		"roto":   {"tcode.roto", "Roto", "1.0.0"},
	})
	p := remoteProviderForUpdate("remoto")
	available := []AvailableExt{
		{Provider: p, ID: "tcode.roto", Name: "Roto", Version: "1.0.0", Subdir: "roto"},
		{Provider: p, ID: "tcode.linter", Name: "Linter", Version: "1.0.0", Subdir: "linter"},
	}
	userRoot := t.TempDir()

	// El manifest se invalida después de ser detectado: la instalación lo
	// vuelve a leer cuando baja los archivos de la subcarpeta.
	if err := os.WriteFile(filepath.Join(src, "roto", "extension.json"), []byte(`{"id":"tcode.roto","name":"Roto","version":"no-semver"}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest roto: %v", err)
	}
	installed, errs := InstallAvailable(available, userRoot, fetchFake(src))
	if len(installed) != 1 || installed[0].Ref() != "remoto/tcode.linter" {
		t.Fatalf("debía instalarse solo la válida: %+v", installed)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "tcode.roto") {
		t.Fatalf("errores inesperados: %v", errs)
	}
	if _, err := os.Stat(filepath.Join(userRoot, "remoto", "tcode.roto")); !os.IsNotExist(err) {
		t.Errorf("el manifest inválido no debía crear nada: %v", err)
	}
}

// TestCheckUpdatesDetectsWithoutApplying: CheckUpdates ve el salto de versión
// pero NO toca la instalación —es la detección que arma la pregunta del prompt
// de arranque, y preguntar no puede cambiar el disco—. Recién UpdateAll baja los
// archivos nuevos.
func TestCheckUpdatesDetectsWithoutApplying(t *testing.T) {
	src := providerFixture(t, map[string][3]string{"linter": {"tcode.linter", "Linter", "1.0.0"}})
	if err := os.WriteFile(filepath.Join(src, "linter", "main.lua"), []byte("viejo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main.lua: %v", err)
	}
	p := remoteProviderForUpdate("remoto")
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.linter", []Provider{p}, userRoot, fetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}

	bumpVersion(t, src, "linter", "tcode.linter", "Linter", "2.0.0", "nuevo\n")
	updates, errs := CheckUpdates([]Provider{p}, userRoot, fetchFake(src))
	if len(errs) != 0 {
		t.Fatalf("CheckUpdates reportó errores: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("CheckUpdates devolvió %d actualizaciones, esperaba 1: %+v", len(updates), updates)
	}
	got := updates[0]
	if got.Ref != "remoto/tcode.linter" || got.OldVer != "1.0.0" || got.NewVer != "2.0.0" {
		t.Errorf("UpdateResult inesperado: %+v", got)
	}

	// Nada se aplicó: el manifest sigue en 1.0.0 y el archivo con el contenido
	// viejo es el que quedó instalado.
	dest := filepath.Join(userRoot, "remoto", "tcode.linter")
	manifest, err := os.ReadFile(filepath.Join(dest, "extension.json"))
	if err != nil {
		t.Fatalf("leyendo el manifest instalado: %v", err)
	}
	if !strings.Contains(string(manifest), "1.0.0") {
		t.Errorf("CheckUpdates aplicó la actualización: el manifest ya dice %s", manifest)
	}
	code, err := os.ReadFile(filepath.Join(dest, "main.lua"))
	if err != nil {
		t.Fatalf("leyendo el archivo instalado: %v", err)
	}
	if string(code) != "viejo\n" {
		t.Errorf("CheckUpdates tocó la instalación: main.lua = %q", code)
	}

	// Y UpdateAll, sobre el mismo estado, sí aplica: la detección compartida no
	// quedó solo en una vista previa.
	if updates, errs := UpdateAll([]Provider{p}, userRoot, fetchFake(src)); len(updates) != 1 || len(errs) != 0 {
		t.Fatalf("UpdateAll no aplicó lo detectado: %+v %v", updates, errs)
	}
	if code, _ := os.ReadFile(filepath.Join(dest, "main.lua")); string(code) != "nuevo\n" {
		t.Errorf("UpdateAll no reemplazó los archivos: main.lua = %q", code)
	}
}

// TestCheckUpdatesReportsNothingWhenVersionsMatch: sin salto de versión no hay
// nada que preguntar ni que aplicar.
func TestCheckUpdatesReportsNothingWhenVersionsMatch(t *testing.T) {
	src := providerFixture(t, map[string][3]string{"linter": {"tcode.linter", "Linter", "1.0.0"}})
	p := remoteProviderForUpdate("remoto")
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.linter", []Provider{p}, userRoot, fetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}
	updates, errs := CheckUpdates([]Provider{p}, userRoot, fetchFake(src))
	if len(errs) != 0 || len(updates) != 0 {
		t.Fatalf("CheckUpdatesSin cambio de versión: %+v %v", updates, errs)
	}
}
