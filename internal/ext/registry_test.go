package ext

import (
	"errors"
	"strings"
	"testing"
)

// TestRegistryRunInvokesHandler verifica el camino feliz: registrar, preguntar
// y ejecutar con su efecto observable (el contador del handler).
func TestRegistryRunInvokesHandler(t *testing.T) {
	r := NewRegistry()
	calls := 0
	if err := r.Register("tcode.ping", func() error { calls++; return nil }); err != nil {
		t.Fatalf("Register falló: %v", err)
	}
	if !r.Has("tcode.ping") {
		t.Fatal("Has(tcode.ping) = false tras registrar")
	}
	if err := r.Run("tcode.ping"); err != nil {
		t.Fatalf("Run falló: %v", err)
	}
	if calls != 1 {
		t.Errorf("handler corrió %d veces, esperaba 1", calls)
	}
	if err := r.Run("tcode.ping"); err != nil {
		t.Fatalf("segundo Run falló: %v", err)
	}
	if calls != 2 {
		t.Errorf("handler corrió %d veces, esperaba 2", calls)
	}
}

// TestRegistryRunReportsUnknownCommand es el error que el controlador traduce
// a mensaje de estado: comando desconocido, distinguible con Is.
func TestRegistryRunReportsUnknownCommand(t *testing.T) {
	r := NewRegistry()
	err := r.Run("tcode.noexiste")
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("Run = %v, esperaba ErrUnknownCommand", err)
	}
	if !strings.Contains(err.Error(), "tcode.noexiste") {
		t.Errorf("error %q no nombra el comando", err)
	}
	if r.Has("tcode.noexiste") {
		t.Error("Has(desconocido) = true")
	}
}

// TestRegistryRunPropagatesHandlerError el error del handler llega intacto a
// Run: el controlador lo muestra sin tragárselo.
func TestRegistryRunPropagatesHandlerError(t *testing.T) {
	r := NewRegistry()
	want := errors.New("el handler explotó")
	if err := r.Register("tcode.roto", func() error { return want }); err != nil {
		t.Fatalf("Register falló: %v", err)
	}
	if err := r.Run("tcode.roto"); !errors.Is(err, want) {
		t.Fatalf("Run = %v, esperaba el error del handler", err)
	}
}

// TestRegistryDuplicateRegistrationKeepsFirst: registrar dos veces el mismo
// id falla y conserva el handler original — un built-in no puede ser pisado
// por un re-registro accidental.
func TestRegistryDuplicateRegistrationKeepsFirst(t *testing.T) {
	r := NewRegistry()
	calls := 0
	first := func() error { calls++; return nil }
	if err := r.Register("tcode.x", first); err != nil {
		t.Fatalf("primer Register falló: %v", err)
	}
	if err := r.Register("tcode.x", func() error { return errors.New("el segundo") }); err == nil {
		t.Fatal("segundo Register no devolvió error")
	}
	if err := r.Run("tcode.x"); err != nil {
		t.Fatalf("Run falló: %v", err)
	}
	if calls != 1 {
		t.Errorf("handler corrió %d veces, esperaba 1 (el primero)", calls)
	}
}

// TestRegistryRejectsInvalidID: los ids que el manifest no acepta tampoco se
// registran; el registro y el manifest comparten la misma gramática.
func TestRegistryRejectsInvalidID(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("", func() error { return nil }); err == nil {
		t.Error("Register con id vacío no devolvió error")
	}
	if err := r.Register("tcode con espacio", func() error { return nil }); err == nil {
		t.Error("Register con id inválido no devolvió error")
	}
}
