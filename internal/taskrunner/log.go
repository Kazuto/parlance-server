package taskrunner

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// maxLogSize caps a run's log; later lines are dropped.
const maxLogSize = 64 << 10

// runLog collects a run's log lines, each with its time.
type runLog struct {
	mu      sync.Mutex
	text    strings.Builder
	cut     bool
	changed bool
}

// Printf adds a line.
func (l *runLog) Printf(format string, args ...any) {
	line := time.Now().UTC().Format("15:04:05") + " " + fmt.Sprintf(format, args...) + "\n"

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cut {
		return
	}

	if l.text.Len()+len(line) > maxLogSize {
		l.text.WriteString("… the log is cut off here\n")
		l.cut = true
	} else {
		l.text.WriteString(line)
	}

	l.changed = true
}

// String returns the log so far.
func (l *runLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.text.String()
}

// takeChanged returns the log and whether it changed since the last call.
func (l *runLog) takeChanged() (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	changed := l.changed
	l.changed = false

	return l.text.String(), changed
}
