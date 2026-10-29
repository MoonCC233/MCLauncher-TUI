package gametitle

import (
	"errors"
	"testing"
	"time"
)

func TestWatchRetriesUntilTheWindowAppears(t *testing.T) {
	attempts := 0
	set := func(pid int, title string) error {
		attempts++
		if attempts < 3 {
			return ErrNotFound
		}
		if pid != 42 || title != "My Minecraft" {
			t.Fatalf("set(%d, %q)", pid, title)
		}
		return nil
	}
	if err := Watch(42, "My Minecraft", set, 5, time.Millisecond); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestWatchGivesUp(t *testing.T) {
	set := func(int, string) error { return ErrNotFound }
	if err := Watch(42, "x", set, 3, time.Millisecond); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestWatchStopsOnUnsupported(t *testing.T) {
	attempts := 0
	set := func(int, string) error { attempts++; return ErrUnsupported }
	if err := Watch(42, "x", set, 5, time.Millisecond); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want it to stop after the first", attempts)
	}
}

func TestWatchIgnoresEmptyInput(t *testing.T) {
	called := false
	set := func(int, string) error { called = true; return nil }
	if err := Watch(0, "x", set, 3, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := Watch(42, "", set, 3, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("nothing should be renamed for an empty pid or title")
	}
}

func TestKeepReappliesUntilDone(t *testing.T) {
	done := make(chan struct{})
	calls := make(chan int, 10)
	var count int
	set := func(int, string) error {
		count++
		calls <- count
		return nil
	}
	go Keep(42, "Title", set, time.Millisecond, done)
	<-calls
	<-calls
	<-calls
	close(done)
	time.Sleep(5 * time.Millisecond)
	if count < 3 {
		t.Errorf("set was called %d times, want it to keep re-applying", count)
	}
}

func TestKeepStopsOnUnsupported(t *testing.T) {
	done := make(chan struct{})
	defer close(done)
	attempts := 0
	finished := make(chan struct{})
	set := func(int, string) error { attempts++; return ErrUnsupported }
	go func() {
		Keep(42, "Title", set, time.Millisecond, done)
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("Keep should return on ErrUnsupported")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestKeepIgnoresEmptyInput(t *testing.T) {
	called := false
	set := func(int, string) error { called = true; return nil }
	Keep(0, "Title", set, time.Millisecond, make(chan struct{}))
	Keep(42, "   ", set, time.Millisecond, make(chan struct{}))
	if called {
		t.Error("nothing should be renamed for an empty pid or title")
	}
}
