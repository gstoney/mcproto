//go:generate go run ../codegen/gen_packet_codec.go -- .
package packet

import (
	"errors"
	"io"
)

type Registry map[int32]func() Packet

// Reader interface for packet decoding.
//
// The slice returned by Read is only valid until the next call to Read or ReadByte.
type Reader interface {
	Read(n int) ([]byte, error)
	ReadByte() (byte, error)
}

const defaultBufferSize = 512
const defaultMaxSize = 1 << 20

var ErrReadTooBig = errors.New("requested read size exceeds maximum")

// BufferedReader wraps an io.Reader to provide buffered, zero-copy reads
// for packet decoding. It implements packet.Reader interface.
//
// The buffer grows dynamically up to maxSize to accommodate large reads.
// Keeps the buffer small as long as the caller doesn't read in large chunk.
// BufferedReader can be reused across packets via Reset to avoid allocation.
type BufferedReader struct {
	src io.Reader

	buf     []byte
	r, w    int
	err     error
	maxSize int
}

func NewBufferedReader(r io.Reader) *BufferedReader {
	return &BufferedReader{
		src:     r,
		buf:     make([]byte, defaultBufferSize),
		maxSize: defaultMaxSize,
	}
}

func NewBufferedReaderSize(r io.Reader, size int, maxSize int) *BufferedReader {
	return &BufferedReader{
		src:     r,
		buf:     make([]byte, size),
		maxSize: maxSize,
	}
}

func (b *BufferedReader) Reset(r io.Reader) {
	b.src = r
	b.r = 0
	b.w = 0
	b.err = nil

	if b.buf == nil {
		b.buf = make([]byte, defaultBufferSize)
	}
	if b.maxSize == 0 {
		b.maxSize = defaultMaxSize
	}
}

func (b *BufferedReader) Buffered() int {
	return b.w - b.r
}

func (b *BufferedReader) fill() {
	if b.r > 0 {
		copy(b.buf, b.buf[b.r:b.w])
		b.w -= b.r
		b.r = 0
	}

	if b.w == len(b.buf) {
		return
	}

	n, err := b.src.Read(b.buf[b.w:])
	b.w += n
	if err != nil {
		b.err = err
	}
}

func (b *BufferedReader) grow(need int) bool {
	if need > b.maxSize {
		return false
	}

	newSize := len(b.buf)
	if newSize == 0 {
		newSize = defaultBufferSize
	}

	for newSize < need {
		newSize *= 2
	}

	if b.maxSize > 0 && newSize > b.maxSize {
		newSize = b.maxSize
	}

	newBuf := make([]byte, newSize)
	copy(newBuf, b.buf[b.r:b.w])

	b.w -= b.r
	b.r = 0
	b.buf = newBuf

	return true
}

func (b *BufferedReader) view(n int) (v []byte, err error) {
	if n < 0 {
		panic("buffered reader: negative view size")
	}

	if n == 0 {
		return nil, nil
	}

	// grow
	if n > len(b.buf) {
		if !b.grow(n) {
			return nil, ErrReadTooBig
		}
	}

	// fill at the end of the buffer
	for b.w-b.r < n && b.err == nil {
		b.fill() // copies unread part to the start, then fill til the end.
	}

	// can't populate the buffer
	if b.w-b.r < n {
		err = b.err

		switch err {
		case io.EOF:
			if b.w-b.r > 0 {
				err = io.ErrUnexpectedEOF
			}
		case nil:
			panic("buffered reader: (unexpected) less than n bytes available with no error")
		}

		return nil, err
	}

	v = b.buf[b.r : b.r+n]
	return
}

// Read returns a slice into the internal buffer for n next bytes.
// The returned slice is only valid until the next Read, ReadByte, or Reset call.
//
// Attempting to read more than maxSize fails with ErrReadTooBig.
func (b *BufferedReader) Read(n int) (v []byte, err error) {
	v, err = b.view(n)
	if err == nil {
		b.r += n
	}

	return
}

// ReadByte returns a next byte by value.
func (b *BufferedReader) ReadByte() (v byte, err error) {
	s, err := b.view(1)
	if err == nil {
		v = s[0]
		b.r += 1
	}
	return
}

// Peek returns a slice into the internal buffer for n next bytes.
// Peek doesn't consume any bytes.
//
// Attempting to peek more than maxSize fails with ErrReadTooBig.
func (b *BufferedReader) Peek(n int) (v []byte, err error) {
	v, err = b.view(n)
	return v, err
}

// Discard skips reading for n bytes, without growing the internal buffer.
func (b *BufferedReader) Discard(n int) (discarded int, err error) {
	if n < 0 {
		panic("buffered reader: negative discard size")
	}

	if n == 0 {
		return 0, nil
	}

	avail := b.w - b.r

	if avail >= n {
		b.r += n
		return n, nil
	}

	b.r = 0
	b.w = 0
	discarded = avail

	for discarded < n && b.err == nil {
		b.fill()

		if n-discarded <= b.w-b.r {
			b.r += n - discarded
			discarded = n
		} else {
			discarded += b.w - b.r
			b.r = 0
			b.w = 0
		}
	}

	if discarded < n {
		if b.err != nil {
			err = b.err
		} else {
			panic("buffered reader: (unexpected) less than n bytes available with no error")
		}
	}

	return
}

type Packet interface {
	ID() int32
	Encode(w io.Writer) error
	Decode(r Reader) error
}

// @gen:r,w
type HandshakePacket struct {
	ProtocolVersion int32  `field:"VarInt"`
	ServerAddr      string `field:"String"`
	ServerPort      uint16 `field:"UnsignedShort"`
	RequestType     int32  `field:"VarInt"`
}

func (p HandshakePacket) ID() int32 {
	return 0
}
