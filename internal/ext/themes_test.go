package ext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestThemesLoad(t *testing.T) {
	m := mustManifest(t, `{
		"id": "demo.temas",
		"version": "1.0.0",
		"contributes": {
			"themes": [
				{"id": "demo.rosa", "label": "Rosa", "file": "themes/rosa.json"}
			]
		}
	}`)
	if len(m.Contributes.Themes) != 1 {
		t.Fatalf("themes = %+v, esperaba 1", m.Contributes.Themes)
	}
	got := m.Contributes.Themes[0]
	if got.ID != "demo.rosa" || got.Label != "Rosa" || got.File != "themes/rosa.json" {
		t.Fatalf("theme = %+v, inesperado", got)
	}
}

func TestManifestThemesReject(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"id inválido", `{"id":"demo.t","version":"1.0.0","contributes":{"themes":[{"id":"!!","label":"X","file":"a.json"}]}}`},
		{"label vacío", `{"id":"demo.t","version":"1.0.0","contributes":{"themes":[{"id":"demo.x","label":"  ","file":"a.json"}]}}`},
		{"file sin json", `{"id":"demo.t","version":"1.0.0","contributes":{"themes":[{"id":"demo.x","label":"X","file":"a.txt"}]}}`},
		{"file absoluto", `{"id":"demo.t","version":"1.0.0","contributes":{"themes":[{"id":"demo.x","label":"X","file":"/tmp/a.json"}]}}`},
		{"file escape", `{"id":"demo.t","version":"1.0.0","contributes":{"themes":[{"id":"demo.x","label":"X","file":"../a.json"}]}}`},
		{"duplicado", `{"id":"demo.t","version":"1.0.0","contributes":{"themes":[{"id":"demo.x","label":"X","file":"a.json"},{"id":"demo.x","label":"Y","file":"b.json"}]}}`},
	}
	for _, c := range cases {
		if _, err := Load([]byte(c.json)); err == nil {
			t.Errorf("%s: se esperaba error", c.name)
		}
	}
}

func TestLoadThemesReadsFiles(t *testing.T) {
	dir := t.TempDir()
	thDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(thDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(thDir, "rosa.json"), []byte(`{"keyword":"#ff79c6"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load([]byte(`{"id":"demo.t","version":"1.0.0","contributes":{"themes":[{"id":"demo.rosa","label":"Rosa","file":"themes/rosa.json"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	got, errs := LoadThemes([]Extension{{Manifest: m, Dir: dir}})
	if len(errs) != 0 {
		t.Fatalf("errs = %v, esperaba ninguno", errs)
	}
	if len(got) != 1 || got[0].ID != "demo.rosa" || got[0].From != "demo.t" {
		t.Fatalf("temas = %+v, inesperado", got)
	}
}

func TestLoadThemesSkipsBroken(t *testing.T) {
	dir := t.TempDir()
	m, err := Load([]byte(`{"id":"demo.t","version":"1.0.0","contributes":{"themes":[{"id":"demo.roto","label":"Roto","file":"noexiste.json"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	got, errs := LoadThemes([]Extension{{Manifest: m, Dir: dir}})
	if len(got) != 0 {
		t.Fatalf("temas = %+v, esperaba ninguno", got)
	}
	if len(errs) != 1 {
		t.Fatalf("errs = %v, esperaba 1", errs)
	}
}
