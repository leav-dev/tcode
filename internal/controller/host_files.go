package controller

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"tcode/internal/ext"
)

// Cotas del proveedor de archivos para scripts (directriz de memoria del
// proyecto: coso mínimo): 64 archivos y 2 MiB totales por pedido.
const (
	maxDirFiles = 64
	maxDirBytes = 2 << 20
)

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