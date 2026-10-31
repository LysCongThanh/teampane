// Package panehost is the process teampane runs inside every pane it
// creates. It signals readiness, receives the real command over a FIFO and
// then execs it. This removes the split-window/send-keys race
// (anthropics/claude-code#40168): the command is never typed into a shell
// that is still starting.
package panehost

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Key validates a tmux pane id ("%12") and returns its numeric part. Only
// digits are accepted, so ids can never escape the state directory.
func Key(paneID string) (string, error) {
	k := strings.TrimPrefix(paneID, "%")
	if k == "" || len(k) > 9 {
		return "", fmt.Errorf("invalid pane id %q", paneID)
	}
	for _, c := range k {
		if c < '0' || c > '9' {
			return "", fmt.Errorf("invalid pane id %q", paneID)
		}
	}
	return k, nil
}

// Files are the per-pane paths inside a state directory.
type Files struct {
	Fifo, Ready, Env string
}

// PaneFiles returns the paths for paneID, creating the panes/ directory.
func PaneFiles(stateDir, paneID string) (Files, error) {
	k, err := Key(paneID)
	if err != nil {
		return Files{}, err
	}
	dir := filepath.Join(stateDir, "panes")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Files{}, err
	}
	base := filepath.Join(dir, k)
	return Files{Fifo: base + ".fifo", Ready: base + ".ready", Env: base + ".env"}, nil
}

// Cleanup removes all files of a pane; missing files are fine.
func (f Files) Cleanup() {
	for _, p := range []string{f.Fifo, f.Ready, f.Env} {
		os.Remove(p)
	}
}

// paneSpecificEnv are variables that describe the *caller's* pane and must
// not leak into the new pane's environment.
var paneSpecificEnv = append([]string{"TMUX_PANE", "PWD", "OLDPWD", "SHLVL", "_"}, backendPaneEnv...)

// backendPaneEnv name the pane a process runs in, per backend; the new
// pane keeps its own values of these.
var backendPaneEnv = []string{"KITTY_WINDOW_ID", "WEZTERM_PANE", "ZELLIJ_PANE_ID", "TEAMPANE_HEADLESS_PANE"}

// WriteEnv snapshots env (minus pane-specific variables) for the new pane.
// Terminals start new panes with *their* environment, not the caller's, so
// without this a teammate would lose PATH tweaks, API settings, etc.
func WriteEnv(path string, env []string) error {
	var keep []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !contains(paneSpecificEnv, name) {
			keep = append(keep, kv)
		}
	}
	data, err := json.Marshal(keep)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ReadEnv loads an environment snapshot.
func ReadEnv(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var env []string
	return env, json.Unmarshal(data, &env)
}

// MergeEnv overlays the pane-specific variables of own onto snapshot and
// sets TMUX_PANE, producing the environment for the pane's command.
func MergeEnv(snapshot, own []string, paneID string) []string {
	out := make([]string, 0, len(snapshot)+4)
	for _, kv := range snapshot {
		name, _, _ := strings.Cut(kv, "=")
		if !contains(paneSpecificEnv, name) {
			out = append(out, kv)
		}
	}
	for _, kv := range own {
		name, _, _ := strings.Cut(kv, "=")
		if contains(backendPaneEnv, name) {
			out = append(out, kv)
		}
	}
	return append(out, "TMUX_PANE="+paneID)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
