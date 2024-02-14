package xen

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"kraftkit.sh/log"
	"xenbits.xenproject.org/git-http/xen.git/tools/golang/xenlight"
)

type XenstoreOperation uint32

const (
	WatchOp    XenstoreOperation = 4
	UnwatchOp  XenstoreOperation = 5
	WatchEvent XenstoreOperation = 15
	Error      XenstoreOperation = 16

	XenStorePathFmt = "/local/domain/%d"
)

func XenstoreSocketPath() string {
	xenstorepath := "/var/run/xenstored"
	if path := os.Getenv("XENSTORED_PATH"); path != "" {
		xenstorepath = path
	}

	return strings.Join([]string{xenstorepath, "socket"}, "/")
}

type XsHeader struct {
	Op     XenstoreOperation
	ReqID  uint32
	TxID   uint32
	Lenght uint32
}

type XsPacket struct {
	Header XsHeader
	Data   []byte
}

type baseWatcher struct {
	closeSignal chan struct{}

	domID  uint32
	conn   net.Conn
	xsPath string
	token  string
}

type Watcher interface {
	Watch(ctx context.Context) (chan struct{}, error)
	Close()
}

func NewWatcher(domID xenlight.Domid) (Watcher, error) {
	conn, err := net.Dial("unix", XenstoreSocketPath())
	if err != nil {
		return nil, err
	}

	return &baseWatcher{
		domID:       uint32(domID),
		conn:        conn,
		xsPath:      fmt.Sprintf(XenStorePathFmt, uint32(domID)),
		token:       "kraftkit" + fmt.Sprintf("%d", uint32(domID)),
		closeSignal: make(chan struct{}),
	}, nil
}

func (packet *XsPacket) pack() []byte {
	data := make([]byte, 0)
	data = binary.LittleEndian.AppendUint32(data, uint32(packet.Header.Op))
	data = binary.LittleEndian.AppendUint32(data, packet.Header.ReqID)
	data = binary.LittleEndian.AppendUint32(data, packet.Header.TxID)
	data = binary.LittleEndian.AppendUint32(data, packet.Header.Lenght)

	data = append(data, packet.Data...)

	return data
}

func unpack(data []byte) XsPacket {
	header := XsHeader{
		Op:     XenstoreOperation(binary.LittleEndian.Uint32(data[0:4])),
		ReqID:  binary.LittleEndian.Uint32(data[4:8]),
		TxID:   binary.LittleEndian.Uint32(data[8:12]),
		Lenght: binary.LittleEndian.Uint32(data[12:16]),
	}
	packet := XsPacket{
		Header: header,
		Data:   data[16 : 16+header.Lenght],
	}
	return packet
}

func (w *baseWatcher) xsWatchRequest() error {

	data := append(append(append([]byte(w.xsPath), '\x00'), []byte(w.token)...), '\x00')
	packet := XsPacket{
		Header: XsHeader{
			Op:     WatchOp,
			ReqID:  0,
			TxID:   0,
			Lenght: uint32(len(data)),
		},
		Data: data,
	}

	if _, err := w.conn.Write(packet.pack()); err != nil {
		return err
	}

	buffer := make([]byte, 4096)
	if _, err := w.conn.Read(buffer); err != nil {
		return err
	}

	packet = unpack(buffer)
	if packet.Header.Op == Error {
		return fmt.Errorf("could not establish communication with xenstore")
	}

	return nil
}

func (w *baseWatcher) Watch(ctx context.Context) (chan struct{}, error) {
	err := w.xsWatchRequest()
	if err != nil {
		return nil, err
	}

	event := make(chan struct{})

	go func() {
		buffer := make([]byte, 4096)
		for {
			select {
			case <-w.closeSignal:
				close(w.closeSignal)
				return
			default:
				if _, err := w.conn.Read(buffer); err != nil {
					if !errors.Is(err, os.ErrClosed) {
						log.G(ctx).Debugf("Error reading from xenstore socket while listening for vm status events: %v\n", err)
					}
					continue
				}

				packet := unpack(buffer)
				strs := SplitData(packet)

				if packet.Header.Op != WatchEvent {
					continue
				}

				if w.token == string(strs[1]) && w.xsPath == string(strs[0]) {
					event <- struct{}{}
				}
			}
		}
	}()

	return event, nil
}

func (w *baseWatcher) Close() {
	w.closeSignal <- struct{}{}
	w.conn.Close()
}

func SplitData(packet XsPacket) []string {
	splitPayload := []string{}
	for _, byteSl := range bytes.Split(packet.Data, []byte("\x00")) {
		splitPayload = append(splitPayload, string(byteSl))
	}

	return splitPayload
}
