package server

import (
	"net"

	"github.com/google/uuid"
)

// A Server defines parameters for running a Minecraft server.
type Server struct {
	Addr               string
	SessionEstablisher SessionEstablisher
	SessionHandler     SessionHandler
}

// SessionEstablisher is given a new accepted connection to handle login process,
// setting up a Transport and Session for SessionHandler to use.
//
// Once the login process is done, and it should switch to config stage and return
// Session and Transport.
type SessionEstablisher func(c net.Conn) (Session, Transport, error)

type SessionHandler func(s *Session, t *Transport) error

// Serve accepts incoming connections on the Listener l,
// creating a new goroutine for each.
// The goroutines read handshake packet and either respond to
// status request or establish session for a login request.
func (s *Server) Serve(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			// LOG accept error
			continue
		}

		go func(s *Server, c net.Conn) {
			defer c.Close()

			session, transport, err := s.SessionEstablisher(c)
			if err != nil {
				// LOG establish error
				return
			}

			s.SessionHandler(&session, &transport)
		}(s, c)
	}
}

type ConnectionMode byte

const (
	_ ConnectionMode = iota
	Status
	Login
	Transfer
	Config
	Play
)

// A Session stores connection and states of a client.
type Session struct {
	LocalAddr  net.Addr
	RemoteAddr net.Addr

	Mode ConnectionMode

	ProtocolVersion int
	ServerAddr      string
	ServerPort      uint16
	Intent          int
	Name            string
	PlayerUUID      uuid.UUID
}
