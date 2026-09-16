package main

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joneskim/bango"
)

//go:embed web
var web embed.FS

type server struct {
	producer []string
	opts     options
	readOnly bool
	token    string
	addr     string

	mu       sync.RWMutex
	panel    bango.Panel
	revision int

	listeners sync.Map
}

func serve(producer []string, opts options, addr string, readOnly bool) int {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fail(err)
	}
	s := &server{producer: producer, opts: opts, readOnly: readOnly, token: hex.EncodeToString(raw)}

	panel, err := produce(producer, opts.want, opts.transport)
	if err != nil {
		return fail(err)
	}
	s.panel = panel

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fail(err)
	}
	s.addr = listener.Addr().String()

	if err := s.writeToken(); err != nil {
		return fail(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.guard(s.page))
	mux.HandleFunc("/panel", s.guard(s.current))
	mux.HandleFunc("/events", s.guard(s.events))
	mux.HandleFunc("/act", s.guard(s.act))

	fmt.Printf("http://%s/?t=%s\n", s.addr, s.token)
	if s.readOnly {
		fmt.Fprintln(os.Stderr, "bango: read-only; actions are refused")
	}

	if opts.watch > 0 {
		go s.poll(time.Duration(opts.watch) * time.Second)
	}
	if err := http.Serve(listener, mux); err != nil {
		return fail(err)
	}
	return exitOK
}

func (s *server) writeToken() error {
	dir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	dir = filepath.Join(dir, "bango")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "serve.token"), []byte(s.token+"\n"), 0o600)
}

func (s *server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !loopback(r.Host, s.addr) {
			http.Error(w, "bango serves loopback only", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, s.addr) {
			http.Error(w, "cross-origin requests are refused", http.StatusForbidden)
			return
		}
		if !s.authorised(r) {
			http.Error(w, "a token is required", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next(w, r)
	}
}

func (s *server) authorised(r *http.Request) bool {
	given := r.Header.Get("X-Bango-Token")
	if given == "" {
		given = r.URL.Query().Get("t")
	}
	return subtle.ConstantTimeCompare([]byte(given), []byte(s.token)) == 1
}

func loopback(host, addr string) bool {
	name, port, err := net.SplitHostPort(host)
	if err != nil {
		name, port = host, ""
	}
	_, want, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if port != "" && port != want {
		return false
	}
	if name == "localhost" {
		return true
	}
	ip := net.ParseIP(name)
	return ip != nil && ip.IsLoopback()
}

func sameOrigin(origin, addr string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return loopback(parsed.Host, addr)
}

func (s *server) page(w http.ResponseWriter, r *http.Request) {
	body, err := web.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'")
	w.Write(body)
}

func (s *server) snapshot() (bango.Panel, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.panel, s.revision
}

func (s *server) current(w http.ResponseWriter, r *http.Request) {
	panel, revision := s.snapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"revision": revision, "readOnly": s.readOnly, "panel": panel,
	})
}

func (s *server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported here", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")

	updates := make(chan int, 4)
	key := new(int)
	s.listeners.Store(key, updates)
	defer s.listeners.Delete(key)

	s.send(w, flusher)
	flusher.Flush()

	beat := time.NewTicker(25 * time.Second)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-updates:
			s.send(w, flusher)
			flusher.Flush()
		case <-beat.C:
			fmt.Fprint(w, ": beat\n\n")
			flusher.Flush()
		}
	}
}

func (s *server) send(w http.ResponseWriter, flusher http.Flusher) {
	panel, revision := s.snapshot()
	body, err := json.Marshal(map[string]any{"revision": revision, "panel": panel})
	if err != nil {
		return
	}
	fmt.Fprintf(w, "id: %d\ndata: %s\n\n", revision, body)
}

func (s *server) announce() {
	s.listeners.Range(func(_, value any) bool {
		select {
		case value.(chan int) <- 1:
		default:
		}
		return true
	})
}

func (s *server) refresh() error {
	panel, err := produce(s.producer, s.opts.want, s.opts.transport)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.panel = panel
	s.revision++
	s.mu.Unlock()
	s.announce()
	return nil
}

func (s *server) poll(every time.Duration) {
	for range time.Tick(every) {
		s.refresh()
	}
}

func (s *server) act(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "post an action", http.StatusMethodNotAllowed)
		return
	}
	if s.readOnly {
		http.Error(w, "this panel is served read-only", http.StatusForbidden)
		return
	}
	var choice Choice
	if err := json.NewDecoder(r.Body).Decode(&choice); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	panel, _ := s.snapshot()
	action, ok := panel.Actions[choice.Action]
	if !ok {
		http.Error(w, "no action called "+choice.Action, http.StatusBadRequest)
		return
	}
	row, found := panel.Find(choice.Row)
	if !found {
		http.Error(w, "no row called "+choice.Row, http.StatusBadRequest)
		return
	}
	if !action.Global && !allows(row, choice.Action) {
		http.Error(w, choice.Action+" is not offered on "+choice.Row, http.StatusForbidden)
		return
	}
	choice.Row = row.TargetID()
	if err := execute(action, &choice, s.opts.transport); err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if err := s.refresh(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.current(w, r)
}

func hostOnly(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return strings.TrimSpace(host)
}
