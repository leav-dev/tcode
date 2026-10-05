package ext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultProviderSource es el proveedor por defecto: el monorepo oficial de tcode
// donde cada subcarpeta con extension.json es una extensión. Es una constante
// en código, NO una entrada de ~/.tcode/providers.json: el editor lo conoce
// siempre, sin configuración, y solo lo REGISTRA (no instala nada solo). Por
// diseño está siempre aprobado: es la fuente que el propio proyecto publica.
const DefaultProviderSource = "https://github.com/leav-dev/tcode-extention"

// DefaultProvider devuelve el proveedor por defecto con su nombre derivado de
// la URL. Si la derivación fallara (no debería: la constante es una URL fija),
// el nombre queda vacío y las funciones que lo necesitan lo tratan como una
// fuente más.
func DefaultProvider() Provider {
	name, err := DeriveName(DefaultProviderSource)
	if err != nil {
		name = ""
	}
	return Provider{Name: name, Source: DefaultProviderSource, Approved: true}
}

// Provider es una fuente de extensiones: un repositorio git (monorepo de
// subcarpetas con extension.json) o una carpeta local con esa misma forma.
// Name es la identidad con la que se namespacan las instalaciones
// (~/.tcode/extensions/<provider>/<id>), Source es la URL o la ruta, y
// Approved es la confianza explícita del usuario: agregar deja approved en
// false y sin aprobar no se instala desde ahí sin confirmación.
type Provider struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Approved bool   `json:"approved"`
}

// ProviderExt es una extensión dentro de un proveedor: los datos de su
// manifest y el subdirectorio que la contiene dentro de la fuente. Subdir es
// un nombre relativo (no una ruta absoluta) porque en un proveedor git la ruta
// válida solo existe mientras vive el clon temporal.
type ProviderExt struct {
	ID      string
	Name    string
	Version string
	Subdir  string
}

// providerNameRe es la gramática de nombres de proveedor: los mismos caracteres
// que un id de extensión. Es también la barrera contra escapes de ruta, porque
// el nombre se usa como carpeta de instalación.
var providerNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// providersFile es el sobre de ~/.tcode/providers.json. Los proveedores
// guardados van en orden de resolución, después del proveedor por defecto.
type providersFile struct {
	Providers []Provider `json:"providers"`
}

// ProvidersFilePath devuelve la ruta del config de proveedores bajo home.
func ProvidersFilePath(home string) string {
	return filepath.Join(home, ".tcode", "providers.json")
}

// LoadProviders lee ~/.tcode/providers.json. Un archivo inexistente no es un
// error: no hay proveedores agregados todavía y solo manda el por defecto. Un
// archivo corrupto sí lo es, nombrando el problema, porque perder approvals en
// silencio dejaría al usuario instalando desde fuentes que creía haber
// revisado.
func LoadProviders(configPath string) ([]Provider, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("leyendo %s: %w", configPath, err)
	}
	var file providersFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("config de proveedores inválida (%s): %w", configPath, err)
	}
	for i, p := range file.Providers {
		if !providerNameRe.MatchString(p.Name) {
			return nil, fmt.Errorf("proveedor %d: nombre inválido %q", i, p.Name)
		}
		if strings.TrimSpace(p.Source) == "" {
			return nil, fmt.Errorf("proveedor %q: fuente vacía", p.Name)
		}
	}
	return file.Providers, nil
}

// SaveProviders escribe ~/.tcode/providers.json. El sobre se recrea entero,
// así que el orden de la lista es el orden de resolución.
func SaveProviders(configPath string, providers []Provider) error {
	if configPath == "" {
		return errors.New("ruta de config de proveedores vacía")
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("creando %s: %w", filepath.Dir(configPath), err)
	}
	data, err := json.MarshalIndent(providersFile{Providers: providers}, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando proveedores: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("escribiendo %s: %w", configPath, err)
	}
	return nil
}

// AllProviders devuelve la lista de resolución: el proveedor por defecto
// primero (siempre aprobado y no guardado en el config) y después los del
// archivo en su orden. Un proveedor guardado con el nombre del por defecto se
// descarta: el por defecto gana ante nombres duplicados, porque es el primero
// y el único que no se puede quitar.
func AllProviders(configPath string) ([]Provider, error) {
	stored, err := LoadProviders(configPath)
	if err != nil {
		return nil, err
	}
	def := DefaultProvider()
	providers := make([]Provider, 0, len(stored)+1)
	providers = append(providers, def)
	seen := map[string]bool{}
	if def.Name != "" {
		seen[def.Name] = true
	}
	for _, p := range stored {
		if seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		providers = append(providers, p)
	}
	return providers, nil
}

