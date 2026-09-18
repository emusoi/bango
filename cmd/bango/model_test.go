package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/emusoi/bango"
)

func twoRows() bango.Panel {
	return bango.Panel{Version: 1, ID: "p", Title: "t", Sections: []bango.Section{{ID: "s", Rows: []bango.Row{
		{ID: "a", Fields: []bango.Field{{Name: "n", Value: "alpha"}}},
		{ID: "b", Fields: []bango.Field{{Name: "n", Value: "beta"}}},
	}}}}
}

func typeIn(m *model2, text string) {
	for _, r := range text {
		m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestTheEndOfNothingIsStillTheStart(t *testing.T) {
	m := newModel(twoRows(), options{}, nil)

	typeIn(m, "/")
	typeIn(m, "zzz")
	m.key(tea.KeyMsg{Type: tea.KeyEsc})
	typeIn(m, "G")
	if m.cursor < 0 {
		t.Fatalf("the cursor went to %d on a list with nothing in it", m.cursor)
	}

	typeIn(m, "/")
	for range "zzz" {
		m.key(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m.key(tea.KeyMsg{Type: tea.KeyEsc})
	if row, ok := m.selected(); !ok || row.ID == "" {
		t.Fatalf("selected = %q %v after the filter was cleared", row.ID, ok)
	}
}
