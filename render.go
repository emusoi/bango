package bango

import "strings"

type Style struct {
	Width  int
	Height int
	Offset int
	ASCII  bool
	Cursor string
	Folded map[string]bool
	Query  string
}

type Line struct {
	Row      Row
	Depth    int
	Section  string
	Header   bool
	Label    string
	Verbatim bool
}

func Lines(p Panel, folded map[string]bool) []Line {
	var out []Line
	for _, section := range p.orderedSections() {
		if section.Label != "" {
			out = append(out, Line{Header: true, Label: section.Label, Section: section.ID})
		}
		if section.Collapsed {
			continue
		}
		for _, row := range section.Rows {
			out = append(out, lines(row, section.ID, 0, folded, section.Layout.verbatim())...)
		}
	}
	return out
}

func lines(row Row, section string, depth int, folded map[string]bool, verbatim bool) []Line {
	out := []Line{{Row: row, Depth: depth, Section: section, Verbatim: verbatim}}
	if folded[row.ID] {
		return out
	}
	for _, child := range row.Children {
		out = append(out, lines(child, section, depth+1, folded, verbatim)...)
	}
	return out
}

func Render(p Panel, style Style) []string {
	if style.Width <= 0 {
		style.Width = 72
	}
	shown := Filter(p, style.Query)
	all := Lines(shown, style.Folded)

	var rows []Row
	for _, line := range all {
		if !line.Header && !line.Verbatim {
			rows = append(rows, line.Row)
		}
	}

	head := []string{clip(p.Title, style.Width)}
	if p.Subtitle != "" {
		head = append(head, clip(p.Subtitle, style.Width))
	}
	head = append(head, "")
	foot := footer(p, style)

	if len(rows) == 0 {
		if p.Empty != "" {
			head = append(head, clip(p.Empty, style.Width))
		}
		return append(head, foot...)
	}

	var body []string
	at := -1
	cols := Columns(rows, style.Width)
	for _, line := range all {
		if line.Header {
			body = append(body, clip(line.Label, style.Width))
			continue
		}
		if line.Row.ID == style.Cursor {
			at = len(body)
		}
		body = append(body, clip(paint(line, cols, style), style.Width))
		if line.Row.ID == style.Cursor {
			body = append(body, detail(line.Row, style.Width)...)
		}
	}
	body = Window(body, at, style.Height-len(head)-len(foot), style.Offset)
	return append(append(head, body...), foot...)
}

func Scroll(top, at, height, total int) int {
	if height <= 0 || total <= height {
		return 0
	}
	if top > total-height {
		top = total - height
	}
	if top < 0 {
		top = 0
	}
	if at < 0 {
		return top
	}
	if at < top {
		return at
	}
	if at >= top+height {
		return at - height + 1
	}
	return top
}

func Window[T any](lines []T, at, height, top int) []T {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	top = Scroll(top, at, height, len(lines))
	return lines[top : top+height]
}

func paint(line Line, cols []Column, style Style) string {
	cursor := " "
	if line.Row.ID == style.Cursor {
		cursor = MarkHere.Glyph(style.ASCII)
	}
	parts := []string{cursor, line.Row.Mark.Glyph(style.ASCII), " "}

	// A verbatim row is one value, written out. Nothing is padded to a column
	// it does not share, and nothing is aligned against a row it only sits near.
	if line.Verbatim {
		if len(line.Row.Fields) > 0 {
			parts = append(parts, line.Row.Fields[0].Value)
		}
		if line.Row.Note != "" {
			parts = append(parts, " "+line.Row.Note)
		}
		return strings.TrimRight(strings.Join(parts, ""), " ")
	}

	values := map[string]Field{}
	for _, field := range line.Row.Fields {
		values[field.Name] = field
	}
	for i, col := range cols {
		value := values[col.Name].Value
		width := col.Width
		if i == 0 && line.Depth > 0 {
			pad := strings.Repeat("  ", line.Depth)
			value = pad + value
		}
		parts = append(parts, Fit(value, col.Kind, width))
		if i < len(cols)-1 {
			parts = append(parts, " ")
		}
	}
	if line.Row.Note != "" {
		parts = append(parts, " "+line.Row.Note)
	}
	return strings.TrimRight(strings.Join(parts, ""), " ")
}

func detail(row Row, width int) []string {
	var out []string
	for _, fact := range row.Facts {
		out = append(out, clip("    "+fact, width))
	}
	for _, preview := range row.Preview {
		out = append(out, clip("    "+preview, width))
	}
	return out
}

func clip(s string, width int) string {
	if width <= 0 || cells(s) <= width {
		return s
	}
	return tail(s, width)
}

func footer(p Panel, style Style) []string {
	if len(p.Hints) == 0 {
		return nil
	}
	return []string{"", clip(strings.Join(p.Hints, " · "), style.Width)}
}
