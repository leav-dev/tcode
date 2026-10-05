package ext

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Info describe una extensión instalada en la raíz de usuario: el id, el
// nombre y la versión que declaró su manifest. El nombre se reporta tal cual
// (vacío si el manifest no lo trae), sin adoptar el id como hace Load.
type Info struct {
	ID      string
	Name    string
	Version string
}

// CloneFunc clona el repositorio url en dest. Es inyectable para testear la
// instalación sin depender de que git exista en el entorno.
type CloneFunc func(url, dest string) error

// clone es el clonador real de la instalación: git clone con profundidad 1.
// Vive en una variable para que los tests puedan sustituirlo y para que
// InstallFromGit lo use por defecto cuando no se inyecta otro.
var clone = cloneGit

// cloneGit clona url en dest con `git clone --depth 1`. Si git no está en el
// PATH, el error lo dice claro. La salida del clone se incluye en el error
// para que una URL inválida, un 404 o un corte de red dejen rastro.
func cloneGit(url, dest string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("se necesita git en el PATH para instalar extensiones desde un repositorio")
	}
	cmd := exec.Command("git", "clone", "--depth", "1", url, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone %s: %w: %s", url, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// InstallFromGit clona el repositorio url, valida su extension.json con Load y
// copia el árbol a userRoot/<id> excluyendo la carpeta .git del repositorio.
// Devuelve el id del manifest instalado. Si cloner es nil, se usa clone (la
// implementación real).
//
// Decisión de confianza: el instalador asume que el autor instala SUS
// extensiones, desde sus repos propios; no hay marketplace, ni firmas, ni
// auditoría externa. La barrera es estructural: el manifest debe parsear y
// cumplir las reglas de Load. El repositorio jamás se ejecuta, solo se copia,
// así que un repo malicioso no corre código en la instalación: el riesgo
// real del proyecto es la confianza en el autor de la extensión.
//
// Si userRoot/<id> ya existe se reemplaza: reinstalar (iterar sobre la propia
// extensión) es el flujo normal del autor. Un manifest inválido aborta SIN
// tocar el destino.
func InstallFromGit(url string, userRoot string, cloner CloneFunc) (string, error) {
	if cloner == nil {
		cloner = clone
	}

	tmp, err := os.MkdirTemp("", "tcode-ext-")
	if err != nil {
		return "", fmt.Errorf("creando el directorio temporal: %w", err)
	}
	defer os.RemoveAll(tmp)

	if err := cloner(url, tmp); err != nil {
		// El clonador ya contextualiza: URL inválida, red caída, 404.
		return "", err
	}

	data, err := os.ReadFile(filepath.Join(tmp, "extension.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("el repositorio no tiene extension.json en su raíz")
		}
		return "", fmt.Errorf("leyendo extension.json: %w", err)
	}

	m, err := Load(data)
	if err != nil {
		return "", fmt.Errorf("manifiesto inválido: %w", err)
	}
	// Defensivo: Load ya exige id, pero si esa regla cambiara, nunca se
	// escribiría bajo una carpeta vacía.
	id := m.ID
	if id == "" {
		return "", errors.New("manifiesto sin id")
	}

	if err := os.MkdirAll(userRoot, 0o755); err != nil {
		return "", fmt.Errorf("creando %s: %w", userRoot, err)
	}

	dest := filepath.Join(userRoot, id)
	// Reemplazo: reinstalar actualiza; es el flujo normal del autor que itera.
	if _, err := os.Stat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return "", fmt.Errorf("reemplazando %s: %w", dest, err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("revisando %s: %w", dest, err)
	}

	if err := copyTree(tmp, dest, true); err != nil {
		return "", fmt.Errorf("copiando a %s: %w", dest, err)
	}
	return id, nil
}

// copyTree copia el árbol src → dst conservando los permisos de los archivos.
// Cuando skipGit es true, la carpeta .git se omite: es el artefacto del
// control de versiones, no contenido de la extensión, y arrastrarla ensucia
// la instalación y las reinstalaciones.
func copyTree(src, dst string, skipGit bool) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if skipGit && d.IsDir() && rel == ".git" {
			return filepath.SkipDir
		}
		if rel == "." {
			// La raíz de destino se crea aquí: WalkDir visita los archivos antes
			// que cualquier subdirectorio, y sin esto un árbol plano (o con .git
			// excluido como única carpeta) fallaría al copiar su primer archivo.
			return os.MkdirAll(dst, 0o755)
		}
		to := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(to, 0o755)
		case d.Type().IsRegular():
			info, err := d.Info()
			if err != nil {
				return err
			}
			return copyFile(path, to, info.Mode())
		}
		return nil
	})
}

// copyFile copia un archivo regular conservando los permisos de origen.
func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// removeIDRe acepta solo ids con la gramática del manifest (alfa inicial,
// después alfanuméricos, punto, guion o guion bajo): la clase exacta de
// carpetas que el instalador escribe bajo la raíz de usuario. Descarta vacíos,
// ".", "..", rutas y cualquier separador, de modo que Remove jamás escape de
// userRoot. No se reutiliza validateID a propósito: aquí el error es único y
// no hay manifest que contextualice.
var removeIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Remove elimina la extensión id de userRoot. El id se sanea antes de tocar
// el disco: vacíos, "."/"..", segmentos con separadores y raíces vacías se
// rechazan con un error claro. Una extensión ausente no es silencio: el autor
// espera feedback al borrar algo que no está instalado.
func Remove(userRoot, id string) error {
	if userRoot == "" {
		return errors.New("raíz de extensiones de usuario vacía")
	}
	if !removeIDRe.MatchString(id) {
		return errors.New("id de extensión inválido")
	}
	dest := filepath.Join(userRoot, id)
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("la extensión %q no está instalada", id)
		}
		return fmt.Errorf("extensions: %w", err)
	}
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("eliminando %s: %w", dest, err)
	}
	return nil
}

// List recorre las carpetas de userRoot y reporta las extensiones instaladas.
// Solo cuentan las carpetas con extension.json que valide Load; una carpeta
// sin manifest o con manifest roto se acumula en errs sin cortar el resto.
// Un root inexistente es lista vacía, no error: aún no hay nada instalado.
//
// El nombre se toma del manifest tal cual ("" si no lo trae): Load adoptaría
// el id como nombre para mensajes de estado, pero listar es reportar, y
// mentir sobre lo declarado oscurecería la salida.
func List(userRoot string) ([]Info, []error) {
	if userRoot == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(userRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("extensions: %w", err)}
	}

	var infos []Info
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(userRoot, e.Name(), "extension.json"))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		// Los campos crudos primero (para reportar el nombre tal cual), la
		// validación estructural completa de Load después.
		var raw struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		if _, err := Load(data); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		infos = append(infos, Info{ID: raw.ID, Name: raw.Name, Version: raw.Version})
	}
	return infos, errs
}
