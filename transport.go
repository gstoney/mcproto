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
	ReadTO               time.Duration
	WriteTO              time.Duration
	MaxPacketLen         int32
	MaxRetainedBufferLen int32
}

// Transport is given a raw net.Conn to deal with compression, encryption
// and parsing into packet structs.
// Also enforces timeouts and more.
type Transport struct {
	conn net.Conn

	reader io.Reader
	writer io.Writer

	pReader payloadReader

	sendMu sync.Mutex
	recvMu sync.Mutex

	// States
	compressionThreshold int
	encryption           bool
	encryptionSecret     []byte

	cfg TransportConfig
}

func NewTransport(conn net.Conn, cfg TransportConfig) Transport {
	t := Transport{
		conn:                 conn,
		reader:               conn,
		writer:               conn,
		compressionThreshold: -1,
		cfg:                  cfg,
	}

	t.pReader.MaxRetained = cfg.MaxRetainedBufferLen

	return t
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

	if t.compressionThreshold >= 0 {
		panic("not implemented")
	}

	t.pReader.Reset(t.reader, packetLen)

	id, err := packet.ReadVarInt(&t.pReader)
	if err != nil {
		return nil, err
	}

	build := reg[id]
	if build == nil {
		return nil, ErrUnknownPacketId
	}

	p := build()
	err = p.Decode(&t.pReader)

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

// Fully buffered reader implementing packet.Reader
//
// A temporary buffer will be allocated when Resetted with n
// exceeding MaxRetained. The temporary buffer can be around
// until Resetted with n smaller than MaxRetained.
type payloadReader struct {
	buf []byte // retained buffer
	cur []byte // current buffer
	off int

	MaxRetained int32 // maximum length of retained buffer
}

func (r *payloadReader) Reset(src io.Reader, n int32) error {
	if r.MaxRetained > 0 && n > r.MaxRetained {
		// exceeded limit: use temporary buffer
		if int32(cap(r.cur)) < n {
			r.cur = make([]byte, n)
		}
		r.cur = r.cur[:n]
	} else {
		if int32(cap(r.buf)) < n {
			r.buf = make([]byte, n)
		}
		r.cur = r.buf[:n]
	}

	_, err := io.ReadFull(src, r.cur)
	if err != nil {
		return err
	}

	r.off = 0
	return nil
}

func (r *payloadReader) Read(n int) ([]byte, error) {
	if r.off+n > len(r.cur) {
		return nil, io.ErrUnexpectedEOF
	}
	b := r.cur[r.off : r.off+n]
	r.off += n
	return b, nil
}

func (r *payloadReader) ReadByte() (byte, error) {
	if r.off >= len(r.cur) {
		return 0, io.ErrUnexpectedEOF
	}
	b := r.cur[r.off]
	r.off++
	return b, nil
}

func (r payloadReader) Remaining() int {
	return len(r.cur) - r.off
}
