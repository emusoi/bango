package main

import (
	"io"
	"strings"
	"testing"
	"time"

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

func onePanel(title, row string) string {
	return `{"bango":1,"id":"s","title":"` + title + `","sections":[{"id":"x","rows":[` +
		`{"id":"` + row + `","fields":[{"name":"n","value":"` + row + `"}]}]}]}` + "\n"
}

func TestAPanelArrivesBeforeTheProducerIsDone(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()
	panels := newStream(r, "")

	go func() { w.Write([]byte(onePanel("first", "a"))) }()

	got := make(chan bango.Panel, 1)
	go func() {
		p, err := panels.next()
		if err != nil {
			t.Error(err)
			return
		}
		got <- p
	}()

	select {
	case p := <-got:
		if p.Title != "first" {
			t.Fatalf("title = %q", p.Title)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nothing was decoded while the producer held the pipe open")
	}

	go func() { w.Write([]byte(onePanel("second", "b"))) }()
	next, err := panels.next()
	if err != nil || next.Title != "second" {
		t.Fatalf("the second panel = %q %v", next.Title, err)
	}
}

func TestABrokenDocumentIsReportedAndTheGoodOneSurvives(t *testing.T) {
	panels := newStream(strings.NewReader(onePanel("good", "a")+"{\"bango\":1,\"id\":\"\"}\n"), "")
	first, err := panels.next()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := panels.next(); err == nil {
		t.Fatal("an invalid panel must be reported")
	}
	if first.Title != "good" {
		t.Errorf("the good panel was lost: %q", first.Title)
	}
}

func TestARedrawKeepsTheCursorOnItsRow(t *testing.T) {
	m := newModel(twoRows(), options{}, nil)
	typeIn(m, "j")
	if row, _ := m.selected(); row.ID != "b" {
		t.Fatalf("cursor is on %q, want b", row.ID)
	}

	moved := twoRows()
	moved.Sections[0].Rows = []bango.Row{
		{ID: "c", Fields: []bango.Field{{Name: "n", Value: "gamma"}}},
		moved.Sections[0].Rows[1],
		moved.Sections[0].Rows[0],
	}
	m.replace(moved)
	if row, _ := m.selected(); row.ID != "b" {
		t.Errorf("after a redraw the cursor is on %q, want b", row.ID)
	}

	m.replace(bango.Panel{Version: 1, ID: "p", Title: "t"})
	if m.cursor != 0 {
		t.Errorf("a panel with no rows left the cursor at %d", m.cursor)
	}
}
