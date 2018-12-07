package main

import (
	"bytes"
	"io"
	"sync"
)

// Buffer implements a hookable io.Writer
type Buffer struct {
	mu     sync.Mutex
	data   bytes.Buffer
	tailer io.Writer
}

func (buffer *Buffer) Hook(w io.Writer) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	buffer.tailer = w
}

func (buffer *Buffer) Write(p []byte) (n int, err error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	if buffer.tailer != nil {
		buffer.tailer.Write(p)
	}

	return buffer.data.Write(p)
}

func (buffer *Buffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	return buffer.data.String()
}
