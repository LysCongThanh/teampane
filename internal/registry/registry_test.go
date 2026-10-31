package registry

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAllocIsUniqueAcrossConcurrentStores(t *testing.T) {
	dir := t.TempDir()
	const workers, each = 8, 25
	var wg sync.WaitGroup
	ids := make(chan string, workers*each)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := Open(dir) // separate Store per goroutine, like separate processes
			if err != nil {
				t.Error(err)
				return
			}
			for i := 0; i < each; i++ {
				err := s.Update(func(st *State) error {
					id := st.Alloc()
					st.Panes = append(st.Panes, &Pane{ID: id, Window: "@1", Created: time.Now()})
					ids <- id
					return nil
				})
				if err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
	}
	s, _ := Open(dir)
	st, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Panes) != workers*each || len(seen) != workers*each {
		t.Fatalf("panes=%d seen=%d", len(st.Panes), len(seen))
	}
}

func TestUpdateErrorDoesNotPersist(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Update(func(st *State) error { st.Alloc(); return os.ErrInvalid })
	st, _ := s.Read()
	if st.NextID != 1 {
		t.Fatalf("failed update leaked state: next=%d", st.NextID)
	}
}

func TestEnsureLeaderAndPrune(t *testing.T) {
	now := time.Now()
	st := &State{NextID: 1}
	lead := st.EnsureLeader("8", now)
	if lead.ID != "%1" || lead.Window != "@1" || !lead.Leader {
		t.Fatalf("leader = %+v", lead)
	}
	if again := st.EnsureLeader("8", now); again != lead {
		t.Fatal("EnsureLeader must be idempotent per native id")
	}
	st.Panes = append(st.Panes,
		&Pane{ID: st.Alloc(), Window: "@1", HostPID: 111, Created: now},
		&Pane{ID: st.Alloc(), Window: "@1", HostPID: 222, Created: now},
		&Pane{ID: st.Alloc(), Window: "@1", Created: now.Add(-time.Minute)},
	)
	removed := st.Prune(func(pid int) bool { return pid == 222 }, now, 30*time.Second)
	if len(removed) != 2 || len(st.Panes) != 2 || st.Panes[1].HostPID != 222 {
		t.Fatalf("prune removed=%d panes=%+v", len(removed), st.Panes)
	}
	if got := st.InWindow("@1"); len(got) != 2 || got[0] != lead {
		t.Fatalf("InWindow = %+v", got)
	}
}

func TestOpenRejectsSymlinkDir(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	link := filepath.Join(base, "link")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(link); err == nil {
		t.Fatal("expected refusal for symlinked state dir")
	}
}

func TestWriteFileAtomicLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.json")
	if err := WriteFileAtomic(p, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
	data, _ := os.ReadFile(p)
	fi, _ := os.Stat(p)
	if string(data) != "two" || fi.Mode().Perm() != 0o600 {
		t.Fatalf("content=%q perm=%v", data, fi.Mode().Perm())
	}
}
