// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package tunnel

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"kraftkit.sh/log"
)

// Relay relays TCP connections to a local listener to a remote host over TLS.
type Relay struct {
	lAddr string
	rAddr string
}

func (r *Relay) Up(ctx context.Context) error {
	l, err := r.listenLocal(ctx)
	if err != nil {
		return err
	}
	defer func() { l.Close() }()
	go func() { <-ctx.Done(); l.Close() }()

	log.G(ctx).Info("Tunnelling ", l.Addr(), " to ", r.rAddr)

	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accepting incoming connection: %w", err)
		}

		c := r.newConnection(conn)
		go c.handle(ctx)
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
	return lc.Listen(ctx, "tcp4", r.lAddr)
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
func (c *connection) handle(ctx context.Context) {
	log.G(ctx).Info("Accepted client connection ", c.conn.RemoteAddr())
	defer func() {
		c.conn.Close()
		log.G(ctx).Info("Closed client connection ", c.conn.RemoteAddr())
	}()

	rc, err := c.relay.dialRemote(ctx)
	if err != nil {
		log.G(ctx).WithError(err).Error("Failed to connect to remote host")
		return
	}
	defer rc.Close()

	// NOTE(antoineco): these calls are critical as they allow reads/writes to be
	// later cancelled, because the deadline applies to all future and pending
	// I/O and can be dynamically extended or reduced.
	_ = rc.SetDeadline(noNetTimeout)
	_ = rc.SetDeadline(noNetTimeout)

	defer func() {
		_ = c.conn.SetDeadline(immediateNetCancel)
	}()

	const bufSize = 32 * 1024 // same as io.Copy

	writerDone := make(chan struct{})
	go func() {
		defer func() {
			_ = rc.SetDeadline(immediateNetCancel)
			writerDone <- struct{}{}
		}()

		writeBuf := make([]byte, bufSize)
		for {
			n, err := c.conn.Read(writeBuf)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					log.G(ctx).WithError(err).Error("Failed to read from client")
				}
				return
			}
			if _, err := rc.Write(writeBuf[:n]); err != nil {
				log.G(ctx).WithError(err).Error("Failed to write to remote host")
				return
			}
		}
	}()

	readBuf := make([]byte, bufSize)
	for {
		n, err := rc.Read(readBuf)
		if err != nil {
			// expected when the connection gets aborted by a deadline
			if !isNetTimeoutError(err) {
				log.G(ctx).WithError(err).Error("Failed to read from remote host")
			}
			break
		}
		if _, err := c.conn.Write(readBuf[:n]); err != nil {
			log.G(ctx).WithError(err).Error("Failed to write to client")
			break
		}
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
