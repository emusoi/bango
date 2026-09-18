package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/emusoi/bango"
)

const (
	exitOK        = 0
	exitNothing   = 1
	exitInvalid   = 2
	exitCancelled = 130
)

type Choice struct {
	Action string `json:"action"`
	Row    string `json:"row"`
	Input  string `json:"input,omitempty"`
	Pick   string `json:"choice,omitempty"`
}

func main() {
	asJSON := flag.Bool("json", false, "print the whole choice")
	ascii := flag.Bool("ascii", false, "force the ASCII mark set")
	width := flag.Int("width", 0, "override the measured width")
	height := flag.Int("height", 0, "override the measured height")
	want := flag.String("id", "", "start on this panel")
	print := flag.Bool("print", false, "render once to stdout and exit")
	plain := flag.Bool("plain", false, "no colour")
	addr := flag.String("serve", "", "serve the panel to a browser on this address")
	readOnly := flag.Bool("read-only", false, "serve without running actions")
	watch := flag.Int("watch", 0, "re-run the producer every N seconds")
	via := flag.String("via", "", "run the producer and its actions through this command")
	viaSSH := flag.String("via-ssh", "", "run the producer and its actions on this host over ssh")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(said())
		os.Exit(exitOK)
	}

	producer := flag.Args()
	if len(producer) > 0 {
		switch producer[0] {
		case "table":
			os.Exit(table(producer[1:]))
		case "check":
			os.Exit(check(producer[1:]))
		case "schema":
			os.Exit(schema(producer[1:]))
		}
	}
	opts := options{ascii: *ascii, width: *width, height: *height, want: *want,
		asJSON: *asJSON, print: *print, plain: *plain}
	switch {
	case *viaSSH != "" && *via != "":
		fmt.Fprintln(os.Stderr, "bango: --via and --via-ssh are two answers to one question")
		os.Exit(exitInvalid)
	case *viaSSH != "":
		opts.transport = bango.ViaSSH(*viaSSH)
	case *via != "":
		opts.transport = bango.Via(strings.Fields(*via))
	}
	if opts.transport.Remote() && len(flag.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "bango: --via needs a producer to run: bango --via-ssh HOST -- CMD")
		os.Exit(exitInvalid)
	}

	opts.watch = *watch
	if opts.watch > 0 && len(producer) == 0 {
		fmt.Fprintln(os.Stderr, "bango: --watch needs a producer to re-run: bango --watch 5 -- CMD")
		os.Exit(exitInvalid)
	}
	if *addr != "" {
		if len(producer) == 0 {
			fmt.Fprintln(os.Stderr, "bango: --serve needs a producer: bango --serve 127.0.0.1:0 -- CMD")
			os.Exit(exitInvalid)
		}
		os.Exit(serve(producer, opts, *addr, *readOnly))
	}
	if len(producer) > 0 {
		os.Exit(drive(producer, opts))
	}
	os.Exit(pipe(os.Stdin, opts))
}

func follow(program *tea.Program, panels *stream) {
	for {
		next, err := panels.next()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			program.Send(complaint(err.Error()))
			return
		}
		program.Send(next)
	}
}

type options struct {
	ascii     bool
	width     int
	height    int
	want      string
	asJSON    bool
	print     bool
	plain     bool
	transport bango.Transport
	watch     int
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "bango:", err)
	return exitInvalid
}

func read(r io.Reader, want string) ([]bango.Panel, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	var out []bango.Panel
	for {
		var p bango.Panel
		err := dec.Decode(&p)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			if len(out) > 0 {
				return out, nil
			}
			return nil, explain(body, err)
		}
		if err := bango.Validate(&p); err != nil {
			return nil, explain(body, err)
		}
		if want == "" || p.ID == want {
			out = append(out, p)
		}
	}
}

func explain(body []byte, err error) error {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return errors.New("the producer printed nothing")
	}
	var wrapped map[string]json.RawMessage
	if json.Unmarshal(body, &wrapped) == nil {
		if _, envelope := wrapped["ok"]; envelope {
			if _, has := wrapped["data"]; has {
				return fmt.Errorf("%w — this looks like a wrapped response; "+
					"pass the document itself, not {ok, data}", err)
			}
		}
	}
	if !strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "[") {
		return fmt.Errorf("%w — the producer printed this, which is not JSON:\n%s",
			err, first(text, 200))
	}
	return fmt.Errorf("%w — the producer printed:\n%s", err, first(text, 200))
}

