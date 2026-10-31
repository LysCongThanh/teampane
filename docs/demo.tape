# CLI demo of teampane using the headless backend (no GUI terminal involved).
# It replays, by hand, the tmux calls Claude Code makes when it spawns a
# split-pane teammate. It does NOT show a real Claude Code agent team.
#
# Record from the repo root:  vhs docs/demo.tape
# Uses a throwaway HOME in /tmp/tpdemo so your real config is never touched.
Output docs/demo.gif

Set Shell bash
Set FontSize 15
Set Width 1100
Set Height 620
Set Padding 16
Set Framerate 12
Set TypingSpeed 35ms
Set WaitTimeout 10s
Set Theme "Catppuccin Mocha"

Hide
Type "rm -rf /tmp/tpdemo && mkdir -p /tmp/tpdemo/bin /tmp/tpdemo/home/.claude && go build -o /tmp/tpdemo/bin/teampane . && cd /tmp/tpdemo/home"
Enter
Type `printf '{"env":{"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS":"1"},"teammateMode":"auto"}' > .claude/settings.json`
Enter
Type "export HOME=/tmp/tpdemo/home PATH=/tmp/tpdemo/bin:/usr/bin:/bin TEAMPANE_BACKEND=headless TEAMPANE_HEADLESS_DIR=/tmp/tpdemo/panes TEAMPANE_HEADLESS_PANE=lead PS1='$ '; unset KITTY_WINDOW_ID KITTY_LISTEN_ON WEZTERM_PANE ZELLIJ TMUX TMUX_PANE"
Enter
Type "clear"
Enter
Sleep 500ms
Show

Type "# teampane CLI demo - headless backend, tmux calls typed by hand"
Enter
Wait /\$\s*$/
Sleep 1s
Type "teampane doctor"
Enter
Wait /\$\s*$/
Sleep 2.5s
Type "teampane install >/dev/null && teampane doctor"
Enter
Wait /\$\s*$/
Sleep 3s
Type "clear"
Enter
Wait /\$\s*$/

Type "# start a shell the way 'teampane run claude' starts Claude Code"
Enter
Wait /\$\s*$/
Type "teampane run -- bash --norc"
Enter
Wait /\$\s*$/
Sleep 1s
Type "which tmux; echo $TMUX_PANE"
Enter
Wait /\$\s*$/
Sleep 1.5s
Type "# Claude Code: where am I?"
Enter
Wait /\$\s*$/
Type "tmux display-message -p '#{session_name}:#{window_id}.#{pane_id}'"
Enter
Wait /\$\s*$/
Sleep 1.5s
Type "# Claude Code: open a placeholder pane for a teammate"
Enter
Wait /\$\s*$/
Type "tmux split-window -d -h -l 70% -P -F '#{pane_id}' -- cat"
Enter
Wait /\$\s*$/
Sleep 1.5s
Type "tmux select-pane -t %2 -T researcher"
Enter
Wait /\$\s*$/
Sleep 1s
Type "tmux respawn-pane -k -t %2 -- 'echo hello from the researcher teammate; sleep 60'"
Enter
Wait /\$\s*$/
Sleep 1s
Type "tmux list-panes -F '#{pane_id} #{pane_title}'"
Enter
Wait /\$\s*$/
Sleep 1.5s
Type "tmux capture-pane -p -t %2"
Enter
Wait /\$\s*$/
Sleep 1.5s
Type "tmux kill-pane -t %2 && tmux list-panes -F '#{pane_id}'"
Enter
Wait /\$\s*$/
Sleep 3s

Hide
Type "exit"
Enter
Show
