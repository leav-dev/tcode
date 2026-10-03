package ext

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func evKey(key tcell.Key, r rune, mods tcell.ModMask) *tcell.EventKey {
	return tcell.NewEventKey(key, r, mods)
}

// TestParseKeybindingProducesPress fija la conversión gramática → tecla
// interna: mods combinados, F-keys, teclas nombradas y la runa del espacio.
func TestParseKeybindingProducesPress(t *testing.T) {
	cases := []struct {
		key  string
		want press
	}{
		{"ctrl+k", press{isRune: true, r: 'k', mods: ModCtrl}},
		{"k", press{isRune: true, r: 'k'}},
		{"alt+shift+f5", press{key: tcell.KeyF5, mods: ModAlt | ModShift}},
		{"escape", press{key: tcell.KeyEsc}},
		{"space", press{isRune: true, r: ' '}},
		{"pageup", press{key: tcell.KeyPgUp}},
		{"shift+enter", press{key: tcell.KeyEnter, mods: ModShift}},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			got, err := parseKeybinding(tc.key)
			if err != nil {
				t.Fatalf("parseKeybinding(%q) falló: %v", tc.key, err)
			}
			if len(got) != 1 {
				t.Fatalf("parseKeybinding(%q) devolvió %d tiempos, esperaba 1", tc.key, len(got))
			}
			if got[0] != tc.want {
				t.Errorf("parseKeybinding(%q) = %+v, esperaba %+v", tc.key, got[0], tc.want)
			}
		})
	}
}

// TestParseKeybindingChord devuelve los dos tiempos del chord en orden.
func TestParseKeybindingChord(t *testing.T) {
	got, err := parseKeybinding("ctrl+k ctrl+g")
	if err != nil {
		t.Fatalf("parseKeybinding falló: %v", err)
	}
	if len(got) != 2 || got[0] != (press{isRune: true, r: 'k', mods: ModCtrl}) || got[1] != (press{isRune: true, r: 'g', mods: ModCtrl}) {
		t.Errorf("chord = %+v, esperaba ctrl+k y ctrl+g", got)
	}
}

// TestKeymapResolveSingle: una tecla simple con mods exactos ejecuta su comando.
// La runa del evento se normaliza a minúscula, pero los mods se comparan
// exactos: ctrl+shift+k nunca dispara un binding ctrl+k.
func TestKeymapResolveSingle(t *testing.T) {
	km := buildKeymap([]Keybinding{{Key: "ctrl+k", Command: "a"}, {Key: "ctrl+shift+k", Command: "b"}})
	if got := km.Resolve(evKey(tcell.KeyRune, 'k', tcell.ModCtrl)); got != "a" {
		t.Errorf("ctrl+k = %q, esperaba a", got)
	}
	if got := km.Resolve(evKey(tcell.KeyRune, 'K', tcell.ModCtrl)); got != "a" {
		t.Errorf("ctrl+K normalizado = %q, esperaba a", got)
	}
	if got := km.Resolve(evKey(tcell.KeyRune, 'K', tcell.ModCtrl|tcell.ModShift)); got != "b" {
		t.Errorf("ctrl+shift+K = %q, esperaba b", got)
	}
	if got := km.Resolve(evKey(tcell.KeyRune, 'k', tcell.ModNone)); got != "" {
		t.Errorf("k suelta = %q, esperaba nada", got)
	}
}

// TestKeymapResolveChord: la primera tecla arma el pendiente sin ejecutar, y la
// segunda completa el comando.
func TestKeymapResolveChord(t *testing.T) {
	km := buildKeymap([]Keybinding{{Key: "ctrl+k ctrl+g", Command: "saludar"}})
	if got := km.Resolve(evKey(tcell.KeyRune, 'k', tcell.ModCtrl)); got != "" {
		t.Fatalf("primera tecla = %q, esperaba nada (pendiente)", got)
	}
	if got := km.Resolve(evKey(tcell.KeyRune, 'g', tcell.ModCtrl)); got != "saludar" {
		t.Fatalf("segunda tecla = %q, esperaba saludar", got)
	}
	if km.hasPending {
		t.Error("el pendiente no se limpió tras completar el chord")
	}
}

// TestKeymapResolveChordCancelled: cualquier otra tecla cancela el pendiente, y
// ese mismo evento luego se evalúa contra singles y chords nuevos.
func TestKeymapResolveChordCancelled(t *testing.T) {
	km := buildKeymap([]Keybinding{{Key: "ctrl+k ctrl+g", Command: "saludar"}, {Key: "ctrl+s", Command: "guardar"}})
	km.Resolve(evKey(tcell.KeyRune, 'k', tcell.ModCtrl))
	if !km.hasPending {
		t.Fatal("no quedó pendiente tras la primera tecla")
	}
	if got := km.Resolve(evKey(tcell.KeyRune, 's', tcell.ModCtrl)); got != "guardar" {
		t.Errorf("tecla canceladora = %q, esperaba guardar (el evento nuevo se evalúa)", got)
	}
	if km.hasPending {
		t.Error("el pendiente siguió vivo tras la tecla canceladora")
	}
	// El chord vuelve a armarse desde cero con la misma secuencia.
	km.Resolve(evKey(tcell.KeyRune, 'k', tcell.ModCtrl))
	km.Resolve(evKey(tcell.KeyRune, 'x', tcell.ModNone))
	if km.hasPending {
		t.Error("el pendiente sobrevivió a una tecla sin match")
	}
}

// TestKeymapSingleWinsOverChordPrefix: si la primera tecla del chord también es
// un single, el single gana y se ejecuta de inmediato — regla documentada.
func TestKeymapSingleWinsOverChordPrefix(t *testing.T) {
	km := buildKeymap([]Keybinding{{Key: "ctrl+k", Command: "single"}, {Key: "ctrl+k ctrl+g", Command: "chord"}})
	got := km.Resolve(evKey(tcell.KeyRune, 'k', tcell.ModCtrl))
	if got != "single" {
		t.Errorf("ctrl+k = %q, esperaba single (nunca pendiente)", got)
	}
	if km.hasPending {
		t.Error("single no deja pendiente")
	}
}

// TestKeymapFirstDuplicateWins: la tabla de resolution es determinista — un
// keybinding repetido mantiene el primero.
func TestKeymapFirstDuplicateWins(t *testing.T) {
	km := buildKeymap([]Keybinding{{Key: "ctrl+k", Command: "primero"}, {Key: "ctrl+k", Command: "segundo"}})
	if got := km.Resolve(evKey(tcell.KeyRune, 'k', tcell.ModCtrl)); got != "primero" {
		t.Errorf("ctrl+k = %q, esperaba primero", got)
	}
}
