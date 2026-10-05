package ext

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
	"tcode/internal/view"
)

// ScriptAPI es el puente que el backend de scripting usa para tocar el editor
// desde Lua: ejecutar otros comandos, leer el buffer activo por líneas,
// insertar texto en el cursor, escribir en la barra de estado y depositar las
// anotaciones de diagnóstico. La implementa el controlador (App) y el Manager
// la inyecta con SetEditor; los hosts la reciben al crearse.
type ScriptAPI interface {
	// RunCommand ejecuta un comando registrado (built-in tcode.* o de otra
	// extensión) por su id.
	RunCommand(id string) error
	// ActiveBuffer devuelve la ruta y el contenido del buffer activo; ok es
	// false cuando no hay buffer abierto.
	ActiveBuffer() (path, content string, ok bool)
	// InsertAtCursor inserta text en la posición del cursor del editor activo.
	InsertAtCursor(text string) error
	// StatusMessage muestra un mensaje en la barra de estado.
	StatusMessage(msg string)
	// Notify muestra una notificación en la esquina superior derecha (toast,
	// con desaparición automática): kind es "success" (default) o "error";
	// un kind desconocido es un error.
	Notify(msg string, kind string) error
	// LineCount devuelve el número de líneas del buffer activo; ok es false
	// cuando no hay buffer abierto.
	LineCount() (n int, ok bool)
	// Line devuelve el texto de la línea n (0-indexada) del buffer activo; ok
	// es false cuando no hay buffer abierto o n está fuera de rango.
	Line(n int) (text string, ok bool)
	// SetDiagnostics reemplaza las anotaciones del buffer activo de UN
	// proveedor (source: el key del script del host): el editor mergea las de
	// todos los proveedores. El backend de scripting deposita acá las
	// anotaciones que la vista renderiza.
	SetDiagnostics(source string, d []view.Diagnostic) error
	// DirFiles devuelve los archivos Go del MISMO directorio que el buffer
	// activo (el contexto multi-archivo del paquete). El host Lua no tiene
	// io/os: el editor es el proveedor de archivos y la extensión la
	// analizadora. Cotas del proveedor: solo *.go, máx 64 archivos y 2 MiB
	// totales; el buffer activo no se repite. Error legible si no hay buffer.
	DirFiles() ([]HostFile, error)
	// GitStatus devuelve información de git del directorio del buffer activo:
	// archivos con cambios (staged, unstaged, untracked) y conteo de líneas
	// agregadas/borradas. El host Lua no tiene io/os: el editor ejecuta git.
	// Error legible si no hay buffer, no es repo git, o git no está disponible.
	GitStatus() (GitInfo, error)
	// GetFileDiff obtiene el diff de un archivo específico con formato unificado
	// para marcar líneas individuales. staged=true para cambios en staging area.
	GetFileDiff(path string, staged bool) ([]FileDiffLine, error)
}

// HostFile es un archivo hermano del buffer activo (mismo directorio,
// contexto de paquete) expuesto a los scripts: ruta absoluta y contenido.
type HostFile struct {
	Path    string
	Content string
}

// ScriptHost posee un estado Lua aislado con el código de un script de
// extensión cargado y la tabla global tcode.* cableada a una ScriptAPI. El
// Manager cachea un host por script de extensión: el estado conserva las
// funciones globales y el estado top-level entre invocaciones de comandos.
type ScriptHost struct {
	L *lua.LState
}

