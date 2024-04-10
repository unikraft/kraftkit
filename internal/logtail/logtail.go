// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package logtail

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/fsnotify/fsnotify"
)

const (
	DefaultTailBufferSize = 4 * 1024
	DefaultTailPeekSize   = 1024
)

// NewLogTail returns a string channel that receives new lines from
// tailing/following the supplied logFile.  Errors can also occur while reading
// the file, which are propagated through the error channel.  If a fatal error
// occurs during the initialization of this method, the last error is returned.
func NewLogTail(ctx context.Context, logFile string) (chan string, chan error, error) {
	f, err := os.Open(logFile)
	if err != nil {
		return nil, nil, fmt.Errorf("opening log file: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, fmt.Errorf("setting up file watcher: %w", err)
	}

	if err := watcher.Add(logFile); err != nil {
		return nil, nil, fmt.Errorf("adding log file to watcher: %w", err)
	}

	logs := make(chan string)
	errs := make(chan error)
	reader := bufio.NewReaderSize(f, DefaultTailBufferSize)

	// Start a goroutine which continuously outputs the logs to the provided
	// channel.
	go func() {
		// First read everything that already exists inside of the log file.
		for {
			if peekAndRead(f, reader, &logs, &errs) {
				break
			}
		}

		for {
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				switch event.Op {
				case fsnotify.Write:
					for {
						if peekAndRead(f, reader, &logs, &errs) {
							break
						}
					}
				}
			}
		}
	}()

	return logs, errs, nil
}

func peekAndRead(file *os.File, reader *bufio.Reader, logs *chan string, errs *chan error) bool {
	// discard leading NUL bytes
	var discarded int

	for {
		b, _ := reader.Peek(DefaultTailPeekSize)
		i := nullPrefixLength(b)

		if i > 0 {
			n, _ := reader.Discard(i)
			discarded += n
		}

		if i < DefaultTailPeekSize {
			break
		}
	}

	s, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		*errs <- err
		return true
	}

	// If we encounter EOF before a line delimiter, ReadBytes() will return the
	// remaining bytes, so push them back onto the buffer, rewind our seek
	// position, and wait for further file changes.  We also have to save our
	// dangling byte count in the event that we want to re-open the file and
	// seek to the end.
	if err == io.EOF {
		l := len(s)

		_, err = file.Seek(-int64(l), io.SeekCurrent)
		if err != nil {
			*errs <- err
			return true
		}

		reader.Reset(file)
		*errs <- io.EOF
		return true
	}

	if len(s) > discarded {
		*logs <- string(s[discarded : len(s)-1])
	}

	return false
}

func nullPrefixLength(b []byte) int {
	for i := range b {
		if b[i] != '\x00' {
			return i
		}
	}

	return len(b)
}
