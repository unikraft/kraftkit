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
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"kraftkit.sh/cpio"
	"kraftkit.sh/initrd"
	"kraftkit.sh/log"
)

// startResponse is the response sent by the server after the token is validated.
type startResponse struct {
	// Free is the number of bytes free on the volume.
	Free uint64 // 8 bytes

	// Total is the total number of bytes on the volume.
	Total uint64 // 8 bytes

	// Maxlen is the maximum file name length that can be sent.
	Maxlen uint64 // 8 bytes
}

func parseStartRespose(resp []byte) (*startResponse, error) {
	var r startResponse

	if len(resp) != 24 {
		return nil, fmt.Errorf("unknown start response")
	}

	err := binary.Read(bytes.NewReader(resp), binary.LittleEndian, &r)
	if err != nil {
		return nil, err
	}

	return &r, nil
}

type stopResponse struct {
	// Free is the number of bytes free on the volume.
	Free uint64 // 8 bytes

	// Total is the total number of bytes on the volume.
	Total uint64 // 8 bytes

	// Maxlen is the maximum file name length that can be sent.
	Maxlen uint64 // 8 bytes
}

func parseStopRespose(resp []byte) (*stopResponse, error) {
	var r stopResponse

	if len(resp) != 24 {
		return nil, fmt.Errorf("unknown stop response")
	}

	err := binary.Read(bytes.NewReader(resp), binary.LittleEndian, &r)
	if err != nil {
		return nil, err
	}

	return &r, nil
}

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
	msgMaxSize = 64 * 1024 // 64K
)

func (r *okResponse) clear() {
	r.status = 0
	r.msglen = 0
	r.message = nil
}

func (r *okResponse) parseMetadata(resp []byte) error {
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

	return nil
}

func (r *okResponse) parse(resp []byte) error {
	if err := r.parseMetadata(resp); err != nil {
		return err
	}

	r.message = resp[8 : 8+r.msglen]

	return nil
}

func (r *okResponse) waitForOK(conn *tls.Conn, errorMsg string) ([]byte, error) {
	retErr := fmt.Errorf(errorMsg)
	for {
		// A message can have at max:
		// status - 4 bytes
		// msglen - 4 bytes
		// msg - 1024 bytes
		var respHeadRawBuf []byte
		var respMsgRawBuf []byte
		respHeadRaw := bytes.NewBuffer(respHeadRawBuf)
		respMsgRaw := bytes.NewBuffer(respMsgRawBuf)

		_, err := io.CopyN(respHeadRaw, conn, 8)
		if err != nil {
			return nil, fmt.Errorf("%w: reading header: %s", retErr, err)
		}

		if err := r.parseMetadata(respHeadRaw.Bytes()); err != nil {
			return nil, fmt.Errorf("%w: parsing header: %s", retErr, err)
		}

		if r.msglen != 0 {
			_, err = io.CopyN(respMsgRaw, conn, int64(r.msglen))
			if err != nil {
				return nil, fmt.Errorf("%w: reading body: %s", retErr, err)
			}
		}

		respRaw := append(respHeadRaw.Bytes(), respMsgRaw.Bytes()...)
		if err := r.parse(respRaw); err != nil {
			return nil, fmt.Errorf("%w: parsing body: %s", retErr, err)
		}

		switch {
		case r.status == 0:
			// If error is unchanged, it means that the server has finished sending
			// and closed the connection without problems.
			if retErr.Error() == errorMsg {
				return r.message, nil
			}

			return nil, retErr
		case r.status == 1:
			return nil, nil
		case r.status == 2:
			return r.message, nil
		case r.status < 0:
			retErr = fmt.Errorf("%w: %s", retErr, strings.TrimSuffix(string(r.message[:len(r.message)-1]), "\n"))
		default:
			return nil, fmt.Errorf("unexpected status: %d", r.status)
		}
	}
}

