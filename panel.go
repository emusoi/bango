package bango

import (
	"slices"
	"strings"
)

const Version = 1

type Panel struct {
	Version  int               `json:"bango"`
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Subtitle string            `json:"subtitle,omitempty"`
	Empty    string            `json:"empty,omitempty"`
	Hints    []string          `json:"hints,omitempty"`
	Order    []string          `json:"order,omitempty"`
	Sections []Section         `json:"sections"`
	Actions  map[string]Action `json:"actions,omitempty"`
}

type Section struct {
	ID        string `json:"id"`
	Label     string `json:"label,omitempty"`
	Rows      []Row  `json:"rows"`
	Collapsed bool   `json:"collapsed,omitempty"`
}

type Row struct {
	ID       string   `json:"id"`
	Mark     Mark     `json:"mark,omitempty"`
	Fields   []Field  `json:"fields"`
	Note     string   `json:"note,omitempty"`
	Actions  []string `json:"actions,omitempty"`
	Facts    []string `json:"facts,omitempty"`
	Preview  []string `json:"preview,omitempty"`
	Children []Row    `json:"children,omitempty"`
	Dim      bool     `json:"dim,omitempty"`
	Target   string   `json:"target,omitempty"`
}

type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Kind  Kind   `json:"kind,omitempty"`
}

type Action struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Verb    string   `json:"verb,omitempty"`
	Args    []string `json:"args,omitempty"`
	Input   string   `json:"input,omitempty"`
	Choices []string `json:"choices,omitempty"`
	Confirm string   `json:"confirm,omitempty"`
	Panel   string   `json:"panel,omitempty"`
	Global  bool     `json:"global,omitempty"`
	Retry   []Retry  `json:"retry,omitempty"`
	Help    string   `json:"help,omitempty"`
}

type Retry struct {
	When  string   `json:"when"`
	Label string   `json:"label"`
	Verb  string   `json:"verb"`
	Args  []string `json:"args,omitempty"`
}

type Kind string

const (
	KindText  Kind = "text"
	KindPath  Kind = "path"
	KindCount Kind = "count"
	KindTime  Kind = "time"
	KindRef   Kind = "ref"
)

type Mark string

const (
	MarkNone     Mark = ""
	MarkHere     Mark = "here"
	MarkNew      Mark = "new"
	MarkWorking  Mark = "working"
	MarkWaiting  Mark = "waiting"
	MarkDone     Mark = "done"
	MarkDirty    Mark = "dirty"
	MarkBlocked  Mark = "blocked"
	MarkDetached Mark = "detached"
)

var unicodeMarks = map[Mark]string{
	MarkNone: " ", MarkHere: "▸", MarkNew: "○", MarkWorking: "◐",
	MarkWaiting: "⏎", MarkDone: "✓", MarkDirty: "●", MarkBlocked: "⨯",
	MarkDetached: "⚠",
}

var asciiMarks = map[Mark]string{
	MarkNone: " ", MarkHere: ">", MarkNew: "o", MarkWorking: "%",
	MarkWaiting: "!", MarkDone: "x", MarkDirty: "*", MarkBlocked: "X",
	MarkDetached: "~",
}

func (m Mark) Glyph(ascii bool) string {
	set := unicodeMarks
	if ascii {
		set = asciiMarks
	}
	if glyph, ok := set[m]; ok {
		return glyph
	}
	return " "
}

func (m Mark) Known() bool {
	_, ok := unicodeMarks[m]
	return ok
}

func (k Kind) Known() bool {
	switch k {
	case "", KindText, KindPath, KindCount, KindTime, KindRef:
		return true
	}
	return false
}

func (r Row) TargetID() string {
	if r.Target != "" {
		return r.Target
	}
	return r.ID
}

func (p Panel) Rows() []Row {
	var out []Row
	for _, section := range p.orderedSections() {
		if section.Collapsed {
			continue
		}
		for _, row := range section.Rows {
			out = append(out, flatten(row)...)
		}
	}
	return out
}

func flatten(row Row) []Row {
	out := []Row{row}
	for _, child := range row.Children {
		out = append(out, flatten(child)...)
	}
	return out
}

func (p Panel) orderedSections() []Section {
	if len(p.Order) == 0 {
		return p.Sections
	}
	rank := map[string]int{}
	for i, id := range p.Order {
		rank[id] = i
	}
	out := slices.Clone(p.Sections)
	slices.SortStableFunc(out, func(a, b Section) int {
		return rankOf(rank, a) - rankOf(rank, b)
	})
	return out
}

func rankOf(rank map[string]int, section Section) int {
	if i, ok := rank[section.ID]; ok {
		return i
	}
	return len(rank)
}

func (p Panel) Find(id string) (Row, bool) {
	for _, row := range p.Rows() {
		if row.ID == id {
			return row, true
		}
	}
	return Row{}, false
}

func (p Panel) ActionNames() []string {
	names := make([]string, 0, len(p.Actions))
	for name := range p.Actions {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (a Action) RetryFor(refusal string) (Retry, bool) {
	for _, retry := range a.Retry {
		if strings.Contains(refusal, retry.When) {
			return retry, true
		}
	}
	return Retry{}, false
}
