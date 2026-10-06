package ext

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// countingFetch arma un FetchFunc que sirve cada URL desde la carpeta local que
// sources mapea, y cuenta cuántas veces se pidió el patrón de manifests (la
// lectura liviana del catálogo de un proveedor). Es la lectura CARA —un partial
// clone contra GitHub— que el snapshot promete no repetir: el contador es lo
// que prueba el arreglo del I/O redundante.
//
// Los proveedores de los tests tienen Source REMOTO a propósito: un proveedor
// local se escanea directo del disco y nunca pasa por el fetcher, así que con
// fuentes locales el contador no vería nada.
func countingFetch(sources map[string]string, reads *int64) FetchFunc {
	return func(url, dest, pattern string) error {
		src, ok := sources[url]
		if !ok {
			return errors.New("origen desconocido: " + url)
		}
		if pattern == manifestsPattern {
			atomic.AddInt64(reads, 1)
		}
		return fetchFake(src)(url, dest, pattern)
	}
}

// TestLoadAllReadsEachProviderOnce es el arreglo del I/O redundante: con dos
// proveedores, LoadAll lee el catálogo de cada uno UNA vez. Antes,
// CheckUpdates + AvailableExtensions los leían dos veces (7,05 s medidos).
func TestLoadAllReadsEachProviderOnce(t *testing.T) {
	srcA := providerFixture(t, map[string][3]string{"linter": {"tcode.linter", "Linter", "1.0.0"}})
	srcB := providerFixture(t, map[string][3]string{"tema": {"tcode.tema", "Tema", "2.0.0"}})

	var reads int64
	snap, errs := LoadAll(
		[]Provider{
			{Name: "a", Source: "https://example.com/a.git", Approved: true},
			{Name: "b", Source: "https://example.com/b.git", Approved: true},
		},
		t.TempDir(),
		countingFetch(map[string]string{
			"https://example.com/a.git": srcA,
			"https://example.com/b.git": srcB,
		}, &reads),
	)
	if len(errs) != 0 {
		t.Fatalf("LoadAll reportó errores: %v", errs)
	}
	if got := atomic.LoadInt64(&reads); got != 2 {
		t.Fatalf("lecturas de catálogo = %d, esperaba 2 (una por proveedor)", got)
	}
	if len(snap.Catalogs["a"]) != 1 || len(snap.Catalogs["b"]) != 1 {
		t.Fatalf("catálogos = %+v, esperaba uno por proveedor", snap.Catalogs)
	}

	// Y derivar las dos vistas NO vuelve a leer: es el mismo snapshot.
	updates, uErrs := snap.Updates()
	if len(uErrs) != 0 {
		t.Fatalf("Updates reportó errores: %v", uErrs)
	}
	if len(updates) != 0 {
		t.Errorf("Updates = %+v, esperaba ninguna con nada instalado", updates)
	}
	if got := len(snap.Available()); got != 2 {
		t.Errorf("novedades = %d, esperaba 2 (una de cada proveedor)", got)
	}
	if got := atomic.LoadInt64(&reads); got != 2 {
		t.Fatalf("derivar volvió a leer los proveedores: %d lecturas, esperaba 2", got)
	}
}

// TestSnapshotUpdatesAreDerived: las actualizaciones salen de comparar el
// catálogo cacheado contra lo instalado, con el mismo resultado que
// CheckUpdates (ref, versión vieja y nueva).
func TestSnapshotUpdatesAreDerived(t *testing.T) {
	src := providerFixture(t, map[string][3]string{"linter": {"tcode.linter", "Linter", "1.0.0"}})
	if err := os.WriteFile(filepath.Join(src, "linter", "main.lua"), []byte("viejo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main.lua: %v", err)
	}
	p := remoteProviderForUpdate("remoto")
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.linter", []Provider{p}, userRoot, fetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}
	bumpVersion(t, src, "linter", "tcode.linter", "Linter", "1.1.0", "nuevo\n")

	var reads int64
	snap, errs := LoadAll([]Provider{p}, userRoot, countingFetch(map[string]string{p.Source: src}, &reads))
	if len(errs) != 0 {
		t.Fatalf("LoadAll reportó errores: %v", errs)
	}

	updates, uErrs := snap.Updates()
	if len(uErrs) != 0 {
		t.Fatalf("Updates reportó errores: %v", uErrs)
	}
	if len(updates) != 1 {
		t.Fatalf("Updates = %+v, esperaba 1", updates)
	}
	if updates[0].Ref != "remoto/tcode.linter" || updates[0].OldVer != "1.0.0" || updates[0].NewVer != "1.1.0" {
		t.Errorf("UpdateResult inesperado: %+v", updates[0])
	}
	// Detectar no aplica nada: la instalación sigue en la versión vieja.
	manifest, err := os.ReadFile(filepath.Join(userRoot, "remoto", "tcode.linter", "extension.json"))
	if err != nil {
		t.Fatalf("leyendo el manifest instalado: %v", err)
	}
	if !strings.Contains(string(manifest), "1.0.0") {
		t.Errorf("Updates aplicó la actualización: el manifest ya dice %s", manifest)
	}
}

