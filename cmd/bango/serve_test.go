package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emusoi/bango"
)

func testServer(t *testing.T, readOnly bool) *server {
	t.Helper()
	var p bango.Panel
	if err := json.Unmarshal([]byte(sample), &p); err != nil {
		t.Fatal(err)
	}
	return &server{panel: p, token: "secret", addr: "127.0.0.1:7777", readOnly: readOnly,
		producer: []string{"printf", "%s", sample}}
}

func ask(s *server, method, path string, body string, set func(*http.Request)) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "http://127.0.0.1:7777"+path, strings.NewReader(body))
	request.Host = "127.0.0.1:7777"
	if set != nil {
		set(request)
	}
	recorder := httptest.NewRecorder()
	mux := http.NewServeMux()
	mux.HandleFunc("/panel", s.guard(s.current))
	mux.HandleFunc("/act", s.guard(s.act))
	mux.ServeHTTP(recorder, request)
	return recorder
}

func TestATokenIsRequired(t *testing.T) {
	s := testServer(t, false)
	if got := ask(s, "GET", "/panel", "", nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("no token got %d", got)
	}
	if got := ask(s, "GET", "/panel?t=wrong", "", nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("wrong token got %d", got)
	}
	if got := ask(s, "GET", "/panel?t=secret", "", nil).Code; got != http.StatusOK {
		t.Fatalf("right token got %d", got)
	}
}

func TestAPageFromAnotherOriginIsRefused(t *testing.T) {
	s := testServer(t, false)
	got := ask(s, "GET", "/panel?t=secret", "", func(r *http.Request) {
		r.Header.Set("Origin", "https://evil.example")
	})
	if got.Code != http.StatusForbidden {
		t.Fatalf("cross-origin got %d", got.Code)
	}
}

func TestAReboundHostIsRefused(t *testing.T) {
	s := testServer(t, false)
	request := httptest.NewRequest("GET", "http://attacker.example/panel?t=secret", nil)
	request.Host = "attacker.example"
	recorder := httptest.NewRecorder()
	s.guard(s.current)(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("rebound host got %d", recorder.Code)
	}
}

func TestReadOnlyRefusesActions(t *testing.T) {
	s := testServer(t, true)
	got := ask(s, "POST", "/act?t=secret", `{"action":"a","row":"r"}`, nil)
	if got.Code != http.StatusForbidden {
		t.Fatalf("read-only got %d: %s", got.Code, got.Body)
	}
}

func TestAnActionNotOfferedOnTheRowIsRefused(t *testing.T) {
	s := testServer(t, false)
	s.panel.Actions = map[string]bango.Action{"wipe": {Key: "x", Verb: "false"}}
	got := ask(s, "POST", "/act?t=secret", `{"action":"wipe","row":"r"}`, nil)
	if got.Code != http.StatusForbidden {
		t.Fatalf("unoffered action got %d: %s", got.Code, got.Body)
	}
}

func TestAnUnknownRowIsRefused(t *testing.T) {
	s := testServer(t, false)
	got := ask(s, "POST", "/act?t=secret", `{"action":"a","row":"nope"}`, nil)
	if got.Code != http.StatusBadRequest {
		t.Fatalf("unknown row got %d", got.Code)
	}
}

func TestTheServedPanelIsTheDocument(t *testing.T) {
	s := testServer(t, false)
	got := ask(s, "GET", "/panel?t=secret", "", nil)
	var payload struct {
		Revision int         `json:"revision"`
		ReadOnly bool        `json:"readOnly"`
		Panel    bango.Panel `json:"panel"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if err := bango.Validate(&payload.Panel); err != nil {
		t.Fatalf("what we serve must validate: %v", err)
	}
}

func TestATerminalIsBorrowedWhenStreamsArePipes(t *testing.T) {
	tty, borrowed := terminal()
	if borrowed {
		defer tty.Close()
		if tty.Name() == "" {
			t.Fatal("borrowed a terminal with no name")
		}
	}
}

func TestAProducerThatStartsFailingIsSaidOutLoud(t *testing.T) {
	s := testServer(t, false)
	if err := s.refresh(); err != nil {
		t.Fatalf("a working producer must not complain: %v", err)
	}
	before, trouble, _ := s.snapshot()
	if trouble != "" {
		t.Fatalf("trouble = %q while the producer works", trouble)
	}

	s.producer = []string{"sh", "-c", "echo the database is gone >&2; exit 1"}
	if err := s.refresh(); err == nil {
		t.Fatal("a broken producer must be reported to the caller")
	}
	after, trouble, _ := s.snapshot()
	if !strings.Contains(trouble, "the database is gone") {
		t.Errorf("trouble = %q, want the producer's own words", trouble)
	}
	if len(after.Sections) != len(before.Sections) {
		t.Error("the last good panel must stay on screen")
	}

	body := ask(s, "GET", "/panel", "", func(r *http.Request) { r.Header.Set("X-Bango-Token", "secret") }).Body.String()
	if !strings.Contains(body, "the database is gone") {
		t.Errorf("the browser is never told: %s", body)
	}

	s.producer = []string{"printf", "%s", sample}
	if err := s.refresh(); err != nil {
		t.Fatal(err)
	}
	if _, trouble, _ = s.snapshot(); trouble != "" {
		t.Errorf("a producer that recovers must clear the complaint, got %q", trouble)
	}
}
