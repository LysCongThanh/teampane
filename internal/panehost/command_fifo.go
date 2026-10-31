package panehost

import (
	"errors"
	"io"
	"os"
	"syscall"
	"time"
)

// ErrNotWaiting means the pane is no longer waiting for a command (its
// FIFO is gone): the command already started, or the pane died.
var ErrNotWaiting = errors.New("pane is not waiting for a command")

// MakeFifo creates the command FIFO (mode 0600), replacing stale files.
func MakeFifo(path string) error {
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return errors.New("refusing symlinked fifo path " + path)
	}
	os.Remove(path)
	return syscall.Mkfifo(path, 0o600)
}

// ReadCommand blocks until a writer delivers a command or timeout passes.
// On timeout it returns "" and a nil error. The FIFO is removed afterwards
// so late writers get ErrNotWaiting instead of blocking.
func ReadCommand(path string, timeout time.Duration) (string, error) {
	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err != nil {
			ch <- result{nil, err}
			return
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		ch <- result{data, err}
	}()
	select {
	case r := <-ch:
		os.Remove(path)
		return string(r.data), r.err
	case <-time.After(timeout):
		// Unblock the reader goroutine's open() with a throwaway writer,
		// then remove the FIFO. A command that raced in is still honoured.
		if w, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			w.Close()
		}
		os.Remove(path)
		select {
		case r := <-ch:
			return string(r.data), r.err
		case <-time.After(time.Second):
			return "", nil
		}
	}
}

// Deliver writes cmd to a waiting pane. It retries while the reader is not
// yet attached (ENXIO) and gives up with ErrNotWaiting if the FIFO does
// not exist, or with an error after timeout.
func Deliver(path, cmd string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		fi, err := os.Lstat(path)
		if err != nil || fi.Mode()&os.ModeNamedPipe == 0 {
			return ErrNotWaiting
		}
		f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			_, werr := io.WriteString(f, cmd)
			cerr := f.Close()
			if werr != nil {
				return werr
			}
			return cerr
		}
		if !errors.Is(err, syscall.ENXIO) {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("timed out delivering command to pane")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// WaitReady polls for a pane's ready marker.
func WaitReady(readyPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("pane did not become ready in " + timeout.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
