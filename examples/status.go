//go:build ignore

package main

import (
	"bytes"
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

	t := mcproto.NewTransport(
		conn,
		conn,
		mcproto.TransportConfig{
			MaxPacketLen: 32768,
		},
	)
	var buf bytes.Buffer

	packet.HandshakePacket{
		ProtocolVersion: int32(*proto),
		ServerAddr:      hostname,
		ServerPort:      uint16(port),
		RequestType:     1,
	}.Encode(&buf)

	err = t.Send(buf.Bytes())
	if err != nil {
		panic(err)
	}

	buf.Reset()
	packet.StatusReqPacket{}.Encode(&buf)

	err = t.Send(buf.Bytes())
	if err != nil {
		panic(err)
	}

	var resp packet.StatusRespPacket
	var bufReader packet.BufferedReader

	r, err := t.Recv()
	if err != nil {
		panic(err)
	}

	bufReader.Reset(r)
	if v, err := packet.ReadVarInt(&bufReader); v != resp.ID() || err != nil {
		panic("invalid resp")
	}

	err = resp.Decode(&bufReader)
	if err != nil {
		panic(err)
	}

	if bufReader.Buffered() > 0 {
		panic("not fully decoded")
	}

	err = r.Close()
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.Response)
}
