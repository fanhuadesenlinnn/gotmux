package model

type Ring struct {
	buf []byte
	max int
}

func NewRing(max int) *Ring {
	return &Ring{max: max}
}

func (r *Ring) Write(p []byte) {
	if r.max <= 0 || len(p) == 0 {
		return
	}
	if len(p) >= r.max {
		r.buf = append(r.buf[:0], p[len(p)-r.max:]...)
		return
	}
	r.buf = append(r.buf, p...)
	if extra := len(r.buf) - r.max; extra > 0 {
		copy(r.buf, r.buf[extra:])
		r.buf = r.buf[:len(r.buf)-extra]
	}
}

func (r *Ring) Reset() {
	r.buf = r.buf[:0]
}

func (r *Ring) Bytes() []byte {
	out := make([]byte, len(r.buf))
	copy(out, r.buf)
	return out
}
