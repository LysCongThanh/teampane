// Package backend drives a terminal's native split panes. Every backend
// talks to its terminal through a Runner (the exec seam), so command lines
// can be tested without a terminal.
package backend

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Direction is where a new pane goes relative to its target.
type Direction int

const (
	// Right splits side by side (tmux split-window -h).
	Right Direction = iota
	// Down splits top/bottom (tmux split-window -v, the tmux default).
	Down
)

// SplitRequest describes a new pane.
type SplitRequest struct {
	Target  string    // native id of the pane to split next to
	Dir     Direction // placement
	Percent int       // share of space for the new pane; 0 = backend default
	Focus   bool      // move focus to the new pane (tmux without -d)
	Cwd     string    // working directory, optional
	Title   string    // initial title, optional
	Argv    []string  // program to run in the new pane
}

// Finding is one doctor check result.
type Finding struct {
	OK  bool
	Msg string
}

// Backend is a terminal (or multiplexer) that can create and drive panes.
type Backend interface {
	Name() string
	// CurrentPane returns the native id of the pane this process runs in.
	CurrentPane() string
	// ScopeKey identifies the terminal instance; panes of one scope share a registry.
	ScopeKey() string
	Split(req SplitRequest) (native string, err error)
	SendText(native, text string) error
	GetText(native string, scrollback bool) (string, error)
	Close(native string) error
	SetTitle(native, title string) error
	Focus(native string) error
	// Check reports whether remote control works (used by `teampane doctor`).
	Check() []Finding
}

// ErrUnsupported is returned for operations a backend cannot perform.
var ErrUnsupported = errors.New("operation not supported by this backend")

// Names lists the backends in auto-detect priority order.
var Names = []string{"kitty", "wezterm", "zellij"}

// Getenv reads an environment variable (os.Getenv in production).
type Getenv func(string) string

// New constructs a backend by name.
func New(name string, getenv Getenv, r Runner) (Backend, error) {
	switch name {
	case "kitty":
		return &Kitty{R: r, ListenOn: getenv("KITTY_LISTEN_ON"), Self: getenv("KITTY_WINDOW_ID"), PID: getenv("KITTY_PID")}, nil
	case "wezterm":
		return &WezTerm{R: r, Self: getenv("WEZTERM_PANE"), Socket: getenv("WEZTERM_UNIX_SOCKET")}, nil
	case "zellij":
		return &Zellij{R: r, Self: getenv("ZELLIJ_PANE_ID"), Session: getenv("ZELLIJ_SESSION_NAME")}, nil
	case "headless":
		return &Headless{Dir: getenv("TEAMPANE_HEADLESS_DIR"), Self: getenv("TEAMPANE_HEADLESS_PANE")}, nil
	}
	return nil, fmt.Errorf("unknown backend %q (want one of %s)", name, strings.Join(Names, ", "))
}

// DetectName picks a backend from the environment. TEAMPANE_BACKEND wins;
// otherwise the innermost multiplexer (zellij) beats the terminal emulator.
func DetectName(getenv Getenv) (string, error) {
	if b := getenv("TEAMPANE_BACKEND"); b != "" {
		return b, nil
	}
	switch {
	case getenv("ZELLIJ") != "" && getenv("ZELLIJ_PANE_ID") != "":
		return "zellij", nil
	case getenv("KITTY_WINDOW_ID") != "":
		return "kitty", nil
	case getenv("WEZTERM_PANE") != "":
		return "wezterm", nil
	}
	return "", errors.New("no supported terminal detected (need kitty, WezTerm or zellij; set TEAMPANE_BACKEND to force one)")
}

// Detect returns the backend for the current environment.
func Detect(getenv Getenv, r Runner) (Backend, error) {
	name, err := DetectName(getenv)
	if err != nil {
		return nil, err
	}
	return New(name, getenv, r)
}

// Default returns the production backend for this process.
func Default() (Backend, error) { return Detect(os.Getenv, ExecRunner{}) }
