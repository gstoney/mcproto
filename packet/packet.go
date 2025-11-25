//go:generate go run ../codegen/gen_packet_codec.go -- .
package packet

import (
	"io"
)

type Packet interface {
	ID() int32
	Encode(w io.Writer) error
	Decode(r io.Reader) error
}

// @gen
type HandshakePacket struct {
	ProtocolVersion int32  `field:"VarInt"`
	ServerAddr      string `field:"String"`
	ServerPort      uint16 `field:"UnsignedShort"`
	RequestType     int32  `field:"VarInt"`
}

func (p HandshakePacket) ID() int32 {
	return 0
}
