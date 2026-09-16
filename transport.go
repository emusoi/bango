package bango

import "strings"

type Transport struct {
	Prefix []string
	Shell  bool
}

func Local() Transport { return Transport{} }

func Via(prefix []string) Transport { return Transport{Prefix: prefix} }

func ViaSSH(host string, extra ...string) Transport {
	prefix := []string{"ssh", "-o", "ControlMaster=auto", "-o", "ControlPersist=60s"}
	prefix = append(prefix, extra...)
	return Transport{Prefix: append(prefix, host), Shell: true}
}

func (t Transport) Remote() bool { return len(t.Prefix) > 0 }

func (t Transport) Argv(verb string, args []string) []string {
	if !t.Remote() {
		return append([]string{verb}, args...)
	}
	if !t.Shell {
		return append(append([]string{}, t.Prefix...), append([]string{verb}, args...)...)
	}
	return append(append([]string{}, t.Prefix...), ShellJoin(append([]string{verb}, args...)))
}

func ShellJoin(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = ShellQuote(arg)
	}
	return strings.Join(quoted, " ")
}

func ShellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if safe(arg) {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

func safe(arg string) bool {
	for _, r := range arg {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.' || r == '/' || r == ':' || r == '=' || r == '@' || r == ',' || r == '+':
		default:
			return false
		}
	}
	return true
}
