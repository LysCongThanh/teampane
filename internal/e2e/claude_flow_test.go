package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestClaudeCodeSpawnSequence replays the tmux calls Claude Code 2.1.x
// makes to spawn and tear down a split-pane teammate.
func TestClaudeCodeSpawnSequence(t *testing.T) {
	e := newEnv(t)
	e.activate()

	if v := e.mustTmux("-V"); !strings.HasPrefix(v, "tmux 3.4") {
		t.Fatalf("-V = %q", v)
	}
	lead := e.mustTmux("display-message", "-p", "#{pane_id}")
	if lead != "%1" {
		t.Fatalf("leader pane = %q", lead)
	}
	win := e.mustTmux("display-message", "-t", lead, "-p", "#{window_id}")
	if got := e.mustTmux("display-message", "-p", "-t", lead, "#{session_name}:#{window_id}.#{pane_id}"); got != "teampane:"+win+"."+lead {
		t.Fatalf("session triple = %q", got)
	}
	if got := e.mustTmux("list-panes", "-t", win, "-F", "#{pane_id}"); got != lead {
		t.Fatalf("list-panes before spawn = %q", got)
	}

	// New protocol: placeholder pane, title, cosmetic options, respawn.
	pane := e.mustTmux("split-window", "-d", "-t", win, "-h", "-l", "70%", "-P", "-F", "#{pane_id}", "--", "cat")
	if pane != "%2" {
		t.Fatalf("new pane = %q", pane)
	}
	e.mustTmux("set-option", "-p", "-t", pane, "pane-border-style", "fg=blue")
	e.mustTmux("select-pane", "-t", pane, "-T", "researcher")
	e.mustTmux("set-option", "-w", "-t", win, "pane-border-status", "top")
	e.mustTmux("select-layout", "-t", win, "main-vertical")
	e.mustTmux("resize-pane", "-t", lead, "-x", "30%")
	e.mustTmux("set-option", "-p", "-t", pane, "remain-on-exit", "failed")
	out := filepath.Join(e.root, "teammate.txt")
	e.mustTmux("respawn-pane", "-k", "-t", pane, "--",
		fmt.Sprintf(`echo visible-output; echo "$TMUX_PANE $MARK" > %s; sleep 30`, out))
	e.waitFile(out, "%2 hello-from-leader") // env snapshot reached the pane

	if got := e.mustTmux("list-panes", "-t", win, "-F", "#{pane_id}"); got != "%1\n%2" {
		t.Fatalf("list-panes after spawn = %q", got)
	}
	if got := e.mustTmux("display-message", "-t", pane, "-p", "#{pane_title}"); got != "researcher" {
		t.Fatalf("title = %q", got)
	}
	if got := e.mustTmux("capture-pane", "-p", "-t", pane); !strings.Contains(got, "visible-output") {
		t.Fatalf("capture-pane = %q", got)
	}
	if _, code := e.tmux("respawn-pane", "-k", "-t", pane, "--", "true"); code != 1 {
		t.Fatal("respawning a running pane must fail")
	}

	// Old protocol: no command, then send-keys "cmd" Enter (#40168 race).
	old := e.mustTmux("split-window", "-d", "-t", pane, "-v", "-P", "-F", "#{pane_id}")
	out3 := filepath.Join(e.root, "old.txt")
	e.mustTmux("send-keys", "-t", old, "echo old-protocol > "+out3, "Enter")
	e.waitFile(out3, "old-protocol")

	e.mustTmux("kill-pane", "-t", pane)
	if got := e.mustTmux("list-panes", "-t", win, "-F", "#{pane_id}"); got != "%1" {
		t.Fatalf("list-panes after kill = %q (old pane %s should be pruned once its shell exits)", got, old)
	}
	if _, code := e.tmux("kill-pane", "-t", lead); code != 1 {
		t.Fatal("killing the leader pane must be refused")
	}
}

// TestUnknownCommandsFailSafely checks unsupported calls are logged and fail.
func TestUnknownCommandsFailSafely(t *testing.T) {
	e := newEnv(t)
	e.activate()
	for _, args := range [][]string{
		{"pipe-pane", "-o", "cat"},
		{"display-message", "-p", "#{pane_width}"},
		{"capture-pane", "-t", "%1"},
		{},
	} {
		if _, code := e.tmux(args...); code != 1 {
			t.Fatalf("tmux %v exit = %d, want 1", args, code)
		}
	}
	data, err := os.ReadFile(filepath.Join(e.root, "state", "teampane", "unknown.log"))
	if err != nil || !strings.Contains(string(data), "pipe-pane") || !strings.Contains(string(data), "pane_width") {
		t.Fatalf("unknown.log = %q %v", data, err)
	}
	if out, code := e.run(binary, "tmux", "has-session", "-t", "nope"); code != 1 || out != "" {
		t.Fatalf("has-session nope = %q %d", out, code)
	}
}

// TestParallelSplitsGetDistinctPanes spawns several teammates at once.
func TestParallelSplitsGetDistinctPanes(t *testing.T) {
	e := newEnv(t)
	e.activate()
	const n = 5
	ids := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ids[i], _ = e.tmux("split-window", "-d", "-h", "-P", "-F", "#{pane_id}", "--", "sleep 30")
		}(i)
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, id := range ids {
		if !strings.HasPrefix(id, "%") || seen[id] {
			t.Fatalf("ids = %v", ids)
		}
		seen[id] = true
	}
	if got := e.mustTmux("list-panes", "-F", "#{pane_id}"); len(strings.Split(got, "\n")) != n+1 {
		t.Fatalf("list-panes = %q", got)
	}
}

// TestNotInsideTeampaneSession checks the shim refuses without activation.
func TestNotInsideTeampaneSession(t *testing.T) {
	e := newEnv(t)
	if _, code := e.run(binary, "tmux", "list-panes"); code != 1 {
		t.Fatal("shim must fail outside a teampane session")
	}
	e.set("TMUX", "/tmp/tmux-1000/default,1,0")
	if out, code := e.run(binary, "env"); code != 0 || !strings.Contains(out, "real tmux") {
		t.Fatalf("env inside real tmux = %q %d", out, code)
	}
}
