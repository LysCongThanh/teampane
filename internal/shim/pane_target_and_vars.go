package shim

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/LysCongThanh/teampane/internal/registry"
)

// sessionName is the single tmux session teampane pretends to run.
const sessionName = "teampane"

// resolve maps a tmux target ("%3", "@1", "teampane:@1.%3", "") to a pane.
// An empty or session-only target means the caller's pane ($TMUX_PANE).
func (c *Ctx) resolve(st *registry.State, target string) (*registry.Pane, error) {
	if i := strings.LastIndexByte(target, '%'); i >= 0 {
		id := "%" + digits(target[i+1:])
		if p := st.Find(id); p != nil {
			return p, nil
		}
		return nil, fmt.Errorf("can't find pane: %s", target)
	}
	if i := strings.LastIndexByte(target, '@'); i >= 0 {
		if panes := st.InWindow("@" + digits(target[i+1:])); len(panes) > 0 {
			return panes[0], nil
		}
		return nil, fmt.Errorf("can't find window: %s", target)
	}
	if cur := c.Getenv("TMUX_PANE"); cur != "" {
		if p := st.Find(cur); p != nil {
			return p, nil
		}
	}
	if p := st.FindNative(c.B.CurrentPane()); p != nil {
		return p, nil
	}
	return nil, fmt.Errorf("can't find current pane (TMUX_PANE=%q)", c.Getenv("TMUX_PANE"))
}

// digits returns the leading decimal digits of s.
func digits(s string) string {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return s[:n]
}

// paneVars are the tmux format variables teampane can answer for a pane.
// Anything else (sizes, terminal features) is deliberately absent so the
// caller gets an error instead of a made-up value.
func (c *Ctx) paneVars(st *registry.State, p *registry.Pane) map[string]string {
	win := st.InWindow(p.Window)
	idx := 0
	for i, q := range win {
		if q.ID == p.ID {
			idx = i
		}
	}
	active := "0"
	if p.ID == c.Getenv("TMUX_PANE") {
		active = "1"
	}
	host, _ := os.Hostname()
	v := map[string]string{
		"pane_id":             p.ID,
		"pane_index":          strconv.Itoa(idx),
		"pane_title":          p.Title,
		"pane_active":         active,
		"pane_dead":           "0",
		"window_id":           p.Window,
		"window_index":        strings.TrimPrefix(p.Window, "@"),
		"window_name":         sessionName,
		"window_panes":        strconv.Itoa(len(win)),
		"session_name":        sessionName,
		"session_id":          "$0",
		"client_control_mode": "0",
		"host":                host,
		"pid":                 strconv.Itoa(os.Getpid()),
		"version":             "3.4",
	}
	if p.HostPID > 0 {
		v["pane_pid"] = strconv.Itoa(p.HostPID)
	}
	return v
}
