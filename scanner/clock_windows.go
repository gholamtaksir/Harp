package scanner

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The monotonic clock Go uses on Windows advances with the timer interrupt,
// which can be as coarse as 15.6 ms; round trips are timed with the
// performance counter instead.
var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procQueryPerformanceCount = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerformanceFreq  = kernel32.NewProc("QueryPerformanceFrequency")
	performanceFrequency      = func() int64 {
		var frequency int64
		if procQueryPerformanceFreq.Find() != nil {
			return 0
		}
		if ok, _, _ := procQueryPerformanceFreq.Call(uintptr(unsafe.Pointer(&frequency))); ok == 0 || frequency <= 0 {
			return 0
		}
		return frequency
	}()
)

func performanceCounter() int64 {
	if performanceFrequency == 0 {
		return 0
	}
	var counter int64
	if ok, _, _ := procQueryPerformanceCount.Call(uintptr(unsafe.Pointer(&counter))); ok == 0 {
		return 0
	}
	return counter
}

type stopwatch struct {
	counter int64
	started time.Time
}

func startStopwatch() stopwatch {
	return stopwatch{counter: performanceCounter(), started: time.Now()}
}

func (s stopwatch) elapsed() time.Duration {
	now := performanceCounter()
	if s.counter == 0 || now == 0 {
		return time.Since(s.started)
	}
	ticks := now - s.counter
	return time.Duration(ticks/performanceFrequency)*time.Second +
		time.Duration(ticks%performanceFrequency)*time.Second/time.Duration(performanceFrequency)
}
