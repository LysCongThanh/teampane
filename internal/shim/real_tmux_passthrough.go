package shim

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/LysCongThanh/teampane/internal/tmuxargs"
	"github.com/LysCongThanh/teampane/internal/xdg"
)

// passthroughGuard is set when teampane hands a call to a real tmux, so a
// misconfigured PATH (a "real" tmux that is really teampane) cannot loop.
const passthroughGuard = "TEAMPANE_PASSTHROUGH"

// inTeampaneSession reports whether this invocation addresses a teampane
// session: the -S socket, or else the first field of $TMUX, lives in
// teampane's state directory.
func (c *Ctx) inTeampaneSession(g tmuxargs.Global) bool {
	socket := g.Socket
	if socket == "" {
		socket = c.Getenv("TMUX")
	}
	_, err := xdg.SessionFromSocket(xdg.StateRoot(), socket)
	return err == nil
}

// findRealTmux returns the first `tmux` on PATH that is not teampane itself
// (neither the shim directory nor anything resolving to this executable).
func findRealTmux(getenv func(string) string) string {
	if getenv(passthroughGuard) != "" {
		return ""
	}
	self, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(self); err == nil {
		self = r
	}
	shimDir := filepath.Clean(xdg.BinDir())
	for _, dir := range filepath.SplitList(getenv("PATH")) {
		if dir == "" || filepath.Clean(dir) == shimDir {
			continue
		}
		cand := filepath.Join(dir, "tmux")
		fi, err := os.Stat(cand)
		if err != nil || fi.IsDir() || fi.Mode().Perm()&0o111 == 0 {
			continue
		}
		if r, err := filepath.EvalSymlinks(cand); err != nil || r == self || filepath.Base(r) == "teampane" {
			continue
		}
		return cand
	}
	return ""
}

// execRealTmux replaces this process with the real tmux. It only returns
// (with status 1) if exec fails.
func (c *Ctx) execRealTmux(path string, args []string) int {
	env := append(os.Environ(), passthroughGuard+"=1")
	err := syscall.Exec(path, append([]string{path}, args...), env)
	return c.fail(err)
}
