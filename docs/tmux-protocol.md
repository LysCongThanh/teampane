# The tmux protocol Claude Code Agent Teams speaks

This is what teampane has to emulate. It was collected on 2026-10-31 from:

1. The official docs: <https://code.claude.com/docs/en/agent-teams> ("Choose a display mode").
2. The tmux call sites in the Claude Code 2.1.316 binary (`strings` on the installed binary, `TmuxBackend` class and helpers).
3. Existing shims, read in full:
   - [stanislc/zellij-claude-teams](https://github.com/stanislc/zellij-claude-teams) (`bin/tmux`, `bin/zellij-pane-wrapper`)
   - [ogallotti/cmux-tmux-shim](https://github.com/ogallotti/cmux-tmux-shim) (`bin-tmux`)
   - [kingjly/WinSplitPane](https://github.com/kingjly/WinSplitPane) (`internal/app/app.go`)
   - [addelong/claude-code-zellij](https://github.com/addelong/claude-code-zellij) (patches the CLI instead of shimming)
4. Issues: [#24122](https://github.com/anthropics/claude-code/issues/24122) (zellij), [#24189](https://github.com/anthropics/claude-code/issues/24189) (Ghostty), [#23574](https://github.com/anthropics/claude-code/issues/23574) (WezTerm), [#31901](https://github.com/anthropics/claude-code/issues/31901) (zellij), [#40168](https://github.com/anthropics/claude-code/issues/40168) (send-keys race).

Claude Code is closed source and changes often; treat this as a snapshot. Anything teampane does not recognise is
appended to `$XDG_STATE_HOME/teampane/unknown.log` and fails with exit status 1, so new calls show up instead of
silently misbehaving.

## When split panes are used

- Agent teams must be enabled: `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` (env or `settings.json` `env`).
- `teammateMode`: `"in-process"` (default) never calls tmux. `"auto"` uses split panes "when you're already running
  inside a tmux session"; `"tmux"` forces split-pane mode. Per session: `claude --teammate-mode tmux`.
- "Inside tmux" is decided from the environment: `TMUX` and `TMUX_PANE`. The first comma-separated field of `TMUX`
  is passed back on every call as `tmux -S <socket> ...`. Availability is probed with `tmux -V` (exit code only).
- When not inside tmux (mode `tmux`), Claude Code starts its own server with `-L claude-swarm-<pid>` and a
  detached session `claude-swarm` / window `swarm-view`. teampane does not emulate this path; `teampane run`
  always puts Claude Code on the "inside tmux" path.

## Calls made in the "inside tmux" path (2.1.316)

| Step | Command | teampane |
|---|---|---|
| availability | `tmux -V` | prints `tmux 3.4 (teampane)` |
| own pane | `display-message -p '#{pane_id}'` (skipped when `TMUX_PANE` is set) | registry lookup |
| own window | `display-message [-t %P] -p '#{window_id}'` | `@N` per leader pane |
| location | `display-message -p -t $TMUX_PANE '#{session_name}:#{window_id}.#{pane_id}'` | `teampane:@1.%1` |
| pane count | `list-panes -t @W -F '#{pane_id}'` | panes of that window, creation order |
| first teammate | `split-window -d -t @W -h -l 70% -P -F '#{pane_id}' -- cat` | new native pane running `teampane pane-host`, prints `%N` once ready |
| more teammates | `split-window -d -t %X -v|-h -P -F '#{pane_id}' -- cat` (alternating) | same |
| styling | `set-option -p -t %N window-style|pane-border-style|pane-active-border-style ...`, `set-option -w -t @W pane-border-status top`, `set-option -p -t %N pane-border-format ...` | accepted, ignored |
| title | `select-pane -t %N -T <agent name>` | registry + native title (kitty, zellij) |
| layout | `select-layout -t @W main-vertical|tiled`, `resize-pane -t %L -x 30%` | accepted, ignored (the terminal's layout wins) |
| launch | `set-option -p -t %N remain-on-exit failed` then `respawn-pane -k -t %N -- '<sh command>'` | command delivered to the waiting pane host over a FIFO, then exec'd via `/bin/sh -c` |
| shutdown | `kill-pane -t %N` | closes the native pane; refuses to close leader panes |
| tmux prefix | `show-options -g prefix` | `prefix C-b` |
| probes | `display-message -p '#{client_control_mode}'`; a tab-separated probe of `#{client_termtype}`, `#{allow-passthrough}`, `#{pane_top}`, ... (image support) | `client_control_mode` = `0`; any unknown variable makes the call fail (logged), which Claude Code treats as "unknown" |

Older versions (and other tools) use `split-window` without a command followed by
`send-keys -t %N '<command>' Enter`. That is the sequence that races shell start-up in #40168. teampane's pane host
receives a `text… Enter` send-keys as the command itself if it arrives within `TEAMPANE_SHELL_WAIT` (default 15s);
otherwise the pane falls back to `$SHELL` and later keys are typed into it.

## Also implemented (seen in shims or other tools)

`list-windows -F`, `capture-pane -p [-S -N]`, `has-session -t`, `new-session -d -s` (name only),
`kill-session`, `new-window` (as a right split with its own window id), `select-pane -t` (focus), key names for
`send-keys` (`Enter`, `C-x`, `M-x`, arrows, `-l` literal, `-N` repeat). Aliases (`splitw`, `send`, `killp`,
`display`, `lsp`, ...) are accepted.

## Format variables

Known: `pane_id`, `pane_index`, `pane_title`, `pane_active`, `pane_dead`, `pane_pid`, `window_id`, `window_index`,
`window_name`, `window_panes`, `session_name`, `session_id`, `client_control_mode`, `host`, `pid`, `version`, and the
short forms `#D #P #S #I #W #T #H`, plus `##`. Everything else (sizes, conditionals, modifiers) is reported as
unsupported instead of being invented.

## Design notes from the existing shims

- zellij-claude-teams introduced the "wrapper waits on a FIFO" idea and the placeholder handling for `-- cat` +
  `respawn-pane`; teampane uses the same approach for every backend.
- New panes are started by the terminal, not by the caller, so they do not inherit Claude Code's environment.
  teampane snapshots the caller's environment (minus pane-specific variables) into a 0600 file that the pane host
  loads and deletes.
- cmux-tmux-shim and WinSplitPane insert spaces between `send-keys` arguments; real tmux does not, and neither
  does teampane.
