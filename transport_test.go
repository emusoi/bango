package bango

import (
	"reflect"
	"strings"
	"testing"
)

func TestLocalArgvIsJustTheCommand(t *testing.T) {
	got := Local().Argv("loco", []string{"resolve", "t7"})
	if !reflect.DeepEqual(got, []string{"loco", "resolve", "t7"}) {
		t.Fatalf("got %q", got)
	}
}

func TestArgvTransportsDoNotQuote(t *testing.T) {
	got := Via([]string{"docker", "exec", "-i", "box"}).Argv("loco", []string{"reopen", "t7", "-m", "still broken"})
	want := []string{"docker", "exec", "-i", "box", "loco", "reopen", "t7", "-m", "still broken"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestSSHQuotesEveryElement(t *testing.T) {
	got := ViaSSH("fedora").Argv("loco", []string{"reopen", "t7", "-m", "still broken; rm -rf /"})
	line := got[len(got)-1]
	if !strings.HasSuffix(line, `'still broken; rm -rf /'`) {
		t.Fatalf("the message was not quoted: %q", line)
	}
	if got[0] != "ssh" || got[len(got)-2] != "fedora" {
		t.Fatalf("host lost: %q", got)
	}
}

func TestQuotingSurvivesQuotes(t *testing.T) {
	cases := map[string]string{
		"plain":            "plain",
		"":                 "''",
		"two words":        `'two words'`,
		"it's":             `'it'\''s'`,
		"$(whoami)":        `'$(whoami)'`,
		"a`b":              "'a`b'",
		"src/auth.ts:42":   "src/auth.ts:42",
		"--message=hello!": `'--message=hello!'`,
	}
	for arg, want := range cases {
		if got := ShellQuote(arg); got != want {
			t.Fatalf("%q quoted as %q, want %q", arg, got, want)
		}
	}
}

func TestShellJoinKeepsArgumentBoundaries(t *testing.T) {
	got := ShellJoin([]string{"loco", "add", "-f", "a b.ts", "-m", "it's fine"})
	want := `loco add -f 'a b.ts' -m 'it'\''s fine'`
	if got != want {
		t.Fatalf("got %s", got)
	}
}
