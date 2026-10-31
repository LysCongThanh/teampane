package xdg

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionRoundTrip(t *testing.T) {
	root := t.TempDir()
	dir := SessionDir(root, "kitty", "unix:/tmp/kitty-1")
	if !strings.HasPrefix(filepath.Base(dir), "kitty-") {
		t.Fatalf("dir = %s", dir)
	}
	got, err := SessionFromSocket(root, TMUXValue(dir, 42))
	if err != nil || got != dir {
		t.Fatalf("round trip = %q %v", got, err)
	}
	if SessionDir(root, "kitty", "a") == SessionDir(root, "kitty", "b") {
		t.Fatal("different scopes must not share a dir")
	}
}

func TestSessionFromSocketRejects(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{
		"",
		"/tmp/tmux-1000/default,123,0",   // real tmux
		filepath.Join(root, "tmux.sock"), // root itself
		filepath.Join(root, "..", "x", "tmux.sock"),
		filepath.Join(root, "a", "b", "tmux.sock"), // nested
		"/etc/tmux.sock",
	} {
		if _, err := SessionFromSocket(root, bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
