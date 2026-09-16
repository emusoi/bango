package bango

import (
	"strings"
	"unicode/utf8"
)

const (
	gutter    = 3
	gap       = 1
	noteCap   = 20
	floorText = 6
	floorPath = 12
	floorRef  = 8
)

type Column struct {
	Name  string
	Kind  Kind
	Width int
}

func Columns(rows []Row, width int) []Column {
	var cols []Column
	index := map[string]int{}
	for _, row := range rows {
		for _, field := range row.Fields {
			at, seen := index[field.Name]
			if !seen {
				index[field.Name] = len(cols)
				cols = append(cols, Column{Name: field.Name, Kind: field.Kind, Width: cells(field.Value)})
				continue
			}
			if w := cells(field.Value); w > cols[at].Width {
				cols[at].Width = w
			}
			if cols[at].Kind == "" && field.Kind != "" {
				cols[at].Kind = field.Kind
			}
		}
	}
	if len(cols) == 0 {
		return cols
	}

	note := 0
	for _, row := range rows {
		if w := cells(row.Note); w > note {
			note = w
		}
	}
	if note > noteCap {
		note = noteCap
	}

	budget := width - gutter - note - gap*len(cols)
	if budget < 1 {
		budget = 1
	}
	total := 0
	for _, col := range cols {
		total += col.Width
	}
	if total <= budget {
		spread(cols, budget-total)
		return cols
	}
	shrink(cols, total-budget)
	return cols
}

func spread(cols []Column, extra int) {
	if extra <= 0 {
		return
	}
	widest, at := -1, -1
	for i, col := range cols {
		if elastic(col.Kind) && col.Width > widest {
			widest, at = col.Width, i
		}
	}
	if at >= 0 {
		cols[at].Width += extra
	}
}

func shrink(cols []Column, need int) {
	for _, group := range [][]Kind{{KindText, KindPath}, {KindRef}} {
		for need > 0 {
			at, widest := -1, -1
			for i, col := range cols {
				if !inGroup(col.Kind, group) || col.Width <= floor(col.Kind) {
					continue
				}
				if col.Width > widest {
					widest, at = col.Width, i
				}
			}
			if at < 0 {
				break
			}
			cols[at].Width--
			need--
		}
	}
}

func inGroup(got Kind, group []Kind) bool {
	for _, kind := range group {
		if matches(got, kind) {
			return true
		}
	}
	return false
}

func matches(got, want Kind) bool {
	if want == KindText {
		return got == "" || got == KindText
	}
	return got == want
}

func elastic(kind Kind) bool {
	return kind == "" || kind == KindText
}

func floor(kind Kind) int {
	switch kind {
	case KindPath:
		return floorPath
	case KindRef:
		return floorRef
	default:
		return floorText
	}
}

func Fit(value string, kind Kind, width int) string {
	if width <= 0 {
		return ""
	}
	if cells(value) <= width {
		if kind == KindCount {
			return strings.Repeat(" ", width-cells(value)) + value
		}
		return value + strings.Repeat(" ", width-cells(value))
	}
	switch kind {
	case KindPath:
		return middle(value, width)
	case KindCount, KindTime:
		return value
	default:
		return tail(value, width)
	}
}

func tail(value string, width int) string {
	runes := []rune(value)
	if width < 2 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func middle(value string, width int) string {
	cut := strings.LastIndex(value, "/")
	if cut < 0 {
		return tail(value, width)
	}
	base := value[cut+1:]
	head := width - cells(base) - 2
	if head < 1 {
		return tail(base, width)
	}
	return string([]rune(value)[:head]) + "…/" + base
}

func cells(s string) int {
	return utf8.RuneCountInString(s)
}
