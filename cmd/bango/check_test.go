package main

import (
	"strings"
	"testing"
)

const brokenPanel = `{"bango":1,"id":"dash",
 "actions":{"drop":{"key":"d","label":"drop","args":["--force"]}},
 "sections":[{"id":"a","rows":[
   {"id":"one","mark":"glowing","fields":[{"name":"x","value":"first"},{"name":"x","value":"dup"}]},
   {"id":"one","target":"nowhere","fields":[{"name":"y","value":"two"}]}]}]}`

func TestCheckNamesEveryFaultNotTheFirst(t *testing.T) {
	faults, err := faultsIn([]byte(brokenPanel), "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"actions.drop.verb",
		"sections[0].rows[0].mark",
		"sections[0].rows[0].fields[1].name",
		"sections[0].rows[1].id",
		"rows.one.target",
	}
	if len(faults) != len(want) {
		t.Fatalf("found %d faults, want %d: %+v", len(faults), len(want), faults)
	}
	for i, path := range want {
		if faults[i].Path != path {
			t.Errorf("fault %d is %q, want %q", i, faults[i].Path, path)
		}
		if faults[i].Reason == "" {
			t.Errorf("fault %d has no reason", i)
		}
	}
}

func TestCheckIsQuietAboutAGoodPanel(t *testing.T) {
	faults, err := faultsIn([]byte(sample), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) != 0 {
		t.Errorf("a valid panel produced %+v", faults)
	}
}

func TestCheckReadsEveryDocumentInTheStream(t *testing.T) {
	faults, err := faultsIn([]byte(brokenPanel+"\n"+sample+"\n"+brokenPanel), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) != 10 {
		t.Fatalf("two broken panels and a good one gave %d faults", len(faults))
	}
	for _, one := range faults {
		if one.Panel != "dash" {
			t.Errorf("a fault is attributed to %q", one.Panel)
		}
	}
}

func TestCheckSaysWhereWhenItWasGivenAFile(t *testing.T) {
	faults, _ := faultsIn([]byte(brokenPanel), "panels/dash.json")
	if len(faults) == 0 || !strings.HasPrefix(faults[0].Panel, "panels/dash.json: ") {
		t.Errorf("panel = %q", faults[0].Panel)
	}
}

func TestCheckExplainsWhatIsNotAPanelAtAll(t *testing.T) {
	if _, err := faultsIn([]byte("not json"), ""); err == nil {
		t.Error("prose must be reported, not counted as clean")
	} else if !strings.Contains(err.Error(), "not json") {
		t.Errorf("the producer's own output must be quoted back: %v", err)
	}
	if _, err := faultsIn(nil, ""); err == nil {
		t.Error("nothing at all must be reported")
	}
}

func TestTheVersionSaysWhatItSpeaks(t *testing.T) {
	said := said()
	if !strings.HasPrefix(said, "bango ") {
		t.Errorf("version = %q", said)
	}
	if !strings.Contains(said, "panel document version 1") {
		t.Errorf("the document version is the one that matters to a producer: %q", said)
	}
	if release() == "" {
		t.Error("a build with no version must still say something")
	}
}
