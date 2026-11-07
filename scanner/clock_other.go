//go:build !windows

package scanner

import "time"

type stopwatch struct{ started time.Time }

func startStopwatch() stopwatch { return stopwatch{started: time.Now()} }

func (s stopwatch) elapsed() time.Duration { return time.Since(s.started) }
