// Package xdg resolves teampane's directories per the XDG base-dir spec,
// and the per-terminal session directory encoded in the fake TMUX value.
package xdg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// base returns $<env> when absolute, else $HOME/<fallback>.
func base(env string, fallback ...string) string {
	if d := os.Getenv(env); d != "" && filepath.IsAbs(d) {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(append([]string{home}, fallback...)...)
}

// StateRoot is $XDG_STATE_HOME/teampane (registries, logs).
func StateRoot() string { return filepath.Join(base("XDG_STATE_HOME", ".local", "state"), "teampane") }

// BinDir is $XDG_DATA_HOME/teampane/bin, the shim directory holding `tmux`.
func BinDir() string {
	return filepath.Join(base("XDG_DATA_HOME", ".local", "share"), "teampane", "bin")
}

// UnknownLog is where unsupported tmux invocations are recorded.
func UnknownLog() string { return filepath.Join(StateRoot(), "unknown.log") }

// SessionDir returns the registry directory for a backend's terminal scope.
// The scope key (socket path, session name) is hashed so it is path-safe.
func SessionDir(root, backendName, scopeKey string) string {
	sum := sha256.Sum256([]byte(scopeKey))
	return filepath.Join(root, backendName+"-"+hex.EncodeToString(sum[:])[:12])
}

// socketName is the pseudo socket file inside a session dir. Claude Code
// passes the first field of $TMUX back to us as `tmux -S <path>`.
const socketName = "tmux.sock"

// TMUXValue builds the fake $TMUX ("socket,pid,session") for a session dir.
func TMUXValue(sessionDir string, pid int) string {
	return filepath.Join(sessionDir, socketName) + "," + strconv.Itoa(pid) + ",0"
}

// SessionFromSocket maps a socket path (from -S or $TMUX) back to its
// session dir, refusing anything outside root so a forged TMUX value cannot
// make the shim write elsewhere.
func SessionFromSocket(root, socket string) (string, error) {
	socket, _, _ = strings.Cut(socket, ",")
	if socket == "" {
		return "", fmt.Errorf("not inside a teampane session (TMUX is empty); start Claude Code with `teampane run claude`")
	}
	if filepath.Base(socket) != socketName {
		return "", fmt.Errorf("TMUX=%q is not a teampane session (a real tmux?)", socket)
	}
	dir := filepath.Clean(filepath.Dir(socket))
	rel, err := filepath.Rel(filepath.Clean(root), dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || strings.Contains(rel, string(filepath.Separator)) {
		return "", fmt.Errorf("TMUX socket %q is outside %s", socket, root)
	}
	return dir, nil
}
