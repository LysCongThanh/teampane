package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const (
	stateFile = "registry.json"
	lockFile  = "registry.lock"
	version   = 1
)

// Store is a registry persisted in a private directory.
type Store struct {
	Dir string
}

// Open prepares dir (mode 0700, owned by the current user) and returns a
// Store. It refuses symlinked or foreign-owned directories because pane
// command FIFOs and environment snapshots live there.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		return nil, fmt.Errorf("state dir %s is not a real directory", dir)
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return nil, fmt.Errorf("state dir %s is not owned by the current user", dir)
	}
	if fi.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return nil, err
		}
	}
	return &Store{Dir: dir}, nil
}

// Read returns a consistent snapshot under a shared lock.
func (s *Store) Read() (*State, error) {
	var st *State
	err := withLock(filepath.Join(s.Dir, lockFile), syscall.LOCK_SH, func() error {
		var err error
		st, err = s.load()
		return err
	})
	return st, err
}

// Update runs fn on the current state under an exclusive lock and saves
// the result atomically when fn returns nil.
func (s *Store) Update(fn func(*State) error) error {
	return withLock(filepath.Join(s.Dir, lockFile), syscall.LOCK_EX, func() error {
		st, err := s.load()
		if err != nil {
			return err
		}
		if err := fn(st); err != nil {
			return err
		}
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		return WriteFileAtomic(filepath.Join(s.Dir, stateFile), data, 0o600)
	})
}

// load reads the state file; a missing file is an empty registry.
func (s *Store) load() (*State, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, stateFile))
	if errors.Is(err, os.ErrNotExist) {
		return &State{Version: version, NextID: 1}, nil
	}
	if err != nil {
		return nil, err
	}
	st := &State{}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("corrupt registry %s: %w", s.Dir, err)
	}
	if st.Version > version {
		return nil, fmt.Errorf("registry version %d is newer than this teampane (%d)", st.Version, version)
	}
	return st, nil
}
