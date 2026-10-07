package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leav-dev/tcode/internal/ext"
)

// pinSeenFile apunta el estado de novedades vistas a un temporal propio del
// test: lo listado persiste entre lecturas sin tocar el HOME real.
func pinSeenFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "extensions-seen.json")
	old := extSeenFilePath
	extSeenFilePath = func() string { return path }
	t.Cleanup(func() { extSeenFilePath = old })
	return path
}

// seenSnapshot arma una lectura con una sola novedad y nada instalado.
func seenSnapshot() ext.Snapshot {
	p := ext.Provider{Name: "remoto", Source: "https://example.com/remoto.git", Approved: true}
	return ext.Snapshot{
		Providers: []ext.Provider{p},
		Catalogs: map[string][]ext.ProviderExt{
			"remoto": {{ID: "tcode.tema", Name: "Tema", Version: "2.0.0", Subdir: "tema"}},
		},
	}
}

// TestSeenNovedadNoReavisa: lo ya listado no vuelve al aviso (sin falsos
// positivos), pero la ventana sigue listándolo y el visto persiste en disco.
func TestSeenNovedadNoReavisa(t *testing.T) {
	seenPath := pinSeenFile(t)
	app, _ := newTestApp(t, "uno")

	app.handleExtSnapshot(extSnapshotEvent{Seq: app.extPrefetchSeq, Snapshot: seenSnapshot()})
	if msg := app.statusBar.Message(); !strings.Contains(msg, "1 novedad") {
		t.Fatalf("mensaje = %q, esperaba el primer aviso de la novedad", msg)
	}

	// La ventana lista TODO aunque ya se haya avisado: el filtro es del aviso.
	if len(app.extAvailable) != 1 {
		t.Fatalf("disponibles = %+v, la ventana no se filtra", app.extAvailable)
	}
	// El visto quedó persistido con su versión.
	data, err := os.ReadFile(seenPath)
	if err != nil || !strings.Contains(string(data), "remoto/tcode.tema@2.0.0") {
		t.Fatalf("estado = %q, err = %v; esperaba la clave vista", data, err)
	}

	// Segunda lectura igual: silencio, sin falso positivo.
	app.statusBar.ClearMessage()
	app.handleExtSnapshot(extSnapshotEvent{Seq: app.extPrefetchSeq, Snapshot: seenSnapshot()})
	if msg := app.statusBar.Message(); msg != "" {
		t.Fatalf("mensaje = %q, lo ya listado no debe re-avisar", msg)
	}
	if len(app.extAvailable) != 1 {
		t.Fatalf("disponibles = %+v, la ventana sigue listando todo", app.extAvailable)
	}
}

// TestSeenBumpReavisa: una versión nueva de la misma extensión vuelve a avisar
// porque su clave cambia: es realmente nueva.
func TestSeenBumpReavisa(t *testing.T) {
	pinSeenFile(t)
	app, _ := newTestApp(t, "uno")

	app.handleExtSnapshot(extSnapshotEvent{Seq: app.extPrefetchSeq, Snapshot: seenSnapshot()})
	app.statusBar.ClearMessage()

	bumped := seenSnapshot()
	bumped.Catalogs["remoto"][0].Version = "2.1.0"
	app.handleExtSnapshot(extSnapshotEvent{Seq: app.extPrefetchSeq, Snapshot: bumped})
	if msg := app.statusBar.Message(); !strings.Contains(msg, "1 novedad") {
		t.Fatalf("mensaje = %q, el bump de versión debe re-avisar", msg)
	}
}