// TestSnapshotAvailableAreDerived: las novedades son el catálogo menos lo
// instalado; la que ya está instalada (y al día) no se ofrece como novedad.
func TestSnapshotAvailableAreDerived(t *testing.T) {
	src := providerFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.0.0"},
		"tema":   {"tcode.tema", "Tema", "2.0.0"},
	})
	p := remoteProviderForUpdate("remoto")
	userRoot := t.TempDir()
	if _, err := InstallByID("tcode.linter", []Provider{p}, userRoot, fetchFake(src), nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}

	snap, errs := LoadAll([]Provider{p}, userRoot, fetchFake(src))
	if len(errs) != 0 {
		t.Fatalf("LoadAll reportó errores: %v", errs)
	}
	available := snap.Available()
	if len(available) != 1 {
		t.Fatalf("novedades = %+v, esperaba solo la del tema", available)
	}
	if available[0].ID != "tcode.tema" || available[0].Ref() != "remoto/tcode.tema" {
		t.Errorf("novedad inesperada: %+v", available[0])
	}
	if updates, _ := snap.Updates(); len(updates) != 0 {
		t.Errorf("Updates = %+v con las versiones iguales, esperaba ninguna", updates)
	}
}

// TestSnapshotReDerivesAfterActionWithoutRereading: instalar NO relee el
// proveedor —el catálogo no cambió, solo la lista local—, así que la instantánea
// que la ventana muestra se repinta al instante. El contador lo demuestra.
func TestSnapshotReDerivesAfterActionWithoutRereading(t *testing.T) {
	src := providerFixture(t, map[string][3]string{
		"linter": {"tcode.linter", "Linter", "1.0.0"},
		"tema":   {"tcode.tema", "Tema", "2.0.0"},
	})
	p := remoteProviderForUpdate("remoto")
	userRoot := t.TempDir()

	var reads int64
	fetch := countingFetch(map[string]string{p.Source: src}, &reads)
	snap, errs := LoadAll([]Provider{p}, userRoot, fetch)
	if len(errs) != 0 {
		t.Fatalf("LoadAll reportó errores: %v", errs)
	}

	if _, err := InstallByID("tcode.tema", []Provider{p}, userRoot, fetch, nil); err != nil {
		t.Fatalf("InstallByID: %v", err)
	}
	// El contador se pone a cero DESPUÉS de instalar (instalar sí lee el
	// catálogo: resuelve el id); lo que se mide acá es que repintar la
	// ventana tras la acción no vuelve a pagarlo.
	atomic.StoreInt64(&reads, 0)

	// Solo List (local): el snapshot se actualiza en su campo Installed.
	infos, listErrs := List(userRoot)
	if len(listErrs) != 0 {
		t.Fatalf("List reportó errores: %v", listErrs)
	}
	snap.Installed = infos

	available := snap.Available()
	if len(available) != 1 || available[0].ID != "tcode.linter" {
		t.Errorf("novedades tras instalar el tema = %+v, esperaba solo el linter", available)
	}
	if got := atomic.LoadInt64(&reads); got != 0 {
		t.Fatalf("lecturas de catálogo = %d tras la acción, esperaba 0: derivar no puede releer", got)
	}
}

// TestLoadAllToleratesBrokenProvider: un repo caído no impide leer el resto, se
// reporta como error y su catálogo simplemente no está (las vistas derivadas lo
// saltan en silencio, sin repetir el mismo error).
func TestLoadAllToleratesBrokenProvider(t *testing.T) {
	src := providerFixture(t, map[string][3]string{"tema": {"tcode.tema", "Tema", "2.0.0"}})
	roto := Provider{Name: "caido", Source: "https://example.com/caido.git", Approved: true}
	bueno := Provider{Name: "bueno", Source: "https://example.com/bueno.git", Approved: true}

	var reads int64
	snap, errs := LoadAll([]Provider{roto, bueno}, t.TempDir(),
		countingFetch(map[string]string{bueno.Source: src}, &reads))
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "caido") {
		t.Fatalf("errores = %v, esperaba uno que nombrara al proveedor caído", errs)
	}
	if _, ok := snap.Catalogs["caido"]; ok {
		t.Error("un proveedor ilegible no puede tener catálogo")
	}
	if len(snap.Catalogs["bueno"]) != 1 {
		t.Fatalf("catálogo del proveedor sano = %+v, esperaba su extensión", snap.Catalogs["bueno"])
	}
	if got := len(snap.Available()); got != 1 {
		t.Errorf("novedades = %d, esperaba 1 (la del proveedor sano)", got)
	}
	if updates, uErrs := snap.Updates(); len(updates) != 0 || len(uErrs) != 0 {
		t.Errorf("Updates = %+v / %v con el único proveedor caído y nada instalado", updates, uErrs)
	}
}

// TestSnapshotUpdatesReportExtensionsWithoutSource: lo que se derivó no
// descarta los casos sin contra qué comparar: la instalación heredada sin
// proveedor y la que viene de un proveedor que ya no está registrado se
// reportan, y no cortan la revisión de las demás.
func TestSnapshotUpdatesReportExtensionsWithoutSource(t *testing.T) {
	snap := Snapshot{
		Providers: []Provider{},
		Catalogs:  map[string][]ProviderExt{},
		Installed: []Info{
			{ID: "plana", Version: "1.0.0"},
			{ID: "huerfana", Version: "1.0.0", Provider: "borrado"},
		},
	}
	updates, errs := snap.Updates()
	if len(updates) != 0 {
		t.Errorf("Updates = %+v, esperaba ninguna", updates)
	}
	if len(errs) != 2 {
		t.Fatalf("errores = %v, esperaba uno por extensión sin fuente", errs)
	}
	if !strings.Contains(errs[0].Error(), "no tiene proveedor") {
		t.Errorf("el primer error debería ser el de la instalación sin proveedor: %v", errs[0])
	}
	if !strings.Contains(errs[1].Error(), "borrado") {
		t.Errorf("el segundo error debería nombrar al proveedor que ya no está: %v", errs[1])
	}
}
