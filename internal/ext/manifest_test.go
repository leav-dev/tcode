package ext

import (
	"encoding/json"
	"strings"
	"testing"
)

const validManifest = `{
	"id": "tcode.demosaludo",
	"name": "Demo Saludo",
	"version": "0.1.0",
	"activation": ["onCommand:tcode.demosaludo.saludar", "onDidOpenBuffer"],
	"contributes": {
		"commands": [
			{"id": "tcode.demosaludo.saludar", "title": "Saludar"}
		],
		"keybindings": [
			{"key": "ctrl+k ctrl+g", "command": "tcode.demosaludo.saludar"}
		],
		"hooks": [
			{"event": "onDidSaveBuffer", "command": "tcode.demosaludo.saludar"}
		]
	}
}`

// mustManifest carga src como manifest válido, abortando el test si falla.
func mustManifest(t *testing.T, src string) *Manifest {
	t.Helper()
	m, err := Load([]byte(src))
	if err != nil {
		t.Fatalf("Load del manifest de referencia falló: %v", err)
	}
	return m
}

// mustMarshal serializa m al esquema JSON del manifest.
func mustMarshal(t *testing.T, m *Manifest) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal falló: %v", err)
	}
	return b
}

// TestValidManifestLoads parsea el manifest completo y verifica que cada campo
// llegue intacto: los arrays de activation, los contributes y los títulos.
func TestValidManifestLoads(t *testing.T) {
	m, err := Load([]byte(validManifest))
	if err != nil {
		t.Fatalf("Load del manifest válido falló: %v", err)
	}
	if m.ID != "tcode.demosaludo" {
		t.Errorf("ID = %q, esperaba tcode.demosaludo", m.ID)
	}
	if m.Version != "0.1.0" {
		t.Errorf("Version = %q, esperaba 0.1.0", m.Version)
	}
	if len(m.Activation) != 2 || m.Activation[0] != "onCommand:tcode.demosaludo.saludar" {
		t.Errorf("Activation = %v, esperaba los dos eventos declarados", m.Activation)
	}
	if len(m.Contributes.Commands) != 1 || m.Contributes.Commands[0].Title != "Saludar" {
		t.Errorf("Commands = %+v, esperaba el comando declarado con título", m.Contributes.Commands)
	}
	if len(m.Contributes.Keybindings) != 1 || m.Contributes.Keybindings[0].Key != "ctrl+k ctrl+g" {
		t.Errorf("Keybindings = %+v, esperaba el binding declarado", m.Contributes.Keybindings)
	}
	if len(m.Contributes.Hooks) != 1 || m.Contributes.Hooks[0].Event != "onDidSaveBuffer" {
		t.Errorf("Hooks = %+v, esperaba el hook declarado", m.Contributes.Hooks)
	}
}

