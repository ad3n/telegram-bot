package bot

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"testing"
)

type (
	scalarFormWriter struct {
		writer io.Writer
		err    error
	}

	shortFieldWriter struct{}

	customScalar int64
)

func (w scalarFormWriter) CreateFormField(string) (io.Writer, error) {
	return w.writer, w.err
}

func (w scalarFormWriter) CreateFormFile(string, string) (io.Writer, error) {
	return w.writer, w.err
}

func (shortFieldWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (customScalar) MarshalJSON() ([]byte, error) {
	return []byte(`"custom"`), nil
}

func TestFormScalarJSONCompatibility(t *testing.T) {
	values := []any{
		int(-1), int8(-128), int16(-32768), int32(math.MinInt32), int64(math.MinInt64),
		int64(math.MaxInt64), uint(1), uint8(255), uint16(65535), uint32(math.MaxUint32),
		uint64(math.MaxUint64), uintptr(123), true, false, int64(0), uint64(0),
		float64(1.234), float32(1.5), json.Number("123.5"), customScalar(42), nil,
	}
	for _, value := range values {
		var dst bytes.Buffer
		form := newRequestForm(scalarFormWriter{writer: &dst})
		if err := addFormFieldDefault(form, "value", value); err != nil {
			t.Fatalf("%T: %v", value, err)
		}

		want, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}

		want = bytes.Trim(want, "\"")
		if !bytes.Equal(dst.Bytes(), want) {
			t.Fatalf("%T: got %q, want %q", value, dst.Bytes(), want)
		}
	}

	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		form := newRequestForm(scalarFormWriter{writer: io.Discard})
		if err := addFormFieldDefault(form, "value", value); err == nil {
			t.Fatal("non-finite float accepted")
		}
	}
}

func TestFormFieldWriteErrors(t *testing.T) {
	failure := errors.New("field creation failed")
	for _, tc := range []struct {
		writer scalarFormWriter
		want   error
	}{
		{scalarFormWriter{writer: shortFieldWriter{}}, io.ErrShortWrite},
		{scalarFormWriter{err: failure}, failure},
	} {
		for _, write := range []func(*requestForm) error{
			func(f *requestForm) error { return addFormFieldDefault(f, "value", int64(42)) },
			func(f *requestForm) error { return addFormFieldString(f, "value", "secret") },
			func(f *requestForm) error { return addFormFieldDefault(f, "value", []int{1, 2}) },
		} {
			form := newRequestForm(tc.writer)
			if err := write(form); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}

			if form.stringReader.Size() != 0 {
				t.Fatal("failed write retained source string")
			}
		}
	}
}

func TestFormScalarNoAllocation(t *testing.T) {
	form := newRequestForm(scalarFormWriter{writer: io.Discard})
	for _, value := range []any{int64(math.MinInt64), uint64(math.MaxUint64), true} {
		allocs := testing.AllocsPerRun(100, func() {
			clear(form.fieldNames)
			if err := addFormFieldDefault(form, "value", value); err != nil {
				panic(err)
			}
		})
		if allocs != 0 {
			t.Fatalf("%T: %v allocations", value, allocs)
		}
	}
}

func TestFormScratchReset(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		form := newRequestForm(nil)
		form.stringReader.Reset("sensitive")
		copy(form.scalarBuffer[:], "123456789")
		if oversized {
			for i := range maxPooledFormParts + 1 {
				form.fieldNames[string(rune(i))] = struct{}{}
			}
		}

		form.reset()
		if form.stringReader.Size() != 0 || form.scalarBuffer != [20]byte{} {
			t.Fatal("reset retained scratch data")
		}
	}
}

func BenchmarkFormScalar(b *testing.B) {
	form := newRequestForm(scalarFormWriter{writer: io.Discard})
	var value any = int64(math.MinInt64)
	b.ReportAllocs()
	for b.Loop() {
		clear(form.fieldNames)
		if err := addFormFieldDefault(form, "value", value); err != nil {
			b.Fatal(err)
		}
	}
}
