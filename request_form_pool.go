package bot

import (
	"bytes"
	"io"
	"sync"
)

const (
	maxPooledFormParts           = 128
	maxPooledMediaBufferCapacity = 64 << 10
)

var (
	requestFormPool = sync.Pool{New: func() any {
		return &requestForm{
			fileParts:  map[string]io.Reader{},
			fieldNames: map[string]struct{}{},
		}
	}}
	mediaBufferPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}
)

func acquireRequestForm(w formWriter) *requestForm {
	form := requestFormPool.Get().(*requestForm)
	form.w = w
	return form
}

func releaseRequestForm(form *requestForm) {
	if form.reset() {
		requestFormPool.Put(form)
	}
}

func (f *requestForm) reset() bool {
	f.w = nil
	f.stringReader.Reset("")
	clear(f.scalarBuffer[:])
	if len(f.fileParts)+len(f.fieldNames) > maxPooledFormParts {
		f.fileParts = nil
		f.fieldNames = nil
		return false
	}

	clear(f.fileParts)
	clear(f.fieldNames)
	return true
}

func acquireMediaBuffer() *bytes.Buffer {
	return mediaBufferPool.Get().(*bytes.Buffer)
}

func releaseMediaBuffer(buf *bytes.Buffer) {
	if resetMediaBuffer(buf) {
		mediaBufferPool.Put(buf)
	}
}

func resetMediaBuffer(buf *bytes.Buffer) bool {
	if buf.Cap() > maxPooledMediaBufferCapacity {
		*buf = bytes.Buffer{}
		return false
	}

	buf.Reset()
	clear(buf.AvailableBuffer()[:buf.Cap()])
	return true
}