// NewScriptHost crea un host con el código dado y su identidad de proveedor
// (source, p. ej. e.Dir::script): tcode.diagnostics.set/clear actúan sobre
// ESA fuente y el editor mergea con las de otras. Abre SOLO las librerías
// base, tablas, strings y math, y nunca OpenLibs/OpenIo/OpenOs: el host no es
// un intérprete para la persona usuaria, es una caja para extensiones, y
// ninguna extensión necesita tocar el sistema de archivos o el entorno del
// proceso (para eso existe la API tcode.* del editor). Exponer os/io abriría
// la puerta a que un script lea o escriba cualquier archivo del equipo.
//
// Decisión de confianza: los repos del autor de tcode bloquean el acceso a
// APIs de host en los scripts de extensión, y acá se replica —el script entra
// por tcode.*, que es la única superficie que tcode controla y audita—. La
// mínima excepción es math, que solo calcula: ningún script necesita el
// mundo exterior para procesar texto.
//
// El código se carga con DoString (parsea y ejecuta el top-level); un error
// de sintaxis cierra el estado y devuelve un error claro, sin dejar un host
// medio armado.
func NewScriptHost(code string, api ScriptAPI, source string) (*ScriptHost, error) {
	// SkipOpenLibs: NewState sin opciones abriría TODAS las librerías (os e
	// io incluidas); acá el estado nace vacío y se abre solo el set mínimo.
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	// El set mínimo y suficiente: lo que un script de control de editor
	// necesita (tipos, pares, strings, math) sin poder tocar el host.
	for _, open := range []struct {
		name string
		fn   func(*lua.LState) int
	}{
		{"base", lua.OpenBase},
		{"tablas", lua.OpenTable},
		{"strings", lua.OpenString},
		{"math", lua.OpenMath},
	} {
		open.fn(L)
	}

	tcode := L.NewTable()
	// tcode.command(id): ejecuta otro comando registrado; su error se eleva
	// como error de Lua y corta el script.
	L.SetField(tcode, "command", L.NewFunction(func(L *lua.LState) int {
		if err := api.RunCommand(L.CheckString(1)); err != nil {
			L.RaiseError("%v", err)
		}
		return 0
	}))
	// tcode.buffer(): sin buffer activo no devuelve nada (nil, nil en Lua);
	// con buffer, la ruta y el contenido completo.
	L.SetField(tcode, "buffer", L.NewFunction(func(L *lua.LState) int {
		path, content, ok := api.ActiveBuffer()
		if !ok {
			return 0
		}
		L.Push(lua.LString(path))
		L.Push(lua.LString(content))
		return 2
	}))
	// tcode.insert(text): inserta en el cursor del editor activo.
	L.SetField(tcode, "insert", L.NewFunction(func(L *lua.LState) int {
		if err := api.InsertAtCursor(L.CheckString(1)); err != nil {
			L.RaiseError("%v", err)
		}
		return 0
	}))
	// tcode.message(msg): texto en la barra de estado.
	L.SetField(tcode, "message", L.NewFunction(func(L *lua.LState) int {
		api.StatusMessage(L.CheckString(1))
		return 0
	}))
	// tcode.notify(msg, kind): notificación en la esquina superior derecha, con
	// desaparición automática —la misma que muestra el editor al guardar. kind
	// es "success" (default) o "error"; un kind desconocido es un error de Lua.
	L.SetField(tcode, "notify", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		kind := ""
		if v := L.Get(2); v.Type() != lua.LTNil {
			if v.Type() != lua.LTString {
				L.RaiseError("tcode.notify: el kind debe ser un string (\"success\" | \"error\")")
			}
			kind = string(v.(lua.LString))
		}
		if err := api.Notify(msg, kind); err != nil {
			L.RaiseError("%v", err)
		}
		return 0
	}))
	// tcode.error(msg): fallo declarado del propio script; Call lo devuelve
	// envuelto para que el controlador lo muestre como causa.
	L.SetField(tcode, "error", L.NewFunction(func(L *lua.LState) int {
		L.RaiseError("%s", L.CheckString(1))
		return 0
	}))
	// tcode.lineCount(): sin buffer activo, error Lua claro (coherente con
	// tcode.command e insert); con buffer, el número de líneas direccionables.
	L.SetField(tcode, "lineCount", L.NewFunction(func(L *lua.LState) int {
		n, ok := api.LineCount()
		if !ok {
			L.RaiseError("sin buffer activo")
		}
		L.Push(lua.LNumber(n))
		return 1
	}))
	// tcode.line(n): texto de la línea n (numeración Lua, 1-indexada) del
	// buffer activo. La traducción Lua → Go (0-indexada) vive acá: del lado
	// del proveedor, api.Line recibe n-1. Sin buffer o fuera de rango, error
	// Lua claro que corta el script.
	L.SetField(tcode, "line", L.NewFunction(func(L *lua.LState) int {
		n := L.CheckNumber(1)
		if n < 1 {
			L.RaiseError("línea fuera de rango: %v", n)
		}
		line, ok := api.Line(int(n) - 1)
		if !ok {
			// Distinguir sin buffer de línea inexistente, como el resto de los
			// errores de la API tcode.*
			if _, has := api.LineCount(); !has {
				L.RaiseError("sin buffer activo")
			}
			L.RaiseError("línea fuera de rango: %v", n)
		}
		L.Push(lua.LString(line))
		return 1
	}))
	// tcode.diagnostics.set(lista) y tcode.diagnostics.clear(): el backend de
	// scripting es el proveedor de las anotaciones del HITO A. La tabla anida
	// en tcode como objeto, no como función: set valida la lista completa
	// ANTES de tocar el editor (todo o nada: un elemento malformado aborta con
	// un error de Lua descriptivo). Cada elemento es {line, message, severity}
	// con line en numeración Lua (1-indexada), traducida acá a la 0-indexada
	// de view.Diagnostic; el editor es el dueño de la representación.
	diagnostics := L.NewTable()
	L.SetField(diagnostics, "set", L.NewFunction(func(L *lua.LState) int {
		list := L.CheckTable(1)
		ds := make([]view.Diagnostic, 0, list.Len())
		for i := 1; i <= list.Len(); i++ {
			item := list.RawGetInt(i)
			if item.Type() != lua.LTTable {
				L.RaiseError("diagnostics: el elemento %d no es una tabla {line, message, severity}", i)
			}
			lineV := L.GetField(item, "line")
			messageV := L.GetField(item, "message")
			severityV := L.GetField(item, "severity")
			if lineV.Type() != lua.LTNumber {
				L.RaiseError("diagnostics: el elemento %d no tiene línea (line >= 1)", i)
			}
			line := int(lineV.(lua.LNumber))
			if line < 1 {
				L.RaiseError("diagnostics: la línea %d está fuera de rango (line >= 1)", line)
			}
			msg := ""
			if messageV.Type() == lua.LTString {
				msg = string(messageV.(lua.LString))
			} else if messageV.Type() != lua.LTNil {
				L.RaiseError("diagnostics: el elemento %d tiene un message no textual", i)
			}
			// Severidad: "error" | "warning" | "info" mapean al enum de
			// view; ausente → default Error, pero un string desconocido es un
			// elemento malformado y aborta el set.
			sev := view.SeverityError
			if severityV.Type() == lua.LTString {
				switch string(severityV.(lua.LString)) {
				case "warning":
					sev = view.SeverityWarning
				case "info":
					sev = view.SeverityInfo
				case "error":
					// default Error ya asignado
				default:
					L.RaiseError("diagnostics: severidad %q desconocida (error|warning|info)", severityV.String())
				}
			} else if severityV.Type() != lua.LTNil {
				L.RaiseError("diagnostics: el elemento %d tiene una severity no textual", i)
			}
			ds = append(ds, view.Diagnostic{Line: line - 1, Message: msg, Severity: sev})
		}
		if err := api.SetDiagnostics(source, ds); err != nil {
			L.RaiseError("%v", err)
		}
		return 0
	}))
	L.SetField(diagnostics, "clear", L.NewFunction(func(L *lua.LState) int {
		if err := api.SetDiagnostics(source, []view.Diagnostic{}); err != nil {
			L.RaiseError("%v", err)
		}
		return 0
	}))
	L.SetField(tcode, "diagnostics", diagnostics)
	// tcode.dir_files(): los archivos Go del mismo directorio que el buffer
	// activo (el paquete abarca varios archivos). Devuelve una tabla de
	// {path, content} o nil cuando no hay buffer, el proveedor falla o no hay
	// hermanos: el script degrada al buffer único. El editor pone las cotas
	// de memoria.
	L.SetField(tcode, "dir_files", L.NewFunction(func(L *lua.LState) int {
		files, err := api.DirFiles()
		if err != nil || len(files) == 0 {
			return 0
		}
		t := L.NewTable()
		for i, f := range files {
			it := L.NewTable()
			it.RawSetString("path", lua.LString(f.Path))
			it.RawSetString("content", lua.LString(f.Content))
			t.RawSetInt(i+1, it)
		}
		L.Push(t)
		return 1
	}))
	// tcode.git.status(): información de git del directorio del buffer activo.
	// Devuelve {staged={}, unstaged={}, untracked={}, added=N, deleted=N} o nil
	// cuando no hay buffer, no es repo git, o git no está disponible.
	git := L.NewTable()
	L.SetField(git, "status", L.NewFunction(func(L *lua.LState) int {
		info, err := api.GitStatus()
		if err != nil {
			return 0
		}
		t := L.NewTable()
		staged := L.NewTable()
		for i, f := range info.StagedFiles {
			staged.RawSetInt(i+1, lua.LString(f))
		}
		t.RawSetString("staged", staged)
		unstaged := L.NewTable()
		for i, f := range info.UnstagedFiles {
			unstaged.RawSetInt(i+1, lua.LString(f))
		}
		t.RawSetString("unstaged", unstaged)
		untracked := L.NewTable()
		for i, f := range info.UntrackedFiles {
			untracked.RawSetInt(i+1, lua.LString(f))
		}
		t.RawSetString("untracked", untracked)
		t.RawSetString("added", lua.LNumber(info.AddedLines))
		t.RawSetString("deleted", lua.LNumber(info.DeletedLines))
		L.Push(t)
		return 1
	}))
	// tcode.git.file_diff(path, staged): diff de un archivo específico con
	// formato unificado. Devuelve {{line=N, type="added"|"deleted"}, ...}
	// o nil cuando no hay diff. staged=true para cambios en staging area.
	L.SetField(git, "file_diff", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		staged := L.CheckBool(2)
		lines, err := api.GetFileDiff(path, staged)
		if err != nil || len(lines) == 0 {
			return 0
		}
		t := L.NewTable()
		for i, l := range lines {
			it := L.NewTable()
			it.RawSetString("line", lua.LNumber(l.Line))
			it.RawSetString("type", lua.LString(l.Type))
			t.RawSetInt(i+1, it)
		}
		L.Push(t)
		return 1
	}))
	L.SetField(tcode, "git", git)
	L.SetGlobal("tcode", tcode)

	if err := L.DoString(code); err != nil {
		L.Close()
		return nil, fmt.Errorf("script: %v", err)
	}
	return &ScriptHost{L: L}, nil
}

// Call invoca la función global fn como la implementación de un comando. Si
// la función no existe es un error que la nombra; un error de Lua (incluidos
// los elevados por la API tcode.*) se envuelve como error de script. El
// recover convierte un pánico imprevisto del host en error en lugar de tumbar
// el editor: un script no puede hacer caer el proceso.
func (h *ScriptHost) Call(fn string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("script panic: %v", r)
		}
	}()
	lf := h.L.GetGlobal(fn)
	if lf == nil || lf.Type() != lua.LTFunction {
		return fmt.Errorf("función %s no definida", fn)
	}
	if err := h.L.CallByParam(lua.P{Fn: lf, NRet: 0, Protect: true}); err != nil {
		return fmt.Errorf("script: %v", err)
	}
	return nil
}

// Close libera el estado Lua del host. El Manager cachea los hosts por script
// y no hay ciclo de cierre global hoy (el proceso libera al terminar); el
// método existe para que el ciclo de vida sea explícito donde haga falta.
func (h *ScriptHost) Close() error {
	h.L.Close()
	return nil
}