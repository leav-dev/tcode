package controller

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leav-dev/tcode/internal/ext"
)

// Cotas del proveedor de archivos para scripts (directriz de memoria del
// proyecto: coso mínimo): 64 archivos y 2 MiB totales por pedido.
const (
	maxDirFiles = 64
	maxDirBytes = 2 << 20
	// maxReadFileBytes es la cota por archivo de la lectura puntual
	// (tcode.read_file): un solo archivo, mismo techo que el pedido total.
	maxReadFileBytes = 2 << 20
)

// readFileExts son las extensiones de código que la lectura puntual expone:
// fuentes analizables para validar imports, no binarios ni datos del proyecto.
var readFileExts = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".mjs": true, ".cjs": true, ".py": true, ".pyw": true,
	".go": true, ".html": true,
}

// DirFiles implementa ScriptAPI: los archivos Go del MISMO directorio que el
// buffer activo (el paquete de Go abarca varios archivos). El host Lua no
// tiene io/os —acá el editor es el proveedor de archivos y la extensión la
// analizadora—. Cotas: solo *.go, máx maxDirFiles archivos y maxDirBytes
// totales, se omite el buffer activo (la extensión ya lo tiene). Sin buffer
// activo → error legible; un directorio ilegible → error; los archivos
// individuales que fallan se saltan en silencio.
func (a *App) DirFiles() ([]ext.HostFile, error) {
	buf := a.activeBuffer()
	if buf == nil {
		return nil, errors.New("sin buffer activo")
	}
	dir := filepath.Dir(buf.Path())
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []ext.HostFile
	var total int
	for _, e := range entries {
		if len(files) >= maxDirFiles || total >= maxDirBytes {
			break
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if p == buf.Path() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() > int64(maxDirBytes-total) {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		total += len(b)
		files = append(files, ext.HostFile{Path: p, Content: string(b)})
	}
	return files, nil
}

// ReadFile implementa ScriptAPI: lee UN archivo por ruta relativa al
// directorio del buffer activo. La ruta debe quedar contenida en él:
// absolutos y escapes con .. se rechazan en el plano léxico, y los symlinks
// se resuelven y se re-validan (un link que apunte afuera no sale). Solo
// expone extensiones de código (readFileExts) y archivos regulares de hasta
// maxReadFileBytes; lo demás —inexistente, ilegible, directorio— es error.
// Es E/S local acotada a un archivo: corre en el hilo del hook sin espera
// perceptible, sin goroutine ni proceso aparte.
func (a *App) ReadFile(relpath string) (ext.HostFile, error) {
	buf := a.activeBuffer()
	if buf == nil {
		return ext.HostFile{}, errors.New("sin buffer activo")
	}
	if relpath == "" || filepath.IsAbs(relpath) {
		return ext.HostFile{}, fmt.Errorf("ruta fuera del directorio: %q", relpath)
	}
	dir := filepath.Dir(buf.Path())
	p := filepath.Join(dir, relpath)
	if p != dir && !strings.HasPrefix(p, dir+string(filepath.Separator)) {
		return ext.HostFile{}, fmt.Errorf("ruta fuera del directorio: %q", relpath)
	}
	// El symlink se resuelve y se re-valida: el plano léxico solo ve el
	// texto del link, no su destino.
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return ext.HostFile{}, err
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return ext.HostFile{}, err
	}
	if real != realDir && !strings.HasPrefix(real, realDir+string(filepath.Separator)) {
		return ext.HostFile{}, fmt.Errorf("ruta fuera del directorio: %q", relpath)
	}
	if !readFileExts[strings.ToLower(filepath.Ext(real))] {
		return ext.HostFile{}, fmt.Errorf("extensión no expuesta: %q", relpath)
	}
	info, err := os.Stat(real)
	if err != nil {
		return ext.HostFile{}, err
	}
	if !info.Mode().IsRegular() {
		return ext.HostFile{}, fmt.Errorf("no es un archivo: %q", relpath)
	}
	if info.Size() > int64(maxReadFileBytes) {
		return ext.HostFile{}, fmt.Errorf("el archivo excede la cota de %d bytes: %q", maxReadFileBytes, relpath)
	}
	b, err := os.ReadFile(real)
	if err != nil {
		return ext.HostFile{}, err
	}
	return ext.HostFile{Path: real, Content: string(b)}, nil
}
