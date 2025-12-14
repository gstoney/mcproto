package main

import (
	"flag"
	"fmt"
	"net"
	"strconv"

	"github.com/gstoney/mcproto"
	"github.com/gstoney/mcproto/packet"
)

func main() {
	addr := flag.String("addr", "localhost:25565", "server address (host:port)")
	proto := flag.Int("proto", 773, "protocol version")

	flag.Parse()

	hostname, portstr, err := net.SplitHostPort(*addr)
	if err != nil {
		panic(err)
	}
	port, err := strconv.Atoi(portstr)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Dialing %s for status retrieval...\n", *addr)

	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	t := mcproto.NewTransport(conn,
		mcproto.TransportConfig{
			MaxPacketLen: 32768,
		},
	)

	t.Send(&packet.HandshakePacket{
		ProtocolVersion: int32(*proto),
		ServerAddr:      hostname,
		ServerPort:      uint16(port),
		RequestType:     1,
	})

	t.Send(&packet.StatusReqPacket{})

	p, err := t.Recv(packet.StatusClientboundRegistry)
	if err != nil {
		panic(err)
	}

	resp, ok := p.(*packet.StatusRespPacket)
	if !ok {
		panic("unexpected response")
	}

	fmt.Println(resp.Response)
}
