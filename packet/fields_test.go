package packet

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type TestCase[T any] struct {
	desc      string
	expectErr error
	v         T
	ser       []byte
}

var varintTc = []TestCase[int32]{
	{
		desc: "Zero",
		v:    0,
		ser:  []byte{0x00},
	},
	{
		desc: "One",
		v:    1,
		ser:  []byte{0x01},
	},
	{
		desc: "Two",
		v:    2,
		ser:  []byte{0x02},
	},
	{
		desc: "Max single byte (127)",
		v:    127,
		ser:  []byte{0x7f},
	},
	{
		desc: "Min two bytes (128)",
		v:    128,
		ser:  []byte{0x80, 0x01},
	},
	{
		desc: "Max two bytes (255)", // The largest value that fits in the first 14 bits (0x3FFF) is 16383, but 255 is a standard boundary test.
		v:    255,
		ser:  []byte{0xff, 0x01},
	},
	{
		desc: "Small three bytes (25565)",
		v:    25565,
		ser:  []byte{0xdd, 0xc7, 0x01},
	},
	{
		desc: "Max three bytes (2097151)",
		v:    2097151,
		ser:  []byte{0xff, 0xff, 0x7f},
	},
	{
		desc: "Max positive int32 (2147483647)",
		v:    2147483647,
		ser:  []byte{0xff, 0xff, 0xff, 0xff, 0x07},
	},
	{
		desc: "Negative one (-1)",
		v:    -1,
		ser:  []byte{0xff, 0xff, 0xff, 0xff, 0x0f},
	},
	{
		desc: "Min negative int32 (-2147483648)",
		v:    -2147483648,
		ser:  []byte{0x80, 0x80, 0x80, 0x80, 0x08},
	},
	{
		desc:      "VarInt too long",
		expectErr: ErrVarIntTooLong,
		v:         2147483647,
		ser:       []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0x07},
	},
	{
		desc:      "Unexpected EOF",
		expectErr: io.ErrUnexpectedEOF,
		v:         2147483647,
		ser:       []byte{0xff, 0xff, 0xff, 0xff},
	},
}

func TestWriteVarInt(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0, 5))
	for _, tC := range varintTc {
		if tC.expectErr != nil {
			continue
		}

		t.Run(tC.desc, func(t *testing.T) {
			err := WriteVarInt(buf, tC.v)
			if err != nil {
				t.Fatalf("WriteVarInt failed: %v", err)
			}

			if !bytes.Equal(buf.Bytes(), tC.ser) {
				t.Errorf("WriteVarInt expected %x, got %x", tC.ser, buf.Bytes())
			}
		})
		buf.Reset()
	}
}

