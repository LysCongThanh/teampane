package backend

import (
	"strings"
)

// Zellij drives panes through `zellij action`. Targeted placement and
// pane-id flags need zellij 0.44.1+; teampane is tested against 0.45.
// zellij has no split-size option, so Percent is ignored.
type Zellij struct {
	R       Runner
	Self    string // ZELLIJ_PANE_ID (numeric)
	Session string // ZELLIJ_SESSION_NAME
}

func (z *Zellij) Name() string        { return "zellij" }
func (z *Zellij) CurrentPane() string { return z.Self }
func (z *Zellij) ScopeKey() string    { return "session:" + z.Session }

func (z *Zellij) action(env []string, args ...string) Cmd {
	return Cmd{Name: "zellij", Args: append([]string{"action"}, args...), Env: env}
}

// Split opens a pane next to the target. zellij places new panes relative
// to the pane named by ZELLIJ_PANE_ID, so the target is passed that way.
func (z *Zellij) Split(req SplitRequest) (string, error) {
	dir := "right"
	if req.Dir == Down {
		dir = "down"
	}
	args := []string{"new-pane", "--direction", dir, "--close-on-exit", "--near-current-pane"}
	if !req.Focus {
		args = append(args, "--no-focus")
	}
	if req.Cwd != "" {
		args = append(args, "--cwd", req.Cwd)
	}
	if req.Title != "" {
		args = append(args, "--name", req.Title)
	}
	args = append(append(args, "--"), req.Argv...)
	var env []string
	if req.Target != "" {
		env = []string{"ZELLIJ_PANE_ID=" + req.Target}
	}
	out, err := z.R.Run(z.action(env, args...))
	if err != nil {
		return "", err
	}
	// zellij prints "terminal_<n>"; ZELLIJ_PANE_ID inside the pane is "<n>".
	return strings.TrimPrefix(strings.TrimSpace(out), "terminal_"), nil
}

func (z *Zellij) SendText(native, text string) error {
	_, err := z.R.Run(z.action(nil, "write-chars", "--pane-id", native, "--", text))
	return err
}

func (z *Zellij) GetText(native string, scrollback bool) (string, error) {
	args := []string{"dump-screen", "--pane-id", native}
	if scrollback {
		args = append(args, "--full")
	}
	return z.R.Run(z.action(nil, args...))
}

func (z *Zellij) Close(native string) error {
	_, err := z.R.Run(z.action(nil, "close-pane", "--pane-id", native))
	return err
}

func (z *Zellij) SetTitle(native, title string) error {
	_, err := z.R.Run(z.action(nil, "rename-pane", "--pane-id", native, "--", title))
	return err
}

func (z *Zellij) Focus(native string) error {
	_, err := z.R.Run(z.action(nil, "focus-pane-id", native))
	return err
}

func (z *Zellij) Check() []Finding {
	out, err := z.R.Run(Cmd{Name: "zellij", Args: []string{"--version"}})
	if err != nil {
		return []Finding{{false, "zellij not runnable: " + err.Error()}}
	}
	res := []Finding{{true, "found " + strings.TrimSpace(out)}}
	if _, err := z.R.Run(z.action(nil, "new-pane", "--help")); err != nil {
		res = append(res, Finding{false, "zellij action unavailable: " + err.Error()})
	}
	return res
}
