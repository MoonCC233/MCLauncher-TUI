//go:build !windows

package gametitle

// Set is not implemented outside Windows: the platforms we ship for have no
// portable way to rename another process' window.
func Set(pid int, title string) error { return ErrUnsupported }
