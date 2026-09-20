package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/emusoi/bango"
)

func press(t *testing.T, m *model, keys ...string) {
	t.Helper()
	for _, key := range keys {
		var msg tea.KeyMsg
		switch key {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEscape}
		case " ":
			msg = tea.KeyMsg{Type: tea.KeySpace}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
		m.key(msg)
	}
}

func twoSections() bango.Panel {
	return bango.Panel{
		Version: bango.Version, ID: "read", Title: "read", Empty: "nothing here",
		Sections: []bango.Section{
			{ID: "a", Label: "one.go", Layout: bango.LayoutLines, Rows: []bango.Row{
				{ID: "a1", Fields: []bango.Field{{Name: "code", Value: "   1   package main"}}},
				{ID: "a2", Mark: bango.MarkWaiting, Fields: []bango.Field{{Name: "code", Value: "   2 + func main() {"}}, Tone: bango.RowAdded},
				{ID: "a3", Fields: []bango.Field{{Name: "code", Value: "   3   }"}}},
			}},
			{ID: "b", Label: "two.go", Layout: bango.LayoutLines, Rows: []bango.Row{
				{ID: "b1", Fields: []bango.Field{{Name: "code", Value: "   1   package two"}}},
				{ID: "b2", Mark: bango.MarkDone, Fields: []bango.Field{{Name: "code", Value: "   2   // done"}}},
			}},
		},
	}
}

func at(t *testing.T, m *model) string {
	t.Helper()
	row, ok := m.selected()
	if !ok {
		return ""
	}
	return row.ID
}

// ] and [ cross a panel between the rows a producer marked, and stop at the
// ends rather than wrapping.
func TestTheTerminalWalksBetweenMarks(t *testing.T) {
	m := &model{panel: twoSections(), folded: map[string]bool{}, width: 80, height: 20}
	press(t, m, "]")
	if got := at(t, m); got != "a2" {
		t.Fatalf("] reached %q, not the first marked row", got)
	}
	press(t, m, "]")
	if got := at(t, m); got != "b2" {
		t.Fatalf("] reached %q, not the next marked row", got)
	}
	press(t, m, "]")
	if got := at(t, m); got != "b2" {
		t.Fatalf("] past the last mark moved to %q", got)
	}
	press(t, m, "[")
	if got := at(t, m); got != "a2" {
		t.Fatalf("[ reached %q", got)
	}
}

// The filter the terminal has always had still walks the same list the cursor
// does, now that folding sits between them.
func TestTheTerminalFiltersWhatItWalks(t *testing.T) {
	m := &model{panel: twoSections(), folded: map[string]bool{}, width: 80, height: 20}
	m.query = "package"
	if got := len(m.rows()); got != 2 {
		t.Fatalf("filtering to \"package\" left %d rows", got)
	}
	press(t, m, "]")
	if got := at(t, m); got != "a1" {
		t.Fatalf("] inside a filter reached %q", got)
	}
}

// The whole of it, in a terminal, through the model a person drives: a panel
// that opens another, an action that asks for something, the panel it lands
// back on, and the way out.
func TestATerminalDrivesAReviewEndToEnd(t *testing.T) {
	wrote := t.TempDir() + "/said"
	m := &model{
		width: 80, height: 24, folded: map[string]bool{},
		producer: []string{"printf", "%s", fileListPanel},
		opts:     options{transport: bango.Local()},
		panel:    mustPanel(t, fileListPanel),
	}

	// open the file under the cursor: its command prints a panel, so it opens
	press(t, m, "enter")
	if m.panel.ID != "one" {
		t.Fatalf("opening a file landed on %q", m.panel.ID)
	}
	if len(m.stack) != 1 {
		t.Fatalf("the panel it was opened from was not kept")
	}

	// ] to the line that wants something, then comment on it
	press(t, m, "]")
	row, _ := m.selected()
	if row.ID != "l2" {
		t.Fatalf("] reached %q", row.ID)
	}
	m.panel.Actions["comment"] = bango.Action{
		Key: "c", Label: "comment", Input: "what is wrong?", Verb: "sh",
		// A placeholder is a whole argument or it is nothing: that is what keeps
		// what a person typed out of a shell.
		Args: []string{"-c", `printf '%s %s' "$1" "$2" > ` + wrote, "sh", "{row}", "{input}"},
	}
	press(t, m, "c")
	if m.state != prompting {
		t.Fatalf("commenting did not ask for anything: state %v", m.state)
	}
	press(t, m, "o", "k", "enter")

	said, err := os.ReadFile(wrote)
	if err != nil {
		t.Fatalf("the action never ran: %v", err)
	}
	if string(said) != "l2 ok" {
		t.Fatalf("the action was given %q", said)
	}

	// an action that prints no panel stays on the panel it was run on
	if m.panel.ID != "one" {
		t.Fatalf("after commenting it is showing %q", m.panel.ID)
	}

	// and q goes back to where the file was opened from, then quits
	press(t, m, "q")
	if m.panel.ID != "files" {
		t.Fatalf("q reached %q", m.panel.ID)
	}
	if _, cmd := m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd == nil {
		t.Fatal("q at the bottom did not quit")
	}
}

func mustPanel(t *testing.T, body string) bango.Panel {
	t.Helper()
	var p bango.Panel
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// A panel embedded inside another panel's action must be one line: it travels
// as a JSON string, and a JSON string holds no newlines.
const oneFilePanel = `{"bango":1,"id":"one","title":"one.go","sections":[{"id":"s","layout":"lines","rows":[{"id":"l1","fields":[{"name":"code","value":"   1   package main"}]},{"id":"l2","mark":"waiting","tone":"added","actions":["comment"],"fields":[{"name":"code","value":"   2 + func main() {"}]}]}],"actions":{"comment":{"key":"c","label":"comment","verb":"true","args":["{row}"],"input":"what is wrong?"}}}`

var fileListPanel = `{"bango":1,"id":"files","title":"review","sections":[{"id":"f","rows":[{"id":"one.go","fields":[{"name":"file","value":"one.go"}],"actions":["open"]}]}],"actions":{"open":{"key":"⏎","label":"open","verb":"printf","args":["%s","` +
	strings.ReplaceAll(oneFilePanel, `"`, `\"`) + `"],"panel":"one"}}}`
