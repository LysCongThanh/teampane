package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Headless runs each "pane" as a detached background process whose output
// goes to a log file. It has no screen, so it exists for CI, end-to-end
// tests and debugging the shim without a GUI terminal. Select it with
// TEAMPANE_BACKEND=headless and TEAMPANE_HEADLESS_DIR=<dir>.
type Headless struct {
	Dir  string // where pane logs and pid files live
	Self string // TEAMPANE_HEADLESS_PANE of the caller
}

func (h *Headless) Name() string        { return "headless" }
func (h *Headless) CurrentPane() string { return h.Self }
func (h *Headless) ScopeKey() string    { return "dir:" + h.Dir }

func (h *Headless) path(native, ext string) string {
	return filepath.Join(h.Dir, filepath.Base(native)+ext)
}

// Split starts argv in its own session with stdout/stderr in <native>.log.
func (h *Headless) Split(req SplitRequest) (string, error) {
	if h.Dir == "" || len(req.Argv) == 0 {
		return "", fmt.Errorf("headless: TEAMPANE_HEADLESS_DIR and a command are required")
	}
	if err := os.MkdirAll(h.Dir, 0o700); err != nil {
		return "", err
	}
	native := "h" + strconv.FormatInt(time.Now().UnixNano(), 36)
	log, err := os.OpenFile(h.path(native, ".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", err
	}
	defer log.Close()
	cmd := exec.Command(req.Argv[0], req.Argv[1:]...)
	cmd.Dir = req.Cwd
	cmd.Env = append(os.Environ(), "TEAMPANE_HEADLESS_PANE="+native)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return native, os.WriteFile(h.path(native, ".pid"), []byte(strconv.Itoa(pid)), 0o600)
}

// SendText is impossible without a terminal.
func (h *Headless) SendText(native, text string) error { return ErrUnsupported }

// GetText returns everything the pane printed.
func (h *Headless) GetText(native string, scrollback bool) (string, error) {
	data, err := os.ReadFile(h.path(native, ".log"))
	return string(data), err
}

// Close terminates the pane's process group.
func (h *Headless) Close(native string) error {
	data, err := os.ReadFile(h.path(native, ".pid"))
	if err != nil {
		return nil // already gone
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		return fmt.Errorf("headless: bad pid file for %s", native)
	}
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	return os.Remove(h.path(native, ".pid"))
}

func (h *Headless) SetTitle(native, title string) error { return nil }
func (h *Headless) Focus(native string) error           { return nil }

func (h *Headless) Check() []Finding {
	if h.Dir == "" {
		return []Finding{{false, "TEAMPANE_HEADLESS_DIR is not set"}}
	}
	return []Finding{{true, "headless backend writes pane output to " + h.Dir}}
}
