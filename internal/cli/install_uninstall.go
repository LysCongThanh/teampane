package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LysCongThanh/teampane/internal/xdg"
)

// selfPath returns the resolved path of the running teampane binary.
func selfPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// isOurLink reports whether link is a symlink that points to a teampane
// binary (this one, or another build named teampane).
func isOurLink(link, exe string) (bool, error) {
	fi, err := os.Lstat(link)
	if err != nil {
		return false, err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	dest, err := os.Readlink(link)
	if err != nil {
		return false, err
	}
	resolved, _ := filepath.EvalSymlinks(link)
	return dest == exe || resolved == exe || filepath.Base(dest) == "teampane", nil
}

// ensureLink creates <bin>/tmux -> teampane. It never replaces a regular
// file or a symlink to anything that is not teampane (e.g. a real tmux).
func ensureLink(bin string) (changed bool, err error) {
	exe, err := selfPath()
	if err != nil {
		return false, err
	}
	link := filepath.Join(bin, "tmux")
	if _, err := os.Lstat(link); err == nil {
		ours, err := isOurLink(link, exe)
		if err != nil {
			return false, err
		}
		if !ours {
			return false, fmt.Errorf("refusing to overwrite %s: it is not a teampane symlink", link)
		}
		if dest, _ := os.Readlink(link); dest == exe {
			return false, nil
		}
		if err := os.Remove(link); err != nil {
			return false, err
		}
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return false, err
	}
	return true, os.Symlink(exe, link)
}

const installNext = `
Start Claude Code through teampane (the fake tmux is only visible to that process):

  teampane run claude

Optional shell alias (~/.zshrc or ~/.bashrc):

  alias claude='teampane run claude'

Claude Code also needs agent teams on and split panes allowed, e.g. in ~/.claude/settings.json:

  "env": { "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1" },
  "teammateMode": "auto"

What teampane run sets: PATH (shim dir first), TMUX=<state>/tmux.sock,<pid>,0,
TMUX_PANE=%%N and TEAMPANE_BACKEND. Check everything with: teampane doctor
`

// installCmd creates the shim dir (never touching a real tmux binary).
func installCmd(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(errw)
	dry := fs.Bool("dry-run", false, "only print what would be done")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	bin := xdg.BinDir()
	exe, err := selfPath()
	if err != nil {
		fmt.Fprintln(errw, "teampane install:", err)
		return 1
	}
	if *dry {
		fmt.Fprintf(out, "would link %s -> %s\n", filepath.Join(bin, "tmux"), exe)
		fmt.Fprintf(out, installNext)
		return 0
	}
	changed, err := ensureLink(bin)
	if err != nil {
		fmt.Fprintln(errw, "teampane install:", err)
		return 1
	}
	verb := "already linked"
	if changed {
		verb = "linked"
	}
	fmt.Fprintf(out, "teampane: %s %s -> %s\n", verb, filepath.Join(bin, "tmux"), exe)
	fmt.Fprintf(out, installNext)
	return 0
}

// uninstallCmd removes the symlink (only if it is ours) and empty dirs.
func uninstallCmd(out, errw io.Writer) int {
	bin := xdg.BinDir()
	link := filepath.Join(bin, "tmux")
	exe, _ := selfPath()
	ours, err := isOurLink(link, exe)
	switch {
	case os.IsNotExist(err):
		fmt.Fprintln(out, "teampane: nothing to remove at", link)
	case err != nil:
		fmt.Fprintln(errw, "teampane uninstall:", err)
		return 1
	case !ours:
		fmt.Fprintf(errw, "teampane uninstall: %s is not a teampane symlink; leaving it alone\n", link)
		return 1
	default:
		if err := os.Remove(link); err != nil {
			fmt.Fprintln(errw, "teampane uninstall:", err)
			return 1
		}
		fmt.Fprintln(out, "teampane: removed", link)
	}
	os.Remove(bin)               // only succeeds when empty
	os.Remove(filepath.Dir(bin)) // likewise
	fmt.Fprintf(out, "State (registries, logs) stays in %s; delete it if you like.\n", xdg.StateRoot())
	fmt.Fprintln(out, "Remove any `teampane run` alias or `teampane env` line from your shell rc.")
	return 0
}
