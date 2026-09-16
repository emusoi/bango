package bango

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden renderings")

type shot struct {
	name   string
	style  Style
	suffix string
}

func shots() []shot {
	return []shot{
		{"confirm", Style{Width: 72, Cursor: "t7"}, ""},
		{"confirm", Style{Width: 40, Cursor: "t7"}, ".narrow"},
		{"tree", Style{Width: 72, Folded: map[string]bool{"server/src/lib": true}}, ""},
		{"marks", Style{Width: 72}, ""},
		{"marks", Style{Width: 72, ASCII: true}, ".ascii"},
		{"empty", Style{Width: 72}, ""},
	}
}

func load(t *testing.T, name string) Panel {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("fixtures", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var p Panel
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if err := Validate(&p); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTargets(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFixturesRenderAsRecorded(t *testing.T) {
	for _, s := range shots() {
		name := s.name + s.suffix
		t.Run(name, func(t *testing.T) {
			got := strings.Join(Render(load(t, s.name), s.style), "\n") + "\n"
			golden := filepath.Join("fixtures", name+".txt")
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Fatalf("rendering changed\n--- want ---\n%s\n--- got ---\n%s", want, got)
			}
		})
	}
}

func TestFixturesNeverExceedTheirWidth(t *testing.T) {
	for _, s := range shots() {
		for _, line := range Render(load(t, s.name), s.style) {
			if cells(line) > s.style.Width {
				t.Fatalf("%s at %d: %q is %d", s.name, s.style.Width, line, cells(line))
			}
		}
	}
}
