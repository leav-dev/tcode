package ext

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// Mods son los modificadores de una tecla en la gramática del manifest. Se
// comparan exactos: "ctrl+k" nunca dispara con Ctrl+Shift.
type Mods uint8

const (
	ModCtrl Mods = 1 << iota
	ModShift
	ModAlt
)

// press es la identidad de una tecla ya resuelta: o bien una runa (letra,
// dígito o espacio, normalizada a minúscula) con sus mods, o bien una tecla
// nombrada de tcell (Enter, F5, Escape…) con sus mods. Es comparable, así que
// sirve de clave de las tablas de resolución.
type press struct {
	isRune bool
	r      rune
	key    tcell.Key
	mods   Mods
}

// chordStep es la segunda mitad de un chord: la tecla que completa el primer
// tiempo y el comando que se ejecuta.
type chordStep struct {
	p   press
	cmd string
}

// Keymap congela los keybindings en tablas de resolución y sostiene la
// máquina de estados del chord pendiente. Un single se ejecuta de inmediato;
// una tecla que abre un chord arma el pendiente sin ejecutar nada; cualquier
// otra tecla cancela el pendiente y se evalúa ella misma como evento nuevo.
// A igual tecla, el single gana sobre ser prefijo de chord (regla documentada).
type Keymap struct {
	singles    map[press]string
	chordFirst map[press][]chordStep
	pending    []chordStep
	hasPending bool
}

// buildKeymap convierte keybindings ya validados en la tabla de resolución.
// Los duplicados mantienen el primero. Entradas que no parsean (no deberían
// existir: el manifest las rechaza) se descartan en silencio.
func buildKeymap(bindings []Keybinding) *Keymap {
	km := &Keymap{
		singles:    make(map[press]string),
		chordFirst: make(map[press][]chordStep),
	}
	for _, b := range bindings {
		ps, err := parseKeybinding(b.Key)
		if err != nil {
			continue
		}
		if len(ps) == 1 {
			if _, ok := km.singles[ps[0]]; !ok {
				km.singles[ps[0]] = b.Command
			}
			continue
		}
		km.chordFirst[ps[0]] = append(km.chordFirst[ps[0]], chordStep{p: ps[1], cmd: b.Command})
	}
	return km
}

// Resolve clasifica un evento de teclado contra la tabla y devuelve el id de
// comando a ejecutar, o "" si la tecla no pertenece a ninguna extensión (o
// quedó como pendiente de chord). Es la única operación mutable: muta el
// pendiente. El controlador la llama después de sus propios atajos, por lo
// que la resolución nunca ve las teclas que el núcleo ya consumió.
func (km *Keymap) Resolve(ev *tcell.EventKey) string {
	p := pressFromEvent(ev)
	if km.hasPending {
		km.hasPending = false
		steps := km.pending
		km.pending = nil
		for _, s := range steps {
			if s.p == p {
				return s.cmd
			}
		}
		// La tecla canceló el chord; cae evaluada como evento nuevo abajo.
	}
	if cmd, ok := km.singles[p]; ok {
		return cmd
	}
	if steps, ok := km.chordFirst[p]; ok {
		km.pending = steps
		km.hasPending = true
		return ""
	}
	return ""
}

// pressFromEvent traduce un evento tcell a la identidad interna. tcell codifica
// las letras en la clave ASCII capitalizada (Key('K')) mientras la runa real
// viaja en Rune(), así que la identidad se decide por la runa: si el evento
// lleva una runa imprimible es una tecla de carácter (normalizada a
// minúscula: la mayúscula llega por Shift, que sigue en sus mods); sin runa,
// es una tecla nombrada (Enter, F5, Escape…).
func pressFromEvent(ev *tcell.EventKey) press {
	var m Mods
	if ev.Modifiers()&tcell.ModCtrl != 0 {
		m |= ModCtrl
	}
	if ev.Modifiers()&tcell.ModShift != 0 {
		m |= ModShift
	}
	if ev.Modifiers()&tcell.ModAlt != 0 {
		m |= ModAlt
	}
	p := press{mods: m}
	if r := ev.Rune(); r != 0 {
		p.isRune = true
		p.r = unicode.ToLower(r)
	} else {
		p.key = ev.Key()
	}
	return p
}

// parseKeybinding valida la gramática Y produce la identidad interna de cada
// tiempo: la gramática vive en un solo lugar.
func parseKeybinding(key string) ([]press, error) {
	if key == "" {
		return nil, errors.New("keybinding: key vacía")
	}
	chords := strings.Split(key, " ")
	if len(chords) > 2 {
		return nil, fmt.Errorf("keybinding: %d tiempos (máximo 2)", len(chords))
	}
	var presses []press
	anyMod := false
	for i, ch := range chords {
		mods, k, err := splitChord(ch)
		if err != nil {
			return nil, fmt.Errorf("keybinding: tiempo %d: %v", i, err)
		}
		p, err := pressForKey(k, mods)
		if err != nil {
			return nil, fmt.Errorf("keybinding: tiempo %d: %v", i, err)
		}
		if len(mods) > 0 {
			anyMod = true
		}
		presses = append(presses, p)
	}
	if len(chords) == 2 && !anyMod {
		return nil, errors.New("keybinding: un chord de dos tiempos necesita al menos un mod")
	}
	return presses, nil
}

var modBits = map[string]Mods{"ctrl": ModCtrl, "shift": ModShift, "alt": ModAlt}

var namedTcell = map[string]tcell.Key{
	"enter": tcell.KeyEnter, "tab": tcell.KeyTab, "escape": tcell.KeyEsc,
	"backspace": tcell.KeyBackspace, "delete": tcell.KeyDelete,
	"insert": tcell.KeyInsert, "home": tcell.KeyHome, "end": tcell.KeyEnd,
	"pageup": tcell.KeyPgUp, "pagedown": tcell.KeyPgDn,
	"up": tcell.KeyUp, "down": tcell.KeyDown, "left": tcell.KeyLeft,
	"right": tcell.KeyRight,
}

// pressForKey convierte una tecla ya separada de sus mods (letra, dígito,
// F-key o nombrada) más la lista de mods a la identidad interna.
func pressForKey(k string, mods []string) (press, error) {
	var m Mods
	for _, mod := range mods {
		m |= modBits[mod]
	}
	if len(k) == 1 && ((k[0] >= 'a' && k[0] <= 'z') || (k[0] >= '0' && k[0] <= '9')) {
		return press{isRune: true, r: rune(k[0]), mods: m}, nil
	}
	if k == "space" {
		return press{isRune: true, r: ' ', mods: m}, nil
	}
	if fKeyRe.MatchString(k) {
		return press{key: fKey(k), mods: m}, nil
	}
	if tk, ok := namedTcell[k]; ok {
		return press{key: tk, mods: m}, nil
	}
	return press{}, fmt.Errorf("tecla inválida %q", k)
}

// fKey mapea "f5" a tcell.KeyF5 (KeyF1..KeyF24 son consecutivos).
func fKey(k string) tcell.Key {
	n := 0
	for _, c := range k[1:] {
		n = n*10 + int(c-'0')
	}
	return tcell.KeyF1 + tcell.Key(n-1)
}
