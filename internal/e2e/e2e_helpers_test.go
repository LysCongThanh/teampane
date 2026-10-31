// Package e2e builds the real teampane binary and drives it exactly like
// Claude Code does — through a `tmux` symlink — using the headless backend,
// so the whole path (shim -> registry -> backend -> pane-host -> FIFO ->
// exec) runs for real without a GUI terminal.
package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var binary string // built once in TestMain

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "teampane-e2e-bin-")
	if err != nil {
		panic(err)
	}
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	binary = filepath.Join(dir, "teampane")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		panic("go build failed: " + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// env is an isolated environment: private XDG dirs, headless backend, and
// no inherited terminal/tmux variables.
type env struct {
	t    *testing.T
	root string
	vars []string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	e := &env{t: t, root: root, vars: []string{
		"HOME=" + root,
		"PATH=" + os.Getenv("PATH"),
		"SHELL=/bin/sh",
		"XDG_STATE_HOME=" + filepath.Join(root, "state"),
		"XDG_DATA_HOME=" + filepath.Join(root, "data"),
		"TEAMPANE_BACKEND=headless",
		"TEAMPANE_HEADLESS_DIR=" + filepath.Join(root, "panes"),
		"TEAMPANE_HEADLESS_PANE=lead",
		"MARK=hello-from-leader",
	}}
	t.Cleanup(e.killAll)
	return e
}

// set overrides a variable.
func (e *env) set(k, v string) {
	for i, kv := range e.vars {
		if strings.HasPrefix(kv, k+"=") {
			e.vars[i] = k + "=" + v
			return
		}
	}
	e.vars = append(e.vars, k+"="+v)
}

// run executes name with args in the environment and returns stdout and the exit code.
func (e *env) run(name string, args ...string) (string, int) {
	e.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env, cmd.Dir = e.vars, e.root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		e.t.Fatalf("%s %v: %v", name, args, err)
	}
	if code != 0 {
		e.t.Logf("%s %v -> exit %d, stderr: %s", name, args, code, errb.String())
	}
	return strings.TrimRight(out.String(), "\n"), code
}

// tmux runs the shim through the installed symlink, as Claude Code would.
func (e *env) tmux(args ...string) (string, int) {
	e.t.Helper()
	// exec.Command resolves names with the test's PATH, not cmd.Env's, so
	// point at the installed symlink explicitly.
	return e.run(filepath.Join(e.root, "data", "teampane", "bin", "tmux"), args...)
}

// mustTmux fails the test on a non-zero exit.
func (e *env) mustTmux(args ...string) string {
	e.t.Helper()
	out, code := e.tmux(args...)
	if code != 0 {
		e.t.Fatalf("tmux %v exited %d", args, code)
	}
	return out
}

// activate installs the shim and imports the variables `teampane run` sets.
func (e *env) activate() {
	e.t.Helper()
	if _, code := e.run(binary, "install"); code != 0 {
		e.t.Fatal("install failed")
	}
	out, code := e.run(binary, "run", "--", "env")
	if code != 0 {
		e.t.Fatal("teampane run env failed")
	}
	for _, line := range strings.Split(out, "\n") {
		for _, k := range []string{"PATH", "TMUX", "TMUX_PANE", "TEAMPANE_BACKEND"} {
			if v, ok := strings.CutPrefix(line, k+"="); ok {
				e.set(k, v)
			}
		}
	}
}

// waitFile polls until path contains want.
func (e *env) waitFile(path, want string) {
	e.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), want) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	data, _ := os.ReadFile(path)
	e.t.Fatalf("%s never contained %q (got %q)", path, want, data)
}

// killAll stops every headless pane process the test started.
func (e *env) killAll() {
	files, _ := filepath.Glob(filepath.Join(e.root, "panes", "*.pid"))
	for _, f := range files {
		if data, err := os.ReadFile(f); err == nil {
			// Each headless pane leads its own process group.
			exec.Command("kill", "--", "-"+strings.TrimSpace(string(data))).Run()
		}
	}
}
