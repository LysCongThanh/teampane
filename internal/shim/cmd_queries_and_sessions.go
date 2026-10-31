package shim

import (
	"fmt"
	"strings"

	"github.com/LysCongThanh/teampane/internal/format"
	"github.com/LysCongThanh/teampane/internal/registry"
	"github.com/LysCongThanh/teampane/internal/tmuxargs"
)

// emit expands f for each pane and prints one line per pane. Unknown
// variables fail the whole command (and are logged) rather than lie.
func (c *Ctx) emit(st *registry.State, panes []*registry.Pane, f string) int {
	var b strings.Builder
	for _, p := range panes {
		out, missing := format.Expand(f, c.paneVars(st, p))
		if len(missing) > 0 {
			return c.unsupported("unsupported format variables " + join(missing))
		}
		b.WriteString(out + "\n")
	}
	fmt.Fprint(c.Out, b.String())
	return 0
}

// cmdDisplayMessage answers `display-message -p [-t pane] format`.
func cmdDisplayMessage(c *Ctx, cmd tmuxargs.Command) int {
	if !cmd.Has("p") {
		return 0 // a status-line message: nothing to show, not an error
	}
	st, err := c.Store.Read()
	if err != nil {
		return c.fail(err)
	}
	p, err := c.resolve(st, cmd.Flags["t"])
	if err != nil {
		return c.fail(err)
	}
	return c.emit(st, []*registry.Pane{p}, join(cmd.Args))
}

// cmdListPanes lists panes of the target's window (-a / -s: all panes).
func cmdListPanes(c *Ctx, cmd tmuxargs.Command) int {
	st, err := c.Store.Read()
	if err != nil {
		return c.fail(err)
	}
	panes := st.Panes
	if !cmd.Has("a") && !cmd.Has("s") {
		p, err := c.resolve(st, cmd.Flags["t"])
		if err != nil {
			return c.fail(err)
		}
		panes = st.InWindow(p.Window)
	}
	f := cmd.Flags["F"]
	if f == "" {
		f = "#{pane_index}: #{pane_id}"
	}
	return c.emit(st, panes, f)
}

// cmdListWindows lists one line per window, described by its first pane.
func cmdListWindows(c *Ctx, cmd tmuxargs.Command) int {
	st, err := c.Store.Read()
	if err != nil {
		return c.fail(err)
	}
	var firsts []*registry.Pane
	for _, w := range st.Windows() {
		firsts = append(firsts, st.InWindow(w)[0])
	}
	f := cmd.Flags["F"]
	if f == "" {
		f = "#{window_index}: #{window_name} (#{window_panes} panes)"
	}
	return c.emit(st, firsts, f)
}

// cmdCapturePane prints a pane's text (-p only; paste buffers are not emulated).
func cmdCapturePane(c *Ctx, cmd tmuxargs.Command) int {
	if !cmd.Has("p") {
		return c.unsupported("capture-pane without -p (paste buffers) is not supported")
	}
	p, err := c.lookup(cmd.Flags["t"])
	if err != nil {
		return c.fail(err)
	}
	start := cmd.Flags["S"]
	txt, err := c.B.GetText(p.Native, strings.HasPrefix(start, "-"))
	if err != nil {
		return c.fail(err)
	}
	if txt != "" && !strings.HasSuffix(txt, "\n") {
		txt += "\n"
	}
	fmt.Fprint(c.Out, txt)
	return 0
}

// sessionArg normalises "=name", "name:" and "name:0.1" to "name".
func sessionArg(t string) string {
	t = strings.TrimPrefix(t, "=")
	name, _, _ := strings.Cut(t, ":")
	return name
}

// cmdHasSession succeeds for the built-in session and recorded ones.
func cmdHasSession(c *Ctx, cmd tmuxargs.Command) int {
	name := sessionArg(cmd.Flags["t"])
	st, err := c.Store.Read()
	if err != nil {
		return c.fail(err)
	}
	if name == "" || name == sessionName || st.HasSession(name) {
		return 0
	}
	fmt.Fprintf(c.Err, "can't find session: %s\n", name)
	return 1
}

// cmdNewSession records a detached, empty session name. Sessions with a
// command or an attached client cannot be emulated.
func cmdNewSession(c *Ctx, cmd tmuxargs.Command) int {
	if !cmd.Has("d") || cmd.Has("P") || len(cmd.Args) > 0 {
		return c.unsupported("only `new-session -d -s name` (no command) is supported")
	}
	name := cmd.Flags["s"]
	err := c.Store.Update(func(st *registry.State) error {
		if name != "" && !st.HasSession(name) {
			st.Sessions = append(st.Sessions, name)
		}
		return nil
	})
	if err != nil {
		return c.fail(err)
	}
	return 0
}

// cmdKillSession forgets a recorded session name (panes are untouched).
func cmdKillSession(c *Ctx, cmd tmuxargs.Command) int {
	name := sessionArg(cmd.Flags["t"])
	err := c.Store.Update(func(st *registry.State) error {
		kept := st.Sessions[:0]
		for _, s := range st.Sessions {
			if s != name {
				kept = append(kept, s)
			}
		}
		st.Sessions = kept
		return nil
	})
	if err != nil {
		return c.fail(err)
	}
	return 0
}

// cmdShowOptions answers the only option Claude Code reads (prefix).
func cmdShowOptions(c *Ctx, cmd tmuxargs.Command) int {
	for _, a := range cmd.Args {
		if a == "prefix" {
			fmt.Fprintln(c.Out, "prefix C-b")
		}
	}
	return 0
}
