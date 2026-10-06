package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
)

// TestCompare ordena versiones por componentes numéricos, con la v opcional y
// el sufijo rompiendo el empate a favor de la pelada.
func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.1.0", "v0.1.1", -1},
		{"v0.1.1", "v0.1.0", 1},
		{"v0.1.0", "v0.1.0", 0},
		{"0.1.0", "v0.1.0", 0},
		{"v1.9.0", "v1.10.0", -1}, // numérico, no lexicográfico
		{"v1.2", "v1.2.0", 0},     // componente faltante = 0
		{"v1.2.3-rc1", "v1.2.3", -1},
		{"v1.2.3", "v1.2.3-rc1", 1},
		{"v1.2.3-rc1", "v1.2.3-rc1", 0},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, esperaba %d", c.a, c.b, got, c.want)
		}
	}
}

// TestNeedsUpdate solo avisa con current conocida y latest más nueva.
func TestNeedsUpdate(t *testing.T) {
	if !NeedsUpdate("v0.1.0", "v0.1.1") {
		t.Error("v0.1.1 sobre v0.1.0 debe avisar")
	}
	for _, tc := range [][2]string{{"v0.1.1", "v0.1.1"}, {"v0.1.1", "v0.1.0"}, {"", "v0.1.1"}, {"v0.1.0", ""}} {
		if NeedsUpdate(tc[0], tc[1]) {
			t.Errorf("NeedsUpdate(%q, %q) debe ser falso", tc[0], tc[1])
		}
	}
}

// TestCurrentVersionPrefiereLdflags: lo inyectado por el linker gana sobre el
// buildinfo; sin ninguno, vacía (dev).
func TestCurrentVersionPrefiereLdflags(t *testing.T) {
	oldV, oldF := version, debugReadBuildInfo
	defer func() { version, debugReadBuildInfo = oldV, oldF }()

	version = "v9.9.9"
	debugReadBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v0.0.1"}}, true
	}
	if got := CurrentVersion(); got != "v9.9.9" {
		t.Fatalf("CurrentVersion = %q, esperaba el ldflags", got)
	}

	version = ""
	debugReadBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	if got := CurrentVersion(); got != "" {
		t.Fatalf("CurrentVersion = %q, esperaba vacía sin fuentes", got)
	}
}

// fakeReleases levanta un fake de la API de releases (tag dado) y de la base
// de descarga (asset + checksums coherentes con el contenido).
func fakeReleases(t *testing.T, tag, asset string, bin []byte) string {
	t.Helper()
	sum := sha256.Sum256(bin)
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q}`, tag)
	})
	mux.HandleFunc("/download/"+asset, func(w http.ResponseWriter, r *http.Request) {
		w.Write(bin)
	})
	mux.HandleFunc("/download/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), asset)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

// pinBases apunta la API y la descarga al fake; el asset es el de la
// plataforma del test.
func pinBases(t *testing.T, url string) {
	t.Helper()
	oldAPI, oldDL := apiBase, downloadBase
	apiBase, downloadBase = url, url+"/download"
	t.Cleanup(func() { apiBase, downloadBase = oldAPI, oldDL })
}

// TestCheckLatestLeeElTag: el chequeo devuelve el tag_name del release latest.
func TestCheckLatestLeeElTag(t *testing.T) {
	name, err := assetName()
	if err != nil {
		t.Skipf("sin asset para esta plataforma: %v", err)
	}
	pinBases(t, fakeReleases(t, "v0.2.0", name, []byte("binario")))
	tag, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}
	if tag != "v0.2.0" {
		t.Fatalf("tag = %q, esperaba v0.2.0", tag)
	}
}

// TestCheckLatestTolerante: sin red (base que no responde) devuelve "" sin
// error: el chequeo nunca rompe nada.
func TestCheckLatestTolerante(t *testing.T) {
	oldAPI := apiBase
	apiBase = "http://127.0.0.1:1" // puerto cerrado: conexión rechazada
	defer func() { apiBase = oldAPI }()
	tag, err := CheckLatest(context.Background())
	if err != nil || tag != "" {
		t.Fatalf("CheckLatest = (%q, %v), esperaba (\"\", nil)", tag, err)
	}
}

// TestUpdateDescargaVerificaYReemplaza: baja el asset, lo valida contra el
// checksum y reemplaza el destino con modo ejecutable.
func TestUpdateDescargaVerificaYReemplaza(t *testing.T) {
	name, err := assetName()
	if err != nil {
		t.Skipf("sin asset para esta plataforma: %v", err)
	}
	pinBases(t, fakeReleases(t, "v0.2.0", name, []byte("nuevo-binario")))
	dest := filepath.Join(t.TempDir(), "tcode")
	if err := os.WriteFile(dest, []byte("viejo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Update(context.Background(), "v0.2.0", dest); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "nuevo-binario" {
		t.Fatalf("destino = %q, %v; esperaba el binario nuevo", got, err)
	}
	if info, err := os.Stat(dest); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("modo = %v, %v; esperaba 0755", info.Mode(), err)
	}
}

// TestUpdateRechazaChecksumRoto: si el binario no coincide con checksums.txt,
// no toca el destino.
func TestUpdateRechazaChecksumRoto(t *testing.T) {
	name, err := assetName()
	if err != nil {
		t.Skipf("sin asset para esta plataforma: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/download/"+name, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("binario-adulterado"))
	})
	mux.HandleFunc("/download/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "0000000000000000000000000000000000000000000000000000000000000000  "+name)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	pinBases(t, server.URL)

	dest := filepath.Join(t.TempDir(), "tcode")
	if err := os.WriteFile(dest, []byte("viejo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Update(context.Background(), "v0.2.0", dest); err == nil {
		t.Fatal("Update con checksum roto debe fallar")
	}
	if got, _ := os.ReadFile(dest); string(got) != "viejo" {
		t.Fatalf("el destino quedó tocado: %q", got)
	}
}
