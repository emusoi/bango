package main

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/emusoi/bango"
)

type mode int

const (
	browsing mode = iota
	filtering
	helping
	prompting
	confirming
	choosing
)

type model2 struct {
	panel    bango.Panel
	stack    []bango.Panel
	retry    *bango.Retry
	opts     options
	producer []string
	folded   map[string]bool
	query    string
	cursor   int
	state    mode
	pending  string
	typed    string
	choiceAt int
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
	case choosing:
		return m.choosing(key)
	}

	if m.retry != nil && key == "r" {
		next := *m.retry
		m.retry = nil
		m.notice = ""
		m.runRetry(next)
		return m, nil
	}

	switch key {
	case "q", "esc", "ctrl+c":
		if key != "ctrl+c" && m.back() {
			return m, nil
		}
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

func (m *model2) choosing(key string) (tea.Model, tea.Cmd) {
	options := m.panel.Actions[m.pending].Choices
	switch key {
	case "esc", "q":
		m.state = browsing
		m.pending = ""
	case "j", "down":
		m.choiceAt = min(m.choiceAt+1, len(options)-1)
	case "k", "up":
		m.choiceAt = max(m.choiceAt-1, 0)
	case "enter":
		m.state = browsing
		name := m.pending
		m.pending = ""
		m.pick(name, options[m.choiceAt])
	}
	return m, nil
}

func (m *model2) pick(name, chosen string) {
	row, ok := m.selected()
	if !ok {
		return
	}
	m.run(name, &Choice{Action: name, Row: row.TargetID(), Pick: chosen})
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
	for _, name := range m.panel.ActionNames() {
		action := m.panel.Actions[name]
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
		if len(action.Choices) > 0 {
			m.pending = name
			m.state = choosing
			m.choiceAt = 0
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
	m.run(name, &Choice{Action: name, Row: row.TargetID(), Input: input})
}

func (m *model2) run(name string, choice *Choice) {
	if len(m.producer) == 0 {
		m.choice = choice
		return
	}
	action := m.panel.Actions[name]
	m.notice = ""
	m.retry = nil

	out, err := execute(action, choice, m.opts.transport)
	if err != nil {
		m.notice = err.Error()
		if next, ok := action.RetryFor(err.Error()); ok {
			m.retry = &next
		}
		return
	}
	if next, ok := asPanel(out, m.opts.want); ok {
		m.stack = append(m.stack, m.panel)
		m.panel = next
		m.cursor = 0
		return
	}
	m.refresh()
}

func asPanel(out []byte, want string) (bango.Panel, bool) {
	panels, err := read(strings.NewReader(string(out)), want)
	if err != nil || len(panels) == 0 {
		return bango.Panel{}, false
	}
	return panels[len(panels)-1], true
}

func (m *model2) back() bool {
	if len(m.stack) == 0 {
		return false
	}
	m.panel = m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	m.cursor = 0
	return true
}

func execute(action bango.Action, choice *Choice, through bango.Transport) ([]byte, error) {
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
	argv := through.Argv(action.Verb, args)
	command := exec.Command(argv[0], argv[1:]...)
	var complaint strings.Builder
	command.Stderr = &complaint
	out, err := command.Output()
	if err != nil {
		text := strings.TrimSpace(complaint.String())
		if text == "" {
			text = strings.TrimSpace(string(out))
		}
		return nil, &refusal{err: err, text: text}
	}
	return out, nil
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

func (m *model2) runRetry(retry bango.Retry) {
	row, ok := m.selected()
	if !ok {
		return
	}
	action := bango.Action{Verb: retry.Verb, Args: retry.Args}
	choice := &Choice{Action: retry.Label, Row: row.TargetID()}
	if _, err := execute(action, choice, m.opts.transport); err != nil {
		m.notice = err.Error()
		return
	}
	m.refresh()
}

func (m *model2) refresh() {
	keep := ""
	if row, ok := m.selected(); ok {
		keep = row.ID
	}
	panel, err := produce(m.producer, m.opts.want, m.opts.transport)
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
	case choosing:
		lines = append(lines, "")
		for i, option := range m.panel.Actions[m.pending].Choices {
			mark := "  "
			if i == m.choiceAt {
				mark = bango.MarkHere.Glyph(m.opts.ascii) + " "
			}
			lines = append(lines, mark+option)
		}
	case helping:
		lines = append(lines, "")
		lines = append(lines, help(m.panel)...)
	}
	if m.notice != "" {
		lines = append(lines, "", m.notice)
		if m.retry != nil {
			lines = append(lines, "r  "+m.retry.Label)
		}
	}
	return strings.Join(lines, "\n")
}

func help(panel bango.Panel) []string {
	out := []string{"actions"}
	for _, name := range panel.ActionNames() {
		action := panel.Actions[name]
		label := action.Label
		if label == "" {
			label = name
		}
		line := "  " + action.Key + "  " + label
		if action.Help != "" {
			line += "  — " + action.Help
		}
		out = append(out, line)
	}
	return append(out, "", "  /  filter", "  ?  this list", "  q  back, then quit")
}
