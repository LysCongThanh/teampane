// Package registry maps tmux pane ids ("%3") to backend-native pane ids and
// keeps the small amount of state a fake tmux needs (windows, titles,
// sessions). It is persisted as JSON under a per-terminal state directory,
// guarded by flock and written atomically.
package registry

import (
	"strconv"
	"strings"
	"time"
)

// Pane is one pane known to the shim.
type Pane struct {
	ID      string    `json:"id"`                 // tmux pane id, e.g. "%3"
	Native  string    `json:"native,omitempty"`   // backend id (kitty window id, wezterm pane id, ...)
	Window  string    `json:"window"`             // tmux window id, e.g. "@1"
	Title   string    `json:"title,omitempty"`    // set via select-pane -T
	HostPID int       `json:"host_pid,omitempty"` // teampane pane-host process; 0 for leaders
	Leader  bool      `json:"leader,omitempty"`   // a pane the user started (never killed by us)
	Created time.Time `json:"created"`
}

// State is the whole registry document.
type State struct {
	Version  int      `json:"version"`
	NextID   int      `json:"next_id"`
	Sessions []string `json:"sessions,omitempty"`
	Panes    []*Pane  `json:"panes"` // creation order
}

// Alloc reserves the next tmux pane id ("%N", N >= 1).
func (s *State) Alloc() string {
	if s.NextID < 1 {
		s.NextID = 1
	}
	id := "%" + strconv.Itoa(s.NextID)
	s.NextID++
	return id
}

// Find returns the pane with tmux id, or nil.
func (s *State) Find(id string) *Pane {
	for _, p := range s.Panes {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// FindNative returns the pane with the backend-native id, or nil.
func (s *State) FindNative(native string) *Pane {
	if native == "" {
		return nil
	}
	for _, p := range s.Panes {
		if p.Native == native {
			return p
		}
	}
	return nil
}

// Remove deletes a pane by tmux id and reports whether it existed.
func (s *State) Remove(id string) bool {
	for i, p := range s.Panes {
		if p.ID == id {
			s.Panes = append(s.Panes[:i], s.Panes[i+1:]...)
			return true
		}
	}
	return false
}

// InWindow returns the panes of a tmux window in creation order.
func (s *State) InWindow(window string) []*Pane {
	var out []*Pane
	for _, p := range s.Panes {
		if p.Window == window {
			out = append(out, p)
		}
	}
	return out
}

// Windows returns distinct window ids in creation order.
func (s *State) Windows() []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range s.Panes {
		if !seen[p.Window] {
			seen[p.Window] = true
			out = append(out, p.Window)
		}
	}
	return out
}

// EnsureLeader returns the pane registered for a native id, registering it
// as a leader with its own window when unknown. Each leader gets window
// "@N" where N is its pane number, so teams of different leaders never mix.
func (s *State) EnsureLeader(native string, now time.Time) *Pane {
	if p := s.FindNative(native); p != nil {
		return p
	}
	id := s.Alloc()
	p := &Pane{ID: id, Native: native, Window: "@" + strings.TrimPrefix(id, "%"), Leader: true, Created: now}
	s.Panes = append(s.Panes, p)
	return p
}

// Prune drops teammate panes whose host process is gone, and panes that
// never reported a host within grace (a failed launch).
func (s *State) Prune(alive func(pid int) bool, now time.Time, grace time.Duration) (removed []*Pane) {
	kept := s.Panes[:0]
	for _, p := range s.Panes {
		dead := !p.Leader && ((p.HostPID > 0 && !alive(p.HostPID)) ||
			(p.HostPID == 0 && now.Sub(p.Created) > grace))
		if dead {
			removed = append(removed, p)
			continue
		}
		kept = append(kept, p)
	}
	s.Panes = kept
	return removed
}

// HasSession reports whether a session name was registered.
func (s *State) HasSession(name string) bool {
	for _, n := range s.Sessions {
		if n == name {
			return true
		}
	}
	return false
}
