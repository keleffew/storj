// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

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

// Hook allows a writer to hook into this buffer
func (buffer *Buffer) Hook(w io.Writer) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	buffer.tailer = w
}

// Write implements io.Writer Write
func (buffer *Buffer) Write(p []byte) (n int, err error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	if buffer.tailer != nil {
		_, _ = buffer.tailer.Write(p)
	}

	return buffer.data.Write(p)
}

// String returns full buffer content as a string
func (buffer *Buffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	return buffer.data.String()
}
