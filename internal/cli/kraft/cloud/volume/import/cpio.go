// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package vimport

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/cpio"
)

type okResponse struct {
	// status is the status code of the response. It can be 1 for success, -1
	// for error, or 0 for finished sending error.
	status int32 // 4 bytes

	// msglen is the length of the message in bytes.
	// It is only set if status is not 1.
	msglen uint32 // 4 bytes

	// message is the message sent by the server.
	// This is only set if status is not 1.
	message []byte // 1024 bytes per message
}

const (
	// The size read by the `volimport` unikernel on one socket read
	msgMaxSize = 32 * 1024 // 32 KiB
)

func (r *okResponse) clear() {
	r.status = 0
	r.msglen = 0
	r.message = nil
}

func (r *okResponse) parse(resp []byte) error {
	r.clear()

	err := binary.Read(bytes.NewReader(resp[:4]), binary.LittleEndian, &r.status)
	if err != nil {
		return err
	}

	if r.status == 1 {
		return nil
	}

	err = binary.Read(bytes.NewReader(resp[4:8]), binary.LittleEndian, &r.msglen)
	if err != nil {
		return err
	}

	r.message = resp[8 : 8+r.msglen]

	return nil
}

func (r *okResponse) waitForOK(conn *tls.Conn, errorMsg string) error {
	retErr := fmt.Errorf(errorMsg)
	for it := 0; ; it++ {
		// A message can have at max:
		// status - 4 bytes
		// msglen - 4 bytes
		// msg - 1024 bytes
		respRaw := make([]byte, 1032)

		_, err := io.ReadAtLeast(conn, respRaw, 4)
		if err != nil {
			return fmt.Errorf("%w: %s", retErr, err)
		}

		if err := r.parse(respRaw); err != nil {
			return fmt.Errorf("%w: %s", retErr, err)
		}
		switch {
		case r.status == 0:
			if errorMsg != retErr.Error() {
				return retErr
			}
			return nil
		case r.status == 1:
			return nil
		case r.status < 0:
			retErr = fmt.Errorf("%w: %s", retErr, strings.TrimSuffix(string(r.message), "\x0a\n"))
		default:
			return fmt.Errorf("unexpected status: %d", r.status)
		}
	}
}

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
func copyCPIO(ctx context.Context, conn *tls.Conn, auth, path string, timeoutS uint64) error {
	var resp okResponse

	// NOTE(antoineco): this call is critical as it allows writes to be later
	// cancelled, because the deadline applies to all future and pending I/O and
	// can be dynamically extended or reduced.
	if timeoutS > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(time.Duration(timeoutS) * time.Second))
		_ = conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutS) * time.Second))
	} else {
		_ = conn.SetWriteDeadline(noNetTimeout)
		_ = conn.SetReadDeadline(noNetTimeout)
	}
	go func() {
		<-ctx.Done()
		_ = conn.SetWriteDeadline(immediateNetCancel)
		_ = conn.SetReadDeadline(immediateNetCancel)
	}()

	if _, err := io.Copy(conn, strings.NewReader(auth)); err != nil {
		return err
	}

	if err := resp.waitForOK(conn, "authentication failed"); err != nil {
		return err
	}

	fi, err := os.Open(path)
	if err != nil {
		return err
	}

	defer fi.Close()

	reader := cpio.NewReader(fi)

	// We need to use a sentinel variable to ensure that the CPIO header of the
	// the `TRAILER!!!` entry is still sent to the importer.
	shouldStop := false

	// Iterate through the files in the archive.
	// Sending a file has a list of steps
	// 1.  Send the raw CPIO header -- wait for OK
	// 2.  Send the name of the file (NUL terminated) -- wait for OK
	// 2'. Stop if last entry detected
	// 3.  Copy the file content piece by piece | Link destination -- wait for OK
	for {
		hdr, raw, err := reader.Next()
		if err == io.EOF {
			shouldStop = true
		} else if err != nil {
			return err
		}

		// 1. Send the header
		n, err := io.CopyN(conn, bytes.NewBuffer(raw.Bytes()), int64(len(raw.Bytes())))
		// NOTE(antoineco): such error can be expected if volimport exited early or
		// a deadline was set due to cancellation. What we should convey in the error
		// is that the data import didn't complete, not the low-level network error.
		if err != nil {
			if !isNetClosedError(err) {
				return err
			}
			if n != int64(len(raw.Bytes())) {
				return fmt.Errorf("incomplete write (%d/%d)", n, len(raw.Bytes()))
			}
			return err
		}

		if err := resp.waitForOK(conn, "header copy failed"); err != nil {
			return err
		}

		nameBytesToSend := []byte(hdr.Name)

		// Add NUL-termination to name string as per CPIO spec
		nameBytesToSend = append(nameBytesToSend, 0x00)

		// 2. Send the file name
		n, err = io.CopyN(conn, bytes.NewReader(nameBytesToSend), int64(len(nameBytesToSend)))
		if err != nil {
			if !isNetClosedError(err) {
				return err
			}
			if n != int64(len(hdr.Name)) {
				return fmt.Errorf("incomplete write (%d/%d)", n, len(hdr.Name))
			}
			return err
		}

		if err := resp.waitForOK(conn, "name copy failed"); err != nil {
			return err
		}

		// 2'. Stop when `TRAILER!!!` met
		if shouldStop {
			break
		}

		// If nothing was copied the entry was a directory which has no size
		empty := true

		// 3. Send the file content. If the file is a link copy the destination
		// as content in this step. Copy runs uninterrupted until the whole size
		// was sent.
		if hdr.Linkname == "" {
			for {
				toSend := msgMaxSize

				if hdr.Size < int64(toSend) {
					toSend = int(hdr.Size)
				}

				buf := make([]byte, toSend)
				bread, err := reader.Read(buf)

				if err == io.EOF {
					break
				} else if err != nil {
					return err
				}

				n, err := io.CopyN(conn, bytes.NewReader(buf), int64(bread))
				if err != nil {
					if !isNetClosedError(err) {
						return err
					}
					if n != int64(bread) {
						return fmt.Errorf("incomplete write (%d/%d)", n, int64(bread))
					}
					return err
				}

				empty = false
			}
		} else {
			bread := len(hdr.Linkname)

			n, err := io.CopyN(conn, bytes.NewReader([]byte(hdr.Linkname)), int64(bread))
			if err != nil {
				if !isNetClosedError(err) {
					return err
				}
				if n != int64(bread) {
					return fmt.Errorf("incomplete write (%d/%d)", n, int64(bread))
				}
				return err
			}

			empty = false
		}

		// Don't wait for ok if nothing was written
		if !empty {
			if err := resp.waitForOK(conn, "file copy failed"); err != nil {
				return err
			}
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
