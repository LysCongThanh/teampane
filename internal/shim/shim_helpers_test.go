package shim

import (
	"testing"
	"time"

	"github.com/LysCongThanh/teampane/internal/backend"
	"github.com/LysCongThanh/teampane/internal/registry"
	"github.com/LysCongThanh/teampane/internal/tmuxargs"
)

func testCtx(env map[string]string) *Ctx {
	getenv := func(k string) string { return env[k] }
	b, _ := backend.New("headless", getenv, nil)
	return &Ctx{Getenv: getenv, B: b, Now: time.Now}
}

func TestResolveTargets(t *testing.T) {
	st := &registry.State{NextID: 1}
	lead := st.EnsureLeader("lead", time.Now())
	mate := &registry.Pane{ID: st.Alloc(), Window: lead.Window, Native: "h1"}
	st.Panes = append(st.Panes, mate)
	c := testCtx(map[string]string{"TMUX_PANE": "%1", "TEAMPANE_HEADLESS_PANE": "lead"})

	cases := map[string]*registry.Pane{
		"":               lead,
		"%2":             mate,
		"@1":             lead,
		"teampane:@1.%2": mate,
		"teampane":       lead,
		"=teampane:":     lead,
	}
	for target, want := range cases {
		got, err := c.resolve(st, target)
		if err != nil || got != want {
			t.Errorf("resolve(%q) = %v %v, want %s", target, got, err, want.ID)
		}
	}
	for _, bad := range []string{"%9", "@7"} {
		if _, err := c.resolve(st, bad); err == nil {
			t.Errorf("resolve(%q) should fail", bad)
		}
	}
	// Without TMUX_PANE the caller is found by its native id.
	c2 := testCtx(map[string]string{"TEAMPANE_HEADLESS_PANE": "h1"})
	if got, _ := c2.resolve(st, ""); got != mate {
		t.Fatalf("native fallback = %v", got)
	}
}

func TestPaneVars(t *testing.T) {
	st := &registry.State{NextID: 1}
	lead := st.EnsureLeader("lead", time.Now())
	mate := &registry.Pane{ID: st.Alloc(), Window: lead.Window, Title: "tester", HostPID: 42}
	st.Panes = append(st.Panes, mate)
	v := testCtx(map[string]string{"TMUX_PANE": "%1"}).paneVars(st, mate)
	want := map[string]string{"pane_id": "%2", "pane_index": "1", "window_id": "@1", "window_index": "1",
		"pane_title": "tester", "pane_active": "0", "pane_pid": "42", "window_panes": "2", "session_name": "teampane"}
	for k, w := range want {
		if v[k] != w {
			t.Errorf("%s = %q, want %q", k, v[k], w)
		}
	}
	if _, ok := v["pane_width"]; ok {
		t.Error("pane_width must not be invented")
	}
}

func TestPercentAndSessionArg(t *testing.T) {
	parse := func(a ...string) tmuxargs.Command { c, _ := tmuxargs.Parse(a); return c }
	if p := percent(parse("split-window", "-l", "70%")); p != 70 {
		t.Errorf("percent -l 70%% = %d", p)
	}
	if p := percent(parse("split-window", "-l", "40")); p != 0 {
		t.Errorf("absolute size should be ignored, got %d", p)
	}
	if p := percent(parse("split-window", "-p", "25")); p != 25 {
		t.Errorf("percent -p 25 = %d", p)
	}
	for in, want := range map[string]string{"=claude-swarm": "claude-swarm", "a:1.2": "a", "": ""} {
		if got := sessionArg(in); got != want {
			t.Errorf("sessionArg(%q) = %q", in, got)
		}
	}
}
