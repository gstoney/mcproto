package mcproto

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"

	"github.com/gstoney/mcproto/packet"
)

func defaultConfig() TransportConfig {
	return TransportConfig{
		MaxPacketLen:       1 << 20, // 1MB
		MaxDecompressedLen: 1 << 21, // 2MB
	}
}

// TestTransport_Roundtrip verifies that a packet can be sent and received
// with identical payload through an uncompressed transport.
func TestTransport_Roundtrip(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())

	payload := []byte("hello minecraft")
	if err := tr.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

// TestTransport_LargePayload verifies that large payloads (64KB) are
// correctly framed and transmitted without corruption.
func TestTransport_LargePayload(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())

	payload := make([]byte, 1<<16) // 64KB
	for i := range payload {
		payload[i] = byte(i)
	}

	if err := tr.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch")
	}
}

// TestTransport_MultiplePackets verifies that multiple packets sent
// sequentially maintain proper frame boundaries and are received in order.
func TestTransport_MultiplePackets(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())

	packets := [][]byte{
		[]byte("first"),
		[]byte("second"),
		[]byte("third"),
	}

	for _, p := range packets {
		if err := tr.Send(p); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	for i, want := range packets {
		pr, err := tr.Recv()
		if err != nil {
			t.Fatalf("Recv[%d]: %v", i, err)
		}

		got, err := io.ReadAll(pr)
		if err != nil {
			t.Fatalf("ReadAll[%d]: %v", i, err)
		}
		if err := pr.Close(); err != nil {
			t.Fatalf("Close[%d]: %v", i, err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("packet[%d]: got %q, want %q", i, got, want)
		}
	}
}

// TestTransport_PartialRead verifies that Close returns ErrNotExhausted
// when the payload is not fully consumed by the caller.
func TestTransport_PartialRead(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())

	payload := []byte("hello minecraft")
	if err := tr.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	// Read only first 5 bytes
	partial := make([]byte, 5)
	n, err := io.ReadFull(pr, partial)
	if err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if n != 5 || string(partial) != "hello" {
		t.Errorf("got %q, want %q", partial[:n], "hello")
	}

	// Close without reading rest should error
	if err := pr.Close(); err != ErrNotExhausted {
		t.Errorf("Close: got %v, want ErrNotExhausted", err)
	}
}

// TestTransport_SkipThenClose verifies that Skip discards remaining payload
// bytes, allowing Close to succeed without ErrNotExhausted.
func TestTransport_SkipThenClose(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())

	payload := []byte("hello minecraft")
	if err := tr.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	// Read partial
	partial := make([]byte, 5)
	io.ReadFull(pr, partial)

	// Skip rest
	skipped, err := pr.Skip()
	if err != nil {
		t.Fatalf("Skip: %v", err)
	}
	if skipped != int32(len(payload)-5) {
		t.Errorf("skipped %d, want %d", skipped, len(payload)-5)
	}

	// Now Close should succeed
	if err := pr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestTransport_Discard verifies that Discard abandons the current frame
// and realigns to the next frame boundary, allowing subsequent packets to be read.
func TestTransport_Discard(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())

	packets := [][]byte{
		[]byte("first"),
		[]byte("second"),
	}

	for _, p := range packets {
		if err := tr.Send(p); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	// Recv first, discard without reading
	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if _, err := pr.Discard(); err != nil {
		t.Fatalf("Discard: %v", err)
	}

	// Should be able to read second packet
	pr, err = tr.Recv()
	if err != nil {
		t.Fatalf("Recv second: %v", err)
	}

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if !bytes.Equal(got, packets[1]) {
		t.Errorf("got %q, want %q", got, packets[1])
	}
}

