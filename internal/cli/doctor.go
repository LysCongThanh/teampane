package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LysCongThanh/teampane/internal/backend"
	"github.com/LysCongThanh/teampane/internal/xdg"
)

// doctorCmd prints one line per check and exits 1 if any check failed.
func doctorCmd(out io.Writer) int {
	var all []backend.Finding
	add := func(f ...backend.Finding) { all = append(all, f...) }

	if t := os.Getenv("TMUX"); t != "" {
		if _, err := xdg.SessionFromSocket(xdg.StateRoot(), t); err != nil {
			add(backend.Finding{OK: true, Msg: "inside a real tmux session: Claude Code uses it directly, teampane is not needed here"})
		} else {
			add(backend.Finding{OK: true, Msg: "teampane session active (TMUX_PANE=" + os.Getenv("TMUX_PANE") + ")"})
		}
	}
	b, err := backend.Default()
	if err != nil {
		add(backend.Finding{OK: false, Msg: err.Error()})
	} else {
		add(backend.Finding{OK: b.CurrentPane() != "", Msg: fmt.Sprintf("backend %s, current pane %q", b.Name(), b.CurrentPane())})
		add(b.Check()...)
	}
	add(linkFinding())
	add(claudeBinaryFinding())
	add(claudeFindings()...)
	if n := countLines(xdg.UnknownLog()); n > 0 {
		add(backend.Finding{OK: true, Msg: fmt.Sprintf("%d unsupported tmux call(s) logged in %s — please report them", n, xdg.UnknownLog())})
	}

	code := 0
	for _, f := range all {
		mark := "ok  "
		if !f.OK {
			mark, code = "FAIL", 1
		}
		fmt.Fprintf(out, "[%s] %s\n", mark, f.Msg)
	}
	return code
}

// linkFinding checks the shim symlink and what `tmux` resolves to.
func linkFinding() backend.Finding {
	link := filepath.Join(xdg.BinDir(), "tmux")
	exe, _ := selfPath()
	if ours, err := isOurLink(link, exe); err != nil || !ours {
		return backend.Finding{OK: false, Msg: "shim not installed at " + link + " — run `teampane install`"}
	}
	msg := "shim installed at " + link
	if p, err := exec.LookPath("tmux"); err == nil && p != link {
		msg += " (outside `teampane run`, tmux resolves to " + p + ")"
	}
	return backend.Finding{OK: true, Msg: msg}
}

// claudeBinaryFinding checks that Claude Code is installed and reports its version.
func claudeBinaryFinding() backend.Finding {
	path, err := exec.LookPath("claude")
	if err != nil {
		return backend.Finding{OK: false, Msg: "claude not found on PATH — install with `npm i -g @anthropic-ai/claude-code` or see the Claude Code docs"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return backend.Finding{OK: true, Msg: fmt.Sprintf("claude found at %s (version unknown)", path)}
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		version = "version unknown"
	}
	if i := strings.IndexByte(version, '\n'); i >= 0 {
		version = strings.TrimSpace(version[:i])
	}
	return backend.Finding{OK: true, Msg: fmt.Sprintf("claude %s at %s", version, path)}
}

// claudeFindings checks the two Claude Code settings split panes need.
func claudeFindings() []backend.Finding {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".claude")
	}
	var s struct {
		TeammateMode string            `json:"teammateMode"`
		Env          map[string]string `json:"env"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	teams := os.Getenv("CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS") == "1" || s.Env["CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"] == "1"
	res := []backend.Finding{{OK: teams, Msg: "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 (agent teams enabled)"}}
	switch s.TeammateMode {
	case "tmux", "auto":
		res = append(res, backend.Finding{OK: true, Msg: "teammateMode is " + s.TeammateMode})
	default:
		mode := s.TeammateMode
		if mode == "" {
			mode = "unset (in-process)"
		}
		res = append(res, backend.Finding{OK: false, Msg: "teammateMode is " + mode +
			`: set "teammateMode": "auto" in settings.json or pass --teammate-mode tmux`})
	}
	return res
}

// countLines counts lines in a file (0 if missing).
func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	for sc := bufio.NewScanner(f); sc.Scan(); {
		n++
	}
	return n
}
