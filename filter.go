package bango

import "strings"

func Matches(row Row, query string) bool {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return true
	}
	hay := strings.ToLower(haystack(row))
	for _, term := range terms {
		if !strings.Contains(hay, term) {
			return false
		}
	}
	return true
}

func haystack(row Row) string {
	parts := []string{row.ID, row.Note}
	for _, field := range row.Fields {
		parts = append(parts, field.Value)
	}
	return strings.Join(parts, " ")
}

func Keep(row Row, query string) (Row, bool) {
	var kept []Row
	for _, child := range row.Children {
		if survivor, ok := Keep(child, query); ok {
			kept = append(kept, survivor)
		}
	}
	if len(kept) > 0 {
		row.Children = kept
		return row, true
	}
	if Matches(row, query) {
		row.Children = nil
		return row, true
	}
	return row, false
}

func Filter(p Panel, query string) Panel {
	if strings.TrimSpace(query) == "" {
		return p
	}
	var sections []Section
	for _, section := range p.Sections {
		var rows []Row
		for _, row := range section.Rows {
			if kept, ok := Keep(row, query); ok {
				rows = append(rows, kept)
			}
		}
		if len(rows) > 0 {
			section.Rows = rows
			sections = append(sections, section)
		}
	}
	p.Sections = sections
	return p
}
