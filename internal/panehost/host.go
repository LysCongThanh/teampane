package panehost

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/LysCongThanh/teampane/internal/backend"
	"github.com/LysCongThanh/teampane/internal/registry"
)

// Modes say what split-window asked the pane to run.
const (
	ModeInline      = "inline"      // a real command: run it right away
	ModePlaceholder = "placeholder" // "cat" placeholder: wait for respawn-pane
	ModeNone        = "none"        // no command: wait briefly for send-keys, else a shell
)

// Options configure one pane host.
type Options struct {
	StateDir string
	PaneID   string // "%N"
	Backend  string // backend name, to learn our own native pane id
	Mode     string
	Cwd      string
	Argv     []string // inline command
	// PlaceholderWait and ShellWait bound how long to wait for a command.
	PlaceholderWait, ShellWait time.Duration
}

// execFn replaces the process; tests swap it out.
var execFn = syscall.Exec

// Run registers the host, signals readiness, obtains the command and execs
// it. It only returns on error (or when execFn is stubbed in tests).
func Run(o Options) error {
	files, err := PaneFiles(o.StateDir, o.PaneID)
	if err != nil {
		return err
	}
	if err := register(o); err != nil {
		return err
	}
	snapshot, err := ReadEnv(files.Env)
	if err != nil {
		snapshot = os.Environ()
	}
	os.Remove(files.Env) // may hold secrets; it is only needed once
	env := MergeEnv(snapshot, os.Environ(), o.PaneID)
	if o.Cwd != "" {
		_ = os.Chdir(o.Cwd)
	}

	var argv []string
	if o.Mode == ModeInline {
		if err := markReady(files.Ready); err != nil {
			return err
		}
		argv = shellArgv(o.Argv)
	} else {
		if err := MakeFifo(files.Fifo); err != nil {
			return err
		}
		if err := markReady(files.Ready); err != nil {
			return err
		}
		wait := o.ShellWait
		if o.Mode == ModePlaceholder {
			wait = o.PlaceholderWait
		}
		cmd, err := ReadCommand(files.Fifo, wait)
		if err != nil {
			return err
		}
		switch {
		case strings.TrimSpace(cmd) != "":
			argv = []string{"/bin/sh", "-c", cmd}
		case o.Mode == ModePlaceholder:
			return errors.New("no command received for " + o.PaneID + " (timed out)")
		default:
			argv = []string{loginShell(env)}
		}
	}
	return execFn(argv[0], argv, env)
}

// register records our pid (and our native id, which is authoritative)
// so list-panes can tell live panes from dead ones.
func register(o Options) error {
	native := ""
	if b, err := backend.New(o.Backend, os.Getenv, backend.ExecRunner{}); err == nil {
		native = b.CurrentPane()
	}
	store, err := registry.Open(o.StateDir)
	if err != nil {
		return err
	}
	return store.Update(func(st *registry.State) error {
		p := st.Find(o.PaneID)
		if p == nil {
			return fmt.Errorf("pane %s is not registered (killed before it started?)", o.PaneID)
		}
		p.HostPID = os.Getpid()
		if native != "" {
			p.Native = native
		}
		return nil
	})
}

func markReady(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600)
}

// shellArgv mirrors tmux: one argument is a shell command line, several
// arguments are an argv executed directly (resolved via PATH by sh).
func shellArgv(args []string) []string {
	if len(args) == 1 {
		return []string{"/bin/sh", "-c", args[0]}
	}
	return append([]string{"/bin/sh", "-c", `exec "$@"`, "sh"}, args...)
}

// loginShell returns $SHELL from env, falling back to /bin/sh.
func loginShell(env []string) string {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "SHELL="); ok && strings.HasPrefix(v, "/") {
			return v
		}
	}
	return "/bin/sh"
}
