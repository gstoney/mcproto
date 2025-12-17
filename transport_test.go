package mcproto

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/gstoney/mcproto/packet"
)

var cfg = TransportConfig{
	MaxPacketLen:         1024,
	MaxRetainedBufferLen: 8096,
}

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
			cfg:                  cfg,
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
			cfg:                  cfg,
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

var statusResp = []byte{
	0x78, 0x00, 0x76, 0x7b, 0x22, 0x76, 0x65, 0x72,
	0x73, 0x69, 0x6f, 0x6e, 0x22, 0x3a, 0x7b, 0x22,
	0x6e, 0x61, 0x6d, 0x65, 0x22, 0x3a, 0x22, 0x50,
	0x61, 0x70, 0x65, 0x72, 0x20, 0x31, 0x2e, 0x32,
	0x31, 0x2e, 0x31, 0x30, 0x22, 0x2c, 0x22, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x22,
	0x3a, 0x37, 0x37, 0x33, 0x7d, 0x2c, 0x22, 0x64,
	0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69,
	0x6f, 0x6e, 0x22, 0x3a, 0x22, 0x41, 0x20, 0x4d,
	0x69, 0x6e, 0x65, 0x63, 0x72, 0x61, 0x66, 0x74,
	0x20, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x22,
	0x2c, 0x22, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72,
	0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6d, 0x61, 0x78,
	0x22, 0x3a, 0x32, 0x30, 0x2c, 0x22, 0x6f, 0x6e,
	0x6c, 0x69, 0x6e, 0x65, 0x22, 0x3a, 0x30, 0x7d,
	0x7d,
}

func BenchmarkRecv(b *testing.B) {
	r := bytes.NewReader(statusResp)

	t := NewTransport(nil, cfg)

	t.reader = r

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := t.Recv(packet.StatusClientboundRegistry); err != nil {
			b.Fatal(err)
		}
		r.Reset(statusResp)
	}
}

func BenchmarkSend(b *testing.B) {
	pack := packet.StatusRespPacket{
		Response: "{\"version\":{\"name\":\"Paper 1.21.10\",\"protocol\":773},\"description\":\"A Minecraft Server\",\"players\":{\"max\":20,\"online\":0}}",
	}

	t := NewTransport(nil, cfg)

	w := bytes.NewBuffer(make([]byte, 0, 1024))
	t.writer = w

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if err := t.Send(&pack); err != nil {
			b.Fatal(err)
		}
		w.Reset()
	}
}