// TestTransport_Remaining verifies that Remaining correctly reports
// the number of unread payload bytes as the caller reads.
func TestTransport_Remaining(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())

	payload := []byte("hello minecraft")
	if err := tr.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	if pr.Remaining() != int32(len(payload)) {
		t.Errorf("Remaining: got %d, want %d", pr.Remaining(), len(payload))
	}

	partial := make([]byte, 5)
	io.ReadFull(pr, partial)

	if pr.Remaining() != int32(len(payload)-5) {
		t.Errorf("Remaining after read: got %d, want %d", pr.Remaining(), len(payload)-5)
	}

	pr.Discard()
}

// TestTransport_PacketTooBig verifies that Recv returns ErrPacketTooBig
// when the frame length exceeds MaxPacketLen.
func TestTransport_PacketTooBig(t *testing.T) {
	var buf bytes.Buffer
	cfg := TransportConfig{
		MaxPacketLen:       100,
		MaxDecompressedLen: 200,
	}
	tr := NewTransport(&buf, &buf, cfg)

	payload := make([]byte, 200)
	if err := tr.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	_, err := tr.Recv()
	if err != ErrPacketTooBig {
		t.Errorf("Recv: got %v, want ErrPacketTooBig", err)
	}
}

// TestTransport_CompressedRoundtrip verifies that a compressed packet can be
// sent and received with identical payload.
func TestTransport_CompressedRoundtrip(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())
	tr.CompressionThreshold = 10

	payload := []byte("hello minecraft compressed payload test")
	if err := tr.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

// TestTransport_CompressedBelowThreshold verifies that packets below the
// compression threshold are received correctly when compression is enabled.
func TestTransport_CompressedBelowThreshold(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())
	tr.CompressionThreshold = 100

	payload := []byte("short")
	if err := tr.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

// TestTransport_CompressedSkipAndClose verifies that Skip discards remaining
// payload bytes in a compressed packet, allowing Close to succeed.
func TestTransport_CompressedSkipAndClose(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())
	tr.CompressionThreshold = 10

	payload := bytes.Repeat([]byte("compressed data "), 10)
	if err := tr.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	// Read partial
	partial := make([]byte, 20)
	io.ReadFull(pr, partial)

	// Skip rest
	skipped, err := pr.Skip()
	if err != nil {
		t.Fatalf("Skip: %v", err)
	}
	if skipped != int32(len(payload)-20) {
		t.Errorf("skipped %d, want %d", skipped, len(payload)-20)
	}

	// Close should succeed
	if err := pr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func compress(b []byte) []byte {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write(b)
	zw.Close()

	return buf.Bytes()
}

// TestTransport_CompressedTrailingData verifies that Close returns
// ErrZlibTrailingData when the frame contains extra bytes after the
// zlib stream ends, indicating a malformed or malicious packet.
func TestTransport_CompressedTrailingData(t *testing.T) {
	var buf, payloadBuf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())
	tr.CompressionThreshold = 10

	// Forge frame with trailing data
	payload := bytes.Repeat([]byte("compressed data "), 10)
	dataLen := len(payload)
	packet.WriteVarInt(&payloadBuf, int32(dataLen))

	payloadBuf.Write(compress(payload))
	payloadBuf.Write([]byte("trailing data")) // Extra bytes after zlib stream

	packet.WriteVarInt(&buf, int32(payloadBuf.Len()))
	payloadBuf.WriteTo(&buf)

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, "hello")
	}

	// Close should detect trailing data and raise error
	if err := pr.Close(); err != ErrZlibTrailingData {
		t.Errorf("Close: got %v, want ErrTrailingData", err)
	}
}

// TestTransport_CompressedPayloadOverrun verifies that Close returns
// ErrZlibPayloadOverrun when the zlib stream produces more bytes than
// the declared decompressed length, indicating a malformed packet.
func TestTransport_CompressedPayloadOverrun(t *testing.T) {
	var buf, frameBuf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())
	tr.CompressionThreshold = 10

	payload := bytes.Repeat([]byte("compressed data "), 10)
	compressed := compress(payload)

	// Lie about decompressed length (claim smaller than actual)
	declaredLen := int32(len(payload) - 50)
	packet.WriteVarInt(&frameBuf, declaredLen)
	frameBuf.Write(compressed)

	packet.WriteVarInt(&buf, int32(frameBuf.Len()))
	frameBuf.WriteTo(&buf)

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	// Read declared amount
	got := make([]byte, declaredLen)
	_, err = io.ReadFull(pr, got)
	if err != nil {
		t.Fatalf("ReadFull: %v", err)
	}

	// Close should detect zlib has more data than declared
	if err := pr.Close(); err != ErrZlibPayloadOverrun {
		t.Errorf("Close: got %v, want ErrZlibPayloadOverrun", err)
	}
}

