package ext

import (
	"os"
	"path/filepath"
	"testing"
)

// availableFixture arma una novedad con proveedor fijo para los tests de
// vistos: lo único que importa es la clave proveedor/id@versión.
func availableFixture(provider, id, version string) AvailableExt {
	return AvailableExt{Provider: Provider{Name: provider}, ID: id, Version: version}
}

// TestFilterUnseenKeepsOnlyNew: lo ya listado no vuelve a contar para el
// aviso; lo nunca visto pasa.
func TestFilterUnseenKeepsOnlyNew(t *testing.T) {
	available := []AvailableExt{
		availableFixture("a", "tcode.linter", "1.0.0"),
		availableFixture("a", "tcode.tema", "2.0.0"),
	}
	seen := Seen{SeenKey("a", "tcode.linter", "1.0.0"): true}

	got := FilterUnseen(available, seen)
	if len(got) != 1 || got[0].ID != "tcode.tema" {
		t.Fatalf("FilterUnseen = %+v, esperaba solo tcode.tema", got)
	}
}

// TestFilterUnseenBumpReavisa: un bump de versión cambia la clave, así que la
// novedad vuelve a avisar aunque el id ya se haya listado.
func TestFilterUnseenBumpReavisa(t *testing.T) {
	available := []AvailableExt{availableFixture("a", "tcode.linter", "1.1.0")}
	seen := Seen{SeenKey("a", "tcode.linter", "1.0.0"): true}

	if got := FilterUnseen(available, seen); len(got) != 1 {
		t.Fatalf("FilterUnseen = %+v, esperaba la versión nueva", got)
	}
}

// TestFilterUnseenWithoutSeenKeepsAll: sin estado, todo es nuevo.
func TestFilterUnseenWithoutSeenKeepsAll(t *testing.T) {
	available := []AvailableExt{availableFixture("a", "tcode.linter", "1.0.0")}

	if got := FilterUnseen(available, nil); len(got) != 1 {
		t.Fatalf("FilterUnseen sin seen = %+v, esperaba todo", got)
	}
}

// TestMarkSeenRegisters: marcar deja las claves en el conjunto, y con otro
// proveedor el mismo id no se tapa (la clave lleva proveedor).
func TestMarkSeenRegisters(t *testing.T) {
	seen := MarkSeen(nil, []AvailableExt{
		availableFixture("a", "tcode.linter", "1.0.0"),
		availableFixture("b", "tcode.linter", "1.0.0"),
	})

	if !seen[SeenKey("a", "tcode.linter", "1.0.0")] || !seen[SeenKey("b", "tcode.linter", "1.0.0")] {
		t.Fatalf("seen = %+v, esperaba ambas claves namespacedas", seen)
	}
}

// TestPruneSeenDropsInstalled: lo instalado deja de guardarse; lo demás (aun
// de un proveedor caído que hoy no lista) se conserva para no re-avisar.
func TestPruneSeenDropsInstalled(t *testing.T) {
	seen := Seen{
		SeenKey("a", "tcode.linter", "1.0.0"): true,
		SeenKey("a", "tcode.tema", "2.0.0"):   true,
	}
	installed := []Info{{ID: "tcode.linter", Version: "1.0.0", Provider: "a"}}

	PruneSeen(seen, installed)
	if seen[SeenKey("a", "tcode.linter", "1.0.0")] {
		t.Fatalf("seen = %+v, la instalada debió podarse", seen)
	}
	if !seen[SeenKey("a", "tcode.tema", "2.0.0")] {
		t.Fatalf("seen = %+v, la no instalada debió conservarse", seen)
	}
}