func first(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[:n] + "…"
}

type stream struct {
	dec  *json.Decoder
	seen *bytes.Buffer
	want string
}

func newStream(r io.Reader, want string) *stream {
	seen := &bytes.Buffer{}
	return &stream{dec: json.NewDecoder(io.TeeReader(r, seen)), seen: seen, want: want}
}

func (s *stream) next() (bango.Panel, error) {
	for {
		var p bango.Panel
		err := s.dec.Decode(&p)
		if errors.Is(err, io.EOF) {
			return bango.Panel{}, io.EOF
		}
		if err != nil {
			return bango.Panel{}, explain(s.seen.Bytes(), err)
		}
		if err := bango.Validate(&p); err != nil {
			return bango.Panel{}, explain(s.seen.Bytes(), err)
		}
		if s.want == "" || p.ID == s.want {
			return p, nil
		}
	}
}

func pipe(r io.Reader, opts options) int {
	panels := newStream(r, opts.want)
	first, err := panels.next()
	if errors.Is(err, io.EOF) {
		return exitNothing
	}
	if err != nil {
		return fail(err)
	}
	if opts.print {
		last := first
		for {
			next, err := panels.next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fail(err)
			}
			last = next
		}
		return run(last, opts, nil, nil)
	}
	return run(first, opts, nil, panels)
}

func drive(producer []string, opts options) int {
	panel, err := produce(producer, opts.want, opts.transport)
	if err != nil {
		return fail(err)
	}
	return run(panel, opts, producer, nil)
}

func produce(producer []string, want string, through bango.Transport) (bango.Panel, error) {
	argv := through.Argv(producer[0], producer[1:])
	cmd := exec.Command(argv[0], argv[1:]...)
	var complaint strings.Builder
	cmd.Stderr = &complaint
	out, err := cmd.Output()
	if err != nil {
		said := strings.TrimSpace(complaint.String())
		if said == "" {
			said = strings.TrimSpace(string(out))
		}
		if said == "" {
			said = err.Error()
		}
		return bango.Panel{}, fmt.Errorf("%s said:\n%s", strings.Join(producer, " "), first(said, 300))
	}
	panels, err := read(strings.NewReader(string(out)), want)
	if err != nil {
		return bango.Panel{}, err
	}
	if len(panels) == 0 {
		return bango.Panel{}, errors.New("the producer printed no panel")
	}
	return panels[len(panels)-1], nil
}

func terminal() (*os.File, bool) {
	piped := func(f *os.File) bool {
		info, err := f.Stat()
		return err != nil || info.Mode()&os.ModeCharDevice == 0
	}
	if !piped(os.Stdin) && !piped(os.Stdout) {
		return nil, false
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, false
	}
	return tty, true
}

func run(panel bango.Panel, opts options, producer []string, more *stream) int {
	if opts.print {
		width := opts.width
		if width == 0 {
			width = 80
		}
		style := bango.Style{Width: width, Height: opts.height, ASCII: opts.ascii}
		fmt.Println(strings.Join(bango.Render(panel, style), "\n"))
		return exitOK
	}
	screen := newModel(panel, opts, producer)
	settings := []tea.ProgramOption{tea.WithAltScreen()}
	if tty, borrowed := terminal(); borrowed {
		defer tty.Close()
		settings = append(settings, tea.WithInput(tty), tea.WithOutput(tty))
	}
	program := tea.NewProgram(screen, settings...)
	if more != nil {
		go follow(program, more)
	}
	final, err := program.Run()
	if err != nil {
		if strings.Contains(err.Error(), "could not open a new TTY") ||
			strings.Contains(err.Error(), "device not configured") {
			return fail(errors.New(
				"there is no terminal to draw on — pipe a panel to bango from a terminal, " +
					"or use --print to render once"))
		}
		return fail(err)
	}
	done := final.(*model)
	if done.failure != nil {
		return fail(done.failure)
	}
	if done.choice == nil {
		return exitCancelled
	}
	if opts.asJSON {
		encoded, _ := json.Marshal(done.choice)
		fmt.Println(string(encoded))
		return exitOK
	}
	fmt.Println(done.choice.Row)
	return exitOK
}
