package ext

import (
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

const hookExt = `{
	"id": "ext.hooks",
	"name": "De Hooks",
	"version": "1.0.0",
	"activation": ["onStartup"],
	"contributes": {
		"commands": [{"id": "ext.hooks.anotar"}],
		"hooks": [{"event": "onDidSaveBuffer", "command": "ext.hooks.anotar"}]
	}
}`

const lazyExt = `{
	"id": "ext.lazy",
	"name": "Perezosa",
	"version": "1.0.0",
	"activation": ["onDidOpenBuffer"],
	"contributes": {
		"commands": [{"id": "ext.lazy.ver"}],
		"hooks": [{"event": "onDidSaveBuffer", "command": "ext.lazy.ver"}]
	}
}`

const onCmdExt = `{
	"id": "ext.oncmd",
	"name": "Por Comando",
	"version": "1.0.0",
	"activation": ["onCommand:ext.oncmd.saludar"],
	"contributes": {
		"commands": [{"id": "ext.oncmd.saludar", "title": "Saludar"}],
		"keybindings": [{"key": "ctrl+alt+s", "command": "ext.oncmd.saludar"}]
	}
}`

// addTestExtensions carga srcs como Extension y las agrega al Manager.
func addTestExtensions(t *testing.T, m *Manager, srcs ...string) {
	t.Helper()
	var exts []Extension
	for i, src := range srcs {
		mf, err := Load([]byte(src))
		if err != nil {
			t.Fatalf("Load del fixture %d falló: %v", i, err)
		}
		exts = append(exts, Extension{Manifest: mf, Dir: "fixture"})
	}
	m.AddExtensions(exts)
}

// TestManagerStartupActivatesAndRunsHooks: onStartup activa de inmediato y los
// hooks de la extensión activa corren cuando el controlador emite el evento.
func TestManagerStartupActivatesAndRunsHooks(t *testing.T) {
	m := NewManager()
	calls := 0
	if err := m.Registry().Register("ext.hooks.anotar", func() error { calls++; return nil }); err != nil {
		t.Fatalf("Register falló: %v", err)
	}
	addTestExtensions(t, m, hookExt)
	if n := m.ActivateEvent(ActivateStartup); n != 1 {
		t.Fatalf("ActivateEvent(onStartup) activó %d, esperaba 1", n)
	}
	if errs := m.Emit(EventDidSaveBuffer); len(errs) != 0 {
		t.Fatalf("Emit reportó errores: %v", errs)
	}
	if calls != 1 {
		t.Errorf("hook corrió %d veces, esperaba 1", calls)
	}
	// El evento de guardado no tiene que re-activar nada: ya estaba activa.
	if n := m.ActivateEvent(EventDidSaveBuffer); n != 0 {
		t.Errorf("reactivación inesperada: %d", n)
	}
}

// TestManagerLazyActivationOnOpen: una extensión con onDidOpenBuffer no corre
// sus hooks de guardado hasta que se abre un buffer; recién ahí queda activa.
func TestManagerLazyActivationOnOpen(t *testing.T) {
	m := NewManager()
	calls := 0
	if err := m.Registry().Register("ext.lazy.ver", func() error { calls++; return nil }); err != nil {
		t.Fatalf("Register falló: %v", err)
	}
	addTestExtensions(t, m, lazyExt)

	if errs := m.Emit(EventDidSaveBuffer); len(errs) != 0 {
		t.Fatalf("Emit antes de activar reportó errores: %v", errs)
	}
	if calls != 0 {
		t.Fatalf("hook corrió %d veces antes de la activación, esperaba 0", calls)
	}
	if m.Active("ext.lazy") {
		t.Fatal("la extensión ya está activa sin haber abierto un buffer")
	}

	m.ActivateEvent(EventDidOpenBuffer)
	if !m.Active("ext.lazy") {
		t.Fatal("la extensión no quedó activa tras abrir un buffer")
	}
	if errs := m.Emit(EventDidSaveBuffer); len(errs) != 0 {
		t.Fatalf("Emit tras activar reportó errores: %v", errs)
	}
	if calls != 1 {
		t.Errorf("hook corrió %d veces tras activar, esperaba 1", calls)
	}
}

// TestManagerOnCommandActivates: ejecutar un comando con onCommand activa la
// extensión; su stub legítimamente avisa que no hay backend de scripting aún.
func TestManagerOnCommandActivates(t *testing.T) {
	m := NewManager()
	addTestExtensions(t, m, onCmdExt)
	err := m.RunCommand("ext.oncmd.saludar")
	if err == nil {
		t.Fatal("RunCommand del comando declarado no avisó la falta de backend")
	}
	if !m.Active("ext.oncmd") {
		t.Fatal("el comando no activó su extensión")
	}
	// Un comando desconocido sigue siendo desconocido y no activa nada.
	before := m.ActiveCount()
	err = m.RunCommand("ext.otra.inexistente")
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("RunCommand desconocido = %v, esperaba ErrUnknownCommand", err)
	}
	if m.ActiveCount() != before {
		t.Errorf("ActiveCount cambió %d → %d ante un comando desconocido", before, m.ActiveCount())
	}
}

