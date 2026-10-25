package bot

import (
	"bytes"
	"io"
	"mime/multipart"
	"reflect"
	"testing"

	"github.com/ad3n/telegram-bot/models"
)

func TestRequestFormCachedMetadataUsesCurrentValues(t *testing.T) {
	params := &struct {
		Value any    `json:"value,omitempty"`
		Text  string `json:"text,omitempty"`
		Skip  string `json:"-"`
	}{Skip: "ignored"}

	for _, test := range []struct {
		value any
		text  string
		want  []string
	}{
		{nil, "", nil},
		{"first", "hello", []string{"value=first", "text=hello"}},
		{&models.InputMediaPhoto{Media: "photo"}, "", []string{`value={"type":"photo","media":"photo"}`}},
		{int64(42), "world", []string{"value=42", "text=world"}},
		{nil, "", nil},
	} {
		params.Value = test.value
		params.Text = test.text
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		count, err := buildRequestForm(writer, params)
		if err != nil {
			t.Fatal(err)
		}

		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}

		reader := multipart.NewReader(&buf, writer.Boundary())
		var got []string
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}

			if err != nil {
				t.Fatal(err)
			}

			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatal(err)
			}

			got = append(got, part.FormName()+"="+string(data))
		}

		if count != len(test.want) || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("count=%d fields=%q, want %q", count, got, test.want)
		}
	}
}

func TestRequestFormMetadataNoAllocation(t *testing.T) {
	paramType := reflect.TypeFor[SendMessageParams]()
	loadRequestFormFields(paramType)
	if allocations := testing.AllocsPerRun(100, func() {
		loadRequestFormFields(paramType)
	}); allocations != 0 {
		t.Fatalf("cached metadata: %v allocations", allocations)
	}
}
