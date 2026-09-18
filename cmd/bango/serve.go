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
	"slices"
	"sync"
	"time"

	"github.com/emusoi/bango"
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
	trouble  string
	revision int

	listeners sync.Map
}

func serve(producer []string, opts options, addr string, readOnly bool) int {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fail(err)
	}
	s := &server{producer: producer, opts: opts, readOnly: readOnly, token: hex.EncodeToString(raw)}

	where, err := loopbackOnly(addr)
	if err != nil {
		return fail(err)
	}

	panel, err := produce(producer, opts.want, opts.transport)
	if err != nil {
		return fail(err)
	}
	s.panel = panel

	listener, err := net.Listen("tcp", where)
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
	httpd := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := httpd.Serve(listener); err != nil {
		return fail(err)
	}
	return exitOK
}

func offered(action bango.Action, pick string) bool {
	if len(action.Choices) == 0 {
		return pick == ""
	}
	return slices.Contains(action.Choices, pick)
}

func loopbackOnly(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("--serve wants host:port, and %q is not that", addr)
	}
	if host == "" {
		return net.JoinHostPort("127.0.0.1", port), nil
	}
	if host == "localhost" {
		return addr, nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return addr, nil
	}
	return "", fmt.Errorf("--serve binds loopback only, and %s is not loopback: "+
		"a panel reachable from the network is one the network can ask for, "+
		"and its actions run here", host)
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

func (s *server) snapshot() (bango.Panel, string, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.panel, s.trouble, s.revision
}

func (s *server) current(w http.ResponseWriter, r *http.Request) {
	panel, trouble, revision := s.snapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"revision": revision, "readOnly": s.readOnly, "panel": panel, "trouble": trouble,
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
	panel, trouble, revision := s.snapshot()
	body, err := json.Marshal(map[string]any{"revision": revision, "panel": panel, "trouble": trouble})
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
	s.mu.Lock()
	if err != nil {
		s.trouble = err.Error()
	} else {
		s.panel, s.trouble = panel, ""
	}
	s.revision++
	s.mu.Unlock()
	s.announce()
	return err
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
	panel, _, _ := s.snapshot()
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
	if !offered(action, choice.Pick) {
		http.Error(w, choice.Action+" does not offer the choice "+choice.Pick, http.StatusForbidden)
		return
	}
	choice.Row = row.TargetID()
	if _, err := execute(action, &choice, s.opts.transport); err != nil {
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