// TestTransport_CompressedPayloadUnderrun verifies that Read returns
// ErrZlibPayloadUnderrun when the zlib stream ends before producing
// the declared number of bytes, indicating a malformed packet.
func TestTransport_CompressedPayloadUnderrun(t *testing.T) {
	var buf, frameBuf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())
	tr.CompressionThreshold = 10

	payload := bytes.Repeat([]byte("compressed data "), 10)
	compressed := compress(payload)

	// Lie about decompressed length (claim larger than actual)
	declaredLen := int32(len(payload) + 50)
	packet.WriteVarInt(&frameBuf, declaredLen)
	frameBuf.Write(compressed)

	packet.WriteVarInt(&buf, int32(frameBuf.Len()))
	frameBuf.WriteTo(&buf)

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	// Try to read declared amount - should fail partway through
	got := make([]byte, declaredLen)
	_, err = io.ReadFull(pr, got)
	if err != ErrZlibPayloadUnderrun {
		t.Errorf("ReadFull: got %v, want ErrZlibPayloadUnderrun", err)
	}
}

// TestTransport_CompressedDiscard verifies that Discard correctly
// realigns to the next frame boundary after partially reading a compressed
// packet, allowing subsequent packets to be received.
func TestTransport_CompressedDiscard(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())
	tr.CompressionThreshold = 10

	packets := [][]byte{
		bytes.Repeat([]byte("first packet data "), 10),
		bytes.Repeat([]byte("second packet data "), 10),
	}

	for _, p := range packets {
		if err := tr.Send(p); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	// Read first partially, then discard
	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	partial := make([]byte, 10)
	io.ReadFull(pr, partial)

	if _, err := pr.Discard(); err != nil {
		t.Fatalf("Discard: %v", err)
	}

	// Second packet should work
	pr, err = tr.Recv()
	if err != nil {
		t.Fatalf("Recv second: %v", err)
	}

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if !bytes.Equal(got, packets[1]) {
		t.Errorf("got %q, want %q", got, packets[1])
	}
}

// TestTransport_CompressedMultiplePackets verifies that multiple compressed
// packets maintain proper frame boundaries and are received in order with
// correct decompression.
func TestTransport_CompressedMultiplePackets(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(&buf, &buf, defaultConfig())
	tr.CompressionThreshold = 50

	packets := [][]byte{
		bytes.Repeat([]byte("a"), 100),
		bytes.Repeat([]byte("b"), 200),
		bytes.Repeat([]byte("c"), 150),
	}

	for _, p := range packets {
		if err := tr.Send(p); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	for i, want := range packets {
		pr, err := tr.Recv()
		if err != nil {
			t.Fatalf("Recv[%d]: %v", i, err)
		}

		got, err := io.ReadAll(pr)
		if err != nil {
			t.Fatalf("ReadAll[%d]: %v", i, err)
		}
		if err := pr.Close(); err != nil {
			t.Fatalf("Close[%d]: %v", i, err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("packet[%d] mismatch", i)
		}
	}
}

// TestTransport_RecvCapturedPacket verifies that Recv correctly parses
// a packet captured from a real Minecraft connection.
func TestTransport_RecvCapturedPacket(t *testing.T) {
	r := bytes.NewReader(capture_compressed_reg)
	var buf bytes.Buffer
	tr := NewTransport(r, &buf, defaultConfig())
	tr.CompressionThreshold = 50

	pr, err := tr.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	want := payload_compressed_reg
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
