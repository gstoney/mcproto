package mcproto

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gstoney/mcproto/packet"
)

type TransportConfig struct {
	readTO  time.Duration
	writeTO time.Duration
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

func (t *Transport) Recv() (packet.Packet, error) {
	panic("not implemented")
}

func (t *Transport) Send(p packet.Packet) error {
	panic("not implemented")
}
