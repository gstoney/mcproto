package mcproto

import (
	"bufio"
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

type byteReader interface {
	io.Reader
	io.ByteReader
}

type byteWriter interface {
	io.Writer
	io.ByteWriter
}

// Transport provides read and write access to a framed stream,
// with compression and encryption handled internally.
// Transport does not deserialize packets.
type Transport struct {
	reader byteReader
	writer byteWriter

	fReader FrameReader
	zReader io.ReadCloser

	zBuffer bytes.Buffer
	zWriter *zlib.Writer

	// States
	CompressionThreshold int
	encryption           bool
	encryptionSecret     []byte

	cfg TransportConfig
}

// NewTransport creates a Transport.
//
// For readers/writers that perform syscalls (e.g. net.Conn), buffering is
// required. Indicate buffered I/O by implementing io.ByteReader/io.ByteWriter.
// If these interfaces are not implemented, the reader/writer will be wrapped
// with bufio.
func NewTransport(r io.Reader, w io.Writer, cfg TransportConfig) Transport {
	var br byteReader
	var bw byteWriter

	if b, ok := r.(byteReader); ok {
		br = b
	} else if r != nil {
		br = bufio.NewReader(r)
	}

	if b, ok := w.(byteWriter); ok {
		bw = b
	} else if w != nil {
		bw = bufio.NewWriter(w)
	}

	t := Transport{
		reader:               br,
		writer:               bw,
		fReader:              FrameReader{br, 0},
		CompressionThreshold: -1,
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

	if t.CompressionThreshold >= 0 {
		decompressedLen, err = packet.ReadVarInt(&t.fReader)
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
	length := len(b)

	if t.CompressionThreshold >= 0 {
		if length >= t.CompressionThreshold {
			t.zBuffer.Reset()
			if t.zWriter == nil {
				t.zWriter = zlib.NewWriter(&t.zBuffer)
			} else {
				t.zWriter.Reset(&t.zBuffer)
			}
			packet.WriteVarInt(&t.zBuffer, int32(length))

			t.zWriter.Write(b)
			t.zWriter.Close()

			length = t.zBuffer.Len()
			err := packet.WriteVarInt(t.writer, int32(length))
			if err != nil {
				return err
			}
			_, err = t.zBuffer.WriteTo(t.writer)
			return err

		} else {
			length += 1

			err := packet.WriteVarInt(t.writer, int32(length))
			if err != nil {
				return err
			}
			err = t.writer.WriteByte(0)
			if err != nil {
				return err
			}

			_, err = t.writer.Write(b)
			return err
		}
	}

	err := packet.WriteVarInt(t.writer, int32(length))
	if err != nil {
		return err
	}
	_, err = t.writer.Write(b)
	if err != nil {
		return err
	}

	if bw, ok := t.writer.(*bufio.Writer); ok {
		err = bw.Flush()
	}
	return err
}

func (t *Transport) EnableEncryption(secret []byte) {
	t.encryptionSecret = secret
	t.encryption = true

	panic("not implemented")
}
