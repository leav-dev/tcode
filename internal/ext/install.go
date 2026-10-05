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
// Provider es el proveedor del que salió la extensión (carpeta namespaced) y
// queda vacío en las instalaciones planas heredadas, que no tienen proveedor.
type Info struct {
	ID       string
	Name     string
	Version  string
	Provider string
}

// Ref devuelve la referencia completa de la extensión: "proveedor/id" cuando
// conoce el proveedor, y el id suelto para las instalaciones planas heredadas.
func (i Info) Ref() string {
	if i.Provider == "" {
		return i.ID
	}
	return i.Provider + "/" + i.ID
}

// CloneFunc clona el repositorio url en dest. Es inyectable para testear la
// instalación sin depender de que git exista en el entorno.
type CloneFunc func(url, dest string) error

// FetchFunc deja en dest SOLO los archivos de url que casan con pattern. El
// pattern es un patrón de sparse-checkout de git (no un glob de shell), no una
// ruta: por eso el mismo clonador sirve para traer todos los manifests de un
// proveedor ("*/extension.json") y los archivos de una extensión concreta
// ("linter"). Es inyectable por la misma razón que CloneFunc.
type FetchFunc func(url, dest, pattern string) error

// clone es el clonador real de la instalación de una extensión suelta: git
// clone con profundidad 1. Vive en una variable para que los tests puedan
// sustituirlo y para que InstallFromGit lo use por defecto cuando no se
// inyecta otro.
var clone = cloneGit

// fetch es el clonador real de la lectura por patrón: clone parcial + sparse
// checkout. Igual que clone, es una variable para que los tests la sustituyan.
var fetch = fetchSparseGit

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

// fetchSparseGit deja en dest solo lo que casa con pattern, con un clone
// parcial: --depth 1 --filter=blob:none --no-checkout trae el árbol de commits
// sin ningún blob, y el sparse checkout en modo no-cono materializa únicamente
// los archivos del patrón. Es lo que hace que LISTAR un proveedor no baje ni
// un .lua, mientras instalar una extensión sí baja sus archivos (y solo los
// suyos).
//
// Si git no está en el PATH, el error lo dice claro. La salida de los comandos
// se incluye en el error para que una URL inválida, un 404 o un corte de red
// dejen rastro.
func fetchSparseGit(url, dest, pattern string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("se necesita git en el PATH para leer extensiones desde un repositorio")
	}
	steps := [][]string{
		{"clone", "--depth", "1", "--filter=blob:none", "--no-checkout", url, dest},
		{"-C", dest, "sparse-checkout", "init", "--no-cone"},
		{"-C", dest, "sparse-checkout", "set", pattern},
		{"-C", dest, "checkout"},
	}
	for _, args := range steps {
		cmd := exec.Command("git", args...)
		// El aviso "filtering not recognized by server" de los transportes que
		// no soportan filtros (file://, rutas locales) no es un fallo: el sparse
		// checkout sigue acotando el materializado. Se ignora a propósito solo
		// ese warning, no la salida útil de los comandos.
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s (%s): %w: %s", args[0], url, err, strings.TrimSpace(string(out)))
		}
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

// InstallResult dice qué quedó instalado: el proveedor del que salió la
// extensión y su id.
type InstallResult struct {
	Provider string
	ID       string
}

// Ref es la referencia legible "proveedor/id" de lo instalado.
func (r InstallResult) Ref() string { return r.Provider + "/" + r.ID }

// ConfirmFunc pregunta al usuario si confía en el proveedor. Recibe el
// proveedor sin aprobar y devuelve true solo con un sí explícito.
type ConfirmFunc func(Provider) bool

// Resolution es el resultado de buscar un id entre los proveedores: qué
// proveedor lo ofrece y en qué subdirectorio vive.
type Resolution struct {
	Provider Provider
	Ext      ProviderExt
}

// ResolveExtension busca id entre los proveedores EN ORDEN: gana el primero que
// lo ofrece, así que el proveedor por defecto resuelve ante una colisión de ids.
// Un proveedor con un nombre fuera de la gramática se salta (su nombre sería
// una carpeta de instalación insegura) y su problema se acumula en el error.
//
// Si nadie lo ofrece, el error lista los ids encontrados (con su proveedor) y
// los problemas de los proveedores que no pudieron leerse: un repo caído no
// puede esconder el hecho de que la extensión no está.
func ResolveExtension(id string, providers []Provider, fetcher FetchFunc) (Resolution, error) {
	var available []string
	var errs []error
	for _, p := range providers {
		if !providerNameRe.MatchString(p.Name) {
			errs = append(errs, fmt.Errorf("proveedor con nombre inválido %q: se ignora", p.Name))
			continue
		}
		exts, err := ListExtensions(p, fetcher)
		if err != nil {
			errs = append(errs, err)
		}
		for _, e := range exts {
			if e.ID == id {
				return Resolution{Provider: p, Ext: e}, nil
			}
			available = append(available, p.Name+"/"+e.ID)
		}
	}
	msg := fmt.Sprintf("no se encontró la extensión %q entre los proveedores", id)
	if len(available) > 0 {
		msg += "; disponibles: " + strings.Join(available, ", ")
	}
	return Resolution{}, errors.Join(append(errs, errors.New(msg))...)
}

