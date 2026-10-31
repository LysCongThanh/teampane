package shim

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LysCongThanh/teampane/internal/xdg"
)

// maxLogSize caps each log file; the previous one is kept as <name>.1.
const maxLogSize = 1 << 20

// logUnknown records an unsupported invocation so new Claude Code tmux
// calls can be discovered (and reported) instead of failing silently.
func logUnknown(args []string, reason string) {
	appendLog(xdg.UnknownLog(), args, reason)
}

// debugLog records every invocation when TEAMPANE_DEBUG=1.
func debugLog(args []string, code int) {
	if os.Getenv("TEAMPANE_DEBUG") != "1" {
		return
	}
	appendLog(filepath.Join(xdg.StateRoot(), "debug.log"), args, fmt.Sprintf("exit=%d", code))
}

// appendLog writes one tab-separated line: time, note, JSON-encoded argv.
// Logging is best effort and never changes the command's outcome.
func appendLog(path string, args []string, note string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxLogSize {
		os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	argv, _ := json.Marshal(args)
	fmt.Fprintf(f, "%s\t%s\t%s\n", time.Now().Format(time.RFC3339), note, argv)
}
