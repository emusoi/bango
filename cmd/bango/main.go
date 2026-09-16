package main

import (
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
	addr := flag.String("serve", "", "serve the panel to a browser on this address")
	readOnly := flag.Bool("read-only", false, "serve without running actions")
	watch := flag.Int("watch", 0, "re-run the producer every N seconds")
	via := flag.String("via", "", "run the producer and its actions through this command")
	viaSSH := flag.String("via-ssh", "", "run the producer and its actions on this host over ssh")
	flag.Parse()

	producer := flag.Args()
	opts := options{ascii: *ascii, width: *width, height: *height, want: *want,
		asJSON: *asJSON, print: *print}
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

type options struct {
	ascii     bool
	width     int
	height    int
	want      string
	asJSON    bool
	print     bool
	transport bango.Transport
	watch     int
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "bango:", err)
	return exitInvalid
}

func read(r io.Reader, want string) ([]bango.Panel, error) {
	dec := json.NewDecoder(r)
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
			return nil, err
		}
		if err := bango.Validate(&p); err != nil {
			return nil, err
		}
		if want == "" || p.ID == want {
			out = append(out, p)
		}
	}
}

func pipe(r io.Reader, opts options) int {
	panels, err := read(r, opts.want)
	if err != nil {
		return fail(err)
	}
	if len(panels) == 0 {
		return exitNothing
	}
	return run(panels[len(panels)-1], opts, nil)
}

func drive(producer []string, opts options) int {
	panel, err := produce(producer, opts.want, opts.transport)
	if err != nil {
		return fail(err)
	}
	return run(panel, opts, producer)
}

func produce(producer []string, want string, through bango.Transport) (bango.Panel, error) {
	argv := through.Argv(producer[0], producer[1:])
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return bango.Panel{}, err
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

func run(panel bango.Panel, opts options, producer []string) int {
	if opts.print {
		width := opts.width
		if width == 0 {
			width = 80
		}
		style := bango.Style{Width: width, ASCII: opts.ascii}
		fmt.Println(strings.Join(bango.Render(panel, style), "\n"))
		return exitOK
	}
	model := newModel(panel, opts, producer)
	program := tea.NewProgram(model, tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		return fail(err)
	}
	done := final.(*model2)
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