// waitForOKs waits for OKs to be sent over the connection and decrements the
// waitgroup counter.
func waitForOKs(conn *tls.Conn, auth string, result chan *stopResponse, waitErr chan *error) {
	var err error
	var final *stopResponse
	resp := okResponse{}

	// Close the context on exit
	// We need to do this because the server might have closed before
	// we got an answer for all messages.
	defer func() {
		waitErr <- &err
		result <- final
	}()

	for {
		var stopRespRaw []byte
		if stopRespRaw, err = resp.waitForOK(conn, "transmission failed"); err != nil {
			if strings.Contains(err.Error(), "EOF") ||
				strings.Contains(err.Error(), "use of closed network connection") ||
				strings.Contains(err.Error(), "i/o timeout") ||
				strings.Contains(err.Error(), "broken pipe") {
				return
			}

			// Send a term signal to the server
			_, err := io.Copy(conn, strings.NewReader(auth))
			if err != nil {
				log.G(context.Background()).Errorf("failed to send term signal to server on error: %s", err)
			}

			_ = conn.SetWriteDeadline(immediateNetCancel)
			_ = conn.SetReadDeadline(immediateNetCancel)

			return
		} else {
			// If we got no error but we got a message then it means we finished
			// We signal the main goroutine to exit.
			if len(stopRespRaw) > 0 {
				final, _ = parseStopRespose(stopRespRaw)

				// Send a term signal to the server
				_, err := io.Copy(conn, strings.NewReader(auth))
				if err != nil {
					log.G(context.Background()).Errorf("failed to send term signal to server on finish: %s", err)
				}

				return
			}
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
func copyCPIO(ctx context.Context, conn *tls.Conn, auth, path string, force bool, timeoutS, size uint64, callback progressCallbackFunc) (free uint64, total uint64, err error) {
	var resp okResponse
	var currentSize uint64

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
		return 0, 0, err
	}

	var startRespRaw []byte
	if startRespRaw, err = resp.waitForOK(conn, "authentication failed"); err != nil {
		return 0, 0, err
	}

	volumeStartStats, err := parseStartRespose(startRespRaw)
	if err != nil {
		return 0, 0, err
	}

	if size > volumeStartStats.Free {
		if force {
			log.G(ctx).Warnf(
				"import might exceed volume capacity (free: %s, required: %s, total: %s)\n",
				humanize.IBytes(volumeStartStats.Free),
				humanize.IBytes(size),
				humanize.IBytes(volumeStartStats.Total),
			)
		} else {
			return 0, 0, fmt.Errorf("not enough free space on volume for input data (%d/%d)", size, volumeStartStats.Free)
		}
	}

	// From this point forward we wait for OKs to be sent on a separate thread
	// When returning errors we will use `returnErrors` to ensure that the
	// correct error is propagated up.

	result := make(chan *stopResponse, 1)
	waitErr := make(chan *error, 1)

	go waitForOKs(conn, auth, result, waitErr)
	defer func() {
		if retErr := <-waitErr; retErr != nil {
			err = *retErr
		}
	}()

	fi, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}

	defer fi.Close()

	reader := cpio.NewReader(fi)

	// We need to use a sentinel variable to ensure that the CPIO header of the
	// the `TRAILER!!!` entry is still sent to the importer.
	shouldStop := false

	// Iterate through the files in the archive.
	// Sending a file has a list of steps
	// 1.  Send the raw CPIO header athe name of the file (NUL terminated)
	// 2'. Stop if last entry detected
	// 2.  Copy the file content piece by piece | Link destination
initrdLoop:
	for {
		hdr, raw, err := reader.Next()
		if err == io.EOF {
			shouldStop = true
		} else if err != nil {
			return 0, 0, err
		}

		// 1. Send the header
		_, err = io.CopyN(conn, bytes.NewBuffer(raw.Bytes()), int64(len(raw.Bytes())))
		// NOTE(antoineco): such error can be expected if volimport exited early or
		// a deadline was set due to cancellation. What we should convey in the error
		// is that the data import didn't complete, not the low-level network error.
		if err != nil {
			break
		}

		currentSize += uint64(len(raw.Bytes()))
		updateProgress(float64(currentSize), float64(size), callback)

		nameBytesToSend := []byte(hdr.Name)

		// Add NUL-termination to name string as per CPIO spec
		nameBytesToSend = append(nameBytesToSend, 0x00)

		// 1. Send the file name
		_, err = io.CopyN(conn, bytes.NewReader(nameBytesToSend), int64(len(nameBytesToSend)))
		if err != nil {
			break
		}

		currentSize += uint64(len(nameBytesToSend))
		updateProgress(float64(currentSize), float64(size), callback)

		// 2'. Stop when `TRAILER!!!` met
		if shouldStop {
			break
		}

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
					return 0, 0, err
				}

				_, err = io.CopyN(conn, bytes.NewReader(buf), int64(bread))
				if err != nil {
					break initrdLoop
				}

				currentSize += uint64(bread)
				updateProgress(float64(currentSize), float64(size), callback)
			}
		} else {
			bread := len(hdr.Linkname)

			_, err := io.CopyN(conn, bytes.NewReader([]byte(hdr.Linkname)), int64(bread))
			if err != nil {
				break
			}

			currentSize += uint64(bread)
			updateProgress(float64(currentSize), float64(size), callback)
		}
	}

	// Wait for finish or error to come from the server
	final := <-result
	if final == nil {
		// If we got here, error will be set in the defer function
		return 0, 0, fmt.Errorf("no stop response received")
	}

	return final.Free, final.Total, nil
}

var (
	// zero time value used to prevent network operations from timing out.
	noNetTimeout = time.Time{}
	// non-zero time far in the past used for immediate cancellation of network operations.
	immediateNetCancel = time.Unix(1, 0)
)

type progressCallbackFunc func(progress float64)

// updateProgress updates the progress bar with the current progress.
// NOTE(craciunoiuc): Currently the entry pad nd the name pad are not taken
// into consideration so the progress will fall behind at times by some bytes.
func updateProgress(progress float64, size float64, callback progressCallbackFunc) {
	pct := progress / size
	// FIXME(antoineco): the TUI component does not turn green at the end of the
	// copy if we call callback() with a value of 1.
	if pct < 1.0 {
		callback(pct)
	}
}