// IsLocalSource dice si source es una carpeta local en disco. Un source con
// esquema ("https://…", "file://…") o con forma scp ("git@host:user/repo")
// nunca es local aunque exista algo con ese nombre; el resto es local si el
// directorio existe. "~" se expande para que la config guarde la ruta real.
func IsLocalSource(source string) bool {
	src := strings.TrimSpace(source)
	if src == "" || strings.Contains(src, "://") || isSCPLike(src) {
		return false
	}
	if strings.HasPrefix(src, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		src = filepath.Join(home, strings.TrimPrefix(src, "~"))
	}
	info, err := os.Stat(src)
	return err == nil && info.IsDir()
}

// IsRemoteSource dice si source es una fuente remota: una URL con esquema
// ("https://…", "git://…", "file://…") o la forma scp ("git@host:user/repo").
// Lo que no es ni local ni remoto es una ruta que no existe, que es un error
// de tipeo y no una fuente: la distinction permite decirlo claro en vez de
// dejar que git falle con "repository not found".
func IsRemoteSource(source string) bool {
	src := strings.TrimSpace(source)
	return src != "" && (strings.Contains(src, "://") || isSCPLike(src))
}

// isSCPLike detecta la sintaxis git@host:user/repo.git, que no lleva esquema
// pero tampoco es una carpeta.
func isSCPLike(src string) bool {
	i := strings.Index(src, ":")
	if i <= 0 {
		return false
	}
	// Un "C:\..." de Windows no cuenta; en la práctica el host es la parte
	// previa al ":".
	host := src[:i]
	return strings.Contains(host, "@")
}

// CanonicalSource normaliza la fuente para persistirla: una carpeta local se
// guarda como ruta absoluta (con ~ expandido), así el config sigue valiendo
// desde cualquier directorio; una fuente remota se guarda tal cual, trimmeada.
func CanonicalSource(source string) (string, error) {
	src := strings.TrimSpace(source)
	if src == "" {
		return "", errors.New("fuente de proveedor vacía")
	}
	if IsLocalSource(src) {
		if strings.HasPrefix(src, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("expandiendo ~: %w", err)
			}
			src = filepath.Join(home, strings.TrimPrefix(src, "~"))
		}
		return filepath.Abs(src)
	}
	return src, nil
}

// DeriveName deriva el nombre de identidad del proveedor desde su fuente: la
// URL git aporta el último componente de la ruta (tcode-extention) y una carpeta
// local aporta su nombre (mis-extensions). Es la misma identidad con la que se
// namespacan las instalaciones, y por eso se valida contra providerNameRe: un
// nombre con separadores o ".." escaparía de ~/.tcode/extensions.
//
// Riesgo documentado: dos proveedores con el mismo nombre colisionan; en la
// resolución gana el primero, así que el segundo queda inalcanzable por id.
func DeriveName(source string) (string, error) {
	src := strings.TrimSpace(source)
	if src == "" {
		return "", errors.New("fuente de proveedor vacía")
	}
	if IsLocalSource(src) {
		if strings.HasPrefix(src, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("expandiendo ~: %w", err)
			}
			src = filepath.Join(home, strings.TrimPrefix(src, "~"))
		}
		abs, err := filepath.Abs(src)
		if err != nil {
			return "", fmt.Errorf("resolviendo %s: %w", src, err)
		}
		return checkProviderName(filepath.Base(abs))
	}
	src = strings.TrimSuffix(src, "/")
	src = strings.TrimSuffix(src, ".git")
	// Se separa el esquema antes de buscar el último componente: sin él, una
	// URL que solo trae host ("https://host/") se quedaría sin separador y
	// derivaría el nombre del host en vez de fallar.
	_, hadScheme := "", false
	if i := strings.Index(src, "://"); i >= 0 {
		src, hadScheme = src[i+3:], true
	}
	if i := strings.LastIndexAny(src, "/:"); i >= 0 {
		src = src[i+1:]
	} else if hadScheme {
		// Solo host y esquema: no hay componente de ruta del que sacar nombre.
		return "", fmt.Errorf("no se pudo derivar un nombre de proveedor desde la fuente %q", source)
	}
	return checkProviderName(src)
}

