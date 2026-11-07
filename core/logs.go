package core

import (
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sblog "github.com/sagernet/sing-box/log"
)

// LogEntry is one log line.
type LogEntry struct {
	ID      int64  `json:"id"`
	Time    int64  `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

var (
	ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	// sing-box prefixes platform messages with "LEVEL[0000] ".
	levelPrefix = regexp.MustCompile(`^[A-Z]+\[\d+\]\s*`)
)

// logBuffer keeps the most recent log entries and batches new ones for
// delivery to the user interface.
type logBuffer struct {
	mu       sync.Mutex
	entries  []LogEntry
	capacity int
	nextID   int64
	pending  []LogEntry
	// level is the most verbose sing-box level kept. sing-box hands every
	// message to platform writers regardless of its configured level.
	level atomic.Int32
}

func newLogBuffer(capacity int) *logBuffer {
	buffer := &logBuffer{capacity: capacity}
	buffer.level.Store(int32(sblog.LevelInfo))
	return buffer
}

// setLevel sets the most verbose level kept, e.g. "info".
func (b *logBuffer) setLevel(name string) {
	level, err := sblog.ParseLevel(name)
	if err != nil {
		level = sblog.LevelInfo
	}
	b.level.Store(int32(level))
}

// WriteMessage implements engine.LogWriter.
func (b *logBuffer) WriteMessage(level sblog.Level, message string) {
	if int32(level) > b.level.Load() {
		return
	}
	message = levelPrefix.ReplaceAllString(ansiEscape.ReplaceAllString(message, ""), "")
	b.add(sblog.FormatLevel(level), strings.TrimSpace(message))
}

func (b *logBuffer) add(level, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	entry := LogEntry{ID: b.nextID, Time: time.Now().UnixMilli(), Level: level, Message: message}
	b.entries = append(b.entries, entry)
	if len(b.entries) > b.capacity {
		b.entries = append([]LogEntry(nil), b.entries[len(b.entries)-b.capacity:]...)
	}
	b.pending = append(b.pending, entry)
	if len(b.pending) > b.capacity {
		b.pending = b.pending[len(b.pending)-b.capacity:]
	}
}

func (b *logBuffer) snapshot() []LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]LogEntry{}, b.entries...)
}

func (b *logBuffer) takePending() []LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	pending := b.pending
	b.pending = nil
	return pending
}

func (b *logBuffer) clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = nil
	b.pending = nil
}
