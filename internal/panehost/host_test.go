package panehost

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LysCongThanh/teampane/internal/registry"
)

// stubExec captures what Run would exec.
type stubExec struct {
	argv []string
	env  []string
	done chan struct{}
}

func (s *stubExec) exec(path string, argv, env []string) error {
	s.argv, s.env = argv, env
	close(s.done)
	return nil
}

func setup(t *testing.T) (string, *stubExec) {
	t.Helper()
	dir := t.TempDir()
	st, err := registry.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Update(func(s *registry.State) error {
		s.Panes = append(s.Panes, &registry.Pane{ID: "%2", Window: "@1", Created: time.Now()})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"KITTY_WINDOW_ID", "WEZTERM_PANE", "ZELLIJ_PANE_ID", "TEAMPANE_HEADLESS_PANE"} {
		if v, ok := os.LookupEnv(k); ok {
			os.Unsetenv(k)
			t.Cleanup(func() { os.Setenv(k, v) })
		}
	}
	stub := &stubExec{done: make(chan struct{})}
	execFn = stub.exec
	t.Cleanup(func() { execFn = nil })
	return dir, stub
}

func TestPlaceholderReceivesRespawnCommand(t *testing.T) {
	dir, stub := setup(t)
	files, _ := PaneFiles(dir, "%2")
	if err := WriteEnv(files.Env, []string{"FOO=bar", "TMUX_PANE=%1", "KITTY_WINDOW_ID=8"}); err != nil {
		t.Fatal(err)
	}
	errc := make(chan error, 1)
	go func() {
		errc <- Run(Options{StateDir: dir, PaneID: "%2", Backend: "headless", Mode: ModePlaceholder, PlaceholderWait: 5 * time.Second})
	}()
	if err := WaitReady(files.Ready, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := Deliver(files.Fifo, "cd /tmp && claude --agent-id x", 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if strings.Join(stub.argv, "|") != "/bin/sh|-c|cd /tmp && claude --agent-id x" {
		t.Fatalf("argv = %q", stub.argv)
	}
	envs := strings.Join(stub.env, "\n")
	if !strings.Contains(envs, "FOO=bar") || !strings.Contains(envs, "TMUX_PANE=%2") || strings.Contains(envs, "TMUX_PANE=%1") || strings.Contains(envs, "KITTY_WINDOW_ID=8") {
		t.Fatalf("env = %s", envs)
	}
	if _, err := os.Stat(files.Env); !os.IsNotExist(err) {
		t.Fatal("env snapshot must be removed after use")
	}
	if err := Deliver(files.Fifo, "late", time.Second); err != ErrNotWaiting {
		t.Fatalf("late delivery err = %v", err)
	}
	st, _ := registry.Open(dir)
	s, _ := st.Read()
	if s.Find("%2").HostPID != os.Getpid() {
		t.Fatal("host pid not registered")
	}
}

func TestInlineAndShellFallback(t *testing.T) {
	dir, stub := setup(t)
	if err := Run(Options{StateDir: dir, PaneID: "%2", Mode: ModeInline, Argv: []string{"echo", "a b"}}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(stub.argv, "|") != `/bin/sh|-c|exec "$@"|sh|echo|a b` {
		t.Fatalf("inline argv = %q", stub.argv)
	}
	stub2 := &stubExec{done: make(chan struct{})}
	execFn = stub2.exec
	t.Setenv("SHELL", "/bin/zsh")
	if err := Run(Options{StateDir: dir, PaneID: "%2", Mode: ModeNone, ShellWait: 50 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if len(stub2.argv) != 1 || stub2.argv[0] != "/bin/zsh" {
		t.Fatalf("fallback argv = %q", stub2.argv)
	}
}

func TestPlaceholderTimeoutFails(t *testing.T) {
	dir, _ := setup(t)
	err := Run(Options{StateDir: dir, PaneID: "%2", Mode: ModePlaceholder, PlaceholderWait: 50 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnregisteredPaneRefused(t *testing.T) {
	dir, _ := setup(t)
	if err := Run(Options{StateDir: dir, PaneID: "%9", Mode: ModeInline, Argv: []string{"true"}}); err == nil {
		t.Fatal("expected refusal for unknown pane")
	}
	if _, err := Key("%../../x"); err == nil {
		t.Fatal("expected invalid key")
	}
}
