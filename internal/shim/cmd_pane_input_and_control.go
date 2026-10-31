package shim

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LysCongThanh/teampane/internal/keys"
	"github.com/LysCongThanh/teampane/internal/panehost"
	"github.com/LysCongThanh/teampane/internal/registry"
	"github.com/LysCongThanh/teampane/internal/tmuxargs"
)

// deliverTimeout bounds waiting for a pane host to open its FIFO.
const deliverTimeout = 3 * time.Second

// lookup resolves a target under a shared lock and returns a copy.
func (c *Ctx) lookup(target string) (registry.Pane, error) {
	st, err := c.Store.Read()
	if err != nil {
		return registry.Pane{}, err
	}
	p, err := c.resolve(st, target)
	if err != nil {
		return registry.Pane{}, err
	}
	return *p, nil
}

// cmdSendKeys: a "text Enter" sequence to a pane still waiting for its
// command becomes that command; everything else is typed into the pane.
func cmdSendKeys(c *Ctx, cmd tmuxargs.Command) int {
	if cmd.Has("X") || cmd.Has("M") {
		return c.unsupported("send-keys -X/-M (copy mode, mouse) is not supported")
	}
	p, err := c.lookup(cmd.Flags["t"])
	if err != nil {
		return c.fail(err)
	}
	if line, ok := keys.CommandLine(cmd.Args, cmd.Has("l")); ok {
		if files, err := panehost.PaneFiles(c.Dir, p.ID); err == nil {
			err = panehost.Deliver(files.Fifo, line, deliverTimeout)
			if err == nil {
				return 0
			}
			if !errors.Is(err, panehost.ErrNotWaiting) {
				return c.fail(err)
			}
		}
	}
	text := keys.Encode(cmd.Args, cmd.Has("l"))
	if n, err := strconv.Atoi(cmd.Flags["N"]); err == nil && n > 1 {
		text = strings.Repeat(text, n)
	}
	if text == "" {
		return 0
	}
	if err := c.B.SendText(p.Native, text); err != nil {
		return c.fail(err)
	}
	return 0
}

// cmdRespawnPane hands the real teammate command to the pane created with
// the "cat" placeholder. Respawning a pane that already runs is refused.
func cmdRespawnPane(c *Ctx, cmd tmuxargs.Command) int {
	p, err := c.lookup(cmd.Flags["t"])
	if err != nil {
		return c.fail(err)
	}
	line := join(cmd.Args)
	if strings.TrimSpace(line) == "" {
		return c.unsupported("respawn-pane without a command is not supported")
	}
	files, err := panehost.PaneFiles(c.Dir, p.ID)
	if err != nil {
		return c.fail(err)
	}
	if err := panehost.Deliver(files.Fifo, line, deliverTimeout); err != nil {
		if errors.Is(err, panehost.ErrNotWaiting) {
			return c.fail(fmt.Errorf("pane %s is already running a command; teampane cannot respawn it", p.ID))
		}
		return c.fail(err)
	}
	return 0
}

// cmdKillPane closes a teammate pane. Leader panes (the user's own) are
// never closed by teampane.
func cmdKillPane(c *Ctx, cmd tmuxargs.Command) int {
	if cmd.Has("a") {
		return c.unsupported("kill-pane -a is not supported")
	}
	p, err := c.lookup(cmd.Flags["t"])
	if err != nil {
		return c.fail(err)
	}
	if p.Leader {
		return c.fail(fmt.Errorf("refusing to close %s: it is not a pane teampane created", p.ID))
	}
	if p.Native != "" {
		if err := c.B.Close(p.Native); err != nil {
			return c.fail(err)
		}
	}
	c.forget(p.ID)
	return 0
}

// cmdSelectPane sets a title (-T) or focuses a pane; style and
// directional flags are accepted and ignored.
func cmdSelectPane(c *Ctx, cmd tmuxargs.Command) int {
	p, err := c.lookup(cmd.Flags["t"])
	if err != nil {
		return c.fail(err)
	}
	if title, ok := cmd.Flag("T"); ok {
		err := c.Store.Update(func(st *registry.State) error {
			if q := st.Find(p.ID); q != nil {
				q.Title = title
			}
			return nil
		})
		if err != nil {
			return c.fail(err)
		}
		if p.Native != "" {
			_ = c.B.SetTitle(p.Native, title) // cosmetic; never fail the spawn over it
		}
		return 0
	}
	for _, f := range []string{"L", "R", "U", "D", "l", "Z", "M", "m", "e", "d"} {
		if cmd.Has(f) {
			return 0
		}
	}
	if _, ok := cmd.Flag("P"); ok || p.Native == "" {
		return 0
	}
	if err := c.B.Focus(p.Native); err != nil {
		return c.fail(err)
	}
	return 0
}
