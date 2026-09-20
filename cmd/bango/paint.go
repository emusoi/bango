package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/emusoi/bango"
	"github.com/mattn/go-runewidth"
)

type palette struct {
	title    lipgloss.Style
	subtitle lipgloss.Style
	section  lipgloss.Style
	row      lipgloss.Style
	selected lipgloss.Style
	dim      lipgloss.Style
	note     lipgloss.Style
	count    lipgloss.Style
	ref      lipgloss.Style
	fact     lipgloss.Style
	footer   lipgloss.Style
	marks    map[bango.Mark]lipgloss.Style
	tones    map[bango.Tone]lipgloss.Style
	rows     map[bango.RowTone]lipgloss.Style
}

func colours() palette {
	grey := lipgloss.AdaptiveColor{Light: "245", Dark: "243"}
	quiet := lipgloss.AdaptiveColor{Light: "240", Dark: "245"}
	accent := lipgloss.AdaptiveColor{Light: "136", Dark: "179"}
	ref := lipgloss.AdaptiveColor{Light: "25", Dark: "110"}
	return palette{
		title:    lipgloss.NewStyle().Bold(true),
		subtitle: lipgloss.NewStyle().Foreground(quiet),
		section:  lipgloss.NewStyle().Foreground(grey).Italic(true),
		row:      lipgloss.NewStyle(),
		selected: lipgloss.NewStyle().Bold(true),
		dim:      lipgloss.NewStyle().Foreground(grey),
		note:     lipgloss.NewStyle().Foreground(quiet).Italic(true),
		count:    lipgloss.NewStyle().Foreground(accent).Bold(true),
		ref:      lipgloss.NewStyle().Foreground(ref),
		fact:     lipgloss.NewStyle().Foreground(quiet),
		footer:   lipgloss.NewStyle().Foreground(grey),
		marks: map[bango.Mark]lipgloss.Style{
			bango.MarkHere:     lipgloss.NewStyle().Foreground(accent).Bold(true),
			bango.MarkWaiting:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "173"}).Bold(true),
			bango.MarkWorking:  lipgloss.NewStyle().Foreground(accent),
			bango.MarkDone:     lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "108"}),
			bango.MarkDirty:    lipgloss.NewStyle().Foreground(accent),
			bango.MarkBlocked:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "124", Dark: "167"}),
			bango.MarkDetached: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "179"}),
			bango.MarkNew:      lipgloss.NewStyle().Foreground(quiet),
		},
		tones: map[bango.Tone]lipgloss.Style{
			bango.ToneChanged: lipgloss.NewStyle().Foreground(accent).Bold(true),
			bango.ToneComment: lipgloss.NewStyle().Foreground(grey).Italic(true),
			bango.ToneString:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "108"}),
			bango.ToneNumber:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "97", Dark: "140"}),
			bango.ToneKeyword: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "125", Dark: "175"}),
			bango.ToneName:    lipgloss.NewStyle().Foreground(ref),
			bango.ToneType:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "23", Dark: "109"}),
		},
		// A terminal has no gentle background to lay under a line, so what a row
		// is shows in the ink instead.
		rows: map[bango.RowTone]lipgloss.Style{
			bango.RowAdded:   lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "22", Dark: "108"}),
			bango.RowRemoved: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "124", Dark: "167"}),
		},
	}
}

// tinted writes a value out with each span in its own tone and everything
// between them plain. The spans were checked in order and without overlap, so
// walking them once is the whole of it.
func tinted(value string, spans []bango.Span, base lipgloss.Style, paint palette) string {
	runes := []rune(value)
	var out strings.Builder
	at := 0
	for _, span := range spans {
		if span.From >= len(runes) {
			break
		}
		to := min(span.To, len(runes))
		if span.From > at {
			out.WriteString(base.Render(string(runes[at:span.From])))
		}
		style, ok := paint.tones[span.Tone]
		if !ok {
			style = base
		}
		out.WriteString(style.Render(string(runes[span.From:to])))
		at = to
	}
	if at < len(runes) {
		out.WriteString(base.Render(string(runes[at:])))
	}
	return out.String()
}

func (m *model) paint(reserve int) []string {
	m.follow(reserve)
	if m.opts.plain {
		return bango.Render(m.shown(), m.style(reserve))
	}
	shown := m.shown()
	all := bango.Lines(shown, m.folded)

	shownRows, inColumns := 0, []bango.Row(nil)
	for _, line := range all {
		if line.Header {
			continue
		}
		shownRows++
		if !line.Verbatim {
			inColumns = append(inColumns, line.Row)
		}
	}

	paint := colours()
	head := []string{paint.title.Render(clip(m.panel.Title, m.width))}
	if m.panel.Subtitle != "" {
		head = append(head, paint.subtitle.Render(clip(m.panel.Subtitle, m.width)))
	}
	head = append(head, "")
	foot := m.paintFooter(paint)

	if shownRows == 0 {
		if m.panel.Empty != "" {
			head = append(head, paint.dim.Render(clip(m.panel.Empty, m.width)))
		}
		return append(head, foot...)
	}

	cursor := ""
	if row, ok := m.selected(); ok {
		cursor = row.ID
	}
	var body []string
	at := -1
	cols := bango.Columns(inColumns, m.width)
	for _, line := range all {
		if line.Header {
			body = append(body, paint.section.Render(clip(line.Label, m.width)))
			continue
		}
		if line.Row.ID == cursor {
			at = len(body)
		}
		body = append(body, m.paintRow(line, cols, cursor, paint))
		if line.Row.ID == cursor {
			for _, fact := range line.Row.Facts {
				body = append(body, paint.fact.Render(clip("    "+fact, m.width)))
			}
			for _, preview := range line.Row.Preview {
				body = append(body, paint.fact.Render(clip("    "+preview, m.width)))
			}
		}
	}
	body = bango.Window(body, at, m.body(reserve), m.top)
	return append(append(head, body...), foot...)
}