// InstallByID instala la extensión id resuelta entre los proveedores, en
// userRoot/<proveedor>/<id>/. Recorre los proveedores en orden (el primero que
// ofrece el id gana) y hace DOS lecturas del proveedor que ofrece la extensión:
// una liviana, solo los manifests, para resolver el id, y otra acotada a la
// subcarpeta de la extensión, para bajar sus archivos. La primera es la razón
// de que listar y buscar no descarguen código: los .lua solo bajan cuando van
// a ejecutarse, y solo los de la extensión elegida.
//
// Confianza: instalar desde un proveedor sin aprobar exige confirm sí (pide el
// usuario). Sin confirm —uso no interactivo— la instalación se rechaza con un
// error que dice qué aprobar. Un manifest inválido aborta SIN tocar el destino.
func InstallByID(id string, providers []Provider, userRoot string, fetcher FetchFunc, confirm ConfirmFunc) (InstallResult, error) {
	if fetcher == nil {
		fetcher = fetch
	}
	if !removeIDRe.MatchString(id) {
		return InstallResult{}, errors.New("id de extensión inválido")
	}

	res, err := ResolveExtension(id, providers, fetcher)
	if err != nil {
		return InstallResult{}, err
	}
	p, ext := res.Provider, res.Ext

	// La confianza se pide ANTES de bajar los archivos de la extensión: la
	// pregunta es "¿confías en esta fuente?", y responder que no tiene que
	// dejar el tree de la extensión en el disco.
	if !p.Approved {
		if confirm == nil {
			return InstallResult{}, fmt.Errorf("el proveedor %q no está aprobado: usá tcode --approve-provider %s", p.Name, p.Name)
		}
		if !confirm(p) {
			return InstallResult{}, fmt.Errorf("instalación cancelada: el proveedor %q no fue aprobado", p.Name)
		}
	}
	// Segunda lectura, ya acotada a la extensión. Un fallo acá NO es "no
	// existe": el id existe y el proveedor es de confianza, así que el error
	// se propaga en vez de caer al siguiente proveedor (que sería otra
	// extensión con el mismo id).
	if err := withProviderRoot(p, fetcher, ext.Subdir, func(root string) error {
		return installSubdir(root, ext.Subdir, p.Name, userRoot)
	}); err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Provider: p.Name, ID: ext.ID}, nil
}

// installSubdir valida el manifest de src y copia el árbol a
// userRoot/<provider>/<id>, reemplazando lo que hubiera (reinstalar actualiza
// y es el flujo normal). Un manifest inválido no crea nada.
func installSubdir(src, subdir, provider, userRoot string) error {
	from := filepath.Join(src, filepath.FromSlash(subdir))
	data, err := os.ReadFile(filepath.Join(from, "extension.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("la extensión no trae extension.json")
		}
		return fmt.Errorf("leyendo extension.json: %w", err)
	}
	m, err := Load(data)
	if err != nil {
		return fmt.Errorf("manifiesto inválido: %w", err)
	}
	dest := filepath.Join(userRoot, provider, m.ID)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creando %s: %w", filepath.Dir(dest), err)
	}
	if _, err := os.Stat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("reemplazando %s: %w", dest, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("revisando %s: %w", dest, err)
	}
	if err := copyTree(from, dest, true); err != nil {
		return fmt.Errorf("copiando a %s: %w", dest, err)
	}
	return nil
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
// RemoveNamespaced borra la extensión id del proveedor provider, bajo
// userRoot/<provider>/<id>. Mismas reglas de saneado que Remove: el nombre del
// proveedor y el id se validan contra removeIDRe antes de tocar el disco, así
// que un ".." o un separador en cualquiera de los dos no puede escapar de
// userRoot.
func RemoveNamespaced(userRoot, provider, id string) error {
	if userRoot == "" {
		return errors.New("raíz de extensiones de usuario vacía")
	}
	if !removeIDRe.MatchString(provider) {
		return errors.New("nombre de proveedor inválido")
	}
	if !removeIDRe.MatchString(id) {
		return errors.New("id de extensión inválido")
	}
	dest := filepath.Join(userRoot, provider, id)
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("la extensión %q no está instalada en el proveedor %q", id, provider)
		}
		return fmt.Errorf("extensions: %w", err)
	}
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("eliminando %s: %w", dest, err)
	}
	return nil
}

