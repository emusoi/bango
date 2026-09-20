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
	mux.HandleFunc("/back", s.guard(s.back))
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

const opened = `{"bango":1,"id":"deeper","title":"deeper","sections":[{"id":"s","rows":[{"id":"r2","fields":[{"name":"a","value":"two"}]}]}]}`

// An action whose command prints a panel opens it, the way it does in a
// terminal, and the page can come back to the one it was opened from.
func TestAnActionThatPrintsAPanelOpensIt(t *testing.T) {
	s := testServer(t, false)
	s.panel.Actions = map[string]bango.Action{
		"open": {Key: "o", Verb: "printf", Args: []string{"%s", opened}, Panel: "deeper", Global: true},
	}
	got := ask(s, "POST", "/act?t=secret", `{"action":"open","row":"r"}`, nil)
	if got.Code != http.StatusOK {
		t.Fatalf("open got %d: %s", got.Code, got.Body)
	}
	if id, depth := served(t, got); id != "deeper" || depth != 1 {
		t.Fatalf("after opening, serving %q at depth %d", id, depth)
	}

	back := ask(s, "POST", "/back?t=secret", "", nil)
	if back.Code != http.StatusOK {
		t.Fatalf("back got %d: %s", back.Code, back.Body)
	}
	if id, depth := served(t, back); id != "t" || depth != 0 {
		t.Fatalf("after back, serving %q at depth %d", id, depth)
	}
}

// Going back from the bottom is not an error; the page simply stays there.
func TestBackAtTheBottomStaysPut(t *testing.T) {
	s := testServer(t, false)
	got := ask(s, "POST", "/back?t=secret", "", nil)
	if got.Code != http.StatusOK {
		t.Fatalf("back got %d: %s", got.Code, got.Body)
	}
	if id, depth := served(t, got); id != "t" || depth != 0 {
		t.Fatalf("serving %q at depth %d", id, depth)
	}
}

func served(t *testing.T, got *httptest.ResponseRecorder) (string, int) {
	t.Helper()
	var payload struct {
		Panel bango.Panel `json:"panel"`
		Depth int         `json:"depth"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Panel.ID, payload.Depth
}

// A global action is the screen's, not a row's, so it runs on a panel with no
// rows at all. Nothing could reach one from a page before.
func TestAGlobalActionRunsWithoutARow(t *testing.T) {
	s := testServer(t, false)
	s.panel.Actions = map[string]bango.Action{
		"browse": {Key: "b", Verb: "printf", Args: []string{"%s", opened}, Panel: "deeper", Global: true},
	}
	got := ask(s, "POST", "/act?t=secret", `{"action":"browse","row":""}`, nil)
	if got.Code != http.StatusOK {
		t.Fatalf("a global action with no row got %d: %s", got.Code, got.Body)
	}
	if id, _ := served(t, got); id != "deeper" {
		t.Fatalf("serving %q", id)
	}
}

// A row's action still belongs to that row.
func TestARowActionStillNeedsItsRow(t *testing.T) {
	s := testServer(t, false)
	s.panel.Actions = map[string]bango.Action{"wipe": {Key: "x", Verb: "false"}}
	got := ask(s, "POST", "/act?t=secret", `{"action":"wipe","row":""}`, nil)
	if got.Code != http.StatusBadRequest {
		t.Fatalf("a row action with no row got %d: %s", got.Code, got.Body)
	}
}

// An action that prints no panel refreshes the screen it was run on. It used to
// re-run the producer the server was started with, so answering a question
// inside an opened panel threw that panel away and went back to the first one.
func TestAnActionInsideAnOpenedPanelStaysThere(t *testing.T) {
	s := testServer(t, false)
	s.panel.Actions = map[string]bango.Action{
		"open":  {Key: "o", Verb: "printf", Args: []string{"%s", opened}, Panel: "deeper", Global: true},
		"touch": {Key: "t", Verb: "true", Global: true},
	}
	if got := ask(s, "POST", "/act?t=secret", `{"action":"open","row":"r"}`, nil); got.Code != http.StatusOK {
		t.Fatalf("open got %d: %s", got.Code, got.Body)
	}

	// The opened panel carries no actions of its own, so the one being run is
	// the one that was on screen when it opened; what matters is where it lands.
	s.mu.Lock()
	s.panel.Actions = map[string]bango.Action{"touch": {Key: "t", Verb: "true", Global: true}}
	s.mu.Unlock()

	got := ask(s, "POST", "/act?t=secret", `{"action":"touch","row":""}`, nil)
	if got.Code != http.StatusOK {
		t.Fatalf("touch got %d: %s", got.Code, got.Body)
	}
	if id, depth := served(t, got); id != "deeper" || depth != 1 {
		t.Fatalf("after an action it is serving %q at depth %d, not the panel it was run on", id, depth)
	}

	back := ask(s, "POST", "/back?t=secret", "", nil)
	if id, depth := served(t, back); id != "t" || depth != 0 {
		t.Fatalf("back reached %q at depth %d", id, depth)
	}
}
