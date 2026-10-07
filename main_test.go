package main

import (
	"strings"
	"testing"

	"github.com/leav-dev/tcode/internal/update"
)

// TestVersionLineMuestraLaVersion: con versión inyectada la imprime;
// sin ella marca dev.
func TestVersionLineMuestraLaVersion(t *testing.T) {
	update.SetVersion("preview-v0.1.3")
	defer update.SetVersion("")
	if got := versionLine(); got != "tcode preview-v0.1.3" {
		t.Fatalf("versionLine = %q, esperaba el tag", got)
	}
	update.SetVersion("")
	if got := versionLine(); !strings.Contains(got, "dev") {
		t.Fatalf("versionLine sin versión = %q, esperaba marca dev", got)
	}
}

// TestSuggestFlagProponePorPrefijo: typos y prefijos proponen el comando;
// sin nada cercano no propone.
func TestSuggestFlagProponePorPrefijo(t *testing.T) {
	cases := []struct{ in, want string }{
		{"--versoin", "--version"},
		{"--vers", "--version"},
		{"-v", "--version"},
		{"--install", "--install-extension"},
		{"--list", "--list-extensions"},
		{"--remove", "--remove-extension"},
		{"--add", "--add-provider"},
		{"--approve", "--approve-provider"},
		{"--upd", "update"},
		{"--hep", "--help"},
		{"--zzz", ""},
		{"-", ""},
	}
	for _, c := range cases {
		if got := suggestFlag(c.in); got != c.want {
			t.Errorf("suggestFlag(%q) = %q, esperaba %q", c.in, got, c.want)
		}
	}
}

// TestGuideCubreLosComandos: la guía lista cada flag del switch más update.
func TestGuideCubreLosComandos(t *testing.T) {
	joined := strings.Join(commandGuideLines, "\n")
	for _, cmd := range []string{
		flagHelp, flagVersion, flagInstallExtension, flagListExtensions,
		flagRemoveExtension, flagAddProvider, flagApproveProvider, "update",
		"uninstall",
	} {
		if !strings.Contains(joined, cmd) {
			t.Errorf("la guía no menciona %s", cmd)
		}
	}
}
