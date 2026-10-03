package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestSessionRoundtrip: guardar y cargar una sesión devuelve los mismos campos,
// y el JSON usa indentación estable de dos espacios.
func TestSessionRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".tcode", "session.json")
	s := Session{
		Version: 1,
		Root:    dir,
		Tabs:    []string{filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")},
		Active:  filepath.Join(dir, "b.txt"),
	}

	if err := SaveSession(path, s); err != nil {
		t.Fatalf("SaveSession falló: %v", err)
	}
	got, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession falló: %v", err)
	}
	if !reflect.DeepEqual(got, s) {
		t.Fatalf("sesión cargada = %+v, se esperaba %+v", got, s)
	}

	// La indentación es estable: dos espacios, campos en el orden del struct.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer el JSON: %v", err)
	}
	if !strings.HasPrefix(string(raw), "{\n  \"version\":") {
		t.Fatalf("el JSON debe abrir con version indentado a 2 espacios: %q", raw)
	}
}

// TestLoadSessionMissingFileReturnsNotExist: un archivo inexistente devuelve la
// sesión cero con un error os.IsNotExist (el arranque distingue "sin sesión" de
// "sesión corrupta").
func TestLoadSessionMissingFileReturnsNotExist(t *testing.T) {
	s, err := LoadSession(filepath.Join(t.TempDir(), "no-existe.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("error = %v, se esperaba os.IsNotExist", err)
	}
	if s.Version != 0 || s.Root != "" || s.Tabs != nil || s.Active != "" {
		t.Fatalf("sesión = %+v, se esperaba la sesión cero", s)
	}
}

// TestLoadSessionInvalidJSONReturnsError: un JSON inválido devuelve error sin
// pánico: la restauración no puede morir por una sesión vieja.
func TestLoadSessionInvalidJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte("{no es json"), 0o644); err != nil {
		t.Fatalf("no se pudo crear el archivo: %v", err)
	}
	if _, err := LoadSession(path); err == nil {
		t.Fatal("JSON inválido debe devolver error")
	}
}

// TestSaveSessionCreatesNestedDirectory: el directorio anidado de la sesión se
// crea si falta, como el .tcode/ de un root recién usado.
func TestSaveSessionCreatesNestedDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "session.json")
	if err := SaveSession(path, Session{Version: 1, Root: dir}); err != nil {
		t.Fatalf("SaveSession falló: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "b")); err != nil {
		t.Fatalf("el directorio anidado debe crearse: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("el archivo debe existir: %v", err)
	}
}

// TestSessionJSONOnlyHasTheExpectedFields: el archivo persiste solo los campos
// de la sesión (version, root, tabs, active), nunca estado en vivo (cursor,
// viewport, foco, panel).
func TestSessionJSONOnlyHasTheExpectedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := SaveSession(path, Session{Version: 7, Root: "/algo", Tabs: []string{}, Active: "x"}); err != nil {
		t.Fatalf("SaveSession falló: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer el JSON: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("el archivo no es JSON: %v", err)
	}
	for _, k := range []string{"version", "root", "tabs", "active"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("falta el campo %q: %s", k, raw)
		}
	}
	if len(m) != 4 {
		t.Fatalf("campos = %d, se esperaban los 4 de la sesión: %s", len(m), raw)
	}
}
