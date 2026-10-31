package backend

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

// recorder is the exec seam used in tests: it records every command line
// and answers with canned stdout.
type recorder struct {
	lines []string
	reply func(c Cmd) string
}

func (r *recorder) Run(c Cmd) (string, error) {
	r.lines = append(r.lines, c.String())
	if r.reply != nil {
		return r.reply(c), nil
	}
	return "", nil
}

// exercise drives every Backend method the shim uses, in a fixed order.
func exercise(t *testing.T, b Backend) {
	t.Helper()
	argv := []string{"/usr/bin/teampane", "pane-host", "--pane", "%2", "--", "cat"}
	if id, err := b.Split(SplitRequest{Target: "8", Dir: Right, Percent: 70, Cwd: "/work/repo", Argv: argv}); err != nil || id == "" {
		t.Fatalf("split: %q %v", id, err)
	}
	if _, err := b.Split(SplitRequest{Target: "9", Dir: Down, Focus: true, Title: "tester", Argv: argv}); err != nil {
		t.Fatal(err)
	}
	must(t, b.SendText("9", "echo hi\r"))
	if _, err := b.GetText("9", false); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetText("9", true); err != nil {
		t.Fatal(err)
	}
	must(t, b.SetTitle("9", "researcher"))
	must(t, b.Focus("8"))
	must(t, b.Close("9"))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func checkGolden(t *testing.T, name string, lines []string) {
	t.Helper()
	got := strings.Join(lines, "\n") + "\n"
	path := filepath.Join("testdata", name+".golden")
	if *update {
		must(t, os.MkdirAll("testdata", 0o755))
		must(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run go test ./internal/backend -update): %v", path, err)
	}
	if got != string(want) {
		t.Fatalf("command lines differ from %s\n--- got\n%s--- want\n%s", path, got, want)
	}
}

func TestKittyGolden(t *testing.T) {
	r := &recorder{reply: func(c Cmd) string {
		if strings.Contains(strings.Join(c.Args, " "), " launch ") {
			return "42\n"
		}
		return ""
	}}
	exercise(t, &Kitty{R: r, ListenOn: "unix:/tmp/kitty-1", Self: "8"})
	checkGolden(t, "kitty", r.lines)
}

func TestKittyWithoutListenOn(t *testing.T) {
	r := &recorder{reply: func(Cmd) string { return "5" }}
	k := &Kitty{R: r, Self: "8", PID: "77"}
	if _, err := k.Split(SplitRequest{Target: "8", Argv: []string{"x"}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.lines[0], "--to") || k.ScopeKey() != "pid:77" {
		t.Fatalf("unexpected: %v scope=%s", r.lines, k.ScopeKey())
	}
}

func TestWezTermGolden(t *testing.T) {
	r := &recorder{reply: func(c Cmd) string {
		if len(c.Args) > 1 && c.Args[1] == "split-pane" {
			return "17\n"
		}
		return ""
	}}
	exercise(t, &WezTerm{R: r, Self: "8"})
	checkGolden(t, "wezterm", r.lines)
}

func TestZellijGolden(t *testing.T) {
	r := &recorder{reply: func(c Cmd) string {
		if len(c.Args) > 1 && c.Args[1] == "new-pane" {
			return "terminal_12\n"
		}
		return ""
	}}
	z := &Zellij{R: r, Self: "8", Session: "work"}
	id, err := z.Split(SplitRequest{Target: "8", Argv: []string{"x"}})
	if err != nil || id != "12" {
		t.Fatalf("zellij id = %q %v", id, err)
	}
	r.lines = nil
	exercise(t, z)
	checkGolden(t, "zellij", r.lines)
}

func TestDetectName(t *testing.T) {
	env := func(m map[string]string) Getenv { return func(k string) string { return m[k] } }
	cases := []struct {
		m    map[string]string
		want string
	}{
		{map[string]string{"KITTY_WINDOW_ID": "1"}, "kitty"},
		{map[string]string{"WEZTERM_PANE": "3"}, "wezterm"},
		{map[string]string{"KITTY_WINDOW_ID": "1", "ZELLIJ": "0", "ZELLIJ_PANE_ID": "2"}, "zellij"},
		{map[string]string{"KITTY_WINDOW_ID": "1", "TEAMPANE_BACKEND": "wezterm"}, "wezterm"},
	}
	for _, c := range cases {
		if got, err := DetectName(env(c.m)); err != nil || got != c.want {
			t.Errorf("DetectName(%v) = %q %v, want %q", c.m, got, err, c.want)
		}
	}
	if _, err := DetectName(env(nil)); err == nil {
		t.Error("expected error without a terminal")
	}
	if _, err := New("ghostty", env(nil), nil); err == nil {
		t.Error("expected error for unknown backend")
	}
}
