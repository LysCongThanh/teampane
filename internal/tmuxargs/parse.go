package tmuxargs

import (
	"fmt"
	"strings"
)

// Global holds the tmux flags given before the subcommand.
type Global struct {
	Socket     string // -S socket-path
	SocketName string // -L socket-name
	Version    bool   // -V
	Rest       []string
}

// Command is one parsed tmux subcommand.
type Command struct {
	Name  string            // canonical name, e.g. "split-window"
	Flags map[string]string // value flags, e.g. "t" -> "%1"
	Bools map[string]bool   // boolean flags, e.g. "d" -> true
	Args  []string          // positional arguments (after flags or "--")
}

// Flag returns the value of a value flag and whether it was given.
func (c Command) Flag(name string) (string, bool) {
	v, ok := c.Flags[name]
	return v, ok
}

// Has reports whether a boolean flag was given.
func (c Command) Has(name string) bool { return c.Bools[name] }

// ParseGlobal strips tmux's global flags (-L, -S, -f, -2, -u, ...) and
// returns them together with the remaining arguments (subcommand first).
func ParseGlobal(args []string) (Global, error) {
	var g Global
	flags, bools, rest, err := getopt(args, globalValueFlags)
	if err != nil {
		return g, err
	}
	g.Socket = flags["S"]
	g.SocketName = flags["L"]
	g.Version = bools["V"]
	g.Rest = rest
	return g, nil
}

// Parse parses a subcommand and its arguments. args[0] is the subcommand.
// Commands with an unknown flag shape are returned with all remaining
// arguments as positional so callers can still log them.
func Parse(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{}, fmt.Errorf("no command")
	}
	name := Canonical(args[0])
	cmd := Command{Name: name, Flags: map[string]string{}, Bools: map[string]bool{}}
	spec, ok := valueFlags[name]
	if !ok {
		cmd.Args = append([]string(nil), args[1:]...)
		return cmd, nil
	}
	flags, bools, rest, err := getopt(args[1:], spec)
	if err != nil {
		return cmd, fmt.Errorf("%s: %w", name, err)
	}
	cmd.Flags, cmd.Bools, cmd.Args = flags, bools, rest
	return cmd, nil
}

// getopt implements POSIX getopt semantics: clustered flags ("-dh"), values
// attached ("-t%1") or separate ("-t %1"), "--" terminator, and parsing
// stops at the first positional argument.
func getopt(args []string, valueSpec string) (map[string]string, map[string]bool, []string, error) {
	flags := map[string]string{}
	bools := map[string]bool{}
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if len(a) < 2 || a[0] != '-' {
			break
		}
		for j := 1; j < len(a); j++ {
			f := string(a[j])
			if !strings.Contains(valueSpec, f) {
				bools[f] = true
				continue
			}
			if j+1 < len(a) {
				flags[f] = a[j+1:]
			} else if i+1 < len(args) {
				i++
				flags[f] = args[i]
			} else {
				return nil, nil, nil, fmt.Errorf("flag -%s requires a value", f)
			}
			break
		}
		i++
	}
	return flags, bools, append([]string(nil), args[i:]...), nil
}
