package shim

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LysCongThanh/teampane/internal/backend"
	"github.com/LysCongThanh/teampane/internal/format"
	"github.com/LysCongThanh/teampane/internal/panehost"
	"github.com/LysCongThanh/teampane/internal/registry"
	"github.com/LysCongThanh/teampane/internal/tmuxargs"
)

// defaultPrintFormat is tmux's format for -P without -F.
const defaultPrintFormat = "#{session_name}:#{window_index}.#{pane_index}"

// cmdSplitWindow: split-window [-dhvP] [-c dir] [-l size] [-t pane] [-F fmt] [cmd...]
func cmdSplitWindow(c *Ctx, cmd tmuxargs.Command) int {
	dir := backend.Down // tmux default is -v
	if cmd.Has("h") {
		dir = backend.Right
	}
	return c.split(cmd, dir, "", false)
}

// cmdNewWindow maps new-window onto a right split that gets its own tmux
// window id, since terminals have no hidden windows to put it in.
func cmdNewWindow(c *Ctx, cmd tmuxargs.Command) int {
	return c.split(cmd, backend.Right, cmd.Flags["n"], true)
}

// split creates a pane hosting `teampane pane-host`, waits until the host
// is ready to receive its command, and only then returns the pane id. That
// ordering is what fixes the send-keys race of anthropics/claude-code#40168.
func (c *Ctx) split(cmd tmuxargs.Command, dir backend.Direction, title string, ownWindow bool) int {
	mode := panehost.ModeInline
	switch {
	case len(cmd.Args) == 0:
		mode = panehost.ModeNone
	case len(cmd.Args) == 1 && strings.TrimSpace(cmd.Args[0]) == "cat":
		mode = panehost.ModePlaceholder // Claude Code: split ... -- cat, then respawn-pane
	}
	cwd := cmd.Flags["c"]
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	var target registry.Pane
	var id string
	err := c.Store.Update(func(st *registry.State) error {
		t, err := c.resolve(st, cmd.Flags["t"])
		if err != nil {
			return err
		}
		target, id = *t, st.Alloc()
		win := t.Window
		if ownWindow {
			win = "@" + strings.TrimPrefix(id, "%")
		}
		st.Panes = append(st.Panes, &registry.Pane{ID: id, Window: win, Title: title, Created: c.Now()})
		return nil
	})
	if err != nil {
		return c.fail(err)
	}
	files, err := panehost.PaneFiles(c.Dir, id)
	if err == nil {
		err = panehost.WriteEnv(files.Env, os.Environ())
	}
	if err != nil {
		c.forget(id)
		return c.fail(err)
	}
	argv := []string{c.Self, "pane-host", "--state", c.Dir, "--pane", id, "--backend", c.B.Name(), "--mode", mode, "--cwd", cwd, "--"}
	if mode == panehost.ModeInline {
		argv = append(argv, cmd.Args...)
	}
	native, err := c.B.Split(backend.SplitRequest{
		Target: target.Native, Dir: dir, Percent: percent(cmd), Focus: !cmd.Has("d"),
		Cwd: cwd, Title: title, Argv: argv,
	})
	if err != nil {
		c.forget(id)
		return c.fail(fmt.Errorf("create pane: %w", err))
	}
	_ = c.Store.Update(func(st *registry.State) error {
		if p := st.Find(id); p != nil && p.Native == "" {
			p.Native = native
		}
		return nil
	})
	if err := panehost.WaitReady(files.Ready, readyTimeout(c)); err != nil {
		_ = c.B.Close(native)
		c.forget(id)
		return c.fail(fmt.Errorf("pane %s: %w", id, err))
	}
	if cmd.Has("P") {
		return c.printPane(id, cmd.Flags["F"])
	}
	return 0
}

// printPane prints a pane through a format (split-window -P / -F).
func (c *Ctx) printPane(id, f string) int {
	if f == "" {
		f = defaultPrintFormat
	}
	st, err := c.Store.Read()
	if err != nil {
		return c.fail(err)
	}
	p := st.Find(id)
	if p == nil {
		return c.fail(fmt.Errorf("pane %s vanished", id))
	}
	out, missing := format.Expand(f, c.paneVars(st, p))
	if len(missing) > 0 {
		return c.unsupported("unsupported format variables " + join(missing))
	}
	fmt.Fprintln(c.Out, out)
	return 0
}

// forget drops a pane from the registry and removes its files.
func (c *Ctx) forget(id string) {
	_ = c.Store.Update(func(st *registry.State) error { st.Remove(id); return nil })
	if f, err := panehost.PaneFiles(c.Dir, id); err == nil {
		f.Cleanup()
	}
}

// percent reads -l N% (or legacy -p N); absolute sizes are ignored.
func percent(cmd tmuxargs.Command) int {
	if l, ok := cmd.Flag("l"); ok && strings.HasSuffix(l, "%") {
		n, _ := strconv.Atoi(strings.TrimSuffix(l, "%"))
		return n
	}
	if p, ok := cmd.Flag("p"); ok {
		n, _ := strconv.Atoi(p)
		return n
	}
	return 0
}

// readyTimeout is TEAMPANE_READY_TIMEOUT (a Go duration) or 15s.
func readyTimeout(c *Ctx) time.Duration {
	if d, err := time.ParseDuration(c.Getenv("TEAMPANE_READY_TIMEOUT")); err == nil && d > 0 {
		return d
	}
	return 15 * time.Second
}
