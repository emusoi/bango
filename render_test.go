package bango

import (
	"encoding/json"
	"fmt"
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
		"sections[0].rows[0].id":   func(p *Panel) { p.Sections[0].Rows[1].ID = "t7" },
		"sections[0].rows[0].mark": func(p *Panel) { p.Sections[0].Rows[0].Mark = "sparkling" },
		"actions.confirm.key": func(p *Panel) {
			p.Sections[0].Rows[0].Actions = nil
			a := p.Actions["confirm"]
			a.Key = "q"
			p.Actions["confirm"] = a
		},
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

func TestOneKeyMayMeanTwoThingsOnDifferentRows(t *testing.T) {
	p := panelFixture()
	p.Actions["forget"] = Action{Key: "y", Label: "forget", Verb: "true", Args: []string{"{row}"}}
	p.Sections[0].Rows[1].Actions = []string{"forget"}
	if err := Validate(&p); err != nil {
		t.Fatalf("disjoint rows may share a key: %v", err)
	}
	p.Sections[0].Rows[1].Actions = []string{"confirm", "forget"}
	if err := Validate(&p); err == nil {
		t.Fatal("one row with two actions on the same key is ambiguous")
	}
}

func TestGlobalKeysCollideWithEveryRow(t *testing.T) {
	p := panelFixture()
	p.Actions["refresh"] = Action{Key: "y", Label: "refresh", Verb: "true", Global: true}
	if err := Validate(&p); err == nil {
		t.Fatal("a global action shares a key with a row action")
	}
}

func TestOrderMayNameASectionThatIsNotHere(t *testing.T) {
	p := panelFixture()
	p.Order = []string{"waiting", "archived", "open"}
	if err := Validate(&p); err != nil {
		t.Fatalf("order is a preference, not a claim: %v", err)
	}
	first := p.orderedSections()[0]
	if first.ID != "waiting" {
		t.Fatalf("ordering broke: %s", first.ID)
	}
}

func TestAPanelWithNoSectionsIsAnEmptyState(t *testing.T) {
	p := Panel{Version: Version, ID: "dashboard", Title: "worktrees",
		Empty: "no worktrees yet — `mia new <branch>`"}
	if err := Validate(&p); err != nil {
		t.Fatalf("a panel may have nothing in it: %v", err)
	}
	out := strings.Join(Render(p, Style{Width: 72}), "\n")
	if !strings.Contains(out, "no worktrees yet") {
		t.Fatal("the empty message is what an empty panel is for")
	}
}

func TestActionNamesAreStable(t *testing.T) {
	p := panelFixture()
	p.Actions["zzz"] = Action{Key: "z", Label: "z", Verb: "true"}
	p.Actions["aaa"] = Action{Key: "A", Label: "a", Verb: "true"}
	first := p.ActionNames()
	for i := 0; i < 20; i++ {
		got := p.ActionNames()
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("help reshuffles between renders: %v then %v", first, got)
			}
		}
	}
	if first[0] != "aaa" {
		t.Fatalf("not sorted: %v", first)
	}
}

func TestRetryIsOfferedForAMatchingRefusal(t *testing.T) {
	action := Action{Verb: "git", Args: []string{"branch", "-d", "{row}"},
		Retry: []Retry{{When: "not fully merged", Label: "delete anyway",
			Verb: "git", Args: []string{"branch", "-D", "{row}"}}}}
	if _, ok := action.RetryFor("error: the branch is not fully merged"); !ok {
		t.Fatal("a refusal that names its own fix was not matched")
	}
	if _, ok := action.RetryFor("permission denied"); ok {
		t.Fatal("an unrelated refusal offered a retry")
	}
}

func TestRetryWithNoConditionAlwaysMatches(t *testing.T) {
	action := Action{Retry: []Retry{{Label: "force", Verb: "true"}}}
	if _, ok := action.RetryFor("anything at all"); !ok {
		t.Fatal("an unconditional retry should match")
	}
}

func tallPanel(n int) Panel {
	p := Panel{Version: 1, ID: "tall", Title: "tall", Hints: []string{"q quit"},
		Sections: []Section{{ID: "s"}}}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("r%02d", i)
		p.Sections[0].Rows = append(p.Sections[0].Rows,
			Row{ID: id, Fields: []Field{{Name: "n", Value: id}}})
	}
	return p
}

