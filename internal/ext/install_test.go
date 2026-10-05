package ext

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
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
		return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			to := filepath.Join(dest, rel)
			if d.IsDir() {
				return os.MkdirAll(to, 0o755)
			}
			if !d.Type().IsRegular() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(to, data, 0o644)
		})
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

// TestInstallFromGitSubdirClonesThatFolder verifica que instalar desde una
// subcarpeta de un monorepo copia SOLO esa carpeta: el manifest válido y su
// auxiliar entran; la otra carpeta del repo y el .git (en la raíz del clon)
// no.
func TestInstallFromGitSubdirClonesThatFolder(t *testing.T) {
	src := t.TempDir()
	sub := filepath.Join(src, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll sub: %v", err)
	}
	toyExt(t, sub, "tcode.subplug", "Sub Plug", "0.3.0")
	if err := os.WriteFile(filepath.Join(sub, "extra.txt"), []byte("extra"), 0o644); err != nil {
		t.Fatalf("WriteFile extra.txt: %v", err)
	}
	// El .git del repo vive en la raíz del clon, junto a otra extensión del
	// monorepo: ninguna de las dos debe llegar a la instalación.
	gitDir := filepath.Join(src, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0o644); err != nil {
		t.Fatalf("WriteFile .git/HEAD: %v", err)
	}
	other := filepath.Join(src, "otro")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatalf("MkdirAll otro: %v", err)
	}
	toyExt(t, other, "tcode.otro", "Outro", "1.0.0")

	userRoot := t.TempDir()
	id, err := InstallFromGitSubdir("file:///monorepo", "sub", userRoot, cloneFake(src))
	if err != nil {
		t.Fatalf("InstallFromGitSubdir: %v", err)
	}
	if id != "tcode.subplug" {
		t.Errorf("id = %q, esperaba tcode.subplug", id)
	}

	dest := filepath.Join(userRoot, id)
	data, err := os.ReadFile(filepath.Join(dest, "extension.json"))
	if err != nil {
		t.Fatalf("extension.json instalado: %v", err)
	}
	if !strings.Contains(string(data), `"version":"0.3.0"`) {
		t.Errorf("extension.json sin la versión instalada: %s", data)
	}
	if _, err := os.Stat(filepath.Join(dest, "extra.txt")); err != nil {
		t.Errorf("archivo auxiliar de la subcarpeta no copiado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "otro")); !os.IsNotExist(err) {
		t.Errorf("la otra carpeta del monorepo no debió copiarse: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git no debió copiarse: %v", err)
	}
}

// TestInstallFromGitSubdirRejectsBrokenManifest verifica que un manifest
// inválido en la subcarpeta aborta sin tocar el destino.
func TestInstallFromGitSubdirRejectsBrokenManifest(t *testing.T) {
	src := t.TempDir()
	sub := filepath.Join(src, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "extension.json"), []byte(`{"id": "tcode.roto", "version": `), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	userRoot := t.TempDir()
	id, err := InstallFromGitSubdir("file:///repo-roto", "sub", userRoot, cloneFake(src))
	if err == nil {
		t.Fatalf("InstallFromGitSubdir aceptó un manifest inválido (id %q)", id)
	}
	if !strings.Contains(err.Error(), "manifiesto inválido") {
		t.Errorf("error sin contexto de manifest: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(userRoot, "tcode.roto")); !os.IsNotExist(statErr) {
		t.Errorf("el destino no debió crearse: %v", statErr)
	}
}

// TestInstallFromGitSubdirMissingFolder verifica el error claro cuando el clon
// no trae la subcarpeta pedida.
func TestInstallFromGitSubdirMissingFolder(t *testing.T) {
	src := t.TempDir() // clon sin la carpeta "sub"
	if err := os.WriteFile(filepath.Join(src, "extension.json"), []byte(`{"id":"tcode.raiz","name":"Raiz","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := InstallFromGitSubdir("file:///repo-sin-sub", "sub", t.TempDir(), cloneFake(src))
	if err == nil {
		t.Fatal("InstallFromGitSubdir aceptó un clon sin la subcarpeta")
	}
	if !strings.Contains(err.Error(), "el repositorio no contiene sub") {
		t.Errorf("error poco claro: %v", err)
	}
}
