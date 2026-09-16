package bango

import "strings"

type Style struct {
	Width  int
	Height int
	ASCII  bool
	Cursor string
	Folded map[string]bool
	Query  string
}

type Line struct {
	Row     Row
	Depth   int
	Section string
	Header  bool
	Label   string
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
			out = append(out, lines(row, section.ID, 0, folded)...)
		}
	}
	return out
}

func lines(row Row, section string, depth int, folded map[string]bool) []Line {
	out := []Line{{Row: row, Depth: depth, Section: section}}
	if folded[row.ID] {
		return out
	}
	for _, child := range row.Children {
		out = append(out, lines(child, section, depth+1, folded)...)
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
		if !line.Header {
			rows = append(rows, line.Row)
		}
	}

	out := []string{clip(p.Title, style.Width)}
	if p.Subtitle != "" {
		out = append(out, clip(p.Subtitle, style.Width))
	}
	out = append(out, "")

	if len(rows) == 0 {
		if p.Empty != "" {
			out = append(out, clip(p.Empty, style.Width))
		}
		return append(out, footer(p, style)...)
	}

	cols := Columns(rows, style.Width)
	for _, line := range all {
		if line.Header {
			out = append(out, clip(line.Label, style.Width))
			continue
		}
		out = append(out, clip(paint(line, cols, style), style.Width))
		if line.Row.ID == style.Cursor {
			out = append(out, detail(line.Row, style.Width)...)
		}
	}
	return append(out, footer(p, style)...)
}

func paint(line Line, cols []Column, style Style) string {
	cursor := " "
	if line.Row.ID == style.Cursor {
		cursor = MarkHere.Glyph(style.ASCII)
	}
	parts := []string{cursor, line.Row.Mark.Glyph(style.ASCII), " "}

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
