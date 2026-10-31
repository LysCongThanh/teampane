package backend

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Cmd is one external command a backend wants to run.
type Cmd struct {
	Name  string
	Args  []string
	Stdin string   // fed to the process when non-empty
	Env   []string // extra KEY=VALUE entries on top of the current environment
}

// String renders the command line for logs and golden tests.
func (c Cmd) String() string {
	var b strings.Builder
	for _, e := range c.Env {
		b.WriteString(e + " ")
	}
	b.WriteString(c.Name)
	for _, a := range c.Args {
		b.WriteString(" " + quote(a))
	}
	if c.Stdin != "" {
		fmt.Fprintf(&b, " <<< %q", c.Stdin)
	}
	return b.String()
}

// quote wraps arguments that contain spaces or quotes for readability.
func quote(a string) string {
	if a == "" || strings.ContainsAny(a, " \t\n'\"") {
		return fmt.Sprintf("%q", a)
	}
	return a
}

// Runner executes commands; tests substitute a recorder.
type Runner interface {
	Run(c Cmd) (stdout string, err error)
}

// ExecRunner runs commands with os/exec and a timeout.
type ExecRunner struct {
	Timeout time.Duration // default 10s
}

// Run executes c and returns its stdout. stderr is folded into the error.
func (r ExecRunner) Run(c Cmd) (string, error) {
	timeout := r.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	cmd := exec.Command(c.Name, c.Args...)
	if len(c.Env) > 0 {
		cmd.Env = append(os.Environ(), c.Env...)
	}
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return out.String(), fmt.Errorf("%s %s: %w: %s", c.Name, firstArg(c.Args), err, strings.TrimSpace(errb.String()))
		}
		return out.String(), nil
	case <-time.After(timeout):
		cmd.Process.Kill()
		<-done
		return out.String(), fmt.Errorf("%s %s: timed out after %s", c.Name, firstArg(c.Args), timeout)
	}
}

// firstArg names the subcommand in error messages without leaking payloads.
func firstArg(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && a != "@" && a != "cli" && a != "action" {
			return a
		}
	}
	return ""
}