func TestReadVarInt(t *testing.T) {
	for _, tC := range varintTc {
		t.Run(tC.desc, func(t *testing.T) {
			// Create a buffer initialized with the serialized bytes (tC.ser)
			r := NewFrameReader(tC.ser)

			// Assume ReadVarInt reads from the io.Reader and returns the decoded int32 and an error
			got, err := ReadVarInt(&r)

			if tC.expectErr != nil {
				if err == nil {
					t.Fatalf("ReadVarInt expected error %v, but succeeded and returned value %d", tC.expectErr, got)
				}
				if !errors.Is(err, tC.expectErr) {
					t.Errorf("ReadVarInt expected error %v, but got error %v", tC.expectErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadVarInt failed: %v", err)
			}

			if got != tC.v {
				t.Errorf("ReadVarInt expected %d, got %d", tC.v, got)
			}

			// Ensure the reader consumed exactly all expected bytes (tC.ser)
			if r.Remaining() != 0 {
				t.Errorf("Reader did not consume all bytes. %d bytes remaining.", r.Remaining())
			}
		})
	}
}

var stringTc = []TestCase[string]{
	{
		desc: "Empty string",
		v:    "",
		ser:  []byte{0x00}, // Length 0, encoded as 0x00
	},
	{
		desc: "ASCII string",
		v:    "Hello",
		ser:  []byte{0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f}, // Length 5 (0x05) + ASCII bytes
	},
	{
		desc: "Unicode string",
		v:    "Go 🎉",                                                 // The emoji is 4 bytes in UTF-8. Total length: 2 + 1 + 4 = 7 bytes
		ser:  []byte{0x07, 0x47, 0x6f, 0x20, 0xf0, 0x9f, 0x8e, 0x89}, // Length 7 (0x07) + UTF-8 bytes
	},
	{
		desc: "Multi byte length (128 bytes)",
		v:    string(bytes.Repeat([]byte{'a'}, 128)),
		ser:  append([]byte{0x80, 0x01}, bytes.Repeat([]byte{'a'}, 128)...),
	},
	{
		desc:      "Read fail: EOF on length VarInt (Length is 0x80)",
		expectErr: io.ErrUnexpectedEOF,
		v:         "",
		ser:       []byte{0x80}, // Missing the second byte of the VarInt length (e.g., length 128)
	},
	{
		desc:      "Read fail: EOF reading string content",
		expectErr: io.ErrUnexpectedEOF,
		v:         "",
		ser:       []byte{0x05, 0x48, 0x65, 0x6c}, // Length 5 (0x05), but only 3 bytes of data follow
	},
	{
		desc:      "Read fail: Negative length prefix",
		expectErr: ErrNegativeLength,
		v:         "",
		ser:       []byte{0xff, 0xff, 0xff, 0xff, 0x0f}, // VarInt encoding for -1
	},
}

func TestWriteString(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0))
	for _, tC := range stringTc {
		if tC.expectErr != nil {
			continue
		}

		t.Run(tC.desc, func(t *testing.T) {
			err := WriteString(buf, tC.v)
			if err != nil {
				t.Fatalf("WriteString failed: %v", err)
			}

			if !bytes.Equal(buf.Bytes(), tC.ser) {
				t.Errorf("WriteString expected %x, got %x", tC.ser, buf.Bytes())
			}
		})
		buf.Reset()
	}
}

func TestReadString(t *testing.T) {
	for _, tC := range stringTc {
		t.Run(tC.desc, func(t *testing.T) {
			// Create a buffer initialized with the serialized bytes (tC.ser)
			r := NewFrameReader(tC.ser)

			got, err := ReadString(&r)

			if tC.expectErr != nil {
				if err == nil {
					t.Fatalf("ReadString expected error %v, but succeeded and returned value %s", tC.expectErr, got)
				}
				if !errors.Is(err, tC.expectErr) {
					t.Errorf("ReadString expected error %v, but got error %v", tC.expectErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadString failed: %v", err)
			}

			if got != tC.v {
				t.Errorf("ReadString expected %s, got %s", tC.v, got)
			}

			// Ensure the reader consumed exactly all expected bytes (tC.ser)
			if r.Remaining() != 0 {
				t.Errorf("Reader did not consume all bytes. %d bytes remaining.", r.Remaining())
			}
		})
	}
}

var pArrayTc = []TestCase[[]byte]{
	{
		desc: "Empty array",
		v:    []byte{},
		ser:  []byte{0x00}, // Length 0, encoded as 0x00
	},
	{
		desc: "Small array (Length 3)",
		v:    []byte{10, 20, 30},
		ser:  []byte{0x03, 10, 20, 30}, // Length 3 (0x03) + data
	},
	{
		desc: "Large array (Length 128)",
		v:    bytes.Repeat([]byte{0xAA}, 128),
		ser:  append([]byte{0x80, 0x01}, bytes.Repeat([]byte{0xAA}, 128)...), // Length 128 (0x80 0x01) + data
	},
	{
		desc:      "Read fail: EOF on length VarInt (Length is 0x80)",
		expectErr: io.ErrUnexpectedEOF,
		ser:       []byte{0x80}, // Missing the second byte of the VarInt length (e.g., length 128)
	},
	{
		desc:      "Read fail: EOF reading array elements",
		expectErr: io.ErrUnexpectedEOF,
		v:         []byte{10, 20, 30},   // Expected array, but stream will be incomplete
		ser:       []byte{0x03, 10, 20}, // Length 3 (0x03), but only 2 bytes of data follow
	},
}

func TestWritePrefixedArray(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0))
	for _, tC := range pArrayTc {
		if tC.expectErr != nil {
			continue
		}

		t.Run(tC.desc, func(t *testing.T) {
			err := WritePrefixedArray(buf, tC.v, WriteByte)
			if err != nil {
				t.Fatalf("WritePrefixedArray failed: %v", err)
			}

			if !bytes.Equal(buf.Bytes(), tC.ser) {
				t.Errorf("WritePrefixedArray expected %x, got %x", tC.ser, buf.Bytes())
			}
		})
		buf.Reset()
	}
}

