package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestSearchResultsNavigateAndSelect(t *testing.T) {
	m := NewSearchResults()
	m.Resize(40, 5)
	m.SetResults([]RepoMatch{
		{Path: "a.txt", Line: 0, Col: 5, Text: "hola mundo"},
		{Path: "b.txt", Line: 9, Col: 0, Text: "otra"},
	}, "mundo")
	if m.Len() != 2 {
		t.Fatalf("Len = %d, esperaba 2", m.Len())
	}
	if sel, _ := m.Selected(); sel.Path != "a.txt" {
		t.Fatalf("selected = %q, esperaba a.txt", sel.Path)
	}
	handled, activate := m.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if !handled || activate {
		t.Fatal("Down debe navegar sin activar")
	}
	if sel, _ := m.Selected(); sel.Path != "b.txt" {
		t.Fatalf("tras Down selected = %q, esperaba b.txt", sel.Path)
	}
	handled, activate = m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !handled || !activate {
		t.Fatal("Enter debe activar la coincidencia")
	}
	handled, _ = m.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if handled {
		t.Fatal("una tecla ajena no es del menú")
	}
}

func TestSearchResultsEmpty(t *testing.T) {
	m := NewSearchResults()
	m.Resize(40, 5)
	m.SetResults(nil, "nada")
	if m.Len() != 0 {
		t.Fatal("vacío debe dar 0")
	}
	if _, ok := m.Selected(); ok {
		t.Fatal("vacío no debe seleccionar nada")
	}
	handled, activate := m.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if handled || activate {
		t.Fatal("Enter en vacío no debe activar")
	}
}
