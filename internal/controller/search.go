package controller

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/leav-dev/tcode/internal/model"
	"github.com/leav-dev/tcode/internal/view"
)

// Límites de la búsqueda en repo: archivos grandes y binarios se saltan,
// y el total de resultados se capa para no ahogar la ventana.
const (
	searchMaxFileBytes = 1 << 20 // 1MB
	searchMaxResults   = 200
)

// findOffsets devuelve los offsets de documento de cada ocurrencia literal
// (case-sensitive) de query, recorriendo línea por línea para no cargar el
// documento entero en memoria (respeta el mmap de la PieceTable).
func findOffsets(buf *model.PieceTable, query string) []int {
	if buf == nil || query == "" {
		return nil
	}
	var out []int
	n := buf.LineCount()
	for line := 0; line < n; line++ {
		content := buf.LineContent(line)
		base := buf.LineStart(line)
		searchFrom := 0
		for {
			idx := strings.Index(string(content[searchFrom:]), query)
			if idx < 0 {
				break
			}
			out = append(out, base+searchFrom+idx)
			searchFrom += idx + len(query)
			if searchFrom >= len(content) {
				break
			}
		}
	}
	return out
}

// nextMatchIndex elige el primer match estrictamente después de cur
// (circular): Enter repetido cicla por el archivo, y al pasar el último
// vuelve al primero (índice 0).
func nextMatchIndex(matches []int, cur, _ int) int {
	if len(matches) == 0 {
		return -1
	}
	for i, off := range matches {
		if off > cur {
			return i
		}
	}
	return 0
}

// searchWorkspace recorre root buscando query como subcadena literal por
// línea. Salta directorios de herramienta (.git, .tcode, node_modules, bin),
// archivos de más de 1MB y archivos con byte NUL (binarios). Devuelve hasta
// 200 resultados ordenados por archivo y línea.
func searchWorkspace(root, query string) []view.RepoMatch {
	if query == "" {
		return nil
	}
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		} else {
			return nil
		}
	}
	var out []view.RepoMatch
	skipDir := map[string]bool{".git": true, ".tcode": true, "node_modules": true, "bin": true}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || len(out) >= searchMaxResults {
			return nil
		}
		if d.IsDir() {
			if path != root && skipDir[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > searchMaxFileBytes {
			return nil
		}
		// Solo archivos regulares: sin esto un FIFO, socket o dispositivo
		// bloquearía el hilo de eventos al leerse (la búsqueda corre
		// sincrónica en el evento de Enter).
		if !info.Mode().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(path)
		// Cota post-lectura: el archivo pudo crecer entre Info y Read.
		if err != nil || len(content) > searchMaxFileBytes || bytes.IndexByte(content, 0) >= 0 {
			return nil
		}
		rel := path
		if r, err := filepath.Rel(root, path); err == nil {
			rel = r
		}
		line := 0
		col := 0
		lineStart := 0
		// Recorrido por bytes con número de línea/columna: cada '\n'
		// cierra la línea actual y la revisamos con Index iterativo.
		flush := func(lineText string, lineNum int) {
			from := 0
			for len(out) < searchMaxResults {
				idx := strings.Index(lineText[from:], query)
				if idx < 0 {
					return
				}
				text := strings.TrimRight(lineText, "\r")
				if len(text) > 120 {
					text = text[:120]
				}
				out = append(out, view.RepoMatch{
					Path: rel,
					Line: lineNum,
					Col:  from + idx,
					Text: text,
				})
				from += idx + len(query)
				if from >= len(lineText) {
					return
				}
			}
		}
		for i := 0; i < len(content); i++ {
			if content[i] == '\n' {
				flush(string(content[lineStart:i]), line)
				line++
				lineStart = i + 1
				col = 0
				continue
			}
			col++
		}
		if lineStart < len(content) {
			flush(string(content[lineStart:]), line)
		} else if len(content) > 0 && content[len(content)-1] == '\n' {
			// Termina en newline: no hay línea extra.
		}
		_ = col
		return nil
	})
	return out
}

// startFind abre el pedido de Ctrl+F sobre el buffer activo. Sin buffer no
// hace nada: con el workspace vacío no hay dónde buscar.
func (a *App) startFind() {
	if a.activeBuffer() == nil {
		return
	}
	a.searchActive = true
	a.searchBuf = ""
	a.searchMatches = nil
	a.searchIdx = 0
	a.refreshSearchPrompt()
	a.redraw()
}

func (a *App) refreshSearchPrompt() {
	a.statusBar.SetPrompt("Buscar: " + a.searchBuf)
}

func (a *App) endSearch() {
	a.searchActive = false
	a.searchBuf = ""
	a.searchMatches = nil
	a.searchIdx = 0
	a.statusBar.SetPrompt("")
}

// handleSearchKey alimenta el pedido de Ctrl+F: tipear edita la query,
// Backspace borra por runa, Enter salta al siguiente match (circular) y
// Escape/Ctrl+C cierra descartando.
func (a *App) handleSearchKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		a.endSearch()
		a.showToast("Cancelado", view.ToastInfo)
		a.redraw()
		return
	case tcell.KeyEnter:
		a.stepSearch()
		a.redraw()
		return
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if r := []rune(a.searchBuf); len(r) > 0 {
			a.searchBuf = string(r[:len(r)-1])
		}
		a.refreshSearchPrompt()
		a.redraw()
		return
	case tcell.KeyRune:
		if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) != 0 {
			return
		}
		if r := ev.Rune(); r != 0 && r != '\n' && r != '\t' {
			a.searchBuf += string(r)
		}
		a.refreshSearchPrompt()
		a.redraw()
		return
	}
}

