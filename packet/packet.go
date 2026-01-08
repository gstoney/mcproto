//go:generate go run ../codegen/gen_packet_codec.go -- .
package packet

import (
	"io"
)

type Registry map[int32]func() Packet

type ZeroCopyReader interface {
	// ReadN provides a view into the buffer instead of copying.
	//
	// Should return io.EOF when the buffer is empty, and no bytes were read.
	// Any partial read returns io.UnexpectedEOF.
	ReadN(n int) ([]byte, error)
	// Maximum size it can read at once.
	// Callers should not call ReadN bigger than this.
	MaxCapacity() int
}

// Reader interface required for packet decoding.
//
// Buffered readers can run on fast path when implementing ZeroCopyReader
type Reader interface {
	io.Reader
	io.ByteReader
}

// Writer interface required for packet encoding.
type Writer interface {
	io.Writer
	io.ByteWriter
}

type Packet interface {
	ID() int32
	Encode(w Writer) error
	Decode(r Reader) error
}

// @gen:r,w
type HandshakePacket struct {
	ProtocolVersion int32  `field:"VarInt"`
	ServerAddr      string `field:"String"`
	ServerPort      uint16 `field:"UnsignedShort"`
	RequestType     int32  `field:"VarInt"`
}

func (p HandshakePacket) ID() int32 {
	return 0
}
