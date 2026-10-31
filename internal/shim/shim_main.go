// Package shim implements the fake `tmux` binary: it parses the tmux
// command Claude Code runs, resolves tmux pane ids through the registry and
// performs the action with the terminal's native panes.
package shim

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/LysCongThanh/teampane/internal/backend"
	"github.com/LysCongThanh/teampane/internal/panehost"
	"github.com/LysCongThanh/teampane/internal/registry"
	"github.com/LysCongThanh/teampane/internal/tmuxargs"
	"github.com/LysCongThanh/teampane/internal/xdg"
)

// FakeVersion is what `tmux -V` prints. Claude Code only checks the exit code.
const FakeVersion = "tmux 3.4 (teampane)"

// Ctx carries everything a command handler needs.
type Ctx struct {
	Out, Err io.Writer
	Getenv   func(string) string
	Store    *registry.Store
	B        backend.Backend
	Self     string // path of the teampane executable (for pane-host)
	Dir      string // session state dir
	Now      func() time.Time
	Raw      []string // original arguments, for logs
}

type handler func(c *Ctx, cmd tmuxargs.Command) int

// handlers lists every tmux command teampane understands. Anything else is
// logged to unknown.log and fails with exit status 1.
var handlers = map[string]handler{
	"split-window":      cmdSplitWindow,
	"new-window":        cmdNewWindow,
	"send-keys":         cmdSendKeys,
	"respawn-pane":      cmdRespawnPane,
	"kill-pane":         cmdKillPane,
	"select-pane":       cmdSelectPane,
	"display-message":   cmdDisplayMessage,
	"list-panes":        cmdListPanes,
	"list-windows":      cmdListWindows,
	"capture-pane":      cmdCapturePane,
	"has-session":       cmdHasSession,
	"new-session":       cmdNewSession,
	"kill-session":      cmdKillSession,
	"set-option":        cmdNoop,
	"set-window-option": cmdNoop,
	"select-layout":     cmdNoop,
	"resize-pane":       cmdNoop,
	"refresh-client":    cmdNoop,
	"show-options":      cmdShowOptions,
}

// Main runs one tmux invocation and returns the exit status.
func Main(args []string) int {
	c := &Ctx{Out: os.Stdout, Err: os.Stderr, Getenv: os.Getenv, Now: time.Now, Raw: args}
	code := c.run(args)
	debugLog(args, code)
	return code
}

func (c *Ctx) run(args []string) int {
	g, err := tmuxargs.ParseGlobal(args)
	if err != nil {
		return c.unsupported(err.Error())
	}
	if g.Version {
		fmt.Fprintln(c.Out, FakeVersion)
		return 0
	}
	if len(g.Rest) == 0 {
		return c.unsupported("teampane is a tmux shim; it cannot start interactive tmux sessions")
	}
	cmd, err := tmuxargs.Parse(g.Rest)
	if err != nil {
		return c.unsupported(err.Error())
	}
	h, ok := handlers[cmd.Name]
	if !ok {
		return c.unsupported("unsupported tmux command " + cmd.Name)
	}
	if g.SocketName != "" && g.Socket == "" {
		return c.unsupported("tmux -L (separate server) is not supported; run Claude Code via `teampane run`")
	}
	if err := c.open(g.Socket); err != nil {
		return c.fail(err)
	}
	return h(c, cmd)
}

// open locates the session registry and backend for this invocation.
func (c *Ctx) open(socket string) error {
	if socket == "" {
		socket = c.Getenv("TMUX")
	}
	dir, err := xdg.SessionFromSocket(xdg.StateRoot(), socket)
	if err != nil {
		return err
	}
	if c.Store, err = registry.Open(dir); err != nil {
		return err
	}
	c.Dir = dir
	if c.B, err = backend.Detect(c.Getenv, backend.ExecRunner{}); err != nil {
		return err
	}
	if c.Self, err = os.Executable(); err != nil {
		return err
	}
	return c.Store.Update(c.housekeep)
}

// housekeep adopts the caller's pane if the registry lost it and prunes
// panes whose host process died (e.g. a teammate that exited by itself).
func (c *Ctx) housekeep(st *registry.State) error {
	if cur := c.Getenv("TMUX_PANE"); cur != "" && st.Find(cur) == nil {
		if k, err := panehost.Key(cur); err == nil {
			st.Panes = append(st.Panes, &registry.Pane{ID: cur, Native: c.B.CurrentPane(), Window: "@" + k, Leader: true, Created: c.Now()})
			if n := atoi(k); n >= st.NextID {
				st.NextID = n + 1
			}
		}
	}
	for _, p := range st.Prune(alive, c.Now(), 30*time.Second) {
		if f, err := panehost.PaneFiles(c.Dir, p.ID); err == nil {
			f.Cleanup()
		}
	}
	return nil
}

// alive reports whether a process exists (signal 0).
func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func atoi(s string) int {
	n := 0
	for _, ch := range s {
		n = n*10 + int(ch-'0')
	}
	return n
}

// fail prints a tmux-style error and returns status 1.
func (c *Ctx) fail(err error) int {
	fmt.Fprintln(c.Err, "teampane:", err)
	return 1
}

// unsupported logs the invocation and fails safely without side effects.
func (c *Ctx) unsupported(reason string) int {
	logUnknown(c.Raw, reason)
	fmt.Fprintf(c.Err, "teampane: %s (logged to %s)\n", reason, xdg.UnknownLog())
	return 1
}

// cmdNoop accepts cosmetic commands (styles, layouts) the terminal handles itself.
func cmdNoop(c *Ctx, cmd tmuxargs.Command) int { return 0 }

// join renders arguments for messages.
func join(args []string) string { return strings.Join(args, " ") }