// checkProviderName valida un nombre derivado: vacío o con caracteres fuera de
// la gramática no sirven ni como etiqueta ni como carpeta.
func checkProviderName(name string) (string, error) {
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("no se pudo derivar un nombre de proveedor desde la fuente (quedó %q)", name)
	}
	if !providerNameRe.MatchString(name) {
		return "", fmt.Errorf("nombre de proveedor inválido %q", name)
	}
	return name, nil
}

// manifestsPattern es el patrón de sparse checkout del LISTADO: todos los
// extension.json del proveedor y nada más. Es el corazón del listado liviano:
// con él, buscar o listar extensiones de un proveedor git no descarga ni un
// archivo .lua.
const manifestsPattern = "*/extension.json"

// ListExtensions lista las extensiones que ofrece un proveedor. Un proveedor
// local se escanea directo; uno git se lee con un clon PARCIAL que materializa
// solo los manifests (sin .lua) en un temporal que se borra al salir. Los
// manifests rotos no cortan el listado: se acumulan en el error devuelto,
// igual que Discover, y las carpetas sin extension.json se ignoran en
// silencio.
//
// fetcher nil usa el lector real (necesita git en el PATH). Como el temporal de
// un clon git se limpia al salir, Subdir es relativo al root leído y no una
// ruta absoluta: solo vale mientras dura la lectura.
func ListExtensions(p Provider, fetcher FetchFunc) ([]ProviderExt, error) {
	var (
		exts []ProviderExt
		err  error
	)
	walkErr := withProviderRoot(p, fetcher, manifestsPattern, func(root string) error {
		exts, err = scanProviderRoot(root)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return exts, err
}

// withProviderRoot da el root legible de un proveedor y ejecuta fn contra él:
// la carpeta local directamente, o un temporal con lo que casa con pattern que
// se borra al salir. Centralizarlo mantiene en un solo lugar la regla de que un
// proveedor local nunca pasa por git, y la de que un temporal se limpia siempre.
func withProviderRoot(p Provider, fetcher FetchFunc, pattern string, fn func(root string) error) error {
	if IsLocalSource(p.Source) {
		return fn(p.Source)
	}
	if fetcher == nil {
		fetcher = fetch
	}
	tmp, err := os.MkdirTemp("", "tcode-provider-")
	if err != nil {
		return fmt.Errorf("creando el directorio temporal: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := fetcher(p.Source, tmp, pattern); err != nil {
		return fmt.Errorf("leyendo el proveedor %s: %w", p.Source, err)
	}
	return fn(tmp)
}

// scanProviderRoot recorre un root de proveedor (carpeta local o clon
// temporal) y traduce sus extensiones a ProviderExt con el subdirectorio
// relativo al root.
func scanProviderRoot(root string) ([]ProviderExt, error) {
	found, errs := Discover(root)
	out := make([]ProviderExt, 0, len(found))
	for _, e := range found {
		rel, err := filepath.Rel(root, e.Dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Dir, err))
			continue
		}
		out = append(out, ProviderExt{
			ID:      e.Manifest.ID,
			Name:    e.Manifest.Name,
			Version: e.Manifest.Version,
			Subdir:  filepath.ToSlash(rel),
		})
	}
	return out, errors.Join(errs...)
}

// ValidateProviderSource comprueba que la fuente exista y tenga la forma de un
// proveedor (subcarpetas con extension.json válidas): un repo o carpeta vacíos
// se rechazan al agregar, no al instalar. Devuelve las extensiones que ofrece
// la fuente, para poder reportar cuántas hay.
//
// La validación es liviana como el listado (solo manifests): un proveedor git
// se lee con el clon parcial, así que agregar no baja código. Lo que sí exige
// es git en el PATH y red cuando la fuente es remota: es el mismo trabajo que
// hará la resolución, así que agregar no valida una ilusión que luego no se
// sostiene.
func ValidateProviderSource(p Provider, fetcher FetchFunc) ([]ProviderExt, error) {
	exts, err := ListExtensions(p, fetcher)
	if len(exts) == 0 {
		if err != nil {
			return nil, fmt.Errorf("la fuente %q no tiene extensiones válidas: %w", p.Source, err)
		}
		if IsLocalSource(p.Source) {
			return nil, fmt.Errorf("la carpeta %q no tiene subcarpetas con extension.json", p.Source)
		}
		return nil, fmt.Errorf("el repositorio %q no tiene subcarpetas con extension.json", p.Source)
	}
	return exts, err
}