func TestReadPrefixedArray(t *testing.T) {
	for _, tC := range pArrayTc {
		t.Run(tC.desc, func(t *testing.T) {
			// Create a buffer initialized with the serialized bytes (tC.ser)
			r := NewFrameReader(tC.ser)

			got, err := ReadPrefixedArray(&r, ReadByte)

			if tC.expectErr != nil {
				if err == nil {
					t.Fatalf("ReadPrefixedArray expected error %v, but succeeded and returned value %x", tC.expectErr, got)
				}
				if !errors.Is(err, tC.expectErr) {
					t.Errorf("ReadPrefixedArray expected error %v, but got error %v", tC.expectErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadPrefixedArray failed: %v", err)
			}

			if !bytes.Equal(got, tC.v) {
				t.Errorf("ReadPrefixedArray expected %x, got %x", tC.v, got)
			}

			// Ensure the reader consumed exactly all expected bytes (tC.ser)
			if r.Remaining() != 0 {
				t.Errorf("Reader did not consume all bytes. %d bytes remaining.", r.Remaining())
			}
		})
	}
}

var optionalTc = []TestCase[Optional[byte]]{
	{
		desc: "Value is Present",
		v:    Optional[byte]{Exists: true, Item: 0x42},
		ser:  []byte{0x01, 0x42}, // True (0x01) + Item (0x42)
	},
	{
		desc: "Value is Absent",
		v:    Optional[byte]{Exists: false, Item: 0x00}, // Item value is ignored when Exists is false
		ser:  []byte{0x00},                              // False (0x00)
	},
	{
		desc:      "Read fail: EOF on Boolean prefix",
		expectErr: io.ErrUnexpectedEOF,
		ser:       []byte{},
	},
	{
		desc:      "Read fail: EOF reading Item when Exists is true",
		expectErr: io.ErrUnexpectedEOF,
		ser:       []byte{0x01}, // True (0x01), but no item byte follows
	},
}

func TestWriteOptional(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0))
	for _, tC := range optionalTc {
		if tC.expectErr != nil {
			continue
		}

		t.Run(tC.desc, func(t *testing.T) {
			err := WriteOptional(buf, tC.v, WriteByte)
			if err != nil {
				t.Fatalf("WriteOptional failed: %v", err)
			}

			if !bytes.Equal(buf.Bytes(), tC.ser) {
				t.Errorf("WriteOptional expected %x, got %x", tC.ser, buf.Bytes())
			}
		})
		buf.Reset()
	}
}

func TestReadOptional(t *testing.T) {
	for _, tC := range optionalTc {
		t.Run(tC.desc, func(t *testing.T) {
			// Create a buffer initialized with the serialized bytes (tC.ser)
			r := NewFrameReader(tC.ser)

			got, err := ReadOptional(&r, ReadByte)

			if tC.expectErr != nil {
				if err == nil {
					t.Fatalf("ReadOptional expected error %v, but succeeded and returned value %v", tC.expectErr, got)
				}
				if !errors.Is(err, tC.expectErr) {
					t.Errorf("ReadOptional expected error %v, but got error %v", tC.expectErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadOptional failed: %v", err)
			}

			if got.Exists != tC.v.Exists {
				t.Errorf("Exists flag mismatch. Expected: %t, Got: %t", tC.v.Exists, got.Exists)
			}

			if got.Exists && got.Item != tC.v.Item {
				t.Errorf("Item mismatch. Expected: %v, Got: %v", tC.v.Item, got.Item)
			}

			// Ensure the reader consumed exactly all expected bytes (tC.ser)
			if r.Remaining() != 0 {
				t.Errorf("Reader did not consume all bytes. %d bytes remaining.", r.Remaining())
			}
		})
	}
}