// RemoveRef borra la extensión que referencia arg en userRoot. Acepta
// "<proveedor>:<id>" (borra exactamente esa) o un id suelto, que se busca en
// todos los proveedores: el primero en el orden de carpetas gana. Devuelve la
// referencia efectivamente borrada.
func RemoveRef(userRoot, arg string) (string, error) {
	if userRoot == "" {
		return "", errors.New("raíz de extensiones de usuario vacía")
	}
	if provider, id, ok := splitRef(arg); ok {
		if err := RemoveNamespaced(userRoot, provider, id); err != nil {
			return "", err
		}
		return provider + ":" + id, nil
	}
	if !removeIDRe.MatchString(arg) {
		return "", errors.New("id de extensión inválido")
	}
	entries, err := os.ReadDir(userRoot)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("extensions: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(userRoot, e.Name(), arg)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			if err := RemoveNamespaced(userRoot, e.Name(), arg); err != nil {
				return "", err
			}
			return e.Name() + ":" + arg, nil
		}
	}
	// Instalación plana heredada: sin proveedor, en la raíz de usuario.
	if _, err := os.Stat(filepath.Join(userRoot, arg)); err == nil {
		if err := Remove(userRoot, arg); err != nil {
			return "", err
		}
		return arg, nil
	}
	return "", fmt.Errorf("la extensión %q no está instalada", arg)
}

// splitRef parte una referencia "proveedor:id". El gramático de ambos lados
// excluye ":", así que la partición es inequívoca.
func splitRef(arg string) (provider, id string, ok bool) {
	i := strings.Index(arg, ":")
	if i <= 0 || i == len(arg)-1 {
		return "", "", false
	}
	return arg[:i], arg[i+1:], true
}

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
// Hay dos layouts y conviven: el namespaced de ahora
// (<root>/<proveedor>/<id>/extension.json) y el plano heredado
// (<root>/<id>/extension.json), que se sigue reportando con el proveedor vacío.
// Solo cuentan las carpetas con extension.json que valide Load; una carpeta sin
// manifest y sin extensiones namespadas dentro se acumula en errs sin cortar el
// resto. Un root inexistente es lista vacía, no error: aún no hay nada
// instalado.
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
		dir := filepath.Join(userRoot, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, "extension.json"))
		if err != nil {
			// Sin manifest en la propia carpeta: puede ser un proveedor con
			// extensiones namespadas dentro. Si tampoco hay, se reporta igual
			// que antes: una carpeta suelta no es una extensión.
			sub, subErrs := listNamespaced(dir, e.Name())
			if len(sub) == 0 {
				if vacia, vacErr := isEmptyDir(dir); vacErr == nil && vacia {
					// Carpeta de proveedor que quedó vacía al borrar su última
					// extensión: no es un problema que reportar.
					continue
				}
				errs = append(errs, append(subErrs, fmt.Errorf("%s: %w", e.Name(), err))...)
				continue
			}
			infos = append(infos, sub...)
			errs = append(errs, subErrs...)
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

// isEmptyDir dice si dir no tiene entradas. Sirve para que una carpeta vacía no
// se reporte como extensión rota: borrar la última extensión de un proveedor
// deja su carpeta, y eso no es nada que avisarle al usuario.
func isEmptyDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// listNamespaced lista las extensiones bajo <root>/<proveedor>/<id>: el layout
// que escribe la instalación por proveedor. Una subcarpeta sin manifest es una
// carpeta común del proveedor y se ignora en silencio, igual que Discover.
func listNamespaced(providerRoot, provider string) ([]Info, []error) {
	entries, err := os.ReadDir(providerRoot)
	if err != nil {
		return nil, []error{fmt.Errorf("%s: %w", provider, err)}
	}
	var infos []Info
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(providerRoot, e.Name(), "extension.json"))
		if err != nil {
			continue // sin manifest, no es una extensión
		}
		var raw struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			errs = append(errs, fmt.Errorf("%s/%s: %w", provider, e.Name(), err))
			continue
		}
		if _, err := Load(data); err != nil {
			errs = append(errs, fmt.Errorf("%s/%s: %w", provider, e.Name(), err))
			continue
		}
		infos = append(infos, Info{ID: raw.ID, Name: raw.Name, Version: raw.Version, Provider: provider})
	}
	return infos, errs
}
