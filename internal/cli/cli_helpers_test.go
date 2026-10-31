package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrependPath(t *testing.T) {
	got := prependPath("/shim", "/usr/bin:/shim:/bin:")
	if got != "/shim:/usr/bin:/bin" {
		t.Fatalf("prependPath = %q", got)
	}
}

func TestWithVarsOverrides(t *testing.T) {
	env := withVars([]string{"A=1", "TMUX=real", "B=2"}, [][2]string{{"TMUX", "fake"}, {"C", "3"}})
	joined := strings.Join(env, " ")
	if joined != "A=1 B=2 TMUX=fake C=3" {
		t.Fatalf("withVars = %q", joined)
	}
}

func TestShQuote(t *testing.T) {
	if got := shQuote(`it's`); got != `'it'\''s'` {
		t.Fatalf("shQuote = %s", got)
	}
}

func TestEnsureLinkNeverOverwritesRealTmux(t *testing.T) {
	bin := t.TempDir()
	real := filepath.Join(bin, "tmux")
	if err := os.WriteFile(real, []byte("#!/bin/sh\necho real tmux\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureLink(bin); err == nil {
		t.Fatal("ensureLink replaced a real tmux file")
	}
	data, _ := os.ReadFile(real)
	if !strings.Contains(string(data), "real tmux") {
		t.Fatal("real tmux was modified")
	}

	other := t.TempDir()
	foreign := filepath.Join(other, "tmux")
	if err := os.Symlink("/usr/bin/tmux", foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureLink(other); err == nil {
		t.Fatal("ensureLink replaced a symlink to a real tmux")
	}
}

func TestEnsureLinkIdempotent(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	changed, err := ensureLink(bin)
	if err != nil || !changed {
		t.Fatalf("first link: %v %v", changed, err)
	}
	changed, err = ensureLink(bin)
	if err != nil || changed {
		t.Fatalf("second link should be a no-op: %v %v", changed, err)
	}
	exe, _ := selfPath()
	if ours, err := isOurLink(filepath.Join(bin, "tmux"), exe); err != nil || !ours {
		t.Fatalf("isOurLink = %v %v", ours, err)
	}
}
