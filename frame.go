package mcproto

import (
	"errors"
	"io"

	"github.com/gstoney/mcproto/packet"
)

var (
	ErrNotExhausted        = errors.New("not exhausted")
	ErrZlibPayloadOverrun  = errors.New("zlib stream exceeds declared payload length")
	ErrZlibPayloadUnderrun = errors.New("zlib stream shorter than declared payload length")
	ErrZlibTrailingData    = errors.New("trailing data in frame after zlib stream ends")
)

// FrameReader wraps a source reader to provide bounded access to one frame at a time.
// It ensures packet frame alignment.
type FrameReader struct {
	src       io.Reader
	remaining int32
}

func (f *FrameReader) Read(p []byte) (n int, err error) {
	if f.remaining <= 0 {
		return 0, io.EOF
	}
	if int32(len(p)) > f.remaining {
		p = p[0:f.remaining]
	}
	n, err = f.src.Read(p)
	f.remaining -= int32(n)

	if err == io.EOF && f.remaining > 0 {
		err = io.ErrUnexpectedEOF
	}
	return
}

func (f *FrameReader) ReadByte() (byte, error) {
	if br, ok := f.src.(io.ByteReader); ok {
		if f.remaining <= 0 {
			return 0, io.EOF
		}
		v, err := br.ReadByte()
		if err == nil {
			f.remaining -= 1
		} else if err == io.EOF && f.remaining > 0 {
			err = io.ErrUnexpectedEOF
		}
		return v, err
	}

	var b [1]byte
	_, err := f.Read(b[:])

	return b[0], err
}

func (f *FrameReader) Next() (length int32, err error) {
	if f.remaining > 0 {
		return f.remaining, ErrNotExhausted
	}

	length, err = packet.ReadVarIntFromReader(f.src)
	if err == nil {
		if length > 0 {
			f.remaining = length
		} else {
			err = errors.New("invalid frame length")
		}
	}
	return
}

func (f *FrameReader) Skip() (n int32, err error) {
	n64, err := io.CopyN(io.Discard, f, int64(f.remaining))
	n = int32(n64)
	return
}

func (f *FrameReader) Remaining() int32 {
	return f.remaining
}

// PayloadReader provides access to a single packet's payload.
//
// Read returns payload bytes. Remaining reports unread payload bytes.
//
// Skip discards remaining payload bytes, enabling validation on Close.
//
// Close validates payload exhaustion and frame integrity, returning an error
// if the payload was not fully consumed or the frame is malformed.
// Close does not realign on error.
//
// Discard abandons the current frame and realigns to the next frame boundary.
// Use Discard to recover from malformed frames or when validation is not needed.
type PayloadReader interface {
	io.ReadCloser
	Skip() (n int32, err error)
	Discard() (n int32, err error)
	Remaining() int32
}

type plainPayload struct {
	*FrameReader
}

func (p plainPayload) Close() (err error) {
	if p.remaining > 0 {
		err = ErrNotExhausted
	}
	return
}

func (p plainPayload) Discard() (n int32, err error) {
	return p.Skip()
}

type compressedPayload struct {
	zr        io.ReadCloser
	fr        *FrameReader
	remaining int32
}

func (p *compressedPayload) Read(b []byte) (n int, err error) {
	if p.remaining <= 0 {
		return 0, io.EOF
	}
	if int32(len(b)) > p.remaining {
		b = b[0:p.remaining]
	}
	n, err = p.zr.Read(b)
	p.remaining -= int32(n)

	if err != nil {
		if err == io.EOF && p.remaining > 0 {
			err = ErrZlibPayloadUnderrun
		}
	}
	return
}

func (p *compressedPayload) Skip() (n int32, err error) {
	n64, err := io.CopyN(io.Discard, p, int64(p.remaining))
	n = int32(n64)
	return
}

func (p *compressedPayload) Discard() (n int32, err error) {
	p.remaining = 0
	return p.fr.Skip()
}

func (p *compressedPayload) Close() (err error) {
	if p.remaining > 0 {
		return ErrNotExhausted
	}

	var buf [1]byte
	n, err := p.zr.Read(buf[:])
	if err == nil || n > 0 {
		return ErrZlibPayloadOverrun
	} else if err != io.EOF {
		return err
	}

	if p.fr.remaining > 0 {
		return ErrZlibTrailingData
	}
	return p.zr.Close()
}

func (p *compressedPayload) Remaining() int32 {
	return p.remaining
}
