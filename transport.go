package mcproto

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"

	"github.com/gstoney/mcproto/packet"
)

var ErrPacketTooBig = errors.New("packet too big")

type TransportConfig struct {
	MaxPacketLen       int32
	MaxDecompressedLen int32
}

// Transport provides read and write access to a framed stream,
// with compression and encryption handled internally.
// Transport does not deserialize packets.
type Transport struct {
	reader io.Reader
	writer io.Writer

	fReader FrameReader
	zReader io.ReadCloser

	// States
	compressionThreshold int
	encryption           bool
	encryptionSecret     []byte

	cfg TransportConfig
}

func NewTransport(r io.Reader, w io.Writer, cfg TransportConfig) Transport {
	t := Transport{
		reader:               r,
		writer:               w,
		fReader:              FrameReader{r, 0},
		compressionThreshold: -1,
		cfg:                  cfg,
	}

	return t
}

func (t *Transport) Recv() (r PayloadReader, err error) {
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

			if t.zReader == nil {
				t.zReader, err = zlib.NewReader(&t.fReader)
			} else {
				err = t.zReader.(zlib.Resetter).Reset(&t.fReader, nil)
			}
			if err != nil {
				return nil, err
			}

			r = &compressedPayload{t.zReader, &t.fReader, decompressedLen}

		} else if decompressedLen < 0 {
			return nil, errors.New("invalid data length")
		}
	}

	return r, err
}

func (t *Transport) Send(b []byte) error {
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
