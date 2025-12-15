package packet

import "io"

type FrameReader struct {
	buf []byte
	off int
}

func NewFrameReader(buf []byte) FrameReader {
	return FrameReader{
		buf: buf,
		off: 0,
	}
}

func (r FrameReader) Remaining() int {
	return len(r.buf) - r.off
}

func (r *FrameReader) ReadByte() (byte, error) {
	if r.off >= len(r.buf) {
		return 0, io.ErrUnexpectedEOF
	}
	b := r.buf[r.off]
	r.off++
	return b, nil
}

func (r *FrameReader) Read(n int) ([]byte, error) {
	if r.off+n > len(r.buf) {
		return nil, io.ErrUnexpectedEOF
	}
	b := r.buf[r.off : r.off+n]
	r.off += n
	return b, nil
}
