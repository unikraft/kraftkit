// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package tunnel

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"kraftkit.sh/log"
)

// Relay relays connections from a local listener to a remote host over TLS.
type Relay struct {
	lAddr string
	rAddr string
	ctype string
	auth  string
	name  string
}

const Heartbeat = "\xf0\x9f\x91\x8b\xf0\x9f\x90\x92\x00"

func (r *Relay) Up(ctx context.Context) error {
	l, err := r.listenLocal(ctx)
	if err != nil {
		return err
	}
	defer func() { l.Close() }()
	go func() { <-ctx.Done(); l.Close() }()

	log.G(ctx).Info("tunnelling ", l.Addr(), " to ", r.rAddr)

	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accepting incoming connection: %w", err)
		}

		c := r.newConnection(conn)
		go c.handle(ctx, []byte(r.auth), r.name)
	}
}

func (r *Relay) ControlUp(ctx context.Context, ready chan struct{}) error {
	rc, err := r.dialRemote(ctx)
	if err != nil {
		return err
	}
	defer rc.Close()
	go func() { <-ctx.Done(); rc.Close() }()

	ready <- struct{}{}
	close(ready)

	// Heartbeat every minute to keep the connection alive
	_, err = io.CopyN(rc, bytes.NewReader([]byte(r.auth+Heartbeat)), int64(len(r.auth)+9))
	if err != nil {
		return err
	}
	for {
		time.Sleep(time.Minute)
		_, err := io.CopyN(rc, bytes.NewReader([]byte(Heartbeat)), 9)
		if err != nil {
			return err
		}
	}
}

// newConnection creates a new connection from the given net.Conn.
func (r *Relay) newConnection(conn net.Conn) *connection {
	return &connection{
		relay: r,
		conn:  conn,
	}
}

func (r *Relay) dialRemote(ctx context.Context) (net.Conn, error) {
	var d tls.Dialer
	return d.DialContext(ctx, "tcp4", r.rAddr)
}

func (r *Relay) listenLocal(ctx context.Context) (net.Listener, error) {
	var lc net.ListenConfig
	return lc.Listen(ctx, r.ctype+"4", r.lAddr)
}

// connection represents the server side of a connection to a local TCP socket.
type connection struct {
	// relay is the relay on which the connection arrived.
	relay *Relay
	// conn is the underlying network connection.
	conn net.Conn
}

// handle handles the client connection by relaying reads and writes from/to
// the remote host.
func (c *connection) handle(ctx context.Context, auth []byte, instance string) {
	defer func() {
		c.conn.Close()
		log.G(ctx).Info("closed client connection ", c.conn.RemoteAddr())
	}()

	rc, err := c.relay.dialRemote(ctx)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to connect to remote host")
		return
	}
	defer rc.Close()

	log.G(ctx).Info("accepted client connection ", c.conn.RemoteAddr(), " to ", rc.LocalAddr(), "->", rc.RemoteAddr())

	// NOTE(antoineco): these calls are critical as they allow reads/writes to be
	// later cancelled, because the deadline applies to all future and pending
	// I/O and can be dynamically extended or reduced.
	_ = rc.SetDeadline(noNetTimeout)
	_ = rc.SetDeadline(noNetTimeout)

	defer func() {
		_ = c.conn.SetDeadline(immediateNetCancel)
	}()

	if len(auth) > 0 {
		_, err = rc.Write(auth)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to write auth to remote host")
			return
		}

		var status []byte
		statusRaw := bytes.NewBuffer(status)
		n, err := io.CopyN(statusRaw, rc, 2)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to read auth status from remote host")
			return
		}

		if n != 2 {
			log.G(ctx).Error("invalid auth status from remote host")
			return
		}

		var statusParsed int16
		err = binary.Read(statusRaw, binary.LittleEndian, &statusParsed)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to parse auth status from remote host")
			return
		}

		if statusParsed == 0 {
			log.G(ctx).Error("no more available connections to remote host. Try again later")
			return
		} else if statusParsed < 0 {
			log.G(ctx).Errorf("internal tunnel error (C=%d), to view logs run:", statusParsed)
			fmt.Fprintf(log.G(ctx).Out, "\n    kraft cloud instance logs %s\n\n", instance)
			return
		}
	}

	writerDone := make(chan struct{})
	go func() {
		defer func() {
			_ = rc.SetDeadline(immediateNetCancel)
			writerDone <- struct{}{}
		}()

		_, err = io.Copy(rc, c.conn)
		if err != nil {
			if isNetClosedError(err) {
				return
			}
			if !isNetTimeoutError(err) {
				log.G(ctx).WithError(err).Error("failed to copy data from client to remote host")
			}
		}
	}()

	_, err = io.Copy(c.conn, rc)
	if err != nil {
		if !isNetTimeoutError(err) {
			log.G(ctx).WithError(err).Error("failed to copy data from remote host to client")
		}
	} else {
		// Connection was closed remote so we just return to close our side
		return
	}

	<-writerDone
}

var (
	// zero time value used to prevent network operations from timing out.
	noNetTimeout = time.Time{}
	// non-zero time far in the past used for immediate cancellation of network operations.
	immediateNetCancel = time.Unix(1, 0)
)

// isNetTimeoutError reports whether err is a network timeout error.
func isNetTimeoutError(err error) bool {
	if neterr := net.Error(nil); errors.As(err, &neterr) {
		return neterr.Timeout()
	}
	return false
}

// isNetClosedError reports whether err is a network closed error.
// - first error is for the case when the writer tries to write but the main
// thread already closed the connection.
// - second error is for when reader is still reading but the remote closed
// the connection.
func isNetClosedError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "connection reset by peer")
}
