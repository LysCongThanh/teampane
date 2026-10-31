package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installFakeClaude(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return path
}

func TestClaudeBinaryFinding(t *testing.T) {
	t.Run("reports version and path", func(t *testing.T) {
		path := installFakeClaude(t, "printf '2.1.7 (Claude Code)\\n'\n")
		got := claudeBinaryFinding()
		if !got.OK || !strings.Contains(got.Msg, "2.1.7 (Claude Code)") || !strings.Contains(got.Msg, path) {
			t.Fatalf("claudeBinaryFinding() = %+v, want version and path", got)
		}
	})

	t.Run("missing binary fails with install hint", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		got := claudeBinaryFinding()
		if got.OK || !strings.Contains(got.Msg, "npm i -g @anthropic-ai/claude-code") {
			t.Fatalf("claudeBinaryFinding() = %+v, want missing-binary install hint", got)
		}
	})

	t.Run("version command failure remains ok", func(t *testing.T) {
		installFakeClaude(t, "exit 1\n")
		got := claudeBinaryFinding()
		if !got.OK || !strings.Contains(got.Msg, "version unknown") {
			t.Fatalf("claudeBinaryFinding() = %+v, want ok with unknown version", got)
		}
	})
}
