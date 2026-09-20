package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/emusoi/bango"
	"github.com/muesli/termenv"
)

// Every rune of the value comes out, once, in order, whatever is painted over
// parts of it. A renderer that drops or doubles a character is worse than one
// that draws it plain.
func TestATintedValueIsStillTheValue(t *testing.T) {
	// A test has no terminal, and lipgloss draws no colour for one that cannot
	// show it. The question here is what it would paint, not what this pipe can.
	was := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(was)

	paint := colours()
	base := lipgloss.NewStyle()
	value := "return t.Expires.Unix() >= now.Unix()"
	spans := []bango.Span{
		{From: 0, To: 6, Tone: bango.ToneKeyword},
		{From: 7, To: 16, Tone: bango.ToneName},
		{From: 24, To: 26, Tone: bango.ToneChanged},
	}
	got := tinted(value, spans, base, paint)
	if stripped := strip(got); stripped != value {
		t.Fatalf("painting changed the text:\n want %q\n  got %q", value, stripped)
	}
	if got == value {
		t.Fatal("nothing was painted")
	}
}

// A span that runs off the end of a value it no longer matches — because the
// value was cut to the width — paints what is left and stops.
func TestATintedValueSurvivesBeingCut(t *testing.T) {
	paint := colours()
	base := lipgloss.NewStyle()
	spans := []bango.Span{{From: 0, To: 6, Tone: bango.ToneKeyword}, {From: 20, To: 40, Tone: bango.ToneString}}
	got := tinted("return x", spans, base, paint)
	if stripped := strip(got); stripped != "return x" {
		t.Fatalf("got %q", stripped)
	}
}

// A value nobody said anything about is written out as it is.
func TestAValueWithoutSpansIsUntouched(t *testing.T) {
	got := tinted("plain text", nil, lipgloss.NewStyle(), colours())
	if got != "plain text" {
		t.Fatalf("got %q", got)
	}
}

func strip(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

// What a row is shows even where there is no background to show it on: a
// terminal says it in the ink.
func TestARowSaysWhatItIsInATerminal(t *testing.T) {
	was := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(was)

	field := bango.Field{Name: "code", Value: "   42 +     return true"}
	plain := bango.Row{ID: "a", Fields: []bango.Field{field}}
	added := bango.Row{ID: "b", Fields: []bango.Field{field}, Tone: bango.RowAdded}
	removed := bango.Row{ID: "c", Fields: []bango.Field{field}, Tone: bango.RowRemoved}

	m := &model{width: 80}
	paint := colours()
	draw := func(row bango.Row) string {
		return m.paintRow(bango.Line{Row: row, Verbatim: true}, nil, "", nil, paint)
	}
	one, two, three := draw(plain), draw(added), draw(removed)
	for _, got := range []string{one, two, three} {
		if !strings.Contains(strip(got), "return true") {
			t.Fatalf("the line is missing: %q", strip(got))
		}
	}
	if two == one || three == one || two == three {
		t.Fatalf("added, removed and neither all look the same:\n %q\n %q\n %q", one, two, three)
	}
}