// TestLoadRejects sintetiza un manifest con el campo indicado roto y espera que
// Load devuelva error nombrando ese campo.
func TestLoadRejects(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(m *Manifest)
		field string
	}{
		{"id vacío", func(m *Manifest) { m.ID = "" }, "id"},
		{"id con espacios", func(m *Manifest) { m.ID = "tcode mala id" }, "id"},
		{"id que arranca con punto", func(m *Manifest) { m.ID = ".tcode" }, "id"},
		{"versión ausente", func(m *Manifest) { m.Version = "" }, "version"},
		{"versión sin dos puntos", func(m *Manifest) { m.Version = "0.1" }, "version"},
		{"versión con prerelease", func(m *Manifest) { m.Version = "0.1.0-beta" }, "version"},
		{"comando duplicado", func(m *Manifest) {
			m.Contributes.Commands = append(m.Contributes.Commands, Command{ID: "tcode.demosaludo.saludar"})
		}, "comando"},
		{"comando sin id", func(m *Manifest) {
			m.Contributes.Commands = append(m.Contributes.Commands, Command{ID: "  "})
		}, "comando"},
		{"hook evento desconocido", func(m *Manifest) {
			m.Contributes.Hooks = append(m.Contributes.Hooks, Hook{Event: "onDidChangeText"})
		}, "hook"},
		{"hook sin comando", func(m *Manifest) {
			m.Contributes.Hooks = append(m.Contributes.Hooks, Hook{Event: "onDidOpenBuffer", Command: " "})
		}, "hook"},
		{"keybinding key vacía", func(m *Manifest) {
			m.Contributes.Keybindings = append(m.Contributes.Keybindings, Keybinding{Key: ""})
		}, "keybinding"},
		{"keybinding sin comando", func(m *Manifest) {
			m.Contributes.Keybindings = append(m.Contributes.Keybindings, Keybinding{Key: "ctrl+k", Command: ""})
		}, "keybinding"},
		{"script sin fn", func(m *Manifest) {
			m.Contributes.Commands = append(m.Contributes.Commands, Command{ID: "tcode.demosaludo.script1", Script: "main.lua"})
		}, "script"},
		{"fn sin script", func(m *Manifest) {
			m.Contributes.Commands = append(m.Contributes.Commands, Command{ID: "tcode.demosaludo.script2", Fn: "correr"})
		}, "script"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := mustManifest(t, validManifest)
			tc.mut(m)
			_, err := Load(mustMarshal(t, m))
			if err == nil {
				t.Fatalf("Load aceptó un manifest con %s roto", tc.field)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.field) {
				t.Errorf("error = %q, debería nombrar el campo %q", err, tc.field)
			}
		})
	}
}

// TestManifestJSONRoundTrip verifica que un manifest cargado se vuelva a
// serializar sin perder campos: el JSON de ida y vuelta es el esquema real.
func TestManifestJSONRoundTrip(t *testing.T) {
	m := mustManifest(t, validManifest)
	got := mustMarshal(t, m)
	again, err := Load(got)
	if err != nil {
		t.Fatalf("Load del manifest re-serializado falló: %v", err)
	}
	if again.ID != m.ID || again.Version != m.Version || len(again.Contributes.Commands) != 1 {
		t.Errorf("el round trip perdió campos: %+v", again)
	}
}

// TestParseKeybindingRejects cubre la gramática mínima de teclas que el
// manifest acepta; el parser completo y la resolución son otra unidad.
func TestParseKeybindingRejects(t *testing.T) {
	cases := []string{
		"", "ctrl", "ctrl+", "+k", "f0", "f25", "ctrl+k ctrl+g ctrl+h",
		"shift+shift+k", "ctr+k", "k k", "ctrl+K", "tab+enter", "ctrl+",
	}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			if err := validateBindingKey(key); err == nil {
				t.Errorf("validateBindingKey(%q) no devolvió error", key)
			}
		})
	}
}

// TestParseKeybindingAccepts fija la gramática válida: mods combinables, una
// tecla por tiempo, y chords de hasta dos tiempos.
func TestParseKeybindingAccepts(t *testing.T) {
	cases := []string{
		"k", "ctrl+k", "ctrl+shift+k", "alt+shift+f5", "shift+enter",
		"f5", "pageup", "ctrl+k ctrl+g", "ctrl+shift+k ctrl+alt+g",
	}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			if err := validateBindingKey(key); err != nil {
				t.Errorf("validateBindingKey(%q) falló: %v", key, err)
			}
		})
	}
}

// TestCommandScriptPairLoads: script+fn juntos son válidos y quedan expuestos
// en el Command cargado; el manifest de scripting declara la implementación
// del comando sin escribirla en el JSON del manifest.
func TestCommandScriptPairLoads(t *testing.T) {
	m := mustManifest(t, `{
		"id": "tcode.scriptdemo",
		"name": "Script Demo",
		"version": "1.0.0",
		"contributes": {
			"commands": [{"id": "tcode.scriptdemo.correr", "script": "main.lua", "fn": "correr"}]
		}
	}`)
	c := m.Contributes.Commands[0]
	if c.Script != "main.lua" || c.Fn != "correr" {
		t.Errorf("Script/Fn = %q/%q, esperaba main.lua/correr", c.Script, c.Fn)
	}
}
