package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/emusoi/bango"
)

type fault struct {
	Panel  string `json:"panel,omitempty"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func check(args []string) int {
	set := flag.NewFlagSet("bango check", flag.ContinueOnError)
	asJSON := set.Bool("json", false, "report as JSON")
	set.Usage = func() {
		fmt.Fprintln(set.Output(), "bango check reads panels and names every fault in them.")
		fmt.Fprintln(set.Output(), "\n  mytool panel dash | bango check")
		fmt.Fprintln(set.Output(), "  bango check fixtures/*.json")
		fmt.Fprintln(set.Output(), "\nIt exits 0 when there is nothing to say and 2 when there is.")
		fmt.Fprintln(set.Output(), "")
		set.PrintDefaults()
	}
	if err := set.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitInvalid
	}

	var faults []fault
	if set.NArg() == 0 {
		body, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fail(err)
		}
		found, err := faultsIn(body, "")
		if err != nil {
			return fail(err)
		}
		faults = found
	}
	for _, name := range set.Args() {
		body, err := os.ReadFile(name)
		if err != nil {
			return fail(err)
		}
		found, err := faultsIn(body, name)
		if err != nil {
			return fail(err)
		}
		faults = append(faults, found...)
	}

	if *asJSON {
		if faults == nil {
			faults = []fault{}
		}
		encoded, _ := json.Marshal(map[string]any{"ok": len(faults) == 0, "problems": faults})
		fmt.Println(string(encoded))
	} else {
		report(faults)
	}
	if len(faults) > 0 {
		return exitInvalid
	}
	return exitOK
}

func faultsIn(body []byte, where string) ([]fault, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var out []fault
	for at := 0; ; at++ {
		var p bango.Panel
		err := dec.Decode(&p)
		if errors.Is(err, io.EOF) {
			if at == 0 {
				return nil, explain(body, errors.New("there is no document here"))
			}
			return out, nil
		}
		if err != nil {
			return nil, explain(body, err)
		}
		for _, problem := range bango.Problems(&p) {
			out = append(out, fault{Panel: named(where, p, at), Path: problem.Path, Reason: problem.Reason})
		}
	}
}

func named(where string, p bango.Panel, at int) string {
	name := p.ID
	if name == "" {
		name = fmt.Sprintf("document %d", at+1)
	}
	if where != "" {
		return where + ": " + name
	}
	return name
}

func report(faults []fault) {
	if len(faults) == 0 {
		fmt.Println("no problems")
		return
	}
	many := len(faults) > 1
	widest := 0
	for _, one := range faults {
		if len(one.Path) > widest {
			widest = len(one.Path)
		}
	}
	shown := ""
	for _, one := range faults {
		if many && one.Panel != shown {
			fmt.Println(one.Panel)
			shown = one.Panel
		}
		fmt.Printf("  %-*s  %s\n", widest, one.Path, one.Reason)
	}
	word := "problems"
	if len(faults) == 1 {
		word = "problem"
	}
	fmt.Printf("%d %s\n", len(faults), word)
}

func schema(args []string) int {
	set := flag.NewFlagSet("bango schema", flag.ContinueOnError)
	set.Usage = func() {
		fmt.Fprintln(set.Output(), "bango schema prints the JSON Schema a panel is held to.")
		fmt.Fprintln(set.Output(), "\n  bango schema > panel.schema.json")
	}
	if err := set.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitInvalid
	}
	os.Stdout.Write(bango.Schema())
	return exitOK
}

func release() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	if tagged := info.Main.Version; tagged != "" && tagged != "(devel)" {
		return tagged
	}
	revision, dirty := "", false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}
	if revision == "" {
		return "devel"
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if dirty {
		revision += "+dirty"
	}
	return "devel-" + revision
}

func said() string {
	return strings.TrimSpace(fmt.Sprintf("bango %s (panel document version %d)", release(), bango.Version))
}
