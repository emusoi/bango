package main

import (
	"strings"
	"testing"

	"github.com/emusoi/bango"
)

func build(t *testing.T, input, spec string, key, section, run string) bango.Panel {
	t.Helper()
	cols, rows, err := readRows(strings.NewReader(input), spec)
	if err != nil {
		t.Fatal(err)
	}
	panel, err := tabulate(cols, rows, "table", "", "nothing", key, section, run)
	if err != nil {
		t.Fatal(err)
	}
	if err := bango.Validate(&panel); err != nil {
		t.Fatalf("the panel it built does not validate: %v", err)
	}
	return panel
}

func ids(p bango.Panel) []string {
	var out []string
	for _, row := range p.Rows() {
		out = append(out, row.ID)
	}
	return out
}

func TestTabsBecomeColumns(t *testing.T) {
	p := build(t, "web\tUp 3 days\tnginx\napi\tExited\tnode\n", "name,status,image:ref", "", "", "")
	if got := ids(p); len(got) != 2 || got[0] != "web" {
		t.Fatalf("ids = %v", got)
	}
	row, _ := p.Find("api")
	if len(row.Fields) != 3 || row.Fields[2].Kind != bango.KindRef {
		t.Errorf("fields = %+v", row.Fields)
	}
}

func TestJSONLinesBecomeColumnsWithoutBeingNamed(t *testing.T) {
	p := build(t, "{\"name\":\"web\",\"restarts\":0,\"ready\":true}\n{\"name\":\"api\",\"restarts\":12,\"ready\":false}\n", "", "", "", "")
	row, ok := p.Find("web")
	if !ok {
		t.Fatalf("keys are sorted, so name comes first and holds the id: %v", ids(p))
	}
	if len(row.Fields) != 3 {
		t.Errorf("every key becomes a column: %+v", row.Fields)
	}
	for _, field := range row.Fields {
		if field.Name == "restarts" && field.Value != "0" {
			t.Errorf("a whole number came back as %q", field.Value)
		}
	}
}

func TestAChosenColumnHoldsTheRowId(t *testing.T) {
	p := build(t, "web\tnginx\napi\tnode\n", "name,image", "image", "", "")
	if got := ids(p); got[0] != "nginx" {
		t.Errorf("ids = %v, want the image column", got)
	}
}

func TestARepeatedIdIsStillUnique(t *testing.T) {
	p := build(t, "alice\tone\nbob\ttwo\nalice\tthree\n", "who,what", "", "", "")
	got := ids(p)
	if len(got) != 3 || got[0] != "alice" || got[2] != "alice-2" {
		t.Errorf("ids = %v", got)
	}
}

func TestGroupingConsumesTheColumnItGroupsBy(t *testing.T) {
	p := build(t, "web\tdefault\napi\tdefault\netcd\tkube-system\n", "pod,ns", "", "ns", "")
	if len(p.Sections) != 2 {
		t.Fatalf("sections = %d", len(p.Sections))
	}
	if p.Sections[0].Label != "default" || p.Sections[1].Label != "kube-system" {
		t.Errorf("labels = %q %q", p.Sections[0].Label, p.Sections[1].Label)
	}
	row, _ := p.Find("web")
	for _, field := range row.Fields {
		if field.Name == "ns" {
			t.Error("the grouping column is the section label, not a field on every row")
		}
	}
}

func TestAPickRunsWhatWasAskedFor(t *testing.T) {
	p := build(t, "web\tnginx\n", "name,image", "", "", "docker logs {row}")
	pick := p.Actions["pick"]
	if pick.Verb != "docker" || strings.Join(pick.Args, " ") != "logs {row}" {
		t.Errorf("pick = %+v", pick)
	}
	plain := build(t, "web\tnginx\n", "name,image", "", "", "")
	if len(plain.Actions) != 0 {
		t.Errorf("without --run there is nothing to run, so nothing is declared: %+v", plain.Actions)
	}
	for _, row := range plain.Rows() {
		if len(row.Actions) != 0 {
			t.Errorf("a row cannot offer an action the panel does not have: %+v", row.Actions)
		}
	}
}

func TestAValueWithATabOrANewlineIsStillOneLine(t *testing.T) {
	p := build(t, "{\"a\":\"two\\nlines\",\"b\":\"x\"}\n", "", "", "", "")
	for _, row := range p.Rows() {
		for _, field := range row.Fields {
			if strings.ContainsAny(field.Value, "\n\r\t") {
				t.Errorf("field %q kept a line break: %q", field.Name, field.Value)
			}
		}
	}
}

func TestWhatItRefusesItNames(t *testing.T) {
	if _, _, err := readRows(strings.NewReader("a\tb\n"), "name,image:nonsense"); err == nil {
		t.Error("an unknown kind must be refused")
	}
	cols, rows, _ := readRows(strings.NewReader("a\tb\n"), "name,image")
	if _, err := tabulate(cols, rows, "table", "", "", "gone", "", ""); err == nil {
		t.Error("--key naming no column must be refused")
	}
	if _, err := tabulate(cols, rows, "table", "", "", "", "gone", ""); err == nil {
		t.Error("--section naming no column must be refused")
	}
}

func TestNoRowsIsAnEmptyPanelNotAFailure(t *testing.T) {
	p := build(t, "", "name", "", "", "")
	if len(p.Sections) != 0 {
		t.Errorf("sections = %d", len(p.Sections))
	}
	if p.Empty == "" {
		t.Error("an empty panel needs something to say")
	}
}
