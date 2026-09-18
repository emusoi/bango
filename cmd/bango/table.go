package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/emusoi/bango"
)

type column struct {
	name string
	kind bango.Kind
}

func table(args []string) int {
	set := flag.NewFlagSet("bango table", flag.ContinueOnError)
	columns := set.String("columns", "", "column names in order, each optionally name:kind")
	key := set.String("key", "", "the column holding each row's id (default: the first)")
	section := set.String("section", "", "group rows by this column")
	title := set.String("title", "", "the panel's title (default: its id)")
	id := set.String("id", "table", "the panel's id")
	empty := set.String("empty", "nothing to show", "what to say when there are no rows")
	run := set.String("run", "", "the command a pick runs, with {row} for the id")
	set.Usage = func() {
		fmt.Fprintln(set.Output(), "bango table turns tab-separated or JSON lines into a panel on stdout.")
		fmt.Fprintln(set.Output(), "\n  docker ps --format '{{.Names}}\\t{{.Status}}' |")
		fmt.Fprintln(set.Output(), "    bango table --columns name,status --run 'docker logs {row}' | bango")
		fmt.Fprintln(set.Output(), "")
		set.PrintDefaults()
	}
	if err := set.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitInvalid
	}

	cols, rows, err := readRows(os.Stdin, *columns)
	if err != nil {
		return fail(err)
	}
	panel, err := tabulate(cols, rows, *id, *title, *empty, *key, *section, *run)
	if err != nil {
		return fail(err)
	}
	if err := bango.Validate(&panel); err != nil {
		return fail(err)
	}
	encoded, err := json.Marshal(panel)
	if err != nil {
		return fail(err)
	}
	fmt.Println(string(encoded))
	return exitOK
}

func parseColumns(spec string) ([]column, error) {
	var out []column
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, kind, _ := strings.Cut(part, ":")
		one := column{name: strings.TrimSpace(name), kind: bango.Kind(strings.TrimSpace(kind))}
		if one.name == "" {
			return nil, fmt.Errorf("a column needs a name, and %q has none", part)
		}
		if !one.kind.Known() {
			return nil, fmt.Errorf("unknown kind %q in %q — text, path, count, time or ref", one.kind, part)
		}
		out = append(out, one)
	}
	return out, nil
}

func readRows(r io.Reader, spec string) ([]column, [][]string, error) {
	named, err := parseColumns(spec)
	if err != nil {
		return nil, nil, err
	}
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var lines []string
	for scan.Scan() {
		if line := scan.Text(); strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if err := scan.Err(); err != nil {
		return nil, nil, err
	}
	if len(lines) == 0 {
		return named, nil, nil
	}
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "{") {
		return jsonRows(lines, named)
	}
	cols, rows := tsvRows(lines, named)
	return cols, rows, nil
}

func tsvRows(lines []string, named []column) ([]column, [][]string) {
	rows := make([][]string, 0, len(lines))
	widest := 0
	for _, line := range lines {
		cells := strings.Split(line, "\t")
		if len(cells) > widest {
			widest = len(cells)
		}
		rows = append(rows, cells)
	}
	return fill(named, widest, func(i int) string { return fmt.Sprintf("field%d", i+1) }), rows
}

func jsonRows(lines []string, named []column) ([]column, [][]string, error) {
	objects := make([]map[string]string, 0, len(lines))
	var order []string
	for at, line := range lines {
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, fmt.Errorf("line %d is not a JSON object: %w", at+1, err)
		}
		flat := make(map[string]string, len(raw))
		for name, value := range raw {
			flat[name] = stringify(value)
		}
		if at == 0 {
			for name := range raw {
				order = append(order, name)
			}
			sort.Strings(order)
		}
		objects = append(objects, flat)
	}
	cols := named
	if len(cols) == 0 {
		for _, name := range order {
			cols = append(cols, column{name: name})
		}
	}
	rows := make([][]string, 0, len(objects))
	for _, object := range objects {
		cells := make([]string, len(cols))
		for i, one := range cols {
			cells[i] = object[one.name]
		}
		rows = append(rows, cells)
	}
	return cols, rows, nil
}

func stringify(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case bool:
		return fmt.Sprintf("%t", typed)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

func fill(named []column, want int, name func(int) string) []column {
	out := append([]column{}, named...)
	for i := len(out); i < want; i++ {
		out = append(out, column{name: name(i)})
	}
	return out
}

func tabulate(cols []column, rows [][]string, id, title, empty, key, section, run string) (bango.Panel, error) {
	if len(cols) == 0 && len(rows) > 0 {
		return bango.Panel{}, errors.New("there are rows but no columns — name them with --columns")
	}
	if title == "" {
		title = id
	}
	at := 0
	if key != "" {
		found := indexOf(cols, key)
		if found < 0 {
			return bango.Panel{}, fmt.Errorf("--key names %q, which is not a column", key)
		}
		at = found
	}
	group := -1
	if section != "" {
		if group = indexOf(cols, section); group < 0 {
			return bango.Panel{}, fmt.Errorf("--section names %q, which is not a column", section)
		}
	}

	panel := bango.Panel{
		Version: bango.Version, ID: id, Title: title, Empty: empty,
		Hints: []string{"⏎ pick", "/ filter", "q quit"},
	}
	if run != "" {
		parts := strings.Fields(run)
		if len(parts) == 0 {
			return bango.Panel{}, errors.New("--run was given nothing to run")
		}
		panel.Actions = map[string]bango.Action{
			"pick": {Key: "⏎", Label: "pick", Verb: parts[0], Args: parts[1:]},
		}
	}

	taken := map[string]int{}
	order := []string{}
	buckets := map[string][]bango.Row{}
	for _, cells := range rows {
		row := bango.Row{ID: unique(taken, clean(cell(cells, at)))}
		if run != "" {
			row.Actions = []string{"pick"}
		}
		for i, one := range cols {
			if i == group {
				continue
			}
			value := clean(cell(cells, i))
			if value == "" {
				continue
			}
			row.Fields = append(row.Fields, bango.Field{Name: one.name, Value: value, Kind: one.kind})
		}
		if len(row.Fields) == 0 {
			row.Fields = []bango.Field{{Name: cols[0].name, Value: row.ID}}
		}
		label := ""
		if group >= 0 {
			label = clean(cell(cells, group))
		}
		if _, seen := buckets[label]; !seen {
			order = append(order, label)
		}
		buckets[label] = append(buckets[label], row)
	}
	for i, label := range order {
		panel.Sections = append(panel.Sections, bango.Section{
			ID: fmt.Sprintf("s%d", i), Label: label, Rows: buckets[label],
		})
	}
	return panel, nil
}

func indexOf(cols []column, name string) int {
	for i, one := range cols {
		if one.name == name {
			return i
		}
	}
	return -1
}

func cell(cells []string, at int) string {
	if at < 0 || at >= len(cells) {
		return ""
	}
	return cells[at]
}

func clean(value string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, value))
}

func unique(taken map[string]int, id string) string {
	if id == "" {
		id = "row"
	}
	id = strings.Map(func(r rune) rune {
		if r == ' ' {
			return '-'
		}
		return r
	}, id)
	taken[id]++
	if seen := taken[id]; seen > 1 {
		return fmt.Sprintf("%s-%d", id, seen)
	}
	return id
}
