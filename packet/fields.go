package packet

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/google/uuid"
)

type WriteFn[T any] func(io.Writer, T) error
type ReadFn[T any] func(Reader) (T, error)

func WriteBoolean(w io.Writer, v bool) (err error) {
	b := byte(0)
	if v {
		b = 1
	}

	_, err = w.Write([]byte{b})
	return
}

func ReadBoolean(r Reader) (v bool, err error) {
	b, err := r.ReadByte()
	if err != nil {
		return
	}

	if b == 0 {
		v = false
	} else if b == 1 {
		v = true
	} else {
		err = errors.New("invalid byte for Boolean field")
	}

	return
}

func WriteByte(w io.Writer, v byte) (err error) {
	_, err = w.Write([]byte{v})
	return
}

func ReadByte(r Reader) (v byte, err error) {
	b, err := r.ReadByte()
	return b, err
}

func WriteUnsignedShort(w io.Writer, v uint16) (err error) {
	return binary.Write(w, binary.BigEndian, v)
}

func ReadUnsignedShort(r Reader) (v uint16, err error) {
	b, err := r.Read(2)
	if err != nil {
		return
	}

	v = binary.BigEndian.Uint16(b)
	return
}

func WriteInt(w io.Writer, v int32) (err error) {
	return binary.Write(w, binary.BigEndian, v)
}

func ReadInt(r Reader) (v int32, err error) {
	b, err := r.Read(4)
	if err != nil {
		return
	}

	v = int32(binary.BigEndian.Uint32(b))
	return
}

func WriteLong(w io.Writer, v int64) (err error) {
	return binary.Write(w, binary.BigEndian, v)
}

func ReadLong(r Reader) (v int64, err error) {
	b, err := r.Read(8)
	if err != nil {
		return
	}

	v = int64(binary.BigEndian.Uint64(b))
	return
}

var ErrVarIntTooLong = errors.New("VarInt is too long")

func WriteVarInt(w io.Writer, v int32) error {
	uv := uint32(v)
	for i := 0; ; i++ {
		b := byte(uv & 0x7F)
		uv >>= 7

		if uv != 0 {
			b |= 0x80
		}

		if _, err := w.Write([]byte{b}); err != nil {
			return err
		}

		if uv == 0 {
			return nil
		}
	}
}

func ReadVarInt(r io.ByteReader) (int32, error) {
	var v int32
	var shift uint

	for n := 0; n < 5; n++ {
		b, err := r.ReadByte()
		if err != nil {
			return v, err
		}

		segment := b & 0x7F
		v |= int32(segment) << shift

		shift += 7

		if (b & 0x80) == 0 {
			return v, nil
		}
	}
	return v, ErrVarIntTooLong
}

var ErrNegativeLength = errors.New("negative length")

func WriteString(w io.Writer, v string) (err error) {
	err = WriteVarInt(w, int32(len(v)))
	if err != nil {
		return
	}
	_, err = w.Write([]byte(v))
	return
}

func ReadString(r Reader) (v string, err error) {
	length := int32(0)
	length, err = ReadVarInt(r)
	if err != nil {
		return
	}

	if length < 0 {
		err = ErrNegativeLength
		return
	}

	buf, err := r.Read(int(length))
	return string(buf), err
}

// Position's serialized form is composed of X, Z which are 26 bits each, and 12 bits of Y.
// Thus, unintended content can be written when the values are out of range
type Position struct {
	X int32
	Y int16
	Z int32
}

func WritePosition(w io.Writer, v Position) (err error) {
	packed := (uint64(v.X&0x3FFFFFF) << 38) |
		(uint64(v.Z&0x3FFFFFF) << 12) |
		(uint64(v.Y & 0xFFF))

	err = binary.Write(w, binary.BigEndian, packed)
	return
}

func ReadPosition(r Reader) (v Position, err error) {
	b, err := r.Read(8)
	if err != nil {
		return
	}

	packed := binary.BigEndian.Uint64(b)

	v.X = int32((packed >> 38) & 0x3FFFFFF)
	v.Z = int32((packed >> 12) & 0x3FFFFFF)
	v.Y = int16(packed & 0xFFF)
	return
}

func WriteUUID(w io.Writer, v uuid.UUID) (err error) {
	_, err = w.Write(v[:])
	return
}

func ReadUUID(r Reader) (v uuid.UUID, err error) {
	b, err := r.Read(16)
	if err != nil {
		return
	}

	v = uuid.UUID(b)
	return
}

func WritePrefixedArray[T any](w io.Writer, v []T, write WriteFn[T]) (err error) {
	err = WriteVarInt(w, int32(len(v)))
	if err != nil {
		return
	}

	for _, item := range v {
		err = write(w, item)
		if err != nil {
			return
		}
	}
	return
}

func ReadPrefixedArray[T any](r Reader, read ReadFn[T]) (v []T, err error) {
	length := int32(0)
	if length, err = ReadVarInt(r); err != nil {
		return
	}

	v = make([]T, length)
	for i := 0; i < int(length); i++ {
		var item T
		if item, err = read(r); err != nil {
			return
		}
		v[i] = item
	}

	return
}

// Optional[T] represents Optional field in a packet
//
// Serialized Optional[T] is prefixed with Boolean of whether the value exists.
// If so, the value T is followed.
type Optional[T any] struct {
	Exists bool
	Item   T
}

func WriteOptional[T any](w io.Writer, v Optional[T], write WriteFn[T]) (err error) {
	err = WriteBoolean(w, v.Exists)
	if err != nil {
		return
	}

	if v.Exists {
		err = write(w, v.Item)
	}
	return
}

func ReadOptional[T any](r Reader, read ReadFn[T]) (v Optional[T], err error) {
	if v.Exists, err = ReadBoolean(r); err != nil {
		return
	}

	if v.Exists {
		v.Item, err = read(r)
	}
	return
}
