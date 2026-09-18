package bango

import (
	"fmt"
	"regexp"
	"strings"
)

const maxDepth = 8

var idPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]+$`)

var reservedKeys = map[string]bool{
	"/": true, "?": true, "q": true, "⎋": true, "esc": true,
	"j": true, "k": true, "g": true, "G": true, " ": true,
}

type Invalid struct {
	Path   string
	Reason string
}

func (e Invalid) Error() string {
	return e.Path + ": " + e.Reason
}

func Validate(p *Panel) error {
	if p.Version == 0 {
		return Invalid{"bango", "no schema version — is this a bango document?"}
	}
	if p.Version != Version {
		return Invalid{"bango", fmt.Sprintf("version %d, this renderer speaks %d", p.Version, Version)}
	}
	if p.ID == "" || !idPattern.MatchString(p.ID) {
		return Invalid{"id", "must be non-empty and free of spaces"}
	}

	for name, action := range p.Actions {
		where := "actions." + name
		if action.Key == "" {
			return Invalid{where + ".key", "an action needs a key"}
		}
		if reservedKeys[action.Key] {
			return Invalid{where + ".key", "key " + action.Key + " is reserved by the renderer"}
		}
		if len(action.Args) > 0 && action.Verb == "" {
			return Invalid{where + ".verb", "args without a verb"}
		}
		if action.Verb == "" {
			return Invalid{where + ".verb", "an action needs a command to run"}
		}
		for i, retry := range action.Retry {
			at := fmt.Sprintf("%s.retry[%d]", where, i)
			if retry.Verb == "" {
				return Invalid{at + ".verb", "a retry needs a command to run"}
			}
			if retry.Label == "" {
				return Invalid{at + ".label", "a retry is offered on r and needs a label to offer"}
			}
		}
	}

	seenSections := map[string]bool{}
	seenRows := map[string]bool{}
	for i := range p.Sections {
		section := &p.Sections[i]
		where := fmt.Sprintf("sections[%d]", i)
		if section.ID == "" || !idPattern.MatchString(section.ID) {
			return Invalid{where + ".id", "must be non-empty and free of spaces"}
		}
		if seenSections[section.ID] {
			return Invalid{where + ".id", "duplicate section id " + section.ID}
		}
		seenSections[section.ID] = true
		for j := range section.Rows {
			if err := validateRow(&section.Rows[j], fmt.Sprintf("%s.rows[%d]", where, j), p, seenRows, 1); err != nil {
				return err
			}
		}
	}

	if err := ValidateTargets(p); err != nil {
		return err
	}
	return unambiguousKeys(p)
}

func unambiguousKeys(p *Panel) error {
	var global []string
	for name, action := range p.Actions {
		if action.Global {
			global = append(global, name)
		}
	}
	for _, row := range p.Rows() {
		seen := map[string]string{}
		for _, name := range append(append([]string{}, row.Actions...), global...) {
			action, ok := p.Actions[name]
			if !ok {
				continue
			}
			if owner, taken := seen[action.Key]; taken && owner != name {
				return Invalid{"actions." + name + ".key",
					"key " + action.Key + " is also " + owner + " on row " + row.ID}
			}
			seen[action.Key] = name
		}
	}
	return nil
}

func validateRow(row *Row, where string, p *Panel, seen map[string]bool, depth int) error {
	if depth > maxDepth {
		return Invalid{where, fmt.Sprintf("nested deeper than %d", maxDepth)}
	}
	if row.ID == "" {
		return Invalid{where + ".id", "a row needs an id"}
	}
	if seen[row.ID] {
		return Invalid{where + ".id", "duplicate row id " + row.ID}
	}
	seen[row.ID] = true
	if !row.Mark.Known() {
		return Invalid{where + ".mark", "unknown mark " + string(row.Mark)}
	}
	if len(row.Fields) == 0 {
		return Invalid{where + ".fields", "a row needs at least one field"}
	}
	names := map[string]bool{}
	for i, field := range row.Fields {
		at := fmt.Sprintf("%s.fields[%d]", where, i)
		if field.Name == "" {
			return Invalid{at + ".name", "a field needs a name"}
		}
		if names[field.Name] {
			return Invalid{at + ".name", "duplicate field " + field.Name}
		}
		names[field.Name] = true
		if !field.Kind.Known() {
			return Invalid{at + ".kind", "unknown kind " + string(field.Kind)}
		}
		if strings.ContainsAny(field.Value, "\n\r\t") {
			return Invalid{at + ".value", "a field value is one line"}
		}
	}
	for _, name := range row.Actions {
		if _, ok := p.Actions[name]; !ok {
			return Invalid{where + ".actions", "no action called " + name}
		}
	}
	for i := range row.Children {
		if err := validateRow(&row.Children[i], fmt.Sprintf("%s.children[%d]", where, i), p, seen, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func ValidateTargets(p *Panel) error {
	ids := map[string]bool{}
	for _, row := range p.Rows() {
		ids[row.ID] = true
	}
	for _, row := range p.Rows() {
		if row.Target != "" && !ids[row.Target] {
			return Invalid{"rows." + row.ID + ".target", "no row called " + row.Target}
		}
	}
	return nil
}
