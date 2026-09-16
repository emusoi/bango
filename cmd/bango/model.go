package main

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joneskim/bango"
)

type mode int

const (
	browsing mode = iota
	filtering
	helping
	prompting
	confirming
)

type model2 struct {
	panel    bango.Panel
	opts     options
	producer []string
	folded   map[string]bool
	query    string
	cursor   int
	state    mode
	pending  string
	typed    string
	notice   string
	choice   *Choice
	failure  error
	width    int
	height   int
}

func newModel(panel bango.Panel, opts options, producer []string) *model2 {
	width, height := opts.width, opts.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}
	return &model2{panel: panel, opts: opts, producer: producer,
		folded: map[string]bool{}, width: width, height: height}
}

func (m *model2) Init() tea.Cmd { return nil }

func (m *model2) rows() []bango.Row {
	var out []bango.Row
	for _, line := range bango.Lines(bango.Filter(m.panel, m.query), m.folded) {
		if !line.Header {
			out = append(out, line.Row)
		}
	}
	return out
}

func (m *model2) selected() (bango.Row, bool) {
	rows := m.rows()
	if len(rows) == 0 {
		return bango.Row{}, false
	}
	if m.cursor >= len(rows) {
		m.cursor = len(rows) - 1
	}
	return rows[m.cursor], true
}

func (m *model2) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		if m.opts.width == 0 {
			m.width = msg.Width
		}
		if m.opts.height == 0 {
			m.height = msg.Height
		}
		return m, nil
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *model2) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.state {
	case filtering:
		return m.typing(key, func(value string) { m.query = value; m.cursor = 0 })
	case prompting:
		return m.typing(key, func(value string) { m.commit(m.pending, value) })
	case confirming:
		if key == "y" {
			m.state = browsing
			m.commit(m.pending, "")
			return m, nil
		}
		m.state = browsing
		m.pending = ""
		return m, nil
	case helping:
		m.state = browsing
		return m, nil
	}

	switch key {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(m.rows()) - 1
	case "/":
		m.state = filtering
		m.typed = m.query
	case "?":
		m.state = helping
	case " ":
		if row, ok := m.selected(); ok && len(row.Children) > 0 {
			m.folded[row.ID] = !m.folded[row.ID]
		}
	default:
		return m.act(key)
	}
	return m, nil
}

func (m *model2) typing(key string, done func(string)) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = browsing
		m.typed = ""
	case "enter":
		m.state = browsing
		value := m.typed
		m.typed = ""
		done(value)
	case "backspace":
		if m.typed != "" {
			runes := []rune(m.typed)
			m.typed = string(runes[:len(runes)-1])
			if m.state == filtering {
				m.query = m.typed
			}
		}
	default:
		if len(key) == 1 {
			m.typed += key
			if m.state == filtering {
				m.query = m.typed
			}
		}
	}
	return m, nil
}

func (m *model2) move(delta int) {
	rows := m.rows()
	if len(rows) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(rows) {
		m.cursor = len(rows) - 1
	}
}

func (m *model2) act(key string) (tea.Model, tea.Cmd) {
	row, ok := m.selected()
	if !ok {
		return m, nil
	}
	for name, action := range m.panel.Actions {
		if !matchKey(action.Key, key) {
			continue
		}
		if !action.Global && !allows(row, name) {
			continue
		}
		if action.Confirm != "" {
			m.pending = name
			m.state = confirming
			return m, nil
		}
		if action.Input != "" {
			m.pending = name
			m.state = prompting
			m.typed = ""
			return m, nil
		}
		return m.commitCmd(name, "")
	}
	return m, nil
}

func allows(row bango.Row, name string) bool {
	for _, allowed := range row.Actions {
		if allowed == name {
			return true
		}
	}
	return false
}

func matchKey(want, got string) bool {
	if want == got {
		return true
	}
	for _, spelling := range spellings[want] {
		if spelling == got {
			return true
		}
	}
	return false
}

var spellings = map[string][]string{
	"⏎": {"enter", "<CR>", "\r", "\n"},
	"⇥": {"tab", "<Tab>"},
}

func (m *model2) commitCmd(name, input string) (tea.Model, tea.Cmd) {
	m.commit(name, input)
	if m.choice != nil {
		return m, tea.Quit
	}
	return m, nil
}

func (m *model2) commit(name, input string) {
	row, ok := m.selected()
	if !ok {
		return
	}
	choice := &Choice{Action: name, Row: row.TargetID(), Input: input}
	if len(m.producer) == 0 {
		m.choice = choice
		return
	}
	action := m.panel.Actions[name]
	if action.Panel != "" {
		m.notice = "panels of panels are not wired yet"
		return
	}
	if err := execute(action, choice); err != nil {
		m.notice = err.Error()
		if retry, ok := retryFor(action, err.Error()); ok {
			m.notice = err.Error() + " — retry with " + retry.Label
		}
	}
	m.refresh()
}

func retryFor(action bango.Action, refusal string) (bango.Retry, bool) {
	for _, retry := range action.Retry {
		if retry.When == "" || strings.Contains(refusal, retry.When) {
			return retry, true
		}
	}
	return bango.Retry{}, false
}

func execute(action bango.Action, choice *Choice) error {
	args := make([]string, 0, len(action.Args))
	for _, arg := range action.Args {
		switch arg {
		case "{row}":
			args = append(args, choice.Row)
		case "{input}":
			args = append(args, choice.Input)
		case "{choice}":
			args = append(args, choice.Pick)
		default:
			args = append(args, arg)
		}
	}
	command := exec.Command(action.Verb, args...)
	out, err := command.CombinedOutput()
	if err != nil {
		return &refusal{err: err, text: strings.TrimSpace(string(out))}
	}
	return nil
}

type refusal struct {
	err  error
	text string
}

func (r *refusal) Error() string {
	if r.text != "" {
		return r.text
	}
	return r.err.Error()
}

func (m *model2) refresh() {
	keep := ""
	if row, ok := m.selected(); ok {
		keep = row.ID
	}
	panel, err := produce(m.producer, m.opts.want)
	if err != nil {
		m.notice = err.Error()
		return
	}
	m.panel = panel
	rows := m.rows()
	for i, row := range rows {
		if row.ID == keep {
			m.cursor = i
			return
		}
	}
	if m.cursor >= len(rows) {
		m.cursor = len(rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *model2) View() string {
	cursor := ""
	if row, ok := m.selected(); ok {
		cursor = row.ID
	}
	style := bango.Style{Width: m.width, Height: m.height, ASCII: m.opts.ascii,
		Cursor: cursor, Folded: m.folded, Query: m.query}
	lines := bango.Render(m.panel, style)
	switch m.state {
	case filtering:
		lines = append(lines, "", "/"+m.typed)
	case prompting:
		lines = append(lines, "", m.panel.Actions[m.pending].Input+" "+m.typed)
	case confirming:
		lines = append(lines, "", m.panel.Actions[m.pending].Confirm+"  y/n")
	case helping:
		lines = append(lines, "")
		lines = append(lines, help(m.panel)...)
	}
	if m.notice != "" {
		lines = append(lines, "", m.notice)
	}
	return strings.Join(lines, "\n")
}

func help(panel bango.Panel) []string {
	out := []string{"actions"}
	for name, action := range panel.Actions {
		line := "  " + action.Key + "  " + action.Label
		if action.Help != "" {
			line += "  — " + action.Help
		}
		if action.Label == "" {
			line = "  " + action.Key + "  " + name
		}
		out = append(out, line)
	}
	return append(out, "  /  filter", "  q  quit")
}
