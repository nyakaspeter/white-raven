package appcore

import (
	"strings"
	"sync"
)

type LogBuffer struct {
	mu    sync.Mutex
	lines []string
	carry string
	limit int
}

func NewLogBuffer(limit int) *LogBuffer { return &LogBuffer{limit: limit} }

func (buffer *LogBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	text := buffer.carry + string(data)
	parts := strings.Split(text, "\n")
	buffer.carry = parts[len(parts)-1]
	buffer.lines = append(buffer.lines, parts[:len(parts)-1]...)
	if excess := len(buffer.lines) - buffer.limit; excess > 0 {
		buffer.lines = append([]string(nil), buffer.lines[excess:]...)
	}
	return len(data), nil
}

func (buffer *LogBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	lines := append([]string(nil), buffer.lines...)
	if buffer.carry != "" {
		lines = append(lines, buffer.carry)
	}
	return strings.Join(lines, "\n")
}

func (buffer *LogBuffer) Clear() {
	buffer.mu.Lock()
	buffer.lines = nil
	buffer.carry = ""
	buffer.mu.Unlock()
}
