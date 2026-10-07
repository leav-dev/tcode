package controller

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// setupGitRepo crea un repo git temporal con un archivo inicial commiteado.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "testrepo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("no se pudo crear dir: %v", err)
	}

	// Inicializar repo git
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init falló: %v\n%s", err, out)
	}

	// Configurar user para commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()

	// Crear archivo inicial
	initialContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte(initialContent), 0644); err != nil {
		t.Fatalf("no se pudo escribir archivo: %v", err)
	}

	// Commit inicial
	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()

	return repoDir
}

// TestGitStatus verifica que GitStatus devuelva información correcta de git.
func TestGitStatus(t *testing.T) {
	repoDir := setupGitRepo(t)

	// Modificar archivo (2 líneas agregadas, 1 borrada)
	modifiedContent := "line1\nline2 modified\nline3\nline4\nline5\n"
	if err := os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("no se pudo modificar archivo: %v", err)
	}

	// Crear archivo nuevo untracked
	if err := os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("new file\n"), 0644); err != nil {
		t.Fatalf("no se pudo crear archivo nuevo: %v", err)
	}

	// Crear App con pantalla simulada
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar pantalla: %v", err)
	}
	s.SetSize(80, 24)
	defer s.Fini()

	app, err := NewAppWithScreen(s, filepath.Join(repoDir, "test.txt"))
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	defer app.ws.CloseAll()

	// Probar GitStatus
	info, err := app.GitStatus()
	if err != nil {
		t.Fatalf("GitStatus falló: %v", err)
	}

	// Verificar archivos untracked
	if len(info.UntrackedFiles) != 1 {
		t.Errorf("UntrackedFiles: esperaba 1, obtuvo %d", len(info.UntrackedFiles))
	}

	// Verificar archivos unstaged
	if len(info.UnstagedFiles) != 1 {
		t.Errorf("UnstagedFiles: esperaba 1, obtuvo %d", len(info.UnstagedFiles))
	}

	// Verificar líneas (3 agregadas, 1 borrada)
	// línea 2 modificada = 1 borrado + 1 agregado, líneas 4-5 = 2 agregados
	if info.AddedLines != 3 {
		t.Errorf("AddedLines: esperaba 3, obtuvo %d", info.AddedLines)
	}
	if info.DeletedLines != 1 {
		t.Errorf("DeletedLines: esperaba 1, obtuvo %d", info.DeletedLines)
	}
}

// TestGitStatusBranch verifica que GitStatus informe la rama actual, y el
// SHA corto en detached HEAD.
func TestGitStatusBranch(t *testing.T) {
	repoDir := setupGitRepo(t)

	newApp := func() *App {
		t.Helper()
		s := tcell.NewSimulationScreen("UTF-8")
		if err := s.Init(); err != nil {
			t.Fatalf("no se pudo inicializar pantalla: %v", err)
		}
		s.SetSize(80, 24)
		t.Cleanup(s.Fini)
		app, err := NewAppWithScreen(s, filepath.Join(repoDir, "test.txt"))
		if err != nil {
			t.Fatalf("NewAppWithScreen falló: %v", err)
		}
		t.Cleanup(app.ws.CloseAll)
		return app
	}
	gitOut := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v falló: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}

	// Rama normal: debe coincidir con git branch --show-current.
	want := gitOut("branch", "--show-current")
	if want == "" {
		t.Fatal("el repo de prueba quedó sin rama actual")
	}
	info, err := newApp().GitStatus()
	if err != nil {
		t.Fatalf("GitStatus falló: %v", err)
	}
	if info.Branch != want {
		t.Errorf("Branch: esperaba %q, obtuvo %q", want, info.Branch)
	}

	// Detached HEAD: debe caer al SHA corto.
	cmd := exec.Command("git", "checkout", "--detach", "HEAD")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout --detach falló: %v\n%s", err, out)
	}
	wantSHA := gitOut("rev-parse", "--short", "HEAD")
	info, err = newApp().GitStatus()
	if err != nil {
		t.Fatalf("GitStatus en detached falló: %v", err)
	}
	if info.Branch != wantSHA {
		t.Errorf("Branch en detached: esperaba %q, obtuvo %q", wantSHA, info.Branch)
	}
}

