//go:generate go run ../codegen/gen_packet_codec.go -- .
package packet

import (
	"io"
)

type Registry map[int32]func() Packet

type Reader interface {
	Read(n int) ([]byte, error)
	ReadByte() (byte, error)
	Remaining() int
}

type Packet interface {
	ID() int32
	Encode(w io.Writer) error
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
