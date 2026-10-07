package controller

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/leav-dev/tcode/internal/ext"
)

// GitStatus implementa ScriptAPI: información de git del directorio del buffer
// activo. El host Lua no tiene io/os —acá el editor ejecuta git y devuelve
// datos estructurados—. Sin buffer activo → error legible; sin repo git →
// error claro; git no disponible → error descriptivo.
func (a *App) GitStatus() (ext.GitInfo, error) {
	buf := a.activeBuffer()
	if buf == nil {
		return ext.GitInfo{}, fmt.Errorf("sin buffer activo")
	}

	dir := filepath.Dir(buf.Path())

	// Verificar que estamos en un repo git
	if err := checkGitRepo(dir); err != nil {
		return ext.GitInfo{}, err
	}

	info := ext.GitInfo{
		StagedFiles:    []string{},
		UnstagedFiles:  []string{},
		UntrackedFiles: []string{},
	}

	// Obtener archivos con cambios (staged, unstaged, untracked)
	if err := a.getChangedFiles(dir, &info); err != nil {
		return ext.GitInfo{}, err
	}

	// Obtener líneas agregadas/borradas
	if err := a.getLineCounts(dir, &info); err != nil {
		return ext.GitInfo{}, err
	}

	// Rama actual: un solo spawn barato (--show-current); en detached HEAD
	// cae al SHA corto. Si no se puede determinar queda vacía y el llamador
	// degrada sin romper.
	info.Branch = getBranch(dir)

	// Último commit: un solo `git log -1`, solo informativo. Sin commits o
	// con git roto queda vacío y el llamador degrada sin romper.
	info.CommitHash, info.CommitSubject, info.CommitAuthor, info.CommitDate = getLastCommit(dir)

	return info, nil
}

// getBranch devuelve la rama actual del repo que contiene a dir: el nombre
// de `git branch --show-current`, el SHA corto de `git rev-parse --short
// HEAD` en detached HEAD, o "" si git no lo puede determinar (repo sin
// commits, git ausente). Nunca devuelve error: la rama es informativa.
func getBranch(dir string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		if branch := strings.TrimSpace(string(out)); branch != "" {
			return branch
		}
	}
	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		if sha := strings.TrimSpace(string(out)); sha != "" {
			return sha
		}
	}
	return ""
}

// getLastCommit devuelve (hash corto, subject, autor, fecha corta) del
// último commit del repo que contiene a dir, en un solo spawn de
// `git log -1`. Cualquier fallo (repo sin commits, git ausente) devuelve
// strings vacías: el commit es informativo y nunca un error.
func getLastCommit(dir string) (hash, subject, author, date string) {
	cmd := exec.Command("git", "log", "-1",
		"--format=%h%x1f%s%x1f%an%x1f%ad", "--date=short")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", ""
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "\x1f")
	if len(parts) != 4 {
		return "", "", "", ""
	}
	return parts[0], parts[1], parts[2], parts[3]
}

// checkGitRepo verifica que el directorio está en un repo git
func checkGitRepo(dir string) error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("no es un repositorio git")
	}
	return nil
}

// getChangedFiles obtiene la lista de archivos con cambios
func (a *App) getChangedFiles(dir string, info *ext.GitInfo) error {
	// git status --porcelain da formato: XY filename
	// X = staged, Y = unstaged, ? = untracked
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git status falló: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			continue
		}

		status := line[:2]
		filename := line[3:]

		// Manejar archivos con espacios (git los cita)
		if strings.HasPrefix(filename, "\"") {
			filename = strings.Trim(filename, "\"")
		}

		// X = staged, Y = working tree, ? = untracked
		if status == "??" {
			info.UntrackedFiles = append(info.UntrackedFiles, filename)
		} else {
			// Staged changes (X no es espacio ni ?)
			if status[0] != ' ' && status[0] != '?' {
				info.StagedFiles = append(info.StagedFiles, filename)
			}
			// Unstaged changes (Y no es espacio)
			if status[1] != ' ' {
				info.UnstagedFiles = append(info.UnstagedFiles, filename)
			}
		}
	}

	return scanner.Err()
}

// getLineCounts obtiene la cantidad de líneas agregadas y borradas
func (a *App) getLineCounts(dir string, info *ext.GitInfo) error {
	// Unstaged changes
	if err := a.getDiffNumstat(dir, false, info); err != nil {
		return err
	}
	// Staged changes
	if err := a.getDiffNumstat(dir, true, info); err != nil {
		return err
	}
	return nil
}

// getDiffNumstat ejecuta git diff --numstat y suma las líneas
func (a *App) getDiffNumstat(dir string, staged bool, info *ext.GitInfo) error {
	args := []string{"diff", "--numstat"}
	if staged {
		args = append(args, "--cached")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git diff falló: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Formato: added\tdeleted\tfilename
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		added, err := strconv.Atoi(parts[0])
		if err != nil {
			// Puede ser "0" o "-" para archivos binarios
			if parts[0] == "-" {
				added = 0
			} else {
				continue
			}
		}

		deleted, err := strconv.Atoi(parts[1])
		if err != nil {
			if parts[1] == "-" {
				deleted = 0
			} else {
				continue
			}
		}

		info.AddedLines += added
		info.DeletedLines += deleted
	}

	return scanner.Err()
}

// GetFileDiff obtiene el diff de un archivo específico con formato unificado
// para marcar líneas individuales. Retorna un slice de FileDiffLine.
func (a *App) GetFileDiff(path string, staged bool) ([]ext.FileDiffLine, error) {
	dir := filepath.Dir(path)

	args := []string{"diff", "-U0"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", path)

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff falló: %w", err)
	}

	return parseUnifiedDiff(string(out)), nil
}

// parseUnifiedDiff parsea un diff con formato unificado (-U0) y extrae
// las líneas con cambios. Retorna el número de línea (1-indexado) y el tipo.
func parseUnifiedDiff(diff string) []ext.FileDiffLine {
	var lines []ext.FileDiffLine
	scanner := bufio.NewScanner(strings.NewReader(diff))

	var newLineNum int
	inHunk := false

	for scanner.Scan() {
		line := scanner.Text()

		// Header de hunk: @@ -old,count +new,count @@
		if strings.HasPrefix(line, "@@") {
			// Parse new line number
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				newPart := parts[2] // +new,count
				if strings.HasPrefix(newPart, "+") {
					newPart = newPart[1:]
					if comma := strings.Index(newPart, ","); comma != -1 {
						newPart = newPart[:comma]
					}
					if num, err := strconv.Atoi(newPart); err == nil {
						newLineNum = num
						inHunk = true
					}
				}
			}
			continue
		}

		if !inHunk {
			continue
		}

		// Líneas del diff
		if strings.HasPrefix(line, "+") {
			// Línea agregada
			lines = append(lines, ext.FileDiffLine{
				Line: newLineNum,
				Type: "added",
			})
			newLineNum++
		} else if strings.HasPrefix(line, "-") {
			// Línea borrada (no incrementa newLineNum)
			lines = append(lines, ext.FileDiffLine{
				Line: newLineNum,
				Type: "deleted",
			})
		} else {
			// Contexto (espacio)
			newLineNum++
		}
	}

	return lines
}
