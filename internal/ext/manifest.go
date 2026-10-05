// Package ext contiene el sistema de extensiones declarativas de tcode, con
// el modelo de VSCode pero nativo: manifest, comandos, keybindings y hooks sin
// ejecutar código. El registro de comandos, el bus de hooks y la activación
// son las costuras donde un futuro backend de scripting (WASM/Lua) se enchufa.
package ext

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Manifest describe una extensión declarativa. El esquema JSON sigue el modelo
// de VSCode recortado a lo que tcode ejecuta hoy: comandos declarados (stubs
// que activan la extensión), keybindings que apuntan a comandos registrados
// (built-ins tcode.* o propios) y hooks evento→comando. activation lista los
// eventos que despiertan la extensión; sin evento declarado, nunca se activa.
type Manifest struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Activation  []string      `json:"activation"`
	Contributes Contributions `json:"contributes"`
}

// Contributions son los puntos de contribución que la extensión declara.
type Contributions struct {
	Commands    []Command    `json:"commands"`
	Keybindings []Keybinding `json:"keybindings"`
	Hooks       []Hook       `json:"hooks"`
}

// Command declara un comando: su id y el título que mostraría una futura
// palette. Con script y fn, la implementación vive en una función Lua del
// script de la extensión (milestone de scripting); sin el par, un comando
// declarado es un stub que al ejecutarse activa la extensión y avisa que
// falta implementación.
type Command struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Script es la ruta del archivo Lua del script de la extensión, relativa
	// al directorio de la extensión (e.Dir); Fn es la función global Lua que
	// implementa el comando. Van obligatoriamente juntas: validate lo exige.
	Script string `json:"script"`
	Fn     string `json:"fn"`
}

// Keybinding re-mapea una tecla (o chord de dos tiempos) a un comando
// registrado. La tecla puede apuntar a un built-in tcode.* sin ningún costo:
// los atajos del núcleo se revisan antes y nunca se pisan.
type Keybinding struct {
	Key     string `json:"key"`
	Command string `json:"command"`
}

// Hook corre un comando cuando ocurre un evento de buffer. Los eventos
// admitidos son los del set del paquete (onDid*Buffer).
type Hook struct {
	Event   string `json:"event"`
	Command string `json:"command"`
}

// Eventos de hook admitidos en el manifest.
const (
	EventDidOpenBuffer  = "onDidOpenBuffer"
	EventDidSaveBuffer  = "onDidSaveBuffer"
	EventDidCloseBuffer = "onDidCloseBuffer"
)

// Eventos de activación admitidos en el manifest.
const (
	ActivateStartup = "onStartup"
	ActivateAlways  = "*"
	ActivateCommand = "onCommand:" // prefijo: ActivateCommand + id
	ActivateOpen    = EventDidOpenBuffer
)

// Load parsea y valida un manifest JSON. Devuelve error nombrando el campo
// roto; un manifest inválido nunca llega al registro. Si falta Name, se
// adopta el ID para que los mensajes de la barra de estado sigan legibles.
func Load(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: JSON inválido: %w", err)
	}
	if m.Name == "" {
		m.Name = m.ID
	}
	if err := validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// validate aplica las reglas del manifest: id y versión obligatorios con
// formato, ids de comando únicos y válidos, eventos de hook en el set
// permitido y keybindings con la gramática mínima parseable.
func validate(m *Manifest) error {
	if err := validateID(m.ID); err != nil {
		return fmt.Errorf("id: %w", err)
	}
	if m.Version == "" {
		return errors.New("version: vacío")
	}
	if !semverRe.MatchString(m.Version) {
		return fmt.Errorf("version: %q no es semver vX.Y.Z", m.Version)
	}

	seen := make(map[string]bool, len(m.Contributes.Commands))
	for i, c := range m.Contributes.Commands {
		if err := validateID(c.ID); err != nil {
			return fmt.Errorf("comando %d: %w", i, err)
		}
		if seen[c.ID] {
			return fmt.Errorf("comando duplicado: %q", c.ID)
		}
		seen[c.ID] = true
		// Script y fn definen la implementación Lua del comando: el par es
		// atómico. Un comando con uno solo quedaría a medio cablear (un
		// script sin función no tiene qué llamar, una función sin script no
		// tiene de dónde cargarse).
		if (c.Script == "") != (c.Fn == "") {
			return fmt.Errorf("comando %s: script y fn van juntos", c.ID)
		}
	}

	for i, h := range m.Contributes.Hooks {
		if !hookEvents[h.Event] {
			return fmt.Errorf("hook %d: evento desconocido %q", i, h.Event)
		}
		if err := validateID(h.Command); err != nil {
			return fmt.Errorf("hook %d: %w", i, err)
		}
	}

	for i, k := range m.Contributes.Keybindings {
		if err := validateBindingKey(k.Key); err != nil {
			return fmt.Errorf("keybinding %d: %w", i, err)
		}
		if err := validateID(k.Command); err != nil {
			return fmt.Errorf("keybinding %d: %w", i, err)
		}
	}
	return nil
}

// validateID exige un id no vacío que arranque alfanumérico y siga con
// alfanuméricos, punto, guion o guion bajo: el convenio publisher.nombre
// entra, y queda fuera cualquier cosa que rompa mensajes o rutas.
func validateID(id string) error {
	if !idRe.MatchString(id) {
		return fmt.Errorf("id inválido %q", id)
	}
	return nil
}

// validateBindingKey valida la gramática de teclas del manifest: uno o dos
// tiempos separados por espacio, cada tiempo con mods (ctrl/shift/alt, sin
// repetir) más una tecla (letra, dígito, F1-F24 o nombrada), y un chord de
// dos tiempos exige al menos un mod: sin eso, "k k" sería ambiguo contra el
// tecleo normal del documento. La gramática vive en parseKeybinding; acá solo
// se descarta el resultado.
func validateBindingKey(key string) error {
	_, err := parseKeybinding(key)
	return err
}

// splitChord separa mods de la tecla y verifica los mods (conocidos y sin
// duplicar). La tecla devuelta es la última parte, ya recortada de mods.
func splitChord(ch string) ([]string, string, error) {
	parts := strings.Split(ch, "+")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return nil, "", errors.New("tecla vacía")
	}
	var mods []string
	seen := map[string]bool{}
	for _, p := range parts[:len(parts)-1] {
		if !validMods[p] {
			return nil, "", fmt.Errorf("modificador desconocido %q", p)
		}
		if seen[p] {
			return nil, "", fmt.Errorf("modificador duplicado %q", p)
		}
		seen[p] = true
		mods = append(mods, p)
	}
	return mods, parts[len(parts)-1], nil
}

var (
	idRe       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	semverRe   = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	fKeyRe     = regexp.MustCompile(`^f([1-9]|1[0-9]|2[0-4])$`)
	validMods  = map[string]bool{"ctrl": true, "shift": true, "alt": true}
	hookEvents = map[string]bool{
		EventDidOpenBuffer:  true,
		EventDidSaveBuffer:  true,
		EventDidCloseBuffer: true,
	}
)
