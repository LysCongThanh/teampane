package backend

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestKittyIntegration drives a real kitty instance. It is skipped unless
// TEAMPANE_IT_KITTY=1 and the test runs inside a kitty window with remote
// control enabled. It opens one split next to the current window, reads
// its screen, types into it and closes it again.
func TestKittyIntegration(t *testing.T) {
	if os.Getenv("TEAMPANE_IT_KITTY") != "1" {
		t.Skip("set TEAMPANE_IT_KITTY=1 inside kitty to run")
	}
	k, err := New("kitty", os.Getenv, ExecRunner{})
	if err != nil || k.CurrentPane() == "" {
		t.Skip("not inside a kitty window")
	}
	if f := k.Check(); !f[0].OK {
		t.Skip(f[0].Msg)
	}
	id, err := k.Split(SplitRequest{
		Target: k.CurrentPane(), Dir: Right, Percent: 40,
		Argv: []string{"sh", "-c", "echo teampane-it-ready; read line; echo got:$line; sleep 30"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close(id)
	waitFor(t, k, id, "teampane-it-ready")
	if err := k.SetTitle(id, "teampane-it"); err != nil {
		t.Fatal(err)
	}
	if err := k.SendText(id, "ping\r"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, k, id, "got:ping")
	if err := k.Close(id); err != nil {
		t.Fatal(err)
	}
	if err := k.Close(id); err != nil {
		t.Fatalf("closing a gone window must be a no-op: %v", err)
	}
}

func waitFor(t *testing.T, b Backend, id, marker string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if txt, err := b.GetText(id, false); err == nil && strings.Contains(txt, marker) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("pane %s never showed %q", id, marker)
}