func TestAPanelTallerThanTheWindowIsCutToIt(t *testing.T) {
	lines := Render(tallPanel(40), Style{Width: 30, Height: 12, Cursor: "r00"})
	if len(lines) != 12 {
		t.Fatalf("rendered %d lines into a window of 12:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if lines[0] != "tall" || lines[len(lines)-1] != "q quit" {
		t.Errorf("the title and the hints must stay pinned:\n%s", strings.Join(lines, "\n"))
	}
}

func TestTheCursorIsAlwaysInTheWindow(t *testing.T) {
	for _, id := range []string{"r00", "r19", "r39"} {
		lines := Render(tallPanel(40), Style{Width: 30, Height: 12, Cursor: id})
		if !strings.Contains(strings.Join(lines, "\n"), id) {
			t.Errorf("cursor %s is off screen:\n%s", id, strings.Join(lines, "\n"))
		}
	}
}

func TestNoHeightMeansNoWindow(t *testing.T) {
	lines := Render(tallPanel(40), Style{Width: 30, Cursor: "r00"})
	if len(lines) < 40 {
		t.Fatalf("without a height every row is drawn, got %d lines", len(lines))
	}
}

func TestScrollHoldsStillUntilTheCursorLeaves(t *testing.T) {
	const height, total = 10, 40
	top := 0
	for at := 0; at < height; at++ {
		if top = Scroll(top, at, height, total); top != 0 {
			t.Fatalf("moving to %d inside the window scrolled to %d", at, top)
		}
	}
	if top = Scroll(top, height, height, total); top != 1 {
		t.Fatalf("stepping one past the window scrolled to %d, want 1", top)
	}
	if top = Scroll(top, 3, height, total); top != 1 {
		t.Fatalf("a row still inside the window scrolled to %d, want 1", top)
	}
	if top = Scroll(top, 0, height, total); top != 0 {
		t.Fatalf("jumping above the window scrolled to %d, want 0", top)
	}
	if top = Scroll(top, 39, height, total); top != 30 {
		t.Fatalf("the last row put the window at %d, want 30", top)
	}
}

func withRetry(retry Retry) *Panel {
	return &Panel{Version: 1, ID: "p", Title: "t",
		Actions: map[string]Action{"go": {Key: "R", Label: "go", Verb: "echo", Retry: []Retry{retry}}},
		Sections: []Section{{ID: "s", Rows: []Row{
			{ID: "a", Actions: []string{"go"}, Fields: []Field{{Name: "n", Value: "x"}}}}}}}
}

func TestARetryIsHeldToTheSameBarAsTheActionItFollows(t *testing.T) {
	if err := Validate(withRetry(Retry{When: "denied", Label: "again", Verb: "echo"})); err != nil {
		t.Fatalf("a whole retry must pass: %v", err)
	}
	for _, one := range []struct {
		retry Retry
		want  string
	}{
		{Retry{When: "denied", Label: "again"}, "actions.go.retry[0].verb"},
		{Retry{When: "denied", Verb: "echo"}, "actions.go.retry[0].label"},
	} {
		err := Validate(withRetry(one.retry))
		if err == nil {
			t.Errorf("%+v was accepted", one.retry)
			continue
		}
		if got := err.(Invalid).Path; got != one.want {
			t.Errorf("path = %q, want %q", got, one.want)
		}
	}
}

func TestATargetMustNameARowThatIsThere(t *testing.T) {
	p := &Panel{Version: 1, ID: "p", Title: "t", Sections: []Section{{ID: "s", Rows: []Row{
		{ID: "a", Target: "b", Fields: []Field{{Name: "n", Value: "x"}}},
		{ID: "b", Fields: []Field{{Name: "n", Value: "y"}}},
	}}}}
	if err := Validate(p); err != nil {
		t.Fatalf("a target naming a real row must pass: %v", err)
	}
	p.Sections[0].Rows[0].Target = "gone"
	err := Validate(p)
	if err == nil {
		t.Fatal("a target naming nothing was accepted")
	}
	if got := err.(Invalid).Path; got != "rows.a.target" {
		t.Errorf("path = %q", got)
	}
}

func TestWidthIsCountedInCellsNotRunes(t *testing.T) {
	if got := cells("東京"); got != 4 {
		t.Errorf("cells(東京) = %d, want 4", got)
	}
	if got := cells("café"); got != 4 {
		t.Errorf("cells(café) = %d, want 4", got)
	}
	if got := cells(Fit("東京タワーの支店", KindText, 9)); got > 9 {
		t.Errorf("Fit to 9 gave %d cells: %q", got, Fit("東京タワーの支店", KindText, 9))
	}
	if got := Fit("types/graphql/utils.ts:115", KindPath, 12); got != "utils.ts:115" {
		t.Errorf("a basename that fits was cut anyway: %q", got)
	}
	if got := cells(Fit("田中さん", KindCount, 10)); got != 10 {
		t.Errorf("a padded wide value is %d cells, want 10", got)
	}
}

func TestAnEmptyPanelOmitsItsSectionsRatherThanNullingThem(t *testing.T) {
	encoded, err := json.Marshal(Panel{Version: Version, ID: "empty", Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "null") {
		t.Errorf("a consumer that is not Go has to read this: %s", encoded)
	}
	var back Panel
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatal(err)
	}
	if err := Validate(&back); err != nil {
		t.Errorf("it must still be a panel: %v", err)
	}
}