// TestUnseenAvailableDerivesFromSnapshot: el snapshot deriva las no vistas con
// el mismo criterio del filtro.
func TestUnseenAvailableDerivesFromSnapshot(t *testing.T) {
	snap := Snapshot{
		Providers: []Provider{{Name: "a"}},
		Catalogs: map[string][]ProviderExt{
			"a": {
				{ID: "tcode.linter", Version: "1.0.0"},
				{ID: "tcode.tema", Version: "2.0.0"},
			},
		},
	}
	seen := Seen{SeenKey("a", "tcode.linter", "1.0.0"): true}

	if got := snap.UnseenAvailable(seen); len(got) != 1 || got[0].ID != "tcode.tema" {
		t.Fatalf("UnseenAvailable = %+v, esperaba solo tcode.tema", got)
	}
}

// TestSeenFileRoundTrip: lo guardado vuelve igual y ordenado en disco.
func TestSeenFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".tcode", "extensions-seen.json")
	seen := Seen{
		SeenKey("b", "tcode.tema", "2.0.0"):   true,
		SeenKey("a", "tcode.linter", "1.0.0"): true,
	}
	if err := SaveSeenFile(path, seen); err != nil {
		t.Fatalf("SaveSeenFile: %v", err)
	}

	loaded, err := LoadSeenFile(path)
	if err != nil {
		t.Fatalf("LoadSeenFile: %v", err)
	}
	if len(loaded) != 2 || !loaded[SeenKey("a", "tcode.linter", "1.0.0")] || !loaded[SeenKey("b", "tcode.tema", "2.0.0")] {
		t.Fatalf("loaded = %+v, esperaba las dos claves", loaded)
	}
}

// TestLoadSeenFileMissingIsEmpty: sin archivo no hay error y no hay vistos.
func TestLoadSeenFileMissingIsEmpty(t *testing.T) {
	seen, err := LoadSeenFile(filepath.Join(t.TempDir(), "no-existe.json"))
	if err != nil {
		t.Fatalf("LoadSeenFile inexistente: %v", err)
	}
	if len(seen) != 0 {
		t.Fatalf("seen = %+v, esperaba vacío", seen)
	}
}

// TestPruneConservaVistosDeProveedorCaido: un proveedor ilegible (ni en
// disponibles ni en instaladas) no pierde sus vistos en el ciclo completo
// marcar → podar → guardar → cargar; la instalada sí se poda.
func TestPruneConservaVistosDeProveedorCaido(t *testing.T) {
	path := filepath.Join(t.TempDir(), "extensions-seen.json")
	seen := Seen{
		SeenKey("a", "tcode.linter", "1.0.0"): true, // se instala después
		SeenKey("b", "tcode.tema", "2.0.0"):   true, // proveedor caído
	}
	available := []AvailableExt{{Provider: Provider{Name: "b"}, ID: "tcode.tema", Version: "2.0.0"}}
	installed := []Info{{ID: "tcode.linter", Version: "1.0.0", Provider: "a"}}

	seen = PruneSeen(MarkSeen(seen, available), installed)
	if seen[SeenKey("a", "tcode.linter", "1.0.0")] {
		t.Fatalf("la instalada debió podarse: %+v", seen)
	}
	if !seen[SeenKey("b", "tcode.tema", "2.0.0")] {
		t.Fatalf("la del proveedor caído debió conservarse: %+v", seen)
	}
	if err := SaveSeenFile(path, seen); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSeenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded[SeenKey("b", "tcode.tema", "2.0.0")] || len(loaded) != 1 {
		t.Fatalf("round trip = %+v, esperaba solo la conservada", loaded)
	}
}

// TestLoadSeenFileCorruptReports: un archivo corrupto se reporta y devuelve
// vacío en vez de romper el arranque.
func TestLoadSeenFileCorruptReports(t *testing.T) {
	path := filepath.Join(t.TempDir(), "extensions-seen.json")
	if err := os.WriteFile(path, []byte("{no-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	seen, err := LoadSeenFile(path)
	if err == nil {
		t.Fatal("LoadSeenFile corrupto debió reportar error")
	}
	if len(seen) != 0 {
		t.Fatalf("seen = %+v, esperaba vacío ante corrupto", seen)
	}
}
