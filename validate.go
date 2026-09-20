package bango

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const maxDepth = 8

var idPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]+$`)

var reservedKeys = map[string]bool{
	"/": true, "?": true, "q": true, "⎋": true, "esc": true,
	"j": true, "k": true, "g": true, "G": true, " ": true,
}

func ReservedKeys() []string {
	out := make([]string, 0, len(reservedKeys))
	for key := range reservedKeys {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

type Invalid struct {
	Path   string
	Reason string
}

func (e Invalid) Error() string {
	return e.Path + ": " + e.Reason
}

func Validate(p *Panel) error {
	if found := Problems(p); len(found) > 0 {
		return found[0]
	}
	return nil
}

func Problems(p *Panel) []Invalid {
	if p.Version == 0 {
		return []Invalid{{"bango", "no schema version — is this a bango document?"}}
	}
	if p.Version != Version {
		return []Invalid{{"bango", fmt.Sprintf("version %d, this renderer speaks %d", p.Version, Version)}}
	}

	var found []Invalid
	if p.ID == "" || !idPattern.MatchString(p.ID) {
		found = append(found, Invalid{"id", "must be non-empty and free of spaces"})
	}
	found = append(found, actionProblems(p)...)

	seenSections := map[string]bool{}
	seenRows := map[string]bool{}
	for i := range p.Sections {
		section := &p.Sections[i]
		where := fmt.Sprintf("sections[%d]", i)
		switch {
		case section.ID == "" || !idPattern.MatchString(section.ID):
			found = append(found, Invalid{where + ".id", "must be non-empty and free of spaces"})
		case seenSections[section.ID]:
			found = append(found, Invalid{where + ".id", "duplicate section id " + section.ID})
		}
		seenSections[section.ID] = true
		switch section.Layout {
		case "", LayoutColumns, LayoutLines:
		default:
			found = append(found, Invalid{where + ".layout",
				"unknown layout " + string(section.Layout) + "; columns or lines"})
		}
		for j := range section.Rows {
			found = append(found, rowProblems(&section.Rows[j],
				fmt.Sprintf("%s.rows[%d]", where, j), p, seenRows, 1)...)
			if section.Layout.verbatim() && len(section.Rows[j].Fields) > 1 {
				found = append(found, Invalid{fmt.Sprintf("%s.rows[%d].fields", where, j),
					"a lines row is one field; there is nothing to align a second against"})
			}
		}
	}

	found = append(found, targetProblems(p)...)
	return append(found, keyProblems(p)...)
}

func actionProblems(p *Panel) []Invalid {
	var found []Invalid
	for _, name := range p.ActionNames() {
		action := p.Actions[name]
		where := "actions." + name
		switch {
		case action.Key == "":
			found = append(found, Invalid{where + ".key", "an action needs a key"})
		case reservedKeys[action.Key]:
			found = append(found, Invalid{where + ".key", "key " + action.Key + " is reserved by the renderer"})
		}
		if action.Verb == "" {
			if len(action.Args) > 0 {
				found = append(found, Invalid{where + ".verb", "args without a verb"})
			} else {
				found = append(found, Invalid{where + ".verb", "an action needs a command to run"})
			}
		}
		for i, retry := range action.Retry {
			at := fmt.Sprintf("%s.retry[%d]", where, i)
			if retry.Verb == "" {
				found = append(found, Invalid{at + ".verb", "a retry needs a command to run"})
			}
			if retry.Label == "" {
				found = append(found, Invalid{at + ".label", "a retry is offered on r and needs a label to offer"})
			}
		}
	}
	return found
}

func keyProblems(p *Panel) []Invalid {
	var global []string
	for _, name := range p.ActionNames() {
		if p.Actions[name].Global {
			global = append(global, name)
		}
	}
	var found []Invalid
	for _, row := range p.Rows() {
		seen := map[string]string{}
		for _, name := range append(append([]string{}, row.Actions...), global...) {
			action, ok := p.Actions[name]
			if !ok {
				continue
			}
			if owner, taken := seen[action.Key]; taken && owner != name {
				found = append(found, Invalid{"actions." + name + ".key",
					"key " + action.Key + " is also " + owner + " on row " + row.ID})
			}
			seen[action.Key] = name
		}
	}
	return found
}

func rowProblems(row *Row, where string, p *Panel, seen map[string]bool, depth int) []Invalid {
	if depth > maxDepth {
		return []Invalid{{where, fmt.Sprintf("nested deeper than %d", maxDepth)}}
	}
	var found []Invalid
	switch {
	case row.ID == "":
		found = append(found, Invalid{where + ".id", "a row needs an id"})
	case seen[row.ID]:
		found = append(found, Invalid{where + ".id", "duplicate row id " + row.ID})
	}
	seen[row.ID] = true
	if !row.Mark.Known() {
		found = append(found, Invalid{where + ".mark", "unknown mark " + string(row.Mark)})
	}
	if len(row.Fields) == 0 {
		found = append(found, Invalid{where + ".fields", "a row needs at least one field"})
	}
	names := map[string]bool{}
	for i, field := range row.Fields {
		at := fmt.Sprintf("%s.fields[%d]", where, i)
		switch {
		case field.Name == "":
			found = append(found, Invalid{at + ".name", "a field needs a name"})
		case names[field.Name]:
			found = append(found, Invalid{at + ".name", "duplicate field " + field.Name})
		}
		names[field.Name] = true
		if !field.Kind.Known() {
			found = append(found, Invalid{at + ".kind", "unknown kind " + string(field.Kind)})
		}
		if strings.ContainsAny(field.Value, "\n\r\t") {
			found = append(found, Invalid{at + ".value", "a field value is one line"})
		}
	}
	for _, name := range row.Actions {
		if _, ok := p.Actions[name]; !ok {
			found = append(found, Invalid{where + ".actions", "no action called " + name})
		}
	}
	for i := range row.Children {
		found = append(found, rowProblems(&row.Children[i],
			fmt.Sprintf("%s.children[%d]", where, i), p, seen, depth+1)...)
	}
	return found
}

func ValidateTargets(p *Panel) error {
	if found := targetProblems(p); len(found) > 0 {
		return found[0]
	}
	return nil
}

func targetProblems(p *Panel) []Invalid {
	ids := map[string]bool{}
	for _, row := range p.Rows() {
		ids[row.ID] = true
	}
	var found []Invalid
	for _, row := range p.Rows() {
		if row.Target != "" && !ids[row.Target] {
			found = append(found, Invalid{"rows." + row.ID + ".target", "no row called " + row.Target})
		}
	}
	return found
}