// stepSearch recalcula los matches de la query actual y salta al siguiente
// después del cursor (circular), informando "i/n" en la barra.
func (a *App) stepSearch() {
	buf := a.activeBuffer()
	ed := a.activeEditor()
	if buf == nil || ed == nil {
		a.endSearch()
		return
	}
	if a.searchBuf == "" {
		a.statusBar.SetMessage("Escribí algo para buscar")
		return
	}
	a.searchMatches = findOffsets(buf, a.searchBuf)
	if len(a.searchMatches) == 0 {
		a.statusBar.SetMessage("Sin coincidencias")
		a.refreshSearchPrompt()
		return
	}
	cur := ed.CursorOffset()
	a.searchIdx = nextMatchIndex(a.searchMatches, cur, a.searchIdx)
	ed.MoveCursorToOffset(a.searchMatches[a.searchIdx])
	a.statusBar.SetMessage(fmt.Sprintf("%d/%d", a.searchIdx+1, len(a.searchMatches)))
	a.refreshSearchPrompt()
	a.syncStatus()
}

// startRepoPrompt abre el pedido de Ctrl+Shift+F: la query vive en la barra
// y Enter corre la búsqueda sobre el disco.
func (a *App) startRepoPrompt() {
	a.repoPromptActive = true
	a.repoBuf = ""
	a.statusBar.SetPrompt("Buscar en repo: ")
	a.redraw()
}

func (a *App) handleRepoPromptKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		a.repoPromptActive = false
		a.repoBuf = ""
		a.statusBar.SetPrompt("")
		a.showToast("Cancelado", view.ToastInfo)
		a.redraw()
		return
	case tcell.KeyEnter:
		query := a.repoBuf
		a.repoPromptActive = false
		a.repoBuf = ""
		a.statusBar.SetPrompt("")
		a.runRepoSearch(query)
		a.redraw()
		return
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if r := []rune(a.repoBuf); len(r) > 0 {
			a.repoBuf = string(r[:len(r)-1])
		}
		a.statusBar.SetPrompt("Buscar en repo: " + a.repoBuf)
		a.redraw()
		return
	case tcell.KeyRune:
		if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) != 0 {
			return
		}
		if r := ev.Rune(); r != 0 && r != '\n' && r != '\t' {
			a.repoBuf += string(r)
		}
		a.statusBar.SetPrompt("Buscar en repo: " + a.repoBuf)
		a.redraw()
		return
	}
}

// runRepoSearch ejecuta la búsqueda sobre ws.Root y abre la ventana de
// resultados; sin coincidencias avisa en la barra sin abrir nada.
func (a *App) runRepoSearch(query string) {
	if strings.TrimSpace(query) == "" {
		a.statusBar.SetMessage("Escribí algo para buscar")
		return
	}
	matches := searchWorkspace(a.ws.Root(), query)
	if len(matches) == 0 {
		a.statusBar.SetMessage("Sin coincidencias")
		return
	}
	a.repoResults.SetTheme(a.theme)
	a.repoResults.SetResults(matches, query)
	a.repoResultsActive = true
	x, y, w, h := a.searchRegion()
	_ = x
	_ = y
	a.repoResults.Resize(w, h)
	a.statusBar.SetMessage(fmt.Sprintf("%d coincidencias", len(matches)))
}

// searchRegion devuelve la región flotante centrada de la ventana de
// resultados: el ancho lo pide el contenido (etiqueta más larga + marco +
// aire) topado al editor, y el alto es matches+marco topado al área.
func (a *App) searchRegion() (x, y, w, h int) {
	width, height := a.screen.Size()
	editorW := width - a.explorerColumn()
	w = a.repoResults.DesiredWidth() + 4
	if w < 24 {
		w = 24
	}
	if w > editorW {
		w = editorW
	}
	h = a.repoResults.Len() + 2
	if h < 5 {
		h = 5
	}
	if h > 20 {
		h = 20
	}
	if h > editorHeight(height) {
		h = editorHeight(height)
	}
	x = a.explorerColumn() + (editorW-w)/2
	y = tabBarHeight + (editorHeight(height)-h)/2
	return
}

// jumpToRepoMatch abre el archivo de la coincidencia del cursor y mueve el
// cursor a su línea/columna, cerrando la ventana.
func (a *App) jumpToRepoMatch() {
	rm, ok := a.repoResults.Selected()
	if !ok {
		a.repoResultsActive = false
		return
	}
	a.repoResultsActive = false
	abs := rm.Path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(a.ws.Root(), rm.Path)
	}
	buf, err := a.ws.Open(abs)
	if err != nil {
		a.statusBar.SetMessage(err.Error())
		return
	}
	line := rm.Line
	if line < 0 {
		line = 0
	}
	if line >= buf.LineCount() {
		line = buf.LineCount() - 1
	}
	content := buf.LineContent(line)
	col := rm.Col
	if col < 0 {
		col = 0
	}
	if col > len(content) {
		col = len(content)
	}
	if ed := a.activeEditor(); ed != nil {
		ed.MoveCursorToOffset(buf.LineStart(line) + col)
	}
	a.explorerFocused = false
	a.syncStatus()
}
