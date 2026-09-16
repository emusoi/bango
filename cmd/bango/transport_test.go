package main

import (
	"strings"
	"testing"

	"github.com/joneskim/bango"
)

const sample = `{"bango":1,"id":"t","title":"t","sections":[{"id":"s","rows":[{"id":"r","fields":[{"name":"a","value":"one"}]}]}]}`

func TestProduceRunsThroughAnArgvTransport(t *testing.T) {
	panel, err := produce([]string{"printf", "%s", sample}, "", bango.Via([]string{"env"}))
	if err != nil {
		t.Fatal(err)
	}
	if panel.ID != "t" || len(panel.Rows()) != 1 {
		t.Fatalf("got %+v", panel)
	}
}

func TestProduceRefusesAnInvalidPanel(t *testing.T) {
	_, err := produce([]string{"printf", "%s", `{"bango":9,"id":"t","sections":[]}`}, "", bango.Local())
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("got %v", err)
	}
}

func TestExecuteSubstitutesWholeArguments(t *testing.T) {
	action := bango.Action{Verb: "printf", Args: []string{"%s", "{input}"}}
	choice := &Choice{Row: "r", Input: "two words; rm -rf /"}
	if err := execute(action, choice, bango.Local()); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteActionsAreQuotedNotSpliced(t *testing.T) {
	argv := bango.ViaSSH("box").Argv("loco", []string{"reopen", "t7", "-m", "two words; rm -rf /"})
	line := argv[len(argv)-1]
	if strings.Contains(line, "; rm -rf /'") == false {
		t.Fatalf("the message lost its quoting: %q", line)
	}
	if strings.HasSuffix(line, "rm -rf /") && !strings.Contains(line, "'") {
		t.Fatalf("unquoted: %q", line)
	}
}