func (m *model) follow(reserve int) {
	cursor := ""
	if row, ok := m.selected(); ok {
		cursor = row.ID
	}
	at, total := -1, 0
	for _, line := range bango.Lines(m.shown(), m.folded) {
		if line.Header {
			total++
			continue
		}
		if line.Row.ID == cursor {
			at = total
			total += len(line.Row.Facts) + len(line.Row.Preview)
		}
		total++
	}
	m.top = bango.Scroll(m.top, at, m.body(reserve), total)
}

func (m *model) body(reserve int) int {
	head := 2
	if m.panel.Subtitle != "" {
		head++
	}
	foot := 0
	if len(m.panel.Hints) > 0 {
		foot = 2
	}
	return m.height - head - foot - reserve
}

func (m *model) paintRow(line bango.Line, cols []bango.Column, cursor string, paint palette) string {
	here := line.Row.ID == cursor
	bar := " "
	if here {
		bar = bango.MarkHere.Glyph(m.opts.ascii)
	}
	pieces := []string{paint.marks[bango.MarkHere].Render(bar)}

	glyph := line.Row.Mark.Glyph(m.opts.ascii)
	if style, ok := paint.marks[line.Row.Mark]; ok {
		pieces = append(pieces, style.Render(glyph))
	} else {
		pieces = append(pieces, glyph)
	}
	pieces = append(pieces, " ")

	base := paint.row
	if line.Row.Dim {
		base = paint.dim
	}
	if style, ok := paint.rows[line.Row.Tone]; ok {
		base = style
	}

	// A verbatim row is one value, written out, with whatever the producer said
	// about parts of it painted in.
	if line.Verbatim {
		if len(line.Row.Fields) > 0 {
			field := line.Row.Fields[0]
			// Cut first, paint second: a width is measured against the value,
			// and an escape code has no width to measure.
			pieces = append(pieces, tinted(clip(field.Value, m.width-3), field.Spans, base, paint))
		}
		if line.Row.Note != "" {
			pieces = append(pieces, paint.note.Render(" "+line.Row.Note))
		}
		return strings.Join(pieces, "")
	}

	values := map[string]bango.Field{}
	for _, field := range line.Row.Fields {
		values[field.Name] = field
	}
	for i, col := range cols {
		field := values[col.Name]
		value := field.Value
		if i == 0 && line.Depth > 0 {
			value = strings.Repeat("  ", line.Depth) + value
		}
		text := bango.Fit(value, col.Kind, col.Width)
		style := paint.row
		switch {
		case line.Row.Dim:
			style = paint.dim
		case col.Kind == bango.KindCount:
			style = paint.count
		case col.Kind == bango.KindRef:
			style = paint.ref
		case col.Kind == bango.KindTime, i > 0 && col.Kind == "":
			if i > 0 && i == len(cols)-1 {
				style = paint.note
			} else if col.Kind == bango.KindTime {
				style = paint.note
			}
		}
		if here && !line.Row.Dim && style.GetForeground() == nil {
			style = paint.selected
		}
		pieces = append(pieces, style.Render(text))
		if i < len(cols)-1 {
			pieces = append(pieces, " ")
		}
	}
	if line.Row.Note != "" {
		style := paint.note
		if isCount(line.Row.Note) {
			style = paint.count
		}
		pieces = append(pieces, " "+style.Render(line.Row.Note))
	}
	return strings.TrimRight(strings.Join(pieces, ""), " ")
}

func isCount(note string) bool {
	if note == "" {
		return false
	}
	return note[0] >= '0' && note[0] <= '9'
}

func (m *model) paintFooter(paint palette) []string {
	if len(m.panel.Hints) == 0 {
		return nil
	}
	return []string{"", paint.footer.Render(clip(strings.Join(m.panel.Hints, " · "), m.width))}
}

func (m *model) style(reserve int) bango.Style {
	cursor := ""
	if row, ok := m.selected(); ok {
		cursor = row.ID
	}
	return bango.Style{Width: m.width, Height: m.height - reserve, Offset: m.top,
		ASCII: m.opts.ascii, Cursor: cursor, Folded: m.folded, Query: m.query}
}

func clip(s string, width int) string {
	if width <= 0 || runewidth.StringWidth(s) <= width {
		return s
	}
	return runewidth.Truncate(s, width, "…")
}
