# teampane

**Claude Code Agent Teams in split panes — in kitty, WezTerm and zellij, without tmux.**

Claude Code's [agent teams](https://code.claude.com/docs/en/agent-teams) can give every teammate its own pane,
but only through tmux or iTerm2. teampane is a small Go binary that pretends to be `tmux` and turns those calls
into your terminal's native splits.

![teampane demo](docs/demo.gif)

> GIF placeholder — regenerate with `vhs docs/demo.tape`.

## Why

Split panes for agent teams are a popular request:

- [#24122](https://github.com/anthropics/claude-code/issues/24122) / [#31901](https://github.com/anthropics/claude-code/issues/31901) — zellij
- [#23574](https://github.com/anthropics/claude-code/issues/23574) — WezTerm
- [#24189](https://github.com/anthropics/claude-code/issues/24189) — Ghostty (no scriptable split API yet, so not supported here)
- [#40168](https://github.com/anthropics/claude-code/issues/40168) — the `send-keys` race that leaves teammate panes empty

Until terminals get native backends, a shim is the practical way to get there.

## Install

```sh
go install github.com/LysCongThanh/teampane@latest
teampane install      # creates ~/.local/share/teampane/bin/tmux -> teampane (never touches a real tmux)
teampane doctor       # checks terminal remote control and Claude Code settings
```

Enable agent teams and split panes in `~/.claude/settings.json`:

```json
{
  "env": { "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1" },
  "teammateMode": "auto"
}
```

Then start Claude Code through teampane:

```sh
teampane run claude
# or: alias claude='teampane run claude'
```

`teampane run` sets `TMUX`, `TMUX_PANE`, `PATH` (shim first) and `TEAMPANE_BACKEND` **for that process only**, so
the rest of your shell never thinks it is inside tmux. If you really want it shell-wide:
`eval "$(teampane env)"` (fish: `teampane env --shell fish | source`). Inside a real tmux session teampane steps
aside.

### Terminal setup

| Terminal | Needs | Notes |
|---|---|---|
| **kitty** (first-class) | `allow_remote_control socket-only` and `listen_on unix:/tmp/kitty-{kitty_pid}` in `kitty.conf`; `enabled_layouts splits,stack` for tmux-like splits | uses `kitty @ launch --location=vsplit/hsplit --next-to`, `send-text --stdin`, `get-text`, `close-window` |
| **WezTerm** | nothing (`wezterm cli` talks to the running GUI) | no per-pane titles in WezTerm |
| **zellij** | 0.44.1+ (tested on 0.45) | panes open near the spawning pane without stealing focus; split size is not settable |

The backend is picked from `ZELLIJ`, `KITTY_WINDOW_ID`, `WEZTERM_PANE` (innermost multiplexer first); force one
with `TEAMPANE_BACKEND=kitty|wezterm|zellij`.

## How it works

1. Claude Code runs `tmux split-window ... -- cat` (a placeholder). teampane creates a native pane running
   `teampane pane-host`, which signals readiness and waits on a FIFO. `split-window` only returns once the pane is
   ready — so the command can never arrive before the pane can take it (#40168).
2. `tmux respawn-pane -k -t %N -- '<teammate command>'` (or the older `send-keys '<cmd>' Enter`) is written to the
   FIFO; the pane host restores Claude Code's environment and `exec`s the command.
3. `list-panes`, `display-message -p`, `select-pane -T`, `capture-pane -p`, `kill-pane` are answered from a small
   registry (`$XDG_STATE_HOME/teampane/…`, flock + atomic writes) that maps tmux ids (`%3`) to native ids.

Full call list and sources: [docs/tmux-protocol.md](docs/tmux-protocol.md).

## Limitations (read these)

- **A stopgap.** This exists until Claude Code (or your terminal) ships a native backend. Claude Code's tmux
  usage is undocumented and can change in any release; teampane may break until it is updated.
- Unknown tmux commands and format variables **fail with exit 1** and are logged to
  `$XDG_STATE_HOME/teampane/unknown.log` — please attach that file to an issue.
- Layout commands (`select-layout`, `resize-pane`, pane styles) are ignored; your terminal's layout decides.
- `respawn-pane` only works on a pane that has not started its command yet.
- No tmux server: `new-session` with a command, attach, copy mode, paste buffers and `-L` sockets are unsupported.
- With a fake `TMUX`, Claude Code's tmux-specific image passthrough probe fails, so inline images may be off.
- Linux and macOS only (FIFOs, `flock`). Ghostty and Windows Terminal have no remote split API to drive.

## Compared with zellij-claude-teams

[stanislc/zellij-claude-teams](https://github.com/stanislc/zellij-claude-teams) pioneered this approach for
zellij (FIFO wrapper, placeholder/respawn handling) — credit and thanks. Use it if you only use zellij and want a
pure-bash install. teampane differs in:

- one static Go binary and several backends (kitty first, WezTerm, zellij);
- `teampane run` scopes the fake tmux to Claude Code instead of the whole shell;
- a locked, atomically-written registry and `teampane doctor`;
- unknown calls are logged and fail instead of being silently accepted.

## Development

```sh
go vet ./... && go test ./...                         # unit + end-to-end (headless backend)
TEAMPANE_IT_KITTY=1 go test ./internal/backend -run Kitty   # opens and closes a real kitty split
TEAMPANE_DEBUG=1 teampane run claude                  # log every tmux call to debug.log
```

See [CONTRIBUTING.md](CONTRIBUTING.md) to add a backend.

## License

MIT © 2026 LysCongThanh
