package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/LysCongThanh/teampane/internal/backend"
	"github.com/LysCongThanh/teampane/internal/registry"
	"github.com/LysCongThanh/teampane/internal/xdg"
)

// errRealTmux means we already run inside tmux, which Claude Code supports natively.
var errRealTmux = errors.New("already inside a real tmux session; teampane is not needed")

// activation computes the variables Claude Code looks at to decide it runs
// inside tmux: TMUX (socket,pid,session) and TMUX_PANE, plus PATH with the
// shim directory first so `tmux` resolves to teampane.
func activation(getenv func(string) string, pid int) ([][2]string, error) {
	root := xdg.StateRoot()
	if t := getenv("TMUX"); t != "" {
		if _, err := xdg.SessionFromSocket(root, t); err != nil {
			return nil, errRealTmux
		}
	}
	b, err := backend.Detect(getenv, backend.ExecRunner{})
	if err != nil {
		return nil, err
	}
	native := b.CurrentPane()
	if native == "" {
		return nil, fmt.Errorf("%s detected but this process is not inside one of its panes", b.Name())
	}
	dir := xdg.SessionDir(root, b.Name(), b.ScopeKey())
	store, err := registry.Open(dir)
	if err != nil {
		return nil, err
	}
	var paneID string
	err = store.Update(func(st *registry.State) error {
		paneID = st.EnsureLeader(native, time.Now()).ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return [][2]string{
		{"PATH", prependPath(xdg.BinDir(), getenv("PATH"))},
		{"TMUX", xdg.TMUXValue(dir, pid)},
		{"TMUX_PANE", paneID},
		{"TEAMPANE_BACKEND", b.Name()},
	}, nil
}

// prependPath puts dir first in a PATH list, removing duplicates of it.
func prependPath(dir, path string) string {
	parts := []string{dir}
	for _, p := range filepath.SplitList(path) {
		if p != "" && filepath.Clean(p) != filepath.Clean(dir) {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, string(os.PathListSeparator))
}

// runCmd execs a command (normally `claude`) with the shim active for that
// process tree only, so the rest of the shell never sees a fake TMUX.
func runCmd(args []string, errw io.Writer) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintln(errw, "teampane run: missing command (try `teampane run claude`)")
		return 2
	}
	env := os.Environ()
	vars, err := activation(os.Getenv, os.Getpid())
	switch {
	case errors.Is(err, errRealTmux):
		// Run unchanged; Claude Code talks to the real tmux.
	case err != nil:
		fmt.Fprintln(errw, "teampane run:", err)
		return 1
	default:
		if _, err := ensureLink(xdg.BinDir()); err != nil {
			fmt.Fprintln(errw, "teampane run:", err)
			return 1
		}
		env = withVars(env, vars)
		os.Setenv("PATH", vars[0][1]) // so LookPath below sees the same PATH
	}
	path, err := exec.LookPath(args[0])
	if err != nil {
		fmt.Fprintln(errw, "teampane run:", err)
		return 127
	}
	err = syscall.Exec(path, args, env)
	fmt.Fprintln(errw, "teampane run:", err)
	return 126
}

// withVars overrides/adds KEY=VALUE pairs in env.
func withVars(env []string, vars [][2]string) []string {
	out := make([]string, 0, len(env)+len(vars))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		override := false
		for _, v := range vars {
			override = override || v[0] == name
		}
		if !override {
			out = append(out, kv)
		}
	}
	for _, v := range vars {
		out = append(out, v[0]+"="+v[1])
	}
	return out
}

// envCmd prints shell code that activates the shim in the current shell.
func envCmd(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	fs.SetOutput(errw)
	shell := fs.String("shell", "sh", "sh (bash/zsh) or fish")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	vars, err := activation(os.Getenv, os.Getppid())
	if err != nil {
		fmt.Fprintf(out, "# teampane: not activating: %v\n", err)
		return 0 // safe to eval from shell rc files everywhere
	}
	if _, err := ensureLink(xdg.BinDir()); err != nil {
		fmt.Fprintf(out, "# teampane: not activating: %v\n", err)
		return 0
	}
	for _, v := range vars {
		if *shell == "fish" {
			fmt.Fprintf(out, "set -gx %s %s;\n", v[0], shQuote(v[1]))
		} else {
			fmt.Fprintf(out, "export %s=%s\n", v[0], shQuote(v[1]))
		}
	}
	return 0
}

// shQuote single-quotes s for POSIX shells and fish.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
