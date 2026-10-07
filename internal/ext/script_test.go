package ext

import (
	"errors"
	"strings"
	"testing"

	"github.com/leav-dev/tcode/internal/view"
)

// notif registra una llamada a Notify: el mensaje y el kind pedido.
type notif struct {
	msg  string
	kind string
}

// fakeAPI implementa ScriptAPI registrando las llamadas, para que los tests
// comprueben el puente script → editor sin tocar un controlador real.
type fakeAPI struct {
	cmds           []string
	msgs           []string
	notifs         []notif
	sections       map[string]string
	inserted       []string
	path           string
	content        string
	bufOK          bool
	insertErr      error
	runErr         error
	lineCount      int
	lineCountCalls int
	lines          []string
	lineCalls      []int
	diags          [][]view.Diagnostic
	diagErr        error
	files          []HostFile
	filesErr       error
	readPath       string
	readFile       HostFile
	readErr        error
	gitInfo        GitInfo
	gitErr         error
	fileDiffLines  []FileDiffLine
	fileDiffErr    error
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

func (f *fakeAPI) Notify(msg, kind string) error {
	f.notifs = append(f.notifs, notif{msg: msg, kind: kind})
	return nil
}

func (f *fakeAPI) SetSection(id, text string) error {
	if f.sections == nil {
		f.sections = map[string]string{}
	}
	if text == "" {
		delete(f.sections, id)
	} else {
		f.sections[id] = text
	}
	return nil
}

func (f *fakeAPI) LineCount() (int, bool) {
	f.lineCountCalls++
	if !f.bufOK {
		return 0, false
	}
	return f.lineCount, true
}

func (f *fakeAPI) Line(n int) (string, bool) {
	f.lineCalls = append(f.lineCalls, n)
	if !f.bufOK || n < 0 || n >= f.lineCount {
		return "", false
	}
	return f.lines[n], true
}

func (f *fakeAPI) SetDiagnostics(source string, d []view.Diagnostic) error {
	f.diags = append(f.diags, append([]view.Diagnostic(nil), d...))
	return f.diagErr
}

func (f *fakeAPI) DirFiles() ([]HostFile, error) { return f.files, f.filesErr }

func (f *fakeAPI) ReadFile(relpath string) (HostFile, error) {
	f.readPath = relpath
	return f.readFile, f.readErr
}

func (f *fakeAPI) GitStatus() (GitInfo, error) { return f.gitInfo, f.gitErr }

func (f *fakeAPI) GetFileDiff(path string, staged bool) ([]FileDiffLine, error) {
	return f.fileDiffLines, f.fileDiffErr
}

// TestScriptHostReadFile: tcode.read_file pide UN archivo por ruta relativa y
// recibe {path, content} con la ruta absoluta canónica.
func TestScriptHostReadFile(t *testing.T) {
	api := &fakeAPI{readFile: HostFile{Path: "/dir/util.ts", Content: "export {}"}}
	h, err := NewScriptHost(`
		function f()
			local r = tcode.read_file("./util.ts")
			tcode.message(r.path .. "|" .. r.content)
		end
	`, api, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	if err := h.Call("f"); err != nil {
		t.Fatalf("Call falló: %v", err)
	}
	if api.readPath != "./util.ts" {
		t.Errorf("ReadFile llamado con %q, esperaba [./util.ts]", api.readPath)
	}
	if len(api.msgs) != 1 || api.msgs[0] != "/dir/util.ts|export {}" {
		t.Errorf("mensajes = %v, esperaba [/dir/util.ts|export {}]", api.msgs)
	}
}

// TestScriptHostReadFileNilOnError: si el proveedor falla (sin buffer, escape,
// inexistente, cota), el host devuelve nil silencioso como dir_files: el
// script degrada sin cortar.
func TestScriptHostReadFileNilOnError(t *testing.T) {
	api := &fakeAPI{readErr: errors.New("no existe")}
	h, err := NewScriptHost(`
		function f()
			local r = tcode.read_file("./falta.ts")
			tcode.message(tostring(r))
		end
	`, api, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	if err := h.Call("f"); err != nil {
		t.Fatalf("Call falló: %v", err)
	}
	if len(api.msgs) != 1 || api.msgs[0] != "nil" {
		t.Errorf("mensajes = %v, esperaba [nil]", api.msgs)
	}
}

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
	`, api, "src")
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

// TestScriptHostNotify: tcode.notify enruta al editor con el kind pedido —
// ausente es "success" (default), "error" pasa igual; un kind desconocido o
// no textual es un error de Lua que corta el script.
func TestScriptHostNotify(t *testing.T) {
	api := &fakeAPI{}
	h, err := NewScriptHost(`
		function main()
			tcode.notify("lista")
			tcode.notify("falló", "error")
		end
	`, api, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	if err := h.Call("main"); err != nil {
		t.Fatalf("Call falló: %v", err)
	}
	want := []notif{{msg: "lista", kind: ""}, {msg: "falló", kind: "error"}}
	if len(api.notifs) != len(want) {
		t.Fatalf("notificaciones = %v, esperaba %v", api.notifs, want)
	}
	for i, w := range want {
		if api.notifs[i] != w {
			t.Errorf("notificación %d = %v, esperaba %v", i, api.notifs[i], w)
		}
	}
}

// TestScriptHostNotifyRejectsNonStringKind: un kind que no es un string
// (p. ej. un número) también es un error de Lua.
func TestScriptHostNotifyRejectsNonStringKind(t *testing.T) {
	h, err := NewScriptHost(`
		function main()
			tcode.notify("x", 42)
		end
	`, &fakeAPI{}, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	err = h.Call("main")
	if err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("Call = %v, esperaba error nombrando el kind", err)
	}
}

// TestScriptHostSetSection: tcode.statusBar.setSection enruta al editor con
// el id y el texto pedidos; el texto vacío remueve la sección y el id vacío
// es un error de Lua.
func TestScriptHostSetSection(t *testing.T) {
	api := &fakeAPI{}
	h, err := NewScriptHost(`
		function main()
			tcode.statusBar.setSection("tcode.gitchanges", "Git: 3 files")
			tcode.statusBar.setSection("tcode.gitchanges", "")
			tcode.statusBar.setSection("tcode.linter", "2 issues")
		end
	`, api, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	if err := h.Call("main"); err != nil {
		t.Fatalf("Call falló: %v", err)
	}
	want := map[string]string{"tcode.linter": "2 issues"}
	if len(api.sections) != len(want) {
		t.Fatalf("secciones = %v, esperaba %v", api.sections, want)
	}
	for id, text := range want {
		if api.sections[id] != text {
			t.Errorf("sección %q = %q, se esperaba %q", id, api.sections[id], text)
		}
	}
}

// TestScriptHostSetSectionRejectsEmptyID: el id vacío es un error de Lua
// claro, igual que los demás errores de la API tcode.*.
func TestScriptHostSetSectionRejectsEmptyID(t *testing.T) {
	h, err := NewScriptHost(`
		function main()
			tcode.statusBar.setSection("", "texto")
		end
	`, &fakeAPI{}, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	err = h.Call("main")
	if err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("Call = %v, esperaba error nombrando el id", err)
	}
}

// TestScriptHostReportsMissingFunction: llamar una función global que el
// script no definió es un error claro que nombra la función.
func TestScriptHostReportsMissingFunction(t *testing.T) {
	h, err := NewScriptHost(`function main() end`, &fakeAPI{}, "src")
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
	_, err := NewScriptHost(`function main(`, &fakeAPI{}, "src")
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
			h, err := NewScriptHost(code, &fakeAPI{}, "src")
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
	`, api, "src")
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
	`, &fakeAPI{}, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	err = h.Call("main")
	if err == nil || !strings.Contains(err.Error(), "fallo propio del script") {
		t.Fatalf("Call = %v, esperaba el error propio del script", err)
	}
}

// TestScriptHostLineAndCount: tcode.lineCount y tcode.line exponen el buffer
// activo por líneas; line traduce la numeración Lua (1-indexada) a la Go
// (0-indexada) del modelo PieceTable.
func TestScriptHostLineAndCount(t *testing.T) {
	api := &fakeAPI{bufOK: true, lineCount: 3, lines: []string{"line1", "line2", "line3"}}
	h, err := NewScriptHost(`
		function f()
			local n = tcode.lineCount()
			local l = tcode.line(1)
			tcode.message(l .. "/" .. n)
		end
	`, api, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	if err := h.Call("f"); err != nil {
		t.Fatalf("Call falló: %v", err)
	}
	if len(api.msgs) != 1 || api.msgs[0] != "line1/3" {
		t.Errorf("mensajes = %v, esperaba [line1/3]", api.msgs)
	}
	if len(api.lineCalls) != 1 || api.lineCalls[0] != 0 {
		t.Errorf("Line llamado con %v, esperaba [0] (traducción 1→0)", api.lineCalls)
	}
	if api.lineCountCalls != 1 {
		t.Errorf("LineCount llamado %d veces, esperaba 1", api.lineCountCalls)
	}
}

// TestScriptHostDiagnosticsSet: tcode.diagnostics.set valida cada elemento de
// la lista Lua {line, message, severity}, traduce la línea 1-indexada a
// 0-indexada y reemplaza las anotaciones del editor en una sola llamada. La
// severidad ausente queda en default (Error) y el message ausente en vacío.
func TestScriptHostDiagnosticsSet(t *testing.T) {
	api := &fakeAPI{bufOK: true, lineCount: 6}
	h, err := NewScriptHost(`
		function f()
			tcode.diagnostics.set({
				{ line = 2, message = "mal", severity = "error" },
				{ line = 5, severity = "warning" },
				{ line = 3 },
			})
		end
	`, api, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	if err := h.Call("f"); err != nil {
		t.Fatalf("Call falló: %v", err)
	}
	if len(api.diags) != 1 {
		t.Fatalf("SetDiagnostics llamada %d veces, esperaba 1", len(api.diags))
	}
	d := api.diags[0]
	if len(d) != 3 {
		t.Fatalf("diags = %+v, esperaba 3 (el set completo de una vez)", d)
	}
	if d[0].Line != 1 || d[0].Severity != view.SeverityError || d[0].Message != "mal" {
		t.Errorf("diag[0] = %+v, esperaba línea 1 severidad Error message mal", d[0])
	}
	if d[1].Line != 4 || d[1].Severity != view.SeverityWarning || d[1].Message != "" {
		t.Errorf("diag[1] = %+v, esperaba línea 4 severidad Warning message vacío", d[1])
	}
	// TRIANGULATE: un elemento sin severity ni message cae en los defaults
	// (severidad Error, message vacío), como declara la especificación.
	if d[2].Line != 2 || d[2].Severity != view.SeverityError || d[2].Message != "" {
		t.Errorf("diag[2] = %+v, esperaba línea 2 severidad Error (default) message vacío", d[2])
	}
}

// TestScriptHostDiagnosticsClear: tcode.diagnostics.clear reemplaza las
// anotaciones del buffer con una lista vacía (apagar el marcador).
func TestScriptHostDiagnosticsClear(t *testing.T) {
	api := &fakeAPI{}
	h, err := NewScriptHost(`
		function f()
			tcode.diagnostics.clear()
		end
	`, api, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	if err := h.Call("f"); err != nil {
		t.Fatalf("Call falló: %v", err)
	}
	if len(api.diags) != 1 || len(api.diags[0]) != 0 {
		t.Fatalf("diags = %v, esperaba una llamada con lista vacía", api.diags)
	}
}

// TestScriptHostDiagnosticsBadItem: un elemento malformado (line no numérica
// o severidad desconocida) aborta el set completo: el error de Lua corta el
// script y el editor jamás recibe anotaciones parciales (todo o nada).
func TestScriptHostDiagnosticsBadItem(t *testing.T) {
	cases := []string{
		`{ line = "x", message = "mal" }`,
		`{ line = 2, message = "mal", severity = "fatal" }`,
	}
	for _, item := range cases {
		t.Run(item, func(t *testing.T) {
			api := &fakeAPI{}
			h, err := NewScriptHost("function f() tcode.diagnostics.set({"+item+"}) end", api, "src")
			if err != nil {
				t.Fatalf("NewScriptHost falló: %v", err)
			}
			defer h.Close()

			err = h.Call("f")
			if err == nil || !strings.Contains(err.Error(), "diagnostics") {
				t.Errorf("Call = %v, esperaba un error de diagnostics", err)
			}
			if len(api.diags) != 0 {
				t.Errorf("SetDiagnostics llamado %d veces, esperaba 0 (todo o nada)", len(api.diags))
			}
		})
	}
}

// TestScriptHostLineOutOfRange: pedir una línea fuera de rango (0 o más allá
// del LineCount) es un error de Lua propagado por Call, no un acceso raro al
// buffer.
func TestScriptHostLineOutOfRange(t *testing.T) {
	cases := []string{
		`function f() tcode.line(0) end`,
		`function f() tcode.line(99) end`,
	}
	for _, code := range cases {
		t.Run(code, func(t *testing.T) {
			api := &fakeAPI{bufOK: true, lineCount: 3, lines: []string{"a", "b", "c"}}
			h, err := NewScriptHost(code, api, "src")
			if err != nil {
				t.Fatalf("NewScriptHost falló: %v", err)
			}
			defer h.Close()

			err = h.Call("f")
			if err == nil || !strings.Contains(err.Error(), "línea fuera de rango") {
				t.Errorf("Call = %v, esperaba error de línea fuera de rango", err)
			}
		})
	}
}

// TestScriptHostGitStatusExposesBranch: tcode.git.status() expone la rama
// como campo branch ("" cuando el editor no la pudo determinar), junto al
// último commit (commit_hash/subject/author/date).
func TestScriptHostGitStatusExposesBranch(t *testing.T) {
	api := &fakeAPI{gitInfo: GitInfo{Branch: "preview", AddedLines: 1,
		CommitHash: "a1b2c3d", CommitSubject: "do things", CommitAuthor: "Ada",
		CommitDate: "2026-10-07"}}
	h, err := NewScriptHost(`
		function main()
			local git = tcode.git.status()
			tcode.statusBar.setSection("tcode.gitchanges",
				"branch=" .. git.branch .. " commit=" .. git.commit_hash .. " " .. git.commit_subject)
		end
	`, api, "src")
	if err != nil {
		t.Fatalf("NewScriptHost falló: %v", err)
	}
	defer h.Close()

	if err := h.Call("main"); err != nil {
		t.Fatalf("Call falló: %v", err)
	}
	want := "branch=preview commit=a1b2c3d do things"
	if api.sections["tcode.gitchanges"] != want {
		t.Errorf("sección = %q, esperaba %q", api.sections["tcode.gitchanges"], want)
	}
}
