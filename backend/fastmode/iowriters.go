package fastmode

import (
	"io"
	"sync"
)

// ringBuffer is a fixed-capacity, thread-safe io.Writer that retains only the
// most recent bytes written to it. winws.exe output is streamed through one so
// that, if the engine dies on start-up, the controller can read the tail and
// explain why (driver load error, access denied, ...) without unbounded memory.
type ringBuffer struct {
	mu   sync.Mutex
	buf  []byte
	size int
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{size: size, buf: make([]byte, 0, size)}
}

func (r *ringBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, p...)
	if len(r.buf) > r.size {
		r.buf = r.buf[len(r.buf)-r.size:]
	}
	return len(p), nil
}

func (r *ringBuffer) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}

// teeWriter fans one stream out to two writers. It is best-effort: a failure
// on either sink is swallowed so it never stops winws output from being
// pumped (a full/rotating log must not kill process supervision).
type teeWriter struct {
	a, b io.Writer
}

func (t *teeWriter) Write(p []byte) (int, error) {
	if t.a != nil {
		_, _ = t.a.Write(p)
	}
	if t.b != nil {
		_, _ = t.b.Write(p)
	}
	return len(p), nil
}
