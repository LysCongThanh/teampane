// Package tmuxargs parses tmux command lines (global flags, subcommand,
// per-subcommand getopt-style flags and positional arguments) without
// executing anything. It only knows the flag shapes; meaning lives in shim.
package tmuxargs

// aliases maps tmux's short command names to their canonical names.
var aliases = map[string]string{
	"splitw":   "split-window",
	"send":     "send-keys",
	"killp":    "kill-pane",
	"display":  "display-message",
	"lsp":      "list-panes",
	"lsw":      "list-windows",
	"ls":       "list-sessions",
	"capturep": "capture-pane",
	"selectp":  "select-pane",
	"has":      "has-session",
	"new":      "new-session",
	"neww":     "new-window",
	"respawnp": "respawn-pane",
	"set":      "set-option",
	"setw":     "set-window-option",
	"show":     "show-options",
	"selectl":  "select-layout",
	"resizep":  "resize-pane",
	"kill-ses": "kill-session",
}

// valueFlags lists, per canonical command, the single-letter flags that take
// a value (getopt "x:" style). Flags not listed here are boolean.
// Source: tmux(1) manual, tmux 3.4.
var valueFlags = map[string]string{
	"split-window":      "celtFp",
	"send-keys":         "cNt",
	"kill-pane":         "t",
	"display-message":   "cdt",
	"list-panes":        "Fft",
	"list-windows":      "Fft",
	"list-sessions":     "Ff",
	"capture-pane":      "bESt",
	"select-pane":       "PTt",
	"has-session":       "t",
	"new-session":       "ceFfnstxy",
	"new-window":        "ceFnt",
	"respawn-pane":      "cet",
	"set-option":        "t",
	"set-window-option": "t",
	"show-options":      "t",
	"select-layout":     "t",
	"resize-pane":       "txy",
	"kill-session":      "t",
	"kill-window":       "t",
	"refresh-client":    "AfBCclt",
}

// globalValueFlags are tmux's own (pre-subcommand) flags that take a value.
const globalValueFlags = "cfLST"

// Canonical resolves a command alias to the canonical tmux command name.
func Canonical(name string) string {
	if c, ok := aliases[name]; ok {
		return c
	}
	return name
}

// Known reports whether teampane knows the flag shape of a command.
func Known(name string) bool {
	_, ok := valueFlags[name]
	return ok
}
