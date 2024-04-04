// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package vimport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"kraftkit.sh/initrd"
)

// buildCPIO generates a CPIO archive from the data at the given source.
func buildCPIO(ctx context.Context, source string) (path string, size int64, err error) {
	if source == "." {
		source, err = os.Getwd()
		if err != nil {
			return "", -1, fmt.Errorf("getting current working directory: %w", err)
		}
	}

	cpio, err := initrd.New(ctx, source)
	if err != nil {
		return "", -1, fmt.Errorf("initializing temp CPIO archive: %w", err)
	}
	cpioPath, err := cpio.Build(ctx)
	if err != nil {
		return "", -1, fmt.Errorf("building temp CPIO archive: %w", err)
	}

	cpioStat, err := os.Stat(cpioPath)
	if err != nil {
		return "", -1, fmt.Errorf("reading information about temp CPIO archive: %w", err)
	}

	return cpioPath, cpioStat.Size(), nil
}

// copyCPIO copies the CPIO archive at the given path over the provided tls.Conn.
func copyCPIO(ctx context.Context, conn *tls.Conn, auth, path string, size int64, callback progressCallbackFunc) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// NOTE(antoineco): this call is critical as it allows writes to be later
	// cancelled, because the deadline applies to all future and pending I/O and
	// can be dynamically extended or reduced.
	_ = conn.SetWriteDeadline(noNetTimeout)
	go func() {
		<-ctx.Done()
		_ = conn.SetWriteDeadline(immediateNetCancel)
	}()

	if _, err = io.Copy(conn, strings.NewReader(auth)); err != nil {
		return err
	}

	n, err := io.Copy(conn, newFileProgress(f, size, callback))
	if err != nil {
		// NOTE(antoineco): such error can be expected if volimport exited early or
		// a deadline was set due to cancellation. What we should convey in the error
		// is that the data import didn't complete, not the low-level network error.
		if !isNetClosedError(err) {
			return err
		}
		if n != size {
			return fmt.Errorf("incomplete write (%d/%d)", n, size)
		}
	}

	return nil
}

var (
	// zero time value used to prevent network operations from timing out.
	noNetTimeout = time.Time{}
	// non-zero time far in the past used for immediate cancellation of network operations.
	immediateNetCancel = time.Unix(1, 0)
)

// isNetClosedError reports whether err is an error encountered while writing a
// response over the network, potentially when the server has gone away.
func isNetClosedError(err error) bool {
	if oe := (*net.OpError)(nil); errors.As(err, &oe) && oe.Op == "write" {
		return true
	}
	return false
}