// TestGitStatusCommit verifica que GitStatus informe el último commit
// (hash corto, subject, autor, fecha). Compara contra git directo para no
// atarse al nombre de rama ni al formato local.
func TestGitStatusCommit(t *testing.T) {
	repoDir := setupGitRepo(t)

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar pantalla: %v", err)
	}
	s.SetSize(80, 24)
	t.Cleanup(s.Fini)
	app, err := NewAppWithScreen(s, filepath.Join(repoDir, "test.txt"))
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	t.Cleanup(app.ws.CloseAll)

	gitOut := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v falló: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}

	info, err := app.GitStatus()
	if err != nil {
		t.Fatalf("GitStatus falló: %v", err)
	}
	if want := gitOut("rev-parse", "--short", "HEAD"); info.CommitHash != want {
		t.Errorf("CommitHash: esperaba %q, obtuvo %q", want, info.CommitHash)
	}
	if want := gitOut("log", "-1", "--format=%s"); info.CommitSubject != want {
		t.Errorf("CommitSubject: esperaba %q, obtuvo %q", want, info.CommitSubject)
	}
	if info.CommitAuthor == "" {
		t.Error("CommitAuthor: esperaba no vacío")
	}
	if info.CommitDate == "" {
		t.Error("CommitDate: esperaba no vacío")
	}
}

// TestGetFileDiff verifica que GetFileDiff devuelva líneas con cambios.
func TestGetFileDiff(t *testing.T) {
	repoDir := setupGitRepo(t)

	// Modificar archivo
	modifiedContent := "line1\nline2 modified\nline3\nline4\n"
	if err := os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("no se pudo modificar archivo: %v", err)
	}

	// Crear App con pantalla simulada
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar pantalla: %v", err)
	}
	s.SetSize(80, 24)
	defer s.Fini()

	app, err := NewAppWithScreen(s, filepath.Join(repoDir, "test.txt"))
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	defer app.ws.CloseAll()

	// Probar GetFileDiff
	lines, err := app.GetFileDiff(filepath.Join(repoDir, "test.txt"), false)
	if err != nil {
		t.Fatalf("GetFileDiff falló: %v", err)
	}

	// Debería haber al menos una línea agregada (line4)
	hasAdded := false
	for _, l := range lines {
		if l.Type == "added" && l.Line == 4 {
			hasAdded = true
			break
		}
	}

	if !hasAdded {
		t.Errorf("GetFileDiff: no se encontró línea agregada en línea 4, líneas: %v", lines)
	}
}

// TestGitStatusStaged verifica que GitStatus detecte cambios staged.
func TestGitStatusStaged(t *testing.T) {
	repoDir := setupGitRepo(t)

	// Modificar archivo y stagear
	modifiedContent := "line1\nline2 modified\nline3\n"
	if err := os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("no se pudo modificar archivo: %v", err)
	}

	cmd := exec.Command("git", "add", "test.txt")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add falló: %v\n%s", err, out)
	}

	// Crear App con pantalla simulada
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("no se pudo inicializar pantalla: %v", err)
	}
	s.SetSize(80, 24)
	defer s.Fini()

	app, err := NewAppWithScreen(s, filepath.Join(repoDir, "test.txt"))
	if err != nil {
		t.Fatalf("NewAppWithScreen falló: %v", err)
	}
	defer app.ws.CloseAll()

	// Probar GitStatus
	info, err := app.GitStatus()
	if err != nil {
		t.Fatalf("GitStatus falló: %v", err)
	}

	// Verificar archivos staged
	if len(info.StagedFiles) != 1 {
		t.Errorf("StagedFiles: esperaba 1, obtuvo %d", len(info.StagedFiles))
	}

	// No debería haber unstaged
	if len(info.UnstagedFiles) != 0 {
		t.Errorf("UnstagedFiles: esperaba 0, obtuvo %d", len(info.UnstagedFiles))
	}
}
