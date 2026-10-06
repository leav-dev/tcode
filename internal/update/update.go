package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// version es la versión del binario, inyectada al compilar los releases con
// -ldflags "-X main.version=vX.Y.Z" (ver .github/workflows/release.yml). Vacía
// en builds de desarrollo: ahí manda debug.ReadBuildInfo (go install) o nada.
var version string

// SetVersion fija la versión en tests (el valor real lo pone el linker).
func SetVersion(v string) { version = v }

// apiBase es la base de la API de releases; variable para apuntar a un fake
// en tests. El download usa la misma base en /download.
var apiBase = "https://api.github.com/repos/leav-dev/tcode"

// downloadBase es la base de descarga de assets; por defecto, la de los
// releases de GitHub. Variable por la misma razón que apiBase.
var downloadBase = "https://github.com/leav-dev/tcode/releases/latest/download"

// httpClient es el cliente del chequeo: timeout corto porque corre al
// arrancar —una red lenta no puede demorar el aviso más que unos segundos—.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// debugReadBuildInfo envuelve a debug.ReadBuildInfo para tests: el valor
// real sale del binario compilado.
var debugReadBuildInfo = debug.ReadBuildInfo

// CurrentVersion devuelve la versión propia del binario: primero la inyectada
// por ldflags (releases), después la del buildinfo (go install) y "" si no hay
// (build de desarrollo). Vacía significa "desconocida", no "cero".
func CurrentVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debugReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return ""
}

// Compare compara dos versiones "vX.Y.Z" (la v inicial es opcional) por
// componentes numéricos: -1 si a < b, 0 si iguales, 1 si a > b. Los sufijos
// no numéricos (-rc1, +build) se ignoran para el orden pero rompen el empate:
// una versión con sufijo se considera anterior a la pelada. Sin ningún número
// parseable, decide la desigualdad cruda de strings (distintas → -1).
func Compare(a, b string) int {
	pa, oka := parse(a)
	pb, okb := parse(b)
	if !oka || !okb {
		if a == b {
			return 0
		}
		return -1
	}
	for i := 0; i < 3; i++ {
		if pa.nums[i] != pb.nums[i] {
			if pa.nums[i] < pb.nums[i] {
				return -1
			}
			return 1
		}
	}
	if pa.suffix == pb.suffix {
		return 0
	}
	if pa.suffix == "" {
		return 1
	}
	if pb.suffix == "" {
		return -1
	}
	if pa.suffix == pb.suffix {
		return 0
	}
	return -1
}

// parsed es una versión normalizada: tres números y el sufijo crudo.
type parsed struct {
	nums   [3]int
	suffix string
}

// parse normaliza "v1.2.3-rc1" a {[1 2 3], "-rc1"}; faltan componentes = 0.
// ok=false si no hay ningún número (no es una versión comparable).
func parse(v string) (parsed, bool) {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	var p parsed
	ok := false
	// El sufijo es lo que cuelga del primer '-' o '+'.
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		p.suffix = v[i:]
		v = v[:i]
	}
	for i, part := range strings.Split(v, ".") {
		if i >= 3 {
			break
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		p.nums[i] = n
		ok = true
	}
	return p, ok
}

// NeedsUpdate dice si latest es más nueva que current. Sin current conocida
// (dev) no hay aviso: un build de desarrollo naggearía siempre.
func NeedsUpdate(current, latest string) bool {
	if current == "" || latest == "" {
		return false
	}
	return Compare(current, latest) < 0
}

// latestResp es lo único que se lee del JSON de /releases/latest.
type latestResp struct {
	Tag string `json:"tag_name"`
}

// CheckLatest pregunta el tag del último release. Devuelve "" sin error cuando
// no se puede saber (sin red, rate limit, JSON raro): el chequeo es tolerante,
// nunca rompe el arranque ni el comando. El error solo sale cuando el servidor
// responde algo afirmativamente roto.
func CheckLatest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/releases/latest", nil)
	if err != nil {
		return "", nil
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return "", nil // rate limit: silencio, no error
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("releases: HTTP %d", resp.StatusCode)
	}
	var lr latestResp
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return "", nil
	}
	return strings.TrimSpace(lr.Tag), nil
}

// assetName mapea la plataforma actual al nombre del asset del release:
// tcode-<os>-<arch>[.exe], el mismo que publican release.yml e install.sh.
func assetName() (string, error) {
	goos, arch := runtime.GOOS, runtime.GOARCH
	switch goos {
	case "linux", "darwin", "windows":
	default:
		return "", fmt.Errorf("plataforma sin binario precompilado: %s/%s", goos, arch)
	}
	name := "tcode-" + goos + "-" + arch
	if goos == "windows" {
		name += ".exe"
	}
	return name, nil
}

// Update descarga el asset del release tag, verifica su sha256 contra
// checksums.txt y reemplaza dest con él (modo 0755). Es descarga + reemplazo
// atómico por rename: un corte a mitad no deja un binario partido. Devuelve
// el tag instalado.
func Update(ctx context.Context, tag, dest string) error {
	name, err := assetName()
	if err != nil {
		return err
	}
	bin, err := download(ctx, downloadBase+"/"+name)
	if err != nil {
		return err
	}
	sums, err := download(ctx, downloadBase+"/checksums.txt")
	if err != nil {
		return err
	}
	want, err := checksumFor(sums, name)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(bin)
	if hex.EncodeToString(sum[:]) != want {
		return fmt.Errorf("sha256 de %s no coincide con checksums.txt", name)
	}
	tmp := dest + ".tcode-update"
	if err := os.WriteFile(tmp, bin, 0o755); err != nil {
		return fmt.Errorf("escribiendo %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("reemplazando %s: %w", dest, err)
	}
	return nil
}

// download baja una URL entera a memoria con el ctx del llamado.
func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("descargando %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("descargando %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// checksumFor extrae de un checksums.txt (formato "sha  archivo") el hash del
// asset pedido.
func checksumFor(sums []byte, name string) (string, error) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && (fields[1] == name || filepath.Base(fields[1]) == name) {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt no trae %s", name)
}
