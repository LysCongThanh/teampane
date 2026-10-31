package backend

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Kitty drives kitty windows through `kitty @` remote control. It needs
// `allow_remote_control yes` (or socket-only) and, for control from outside
// the window's tty, `listen_on unix:...` (exported as KITTY_LISTEN_ON).
// Splits only land side-by-side when the tab uses the "splits" layout.
type Kitty struct {
	R        Runner
	ListenOn string // KITTY_LISTEN_ON
	Self     string // KITTY_WINDOW_ID
	PID      string // KITTY_PID
}

func (k *Kitty) Name() string        { return "kitty" }
func (k *Kitty) CurrentPane() string { return k.Self }

// ScopeKey is the remote-control socket, else the kitty process id.
func (k *Kitty) ScopeKey() string {
	if k.ListenOn != "" {
		return k.ListenOn
	}
	return "pid:" + k.PID
}

// rc builds a `kitty @ [--to socket] <args>` command.
func (k *Kitty) rc(stdin string, args ...string) Cmd {
	full := []string{"@"}
	if k.ListenOn != "" {
		full = append(full, "--to", k.ListenOn)
	}
	return Cmd{Name: "kitty", Args: append(full, args...), Stdin: stdin}
}

// Split launches a new kitty window next to the target window.
func (k *Kitty) Split(req SplitRequest) (string, error) {
	loc := "vsplit" // vertical divider: new window on the right
	if req.Dir == Down {
		loc = "hsplit"
	}
	args := []string{"launch", "--type=window", "--location=" + loc}
	if req.Target != "" {
		args = append(args, "--match", "window_id:"+req.Target, "--next-to", "id:"+req.Target)
	}
	if !req.Focus {
		args = append(args, "--keep-focus")
	}
	if req.Percent > 0 && req.Percent < 100 {
		args = append(args, "--bias="+strconv.Itoa(req.Percent))
	}
	if req.Cwd != "" {
		args = append(args, "--cwd="+req.Cwd)
	}
	if req.Title != "" {
		args = append(args, "--title="+req.Title)
	}
	args = append(append(args, "--"), req.Argv...)
	out, err := k.R.Run(k.rc("", args...))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// SendText types text into a window; --stdin avoids escape processing.
func (k *Kitty) SendText(native, text string) error {
	_, err := k.R.Run(k.rc(text, "send-text", "--match", "id:"+native, "--stdin"))
	return err
}

// GetText returns the screen (or screen + scrollback) of a window.
func (k *Kitty) GetText(native string, scrollback bool) (string, error) {
	extent := "--extent=screen"
	if scrollback {
		extent = "--extent=all"
	}
	return k.R.Run(k.rc("", "get-text", "--match", "id:"+native, extent))
}

// Close closes a window; a window that is already gone is not an error.
func (k *Kitty) Close(native string) error {
	_, err := k.R.Run(k.rc("", "close-window", "--match", "id:"+native, "--ignore-no-match"))
	return err
}

func (k *Kitty) SetTitle(native, title string) error {
	_, err := k.R.Run(k.rc("", "set-window-title", "--match", "id:"+native, "--", title))
	return err
}

func (k *Kitty) Focus(native string) error {
	_, err := k.R.Run(k.rc("", "focus-window", "--match", "id:"+native))
	return err
}

// kittyOSWindow mirrors the parts of `kitty @ ls` output doctor needs.
type kittyOSWindow struct {
	Tabs []struct {
		Layout  string `json:"layout"`
		Windows []struct {
			ID int `json:"id"`
		} `json:"windows"`
	} `json:"tabs"`
}

// Check verifies remote control and the layout of the caller's tab.
func (k *Kitty) Check() []Finding {
	out, err := k.R.Run(k.rc("", "ls"))
	if err != nil {
		return []Finding{{false, "kitty remote control failed: " + err.Error() +
			" — add `allow_remote_control socket-only` and `listen_on unix:/tmp/kitty-{kitty_pid}` to kitty.conf, then restart kitty"}}
	}
	res := []Finding{{true, "kitty remote control reachable"}}
	if k.ListenOn == "" {
		res = append(res, Finding{false, "KITTY_LISTEN_ON is empty: control works only from inside kitty windows; set `listen_on unix:/tmp/kitty-{kitty_pid}` for reliability"})
	}
	var wins []kittyOSWindow
	if json.Unmarshal([]byte(out), &wins) == nil {
		for _, w := range wins {
			for _, t := range w.Tabs {
				for _, win := range t.Windows {
					if strconv.Itoa(win.ID) != k.Self {
						continue
					}
					if t.Layout == "splits" {
						res = append(res, Finding{true, "current tab uses the splits layout"})
					} else {
						res = append(res, Finding{false, "current tab layout is " + strconv.Quote(t.Layout) +
							": panes will follow that layout; add `enabled_layouts splits,stack` to kitty.conf for tmux-like splits"})
					}
				}
			}
		}
	}
	return res
}
