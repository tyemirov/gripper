//go:build darwin

// Package proctrack provides macOS process tracking using kqueue.
//
// The KqueueTracker follows a root process across forks/execs/exits by registering
// an EVFILT_PROC kevent with NOTE_FORK|NOTE_EXEC|NOTE_EXIT|NOTE_TRACK. Newly created
// children are discovered through NOTE_FORK/NOTE_TRACK events and added to the set.
//
// On timeout, SignalAll delivers a signal to every tracked pid. FallbackDescendants
// offers a final sweep via `ps` in case the tracker missed anything.
package proctrack

import (
	"bufio"
	"fmt"
	"os/exec"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// Tracker represents a process tracker.
type Tracker interface {
	StartTrackingRoot(rootPid int) error
	RootPid() int
	SignalAll(sig syscall.Signal) error
	Close() error
}

// KqueueTracker implements Tracker via BSD kqueue on macOS.
type KqueueTracker struct {
	kqueueFd        int
	rootPid         int
	mutex           sync.RWMutex
	trackedPids     map[int]struct{}
	dispatchStop    chan struct{}
	dispatchStopped chan struct{}
}

// NewKqueueTracker constructs a KqueueTracker. If kqueue creation fails,
// an error is returned and the caller may fall back to process groups.
func NewKqueueTracker() (*KqueueTracker, error) {
	fd, err := unix.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("kqueue: %w", err)
	}
	return &KqueueTracker{
		kqueueFd:        fd,
		trackedPids:     make(map[int]struct{}, 256),
		dispatchStop:    make(chan struct{}),
		dispatchStopped: make(chan struct{}),
	}, nil
}

// StartTrackingRoot registers EVFILT_PROC on the root pid and launches
// the event dispatch loop.
func (t *KqueueTracker) StartTrackingRoot(rootPid int) error {
	t.mutex.Lock()
	t.rootPid = rootPid
	t.trackedPids[rootPid] = struct{}{}
	t.mutex.Unlock()

	if err := t.addProcEvent(rootPid); err != nil {
		return err
	}
	go t.dispatchLoop()
	return nil
}

func (t *KqueueTracker) RootPid() int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.rootPid
}

func (t *KqueueTracker) SignalAll(sig syscall.Signal) error {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	for pid := range t.trackedPids {
		_ = syscall.Kill(pid, sig)
	}
	return nil
}

func (t *KqueueTracker) Close() error {
	close(t.dispatchStop)
	closeErr := unix.Close(t.kqueueFd)
	<-t.dispatchStopped
	return closeErr
}

func (t *KqueueTracker) addProcEvent(pid int) error {
	change := unix.Kevent_t{
		Ident:  uint64(pid),
		Filter: unix.EVFILT_PROC,
		Flags:  unix.EV_ADD | unix.EV_ENABLE | unix.EV_CLEAR,
		Fflags: unix.NOTE_FORK | unix.NOTE_EXEC | unix.NOTE_EXIT | unix.NOTE_TRACK,
	}
	_, err := unix.Kevent(t.kqueueFd, []unix.Kevent_t{change}, nil, nil)
	if err != nil {
		return fmt.Errorf("kevent add pid %d: %w", pid, err)
	}
	return nil
}

func (t *KqueueTracker) deleteProcEvent(pid int) {
	change := unix.Kevent_t{
		Ident:  uint64(pid),
		Filter: unix.EVFILT_PROC,
		Flags:  unix.EV_DELETE,
		Fflags: unix.NOTE_FORK | unix.NOTE_EXEC | unix.NOTE_EXIT | unix.NOTE_TRACK,
	}
	_, _ = unix.Kevent(t.kqueueFd, []unix.Kevent_t{change}, nil, nil)
}

func (t *KqueueTracker) dispatchLoop() {
	defer close(t.dispatchStopped)
	events := make([]unix.Kevent_t, 64)
	for {
		select {
		case <-t.dispatchStop:
			return
		default:
		}

		n, err := unix.Kevent(t.kqueueFd, nil, events, nil)
		if err != nil && err != unix.EINTR {
			return
		}
		for i := 0; i < n; i++ {
			kev := events[i]
			pid := int(kev.Ident)
			flags := kev.Fflags

			if flags&unix.NOTE_EXIT != 0 {
				t.mutex.Lock()
				delete(t.trackedPids, pid)
				t.mutex.Unlock()
				t.deleteProcEvent(pid)
				continue
			}
			if flags&unix.NOTE_FORK != 0 || flags&unix.NOTE_TRACK != 0 || flags&unix.NOTE_EXEC != 0 {
				t.mutex.Lock()
				t.trackedPids[pid] = struct{}{}
				t.mutex.Unlock()
				_ = t.addProcEvent(pid)
			}
		}
	}
}

// FallbackDescendants performs a best-effort descendant listing using `ps`.
func FallbackDescendants(rootPid int) ([]int, error) {
	command := exec.Command("ps", "-axo", "pid,ppid")
	output, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}

	type relation struct{ child, parent int }
	relations := make([]relation, 0, 512)

	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		var child, parent int
		if _, scanErr := fmt.Sscanf(scanner.Text(), "%d %d", &child, &parent); scanErr == nil {
			relations = append(relations, relation{child: child, parent: parent})
		}
	}
	_ = command.Wait()

	descendants := make([]int, 0, 512)
	visited := map[int]bool{rootPid: true}
	queue := []int{rootPid}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, r := range relations {
			if r.parent == current && !visited[r.child] {
				visited[r.child] = true
				descendants = append(descendants, r.child)
				queue = append(queue, r.child)
			}
		}
	}
	return descendants, nil
}
