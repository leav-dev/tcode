package ext

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// ScriptAPI es el puente que el backend de scripting usa para tocar el editor
// desde Lua: ejecutar otros comandos, leer el buffer activo, insertar texto en
// el cursor y escribir en la barra de estado. La implementa el controlador
// (App) y el Manager la inyecta con SetEditor; los hosts la reciben al crearse.
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
}

// ScriptHost posee un estado Lua aislado con el código de un script de
// extensión cargado y la tabla global tcode.* cableada a una ScriptAPI. El
// Manager cachea un host por script de extensión: el estado conserva las
// funciones globales y el estado top-level entre invocaciones de comandos.
type ScriptHost struct {
	L *lua.LState
}

// NewScriptHost crea un host con el código dado. Abre SOLO las librerías
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
func NewScriptHost(code string, api ScriptAPI) (*ScriptHost, error) {
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
	// tcode.error(msg): fallo declarado del propio script; Call lo devuelve
	// envuelto para que el controlador lo muestre como causa.
	L.SetField(tcode, "error", L.NewFunction(func(L *lua.LState) int {
		L.RaiseError("%s", L.CheckString(1))
		return 0
	}))
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