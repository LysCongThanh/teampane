// Package cli implements the `teampane` command (everything except the
// tmux shim itself, which lives in package shim).
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"github.com/LysCongThanh/teampane/internal/panehost"
	"github.com/LysCongThanh/teampane/internal/shim"
)

// Version is set at build time by GoReleaser (-ldflags "-X ...cli.Version=").
var Version = "dev"

const usage = `teampane — split-pane Claude Code Agent Teams in kitty, WezTerm and zellij
by pretending to be tmux.

Usage:
  teampane run [--] <command> [args...]   run a command (e.g. claude) with the tmux shim active
  teampane env [--shell sh|fish]         print exports that activate the shim in a shell
  teampane install [--dry-run]           create the shim dir with a tmux symlink
  teampane uninstall                     remove the tmux symlink
  teampane doctor                        check terminal, remote control and Claude Code settings
  teampane tmux <tmux args...>           run the shim directly (debugging)
  teampane version

Environment: TEAMPANE_BACKEND (kitty|wezterm|zellij) forces a backend;
TEAMPANE_DEBUG=1 logs every tmux call to $XDG_STATE_HOME/teampane/debug.log.
`

// Main dispatches a teampane subcommand and returns the exit status.
func Main(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	out, errw := os.Stdout, os.Stderr
	switch args[0] {
	case "run":
		return runCmd(args[1:], errw)
	case "env":
		return envCmd(args[1:], out, errw)
	case "install":
		return installCmd(args[1:], out, errw)
	case "uninstall":
		return uninstallCmd(out, errw)
	case "doctor":
		return doctorCmd(out)
	case "tmux":
		return shim.Main(args[1:])
	case "pane-host":
		return paneHostCmd(args[1:], errw)
	case "version", "--version", "-v":
		fmt.Fprintln(out, "teampane", version())
		return 0
	case "help", "--help", "-h":
		fmt.Fprint(out, usage)
		return 0
	}
	fmt.Fprintf(errw, "teampane: unknown command %q\n\n%s", args[0], usage)
	return 2
}

// version prefers the GoReleaser-injected Version and falls back to the
// module version recorded by `go install module@vX.Y.Z`.
func version() string {
	if Version != "dev" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return Version
}

// paneHostCmd is the hidden entry point that runs inside each new pane.
func paneHostCmd(args []string, errw io.Writer) int {
	fs := flag.NewFlagSet("pane-host", flag.ContinueOnError)
	fs.SetOutput(errw)
	o := panehost.Options{}
	fs.StringVar(&o.StateDir, "state", "", "session state dir")
	fs.StringVar(&o.PaneID, "pane", "", "tmux pane id")
	fs.StringVar(&o.Backend, "backend", "", "backend name")
	fs.StringVar(&o.Mode, "mode", panehost.ModeInline, "inline|placeholder|none")
	fs.StringVar(&o.Cwd, "cwd", "", "working directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	o.Argv = fs.Args()
	o.PlaceholderWait = envDuration("TEAMPANE_COMMAND_TIMEOUT", 10*time.Minute)
	o.ShellWait = envDuration("TEAMPANE_SHELL_WAIT", 15*time.Second)
	err := panehost.Run(o)
	// Run only returns on failure. Keep the pane open briefly so the user
	// can read why, since most terminals close the pane on exit.
	fmt.Fprintln(errw, "teampane pane-host:", err)
	time.Sleep(5 * time.Second)
	return 1
}

// envDuration parses a Go duration from the environment with a default.
func envDuration(name string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(os.Getenv(name)); err == nil && d > 0 {
		return d
	}
	return def
}
