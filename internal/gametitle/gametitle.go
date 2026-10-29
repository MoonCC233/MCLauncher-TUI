// Package gametitle renames the Minecraft window once the game has opened it.
package gametitle

import (
	"errors"
	"log"
	"strings"
	"time"
)

// ErrUnsupported is returned on platforms without a window API.
var ErrUnsupported = errors.New("renaming the game window is not supported on this platform")

// ErrNotFound means no window of the process was ready yet.
var ErrNotFound = errors.New("no game window found")

// Setter applies a title to the main window of a process.
type Setter func(pid int, title string) error

// Watch retries the setter until it succeeds or the attempts run out, because the
// game creates its window a few seconds after the process starts.
func Watch(pid int, title string, set Setter, attempts int, delay time.Duration) error {
	if pid <= 0 || title == "" {
		return nil
	}
	if set == nil {
		set = Set
	}
	if attempts <= 0 {
		attempts = 1
	}
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if err = set(pid, title); err == nil {
			return nil
		}
		if errors.Is(err, ErrUnsupported) {
			return err
		}
		if attempt < attempts-1 {
			time.Sleep(delay)
		}
	}
	return err
}

// Keep applies the title and then keeps re-applying it until done is closed.
// Minecraft sets its own title while the window initialises, so a single write
// can be overwritten a moment later; polling keeps ours in place.
func Keep(pid int, title string, set Setter, interval time.Duration, done <-chan struct{}) {
	if pid <= 0 || strings.TrimSpace(title) == "" {
		return
	}
	if set == nil {
		set = Set
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}

	applied := false
	for {
		err := set(pid, title)
		switch {
		case err == nil && !applied:
			applied = true
			log.Printf("game window title set to %q (pid %d)", title, pid)
		case errors.Is(err, ErrUnsupported):
			log.Printf("game window title: %v", err)
			return
		}
		select {
		case <-done:
			if !applied {
				log.Printf("game window title: no window to rename for pid %d", pid)
			}
			return
		case <-time.After(interval):
		}
	}
}
