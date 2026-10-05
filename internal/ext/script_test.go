package ext

import (
	"errors"
	"strings"
	"testing"
)

// fakeAPI implementa ScriptAPI registrando las llamadas, para que los tests
// comprueben el puente script → editor sin tocar un controlador real.
type fakeAPI struct {
	cmds      []string
	msgs      []string
	inserted  []string
	path      string
	content   string
	bufOK     bool
	insertErr error
	runErr    error
}

func (f *fakeAPI) RunCommand(id string) error {
	f.cmds = append(f.cmds, id)
	return f.runErr
}

func (f *fakeAPI) ActiveBuffer() (string, string, bool) {
	return f.path, f.content, f.bufOK
}

func (f *fakeAPI) InsertAtCursor(text string) error {
	f.inserted = append(f.inserted, text)
	return f.insertErr
}

func (f *fakeAPI) StatusMessage(msg string) { f.msgs = append(f.msgs, msg) }

// TestScriptHostCallsFunction: la función global llama a la API tcode.* y el
// host la enruta al editor. tcode.message y tcode.buffer (con ok=false: sin
// buffer activo, la función Lua simplemente recibe nil) corren limpio, y el
// error de tcode.command se propaga como error del Call.
func TestScriptHostCallsFunction(t *testing.T) {
	api := &fakeAPI{runErr: errors.New("comando roto")}
	h, err := NewScriptHost(`
		function main()
			tcode.message("hola desde lua")
			local path, content = tcode.buffer()
			tcode.command("tcode.otro")
		end
	`, api)
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	err = h.Call("main")
	if err == nil {
		t.Fatal("Call no propagó el error del comando invocado")
	}
	if !strings.Contains(err.Error(), "comando roto") {
		t.Errorf("error = %v, debería mencionar el error del comando", err)
	}
	if len(api.msgs) != 1 || api.msgs[0] != "hola desde lua" {
		t.Errorf("mensajes = %v, esperaba [hola desde lua]", api.msgs)
	}
	if len(api.cmds) != 1 || api.cmds[0] != "tcode.otro" {
		t.Errorf("comandos = %v, esperaba [tcode.otro]", api.cmds)
	}
}

// TestScriptHostReportsMissingFunction: llamar una función global que el
// script no definió es un error claro que nombra la función.
func TestScriptHostReportsMissingFunction(t *testing.T) {
	h, err := NewScriptHost(`function main() end`, &fakeAPI{})
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	err = h.Call("noexiste")
	if err == nil || !strings.Contains(err.Error(), "noexiste") {
		t.Fatalf("Call = %v, esperaba error nombrando la función ausente", err)
	}
}

// TestScriptHostReportsSyntaxErrors: código Lua roto no crea un host: el
// error de carga es claro y no queda un estado a medio configurar.
func TestScriptHostReportsSyntaxErrors(t *testing.T) {
	_, err := NewScriptHost(`function main(`, &fakeAPI{})
	if err == nil {
		t.Fatal("NewScriptHost aceptó código con error de sintaxis")
	}
	if !strings.Contains(err.Error(), "script") {
		t.Errorf("error = %v, debería venir envuelto como error de script", err)
	}
}

// TestScriptHostNoHostLibraries: el host solo abre base, tablas, strings y
// math; os e io (como el resto del entorno) no existen y usarlos es un error
// de Lua en tiempo de llamada, no un escape del host.
func TestScriptHostNoHostLibraries(t *testing.T) {
	cases := []string{
		`function main() return os.getenv("HOME") end`,
		`function main() return io.open("x") end`,
	}
	for _, code := range cases {
		t.Run(code, func(t *testing.T) {
			h, err := NewScriptHost(code, &fakeAPI{})
			if err != nil {
				t.Fatalf("NewScriptHost falló: %v", err)
			}
			defer h.Close()
			if err := h.Call("main"); err == nil {
				t.Fatalf("Call corrió con acceso a librerías del host (code=%q)", code)
			}
		})
	}
}

// TestScriptHostInsert: tcode.insert llega a InsertAtCursor del editor con el
// texto exacto.
func TestScriptHostInsert(t *testing.T) {
	api := &fakeAPI{}
	h, err := NewScriptHost(`
		function main()
			tcode.insert("texto insertado")
		end
	`, api)
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	if err := h.Call("main"); err != nil {
		t.Fatalf("Call falló: %v", err)
	}
	if len(api.inserted) != 1 || api.inserted[0] != "texto insertado" {
		t.Errorf("inserted = %v, esperaba [texto insertado]", api.inserted)
	}
}

// TestScriptHostErrorFunction: tcode.error eleva un error de script con el
// mensaje propio de la extensión, que Call devuelve envuelto.
func TestScriptHostErrorFunction(t *testing.T) {
	h, err := NewScriptHost(`
		function main()
			tcode.error("fallo propio del script")
		end
	`, &fakeAPI{})
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	err = h.Call("main")
	if err == nil || !strings.Contains(err.Error(), "fallo propio del script") {
		t.Fatalf("Call = %v, esperaba el error propio del script", err)
	}
}