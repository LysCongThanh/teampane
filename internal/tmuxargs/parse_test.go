package tmuxargs

import (
	"reflect"
	"testing"
)

// Argument vectors below are copied from the calls Claude Code 2.1.316 makes
// (see docs/tmux-protocol.md).
func TestParseClaudeSplitWindow(t *testing.T) {
	cmd, err := Parse([]string{"split-window", "-d", "-t", "%1", "-h", "-l", "70%", "-P", "-F", "#{pane_id}", "--", "cat"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "split-window" || !cmd.Has("d") || !cmd.Has("h") || !cmd.Has("P") {
		t.Fatalf("bad bools: %+v", cmd)
	}
	if v, _ := cmd.Flag("t"); v != "%1" {
		t.Fatalf("target = %q", v)
	}
	if v, _ := cmd.Flag("l"); v != "70%" {
		t.Fatalf("size = %q", v)
	}
	if v, _ := cmd.Flag("F"); v != "#{pane_id}" {
		t.Fatalf("format = %q", v)
	}
	if !reflect.DeepEqual(cmd.Args, []string{"cat"}) {
		t.Fatalf("args = %q", cmd.Args)
	}
}

func TestParseClusteredAndAttached(t *testing.T) {
	cmd, err := Parse([]string{"splitw", "-dhP", "-t%3", "-l50%", "echo", "-x"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "split-window" || !cmd.Has("d") || !cmd.Has("h") || !cmd.Has("P") {
		t.Fatalf("bad parse: %+v", cmd)
	}
	if cmd.Flags["t"] != "%3" || cmd.Flags["l"] != "50%" {
		t.Fatalf("flags = %v", cmd.Flags)
	}
	// Parsing stops at the first positional: "-x" belongs to the command.
	if !reflect.DeepEqual(cmd.Args, []string{"echo", "-x"}) {
		t.Fatalf("args = %q", cmd.Args)
	}
}

func TestParseRespawnAndSendKeys(t *testing.T) {
	cmd, err := Parse([]string{"respawn-pane", "-k", "-t", "%2", "--", "cd /x && claude --agent-id a"})
	if err != nil {
		t.Fatal(err)
	}
	if !cmd.Has("k") || cmd.Flags["t"] != "%2" || len(cmd.Args) != 1 {
		t.Fatalf("respawn parse: %+v", cmd)
	}
	sk, err := Parse([]string{"send-keys", "-t", "%2", "-l", "hello world", "Enter"})
	if err != nil {
		t.Fatal(err)
	}
	if !sk.Has("l") || !reflect.DeepEqual(sk.Args, []string{"hello world", "Enter"}) {
		t.Fatalf("send-keys parse: %+v", sk)
	}
}

func TestParseMissingValue(t *testing.T) {
	if _, err := Parse([]string{"kill-pane", "-t"}); err == nil {
		t.Fatal("expected error for missing -t value")
	}
}

func TestParseUnknownCommandKeepsArgs(t *testing.T) {
	cmd, err := Parse([]string{"pipe-pane", "-o", "cat"})
	if err != nil || Known(cmd.Name) {
		t.Fatalf("unexpected: %+v %v", cmd, err)
	}
	if !reflect.DeepEqual(cmd.Args, []string{"-o", "cat"}) {
		t.Fatalf("args = %q", cmd.Args)
	}
}

func TestParseGlobal(t *testing.T) {
	g, err := ParseGlobal([]string{"-S", "/tmp/s", "-L", "claude-swarm-1", "list-panes", "-F", "#{pane_id}"})
	if err != nil {
		t.Fatal(err)
	}
	if g.Socket != "/tmp/s" || g.SocketName != "claude-swarm-1" || g.Rest[0] != "list-panes" {
		t.Fatalf("global = %+v", g)
	}
	v, _ := ParseGlobal([]string{"-V"})
	if !v.Version || len(v.Rest) != 0 {
		t.Fatalf("version parse = %+v", v)
	}
}
