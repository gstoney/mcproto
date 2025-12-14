package mcproto

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/gstoney/mcproto/packet"
)

type TestCase struct {
	desc      string
	expectErr error
	v         packet.Packet
	reg       packet.Registry
	ser       []byte
}

var cases = []TestCase{
	{
		desc: "PingReqPacket",
		v:    &packet.StatusReqPacket{},
		reg:  packet.StatusServerboundRegistry,
		ser:  []byte{0x01, 0x00},
	},
	{
		desc:      "Packet length too big",
		expectErr: ErrPacketTooBig,
		ser:       append([]byte{0x81, 0x08}, bytes.Repeat([]byte{0}, 1025)...),
	},
	{
		desc:      "Unknown packet id",
		expectErr: ErrUnknownPacketId,
		reg:       packet.StatusServerboundRegistry,
		ser:       []byte{0x01, 0x10},
	},
}

func testRecv(tc TestCase) func(*testing.T) {
	return func(t *testing.T) {
		r := bytes.NewReader(tc.ser)

		tr := Transport{
			reader:               r,
			compressionThreshold: -1,
			encryption:           false,
			cfg: TransportConfig{
				MaxPacketLen: 1024,
			},
		}

		got, err := tr.Recv(tc.reg)

		if tc.expectErr != nil {
			if err == nil {
				t.Fatalf("Recv expected error %v, but succeeded and returned value %v", tc.expectErr, got)
			}
			if !errors.Is(err, tc.expectErr) {
				t.Errorf("Recv expected error %v, but got error %v", tc.expectErr, err)
			}
			return
		}

		if err != nil {
			t.Errorf("Recv failed: %v", err)
		}

		if !reflect.DeepEqual(got, tc.v) {
			t.Errorf("Recv expected %v, got %v", tc.v, got)
		}

		if r.Len() != 0 {
			t.Errorf("Reader did not consume all bytes. %d bytes remaining.", r.Len())
		}
	}
}

func TestRecv(t *testing.T) {
	for _, tC := range cases {
		t.Run(tC.desc, testRecv(tC))
	}
}

func testSend(tc TestCase) func(*testing.T) {
	return func(t *testing.T) {
		w := bytes.NewBuffer(make([]byte, 0))

		tr := Transport{
			writer:               w,
			compressionThreshold: -1,
			encryption:           false,
		}

		err := tr.Send(tc.v)

		if err != nil {
			t.Errorf("Send failed: %v", err)
		}

		if !bytes.Equal(w.Bytes(), tc.ser) {
			t.Errorf("Send expected %x, got %x", tc.ser, w.Bytes())
		}
	}
}

func TestSend(t *testing.T) {
	for _, tC := range cases {
		if tC.v == nil || tC.expectErr != nil {
			continue
		}
		t.Run(tC.desc, testSend(tC))
	}
}
