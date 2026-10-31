package backend

import (
	"strconv"
	"strings"
)

// WezTerm drives panes through `wezterm cli`, which talks to the running
// GUI/mux over WEZTERM_UNIX_SOCKET. WezTerm has no per-pane title API, so
// SetTitle is a no-op.
type WezTerm struct {
	R      Runner
	Self   string // WEZTERM_PANE
	Socket string // WEZTERM_UNIX_SOCKET
}

func (w *WezTerm) Name() string        { return "wezterm" }
func (w *WezTerm) CurrentPane() string { return w.Self }

func (w *WezTerm) ScopeKey() string {
	if w.Socket != "" {
		return w.Socket
	}
	return "default"
}

func (w *WezTerm) cli(stdin string, args ...string) Cmd {
	return Cmd{Name: "wezterm", Args: append([]string{"cli"}, args...), Stdin: stdin}
}

// Split runs `wezterm cli split-pane` and restores focus unless asked not to.
func (w *WezTerm) Split(req SplitRequest) (string, error) {
	args := []string{"split-pane"}
	if req.Target != "" {
		args = append(args, "--pane-id", req.Target)
	}
	if req.Dir == Down {
		args = append(args, "--bottom")
	} else {
		args = append(args, "--right")
	}
	if req.Percent > 0 && req.Percent < 100 {
		args = append(args, "--percent", strconv.Itoa(req.Percent))
	}
	if req.Cwd != "" {
		args = append(args, "--cwd", req.Cwd)
	}
	args = append(append(args, "--"), req.Argv...)
	out, err := w.R.Run(w.cli("", args...))
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(out)
	if !req.Focus && req.Target != "" {
		// split-pane focuses the new pane; hand focus back (tmux -d).
		_ = w.Focus(req.Target)
	}
	return id, nil
}

// SendText types text without bracketed paste so Enter executes.
func (w *WezTerm) SendText(native, text string) error {
	_, err := w.R.Run(w.cli(text, "send-text", "--pane-id", native, "--no-paste"))
	return err
}

func (w *WezTerm) GetText(native string, scrollback bool) (string, error) {
	args := []string{"get-text", "--pane-id", native}
	if scrollback {
		args = append(args, "--start-line", "-10000")
	}
	return w.R.Run(w.cli("", args...))
}

func (w *WezTerm) Close(native string) error {
	_, err := w.R.Run(w.cli("", "kill-pane", "--pane-id", native))
	return err
}

// SetTitle is unsupported per pane in WezTerm; titles stay in the registry.
func (w *WezTerm) SetTitle(native, title string) error { return nil }

func (w *WezTerm) Focus(native string) error {
	_, err := w.R.Run(w.cli("", "activate-pane", "--pane-id", native))
	return err
}

func (w *WezTerm) Check() []Finding {
	if _, err := w.R.Run(w.cli("", "list")); err != nil {
		return []Finding{{false, "wezterm cli failed: " + err.Error()}}
	}
	return []Finding{{true, "wezterm cli reachable"}}
}
