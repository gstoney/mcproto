package mcproto

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gstoney/mcproto/packet"
)

var ErrPacketTooBig = errors.New("packet too big")
var ErrUnknownPacketId = errors.New("unknown packet id")

type TransportConfig struct {
	ReadTO       time.Duration
	WriteTO      time.Duration
	MaxPacketLen int32
}

// Transport is given a raw net.Conn to deal with compression, encryption
// and parsing into packet structs.
// Also enforces timeouts and more.
type Transport struct {
	conn net.Conn

	reader io.Reader
	writer io.Writer

	sendMu sync.Mutex
	recvMu sync.Mutex

	// States
	compressionThreshold int
	encryption           bool
	encryptionSecret     []byte

	cfg TransportConfig
}

func NewTransport(conn net.Conn, cfg TransportConfig) Transport {
	return Transport{
		conn:                 conn,
		reader:               conn,
		writer:               conn,
		compressionThreshold: -1,
		cfg:                  cfg,
	}
}

func (t *Transport) Recv(reg packet.Registry) (packet.Packet, error) {
	t.recvMu.Lock()
	defer t.recvMu.Unlock()

	if t.cfg.ReadTO > 0 {
		t.conn.SetReadDeadline(time.Now().Add(t.cfg.ReadTO))
	}

	packetLen, err := packet.ReadVarIntFromReader(t.reader)
	if err != nil {
		return nil, err
	}

	if packetLen > t.cfg.MaxPacketLen {
		return nil, ErrPacketTooBig
	}

	buf := make([]byte, packetLen)
	_, err = io.ReadFull(t.reader, buf)
	if err != nil {
		return nil, err
	}
	reader := newPayloadReader(buf)

	if t.compressionThreshold >= 0 {
		panic("not implemented")
	}

	id, err := packet.ReadVarInt(&reader)
	if err != nil {
		return nil, err
	}

	build := reg[id]
	if build == nil {
		return nil, ErrUnknownPacketId
	}

	p := build()
	err = p.Decode(&reader)

	return p, err
}

func (t *Transport) Send(p packet.Packet) error {
	t.sendMu.Lock()
	defer t.sendMu.Unlock()

	if t.cfg.WriteTO > 0 {
		t.conn.SetWriteDeadline(time.Now().Add(t.cfg.WriteTO))
	}

	buf := bytes.NewBuffer(make([]byte, 0))
	err := p.Encode(buf)
	if err != nil {
		return err
	}

	lenbuf := bytes.NewBuffer(make([]byte, 0, 5))
	err = packet.WriteVarInt(lenbuf, int32(buf.Len()))
	if err != nil {
		return err
	}

	_, err = lenbuf.WriteTo(t.writer)
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(t.writer)
	return err
}

type payloadReader struct {
	buf []byte
	off int
}

func newPayloadReader(buf []byte) payloadReader {
	return payloadReader{
		buf: buf,
		off: 0,
	}
}

func (r *payloadReader) Read(n int) ([]byte, error) {
	if r.off+n > len(r.buf) {
		return nil, io.ErrUnexpectedEOF
	}
	b := r.buf[r.off : r.off+n]
	r.off += n
	return b, nil
}

func (r *payloadReader) ReadByte() (byte, error) {
	if r.off >= len(r.buf) {
		return 0, io.ErrUnexpectedEOF
	}
	b := r.buf[r.off]
	r.off++
	return b, nil
}

func (r payloadReader) Remaining() int {
	return len(r.buf) - r.off
}
