package main

import (
	"testing"

	"github.com/emusoi/bango"
)

func TestServingBindsWhereItSaysItBinds(t *testing.T) {
	for _, one := range []struct{ addr, want string }{
		{"127.0.0.1:0", "127.0.0.1:0"},
		{"localhost:8080", "localhost:8080"},
		{"[::1]:0", "[::1]:0"},
		{":8080", "127.0.0.1:8080"},
	} {
		got, err := loopbackOnly(one.addr)
		if err != nil || got != one.want {
			t.Errorf("loopbackOnly(%q) = %q %v, want %q", one.addr, got, err, one.want)
		}
	}
	for _, addr := range []string{"0.0.0.0:8080", "192.168.1.5:8080", "[::]:8080", "8080"} {
		if got, err := loopbackOnly(addr); err == nil {
			t.Errorf("loopbackOnly(%q) = %q, want a refusal", addr, got)
		}
	}
}

func TestOnlyAChoiceTheScreenShowedIsAccepted(t *testing.T) {
	picky := bango.Action{Key: "m", Label: "move", Verb: "git", Choices: []string{"todo", "done"}}
	if !offered(picky, "done") {
		t.Error("a choice from the list must be accepted")
	}
	if offered(picky, "anything-else") {
		t.Error("a choice the panel never listed must be refused")
	}
	if offered(picky, "") {
		t.Error("an action that offers choices needs one of them")
	}

	plain := bango.Action{Key: "o", Label: "open", Verb: "git"}
	if !offered(plain, "") {
		t.Error("an action with no choices takes none")
	}
	if offered(plain, "smuggled") {
		t.Error("an action with no choices must not take one anyway")
	}
}
