package bango

import (
	"strings"
	"testing"
)

func panelFixture() Panel {
	return Panel{
		Version: Version, ID: "confirm",
		Title:    "loco · #3386 Retry the workflow login bypass",
		Subtitle: "4/9 read · 3 open · 2 to confirm",
		Hints:    []string{"⏎ open", "y confirm", "? all"},
		Sections: []Section{{ID: "waiting", Label: "to confirm", Rows: []Row{
			{ID: "t7", Mark: MarkWaiting, Note: "claude",
				Fields: []Field{
					{Name: "where", Value: "src/auth/token.ts:42", Kind: KindPath},
					{Name: "type", Value: "issue"},
					{Name: "what", Value: "off-by-one on expiry"},
					{Name: "age", Value: "18m", Kind: KindTime},
				},
				Facts:   []string{"claimed  used <= ; added a test"},
				Preview: []string{"-   if (exp < now) {", "+   if (exp <= now) {"},
				Actions: []string{"confirm"}},
			{ID: "t12", Mark: MarkWaiting, Note: "codex",
				Fields: []Field{
					{Name: "where", Value: "src/session/index.ts:88", Kind: KindPath},
					{Name: "type", Value: "issue"},
					{Name: "what", Value: "swallowed error"},
					{Name: "age", Value: "2h", Kind: KindTime},
				}},
		}}},
		Actions: map[string]Action{
			"confirm": {Key: "y", Label: "confirm", Verb: "loco", Args: []string{"resolve", "{row}"}},
		},
	}
}

func TestValidateAcceptsFixture(t *testing.T) {
	p := panelFixture()
	if err := Validate(&p); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNamesTheField(t *testing.T) {
	cases := map[string]func(*Panel){
		"bango":                    func(p *Panel) { p.Version = 7 },
		"id":                       func(p *Panel) { p.ID = "has space" },
		"sections":                 func(p *Panel) { p.Sections = nil },
		"sections[0].rows[0].id":   func(p *Panel) { p.Sections[0].Rows[1].ID = "t7" },
		"sections[0].rows[0].mark": func(p *Panel) { p.Sections[0].Rows[0].Mark = "sparkling" },
		"actions.confirm.key":      func(p *Panel) { p.Sections[0].Rows[0].Actions = nil; a := p.Actions["confirm"]; a.Key = "q"; p.Actions["confirm"] = a },
	}
	for want, breakIt := range cases {
		p := panelFixture()
		breakIt(&p)
		err := Validate(&p)
		if err == nil {
			t.Fatalf("%s: expected a failure", want)
		}
		if !strings.HasPrefix(err.(Invalid).Path, strings.Split(want, "[")[0]) {
			t.Fatalf("%s: got %q", want, err.Error())
		}
	}
}

func TestMiddleTruncationKeepsTheBasename(t *testing.T) {
	got := Fit("web-client/lib/utils/index.ts", KindPath, 20)
	if !strings.HasSuffix(got, "index.ts") {
		t.Fatalf("basename lost: %q", got)
	}
	if cells(got) != 20 {
		t.Fatalf("width %d, want 20: %q", cells(got), got)
	}
}

func TestCountsRightAlign(t *testing.T) {
	if got := Fit("7", KindCount, 4); got != "   7" {
		t.Fatalf("got %q", got)
	}
}

func TestTimeNeverTruncates(t *testing.T) {
	if got := Fit("18m", KindTime, 2); got != "18m" {
		t.Fatalf("got %q", got)
	}
}

func TestNarrowWindowStillFits(t *testing.T) {
	p := panelFixture()
	for _, width := range []int{40, 60, 72, 120} {
		for _, line := range Render(p, Style{Width: width}) {
			if cells(line) > width {
				t.Fatalf("width %d: line of %d: %q", width, cells(line), line)
			}
		}
	}
}

func TestCursorShowsFactsAndPreview(t *testing.T) {
	p := panelFixture()
	out := strings.Join(Render(p, Style{Width: 72, Cursor: "t7"}), "\n")
	if !strings.Contains(out, "claimed  used <=") {
		t.Fatal("facts missing under the cursor")
	}
	out = strings.Join(Render(p, Style{Width: 72}), "\n")
	if strings.Contains(out, "claimed  used <=") {
		t.Fatal("facts shown with no cursor")
	}
}

func TestFilterHidesEmptySections(t *testing.T) {
	p := Filter(panelFixture(), "swallowed")
	if len(p.Sections) != 1 || len(p.Sections[0].Rows) != 1 {
		t.Fatalf("got %d sections", len(p.Sections))
	}
	if p.Sections[0].Rows[0].ID != "t12" {
		t.Fatalf("kept the wrong row")
	}
}

func TestEmptyPanelSaysSo(t *testing.T) {
	p := panelFixture()
	p.Empty = "nothing filed yet"
	out := strings.Join(Render(p, Style{Width: 72, Query: "zzzz"}), "\n")
	if !strings.Contains(out, "nothing filed yet") {
		t.Fatal("empty message missing")
	}
}

func TestAsciiMarks(t *testing.T) {
	if MarkWaiting.Glyph(true) != "!" || MarkWaiting.Glyph(false) != "⏎" {
		t.Fatal("mark sets disagree")
	}
}