// TestManagerResolveWorksBeforeActivation: los keybindings declarados
// resuelven desde el arranque (como VSCode), aunque la extensión esté
// inactiva; ejecutarlos activa la extensión dueña.
func TestManagerResolveWorksBeforeActivation(t *testing.T) {
	m := NewManager()
	addTestExtensions(t, m, onCmdExt)
	got := m.Resolve(evKey(tcell.KeyRune, 's', tcell.ModCtrl|tcell.ModAlt))
	if got != "ext.oncmd.saludar" {
		t.Fatalf("Resolve = %q, esperaba ext.oncmd.saludar", got)
	}
	if m.Active("ext.oncmd") {
		t.Fatal("resolver no debe activar; solo ejecutar el comando lo activa")
	}
}

// TestManagerHookErrorIsReported: el error de un hook (acá, el stub del
// comando declarado avisando que falta el backend) se acumula y se devuelve,
// sin romper el resto del Emit.
func TestManagerHookErrorIsReported(t *testing.T) {
	m := NewManager()
	addTestExtensions(t, m, hookExt)
	m.ActivateEvent(ActivateStartup)
	errs := m.Emit(EventDidSaveBuffer)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "sin implementación") {
		t.Fatalf("Emit = %v, esperaba el error del stub del hook", errs)
	}
}

// TestManagerStarActivatesOnAnyEvent y los hooks corren tras el primer evento.
func TestManagerStarActivatesOnAnyEvent(t *testing.T) {
	src := `{
		"id": "ext.estrella",
		"name": "Estrella",
		"version": "1.0.0",
		"activation": ["*"],
		"contributes": {
			"commands": [{"id": "ext.estrella.x"}],
			"hooks": [{"event": "onDidCloseBuffer", "command": "ext.estrella.x"}]
		}
	}`
	m := NewManager()
	calls := 0
	if err := m.Registry().Register("ext.estrella.x", func() error { calls++; return nil }); err != nil {
		t.Fatalf("Register falló: %v", err)
	}
	addTestExtensions(t, m, src)
	if m.Active("ext.estrella") {
		t.Fatal("la extensión estrella está activa sin ningún evento")
	}
	m.ActivateEvent(EventDidCloseBuffer)
	if !m.Active("ext.estrella") {
		t.Fatal("el primer evento no activó la extensión estrella")
	}
	if errs := m.Emit(EventDidCloseBuffer); len(errs) != 0 {
		t.Fatalf("Emit reportó errores: %v", errs)
	}
	if calls != 1 {
		t.Errorf("hook corrió %d veces, esperaba 1", calls)
	}
}

// TestManagerEmitIsReentrantSafe: un hook cuyo comando vuelve a emitir el
// mismo evento no puede recurrir infinitamente (stack overflow). El Emit
// anidado se corta por el guard y el hook corre una sola vez.
func TestManagerEmitIsReentrantSafe(t *testing.T) {
	m := NewManager()
	calls := 0
	if err := m.Registry().Register("loop.close", func() error {
		calls++
		m.Emit(EventDidCloseBuffer) // reentrancia: debe cortarse
		return nil
	}); err != nil {
		t.Fatalf("Register falló: %v", err)
	}
	addTestExtensions(t, m, `{
		"id": "ext.loop",
		"name": "Loop",
		"version": "1.0.0",
		"activation": ["onStartup"],
		"contributes": {
			"hooks": [{"event": "onDidCloseBuffer", "command": "loop.close"}]
		}
	}`)
	m.ActivateEvent(ActivateStartup)

	if errs := m.Emit(EventDidCloseBuffer); len(errs) != 0 {
		t.Fatalf("Emit reportó errores: %v", errs)
	}
	if calls != 1 {
		t.Fatalf("el hook corrió %d veces, esperaba 1 (el guard cortó la recursión)", calls)
	}
}

// TestManagerDuplicateStubKeepsFirst: dos extensiones declarando el mismo
// comando no se pisan entre sí: el stub del primero gana (regla del registro).
func TestManagerDuplicateStubKeepsFirst(t *testing.T) {
	m := NewManager()
	addTestExtensions(t, m, onCmdExt, onCmdExt)
	if err := m.RunCommand("ext.oncmd.saludar"); err == nil {
		t.Fatal("el stub duplicado no avisó la falta de backend")
	}
}
