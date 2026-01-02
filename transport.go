package mcproto

import (
	"bytes"
	"errors"
	"io"
	"net"
	"time"

	"github.com/gstoney/mcproto/packet"
)

var ErrPacketTooBig = errors.New("packet too big")

type TransportConfig struct {
	ReadTO             time.Duration
	WriteTO            time.Duration
	MaxPacketLen       int32
	MaxDecompressedLen int32
}

// Transport provides read and write access to a framed stream,
// with compression and encryption handled internally.
// Transport does not deserialize packets.
type Transport struct {
	conn net.Conn

	reader io.Reader
	writer io.Writer

	fReader FrameReader

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
		fReader:              FrameReader{conn, 0},
		compressionThreshold: -1,
		cfg:                  cfg,
	}

	return t
}

func (t *Transport) Recv() (r PayloadReader, err error) {
	if t.cfg.ReadTO > 0 {
		t.conn.SetReadDeadline(time.Now().Add(t.cfg.ReadTO))
	}

	frameLength, err := t.fReader.Next()
	if err != nil {
		return nil, err
	}

	if frameLength > t.cfg.MaxPacketLen {
		return nil, ErrPacketTooBig
	}

	r = plainPayload{&t.fReader}

	decompressedLen := int32(0)

	if t.compressionThreshold >= 0 {
		decompressedLen, err = packet.ReadVarIntFromReader(&t.fReader)
		if err != nil {
			return nil, err
		}

		if decompressedLen > 0 {
			if decompressedLen > t.cfg.MaxDecompressedLen {
				return nil, ErrPacketTooBig
			}

			panic("not implemented")

		} else if decompressedLen < 0 {
			return nil, errors.New("invalid data length")
		}
	}

	return r, err
}

func (t *Transport) Send(b []byte) error {
	if t.cfg.WriteTO > 0 {
		t.conn.SetWriteDeadline(time.Now().Add(t.cfg.WriteTO))
	}

	lenbuf := bytes.NewBuffer(make([]byte, 0, 5))
	err := packet.WriteVarInt(lenbuf, int32(len(b)))
	if err != nil {
		return err
	}

	if t.compressionThreshold >= 0 {
		panic("not implemented")
	}

	_, err = lenbuf.WriteTo(t.writer)
	if err != nil {
		return err
	}
	_, err = t.writer.Write(b)
	return err
}
