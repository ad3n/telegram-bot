package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/go-telegram/bot/models"
)

type retainedRequest struct {
	req  *http.Request
	body []byte
}

func TestRequestFormReset(t *testing.T) {
	for _, count := range []int{0, maxPooledFormParts, maxPooledFormParts + 1} {
		form := newRequestForm(multipart.NewWriter(io.Discard))
		for i := range count {
			key := fmt.Sprint(i)
			if i%2 == 0 {
				form.fileParts[key] = strings.NewReader("attachment")
				continue
			}

			form.fieldNames[key] = struct{}{}
		}

		if got, want := form.reset(), count <= maxPooledFormParts; got != want {
			t.Fatalf("%d parts: reusable=%v, want %v", count, got, want)
		}

		if form.w != nil || len(form.fileParts) != 0 || len(form.fieldNames) != 0 {
			t.Fatal("reset retains writer, reader, or field references")
		}

		if count > maxPooledFormParts && (form.fileParts != nil || form.fieldNames != nil) {
			t.Fatal("oversized maps retained")
		}
	}
}

func TestMediaBufferReset(t *testing.T) {
	for _, capacity := range []int{0, 1024, maxPooledMediaBufferCapacity, maxPooledMediaBufferCapacity + 1} {
		for _, consumed := range []int{0, capacity / 2, capacity} {
			storage := bytes.Repeat([]byte{'x'}, capacity)
			buf := bytes.NewBuffer(storage)
			buf.Next(consumed)
			if got, want := resetMediaBuffer(buf), capacity <= maxPooledMediaBufferCapacity; got != want {
				t.Fatalf("capacity %d: reusable=%v, want %v", capacity, got, want)
			}

			if buf.Len() != 0 {
				t.Fatal("reset retained readable data")
			}

			if capacity > maxPooledMediaBufferCapacity {
				if buf.Cap() != 0 {
					t.Fatal("oversized buffer retained")
				}

				continue
			}

			for _, b := range storage {
				if b != 0 {
					t.Fatal("consumed buffer still contains previous request data")
				}
			}
		}
	}
}

func TestPooledFormsConcurrentIsolation(t *testing.T) {
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Go(func() {
			for iteration := range 100 {
				payload := fmt.Sprintf("worker-%d-request-%d", worker, iteration)
				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				attachment := strings.NewReader(payload)
				params := &SendMediaGroupParams{ChatID: int64(worker + 1), Media: []models.InputMedia{
					&models.InputMediaPhoto{Media: "attach://photo", Caption: payload, MediaAttachment: attachment},
					&models.InputMediaPhoto{Media: "attach://photo", Caption: payload, MediaAttachment: attachment},
				}}
				if _, err := buildRequestForm(writer, params); err != nil {
					t.Error(err)
					return
				}

				if err := writer.Close(); err != nil {
					t.Error(err)
					return
				}

				reader := multipart.NewReader(&body, writer.Boundary())
				parts := map[string][]byte{}
				for {
					part, err := reader.NextPart()
					if err == io.EOF {
						break
					}

					if err != nil {
						t.Error(err)
						return
					}

					content, err := io.ReadAll(part)
					if err != nil {
						t.Error(err)
						return
					}

					if _, duplicate := parts[part.FormName()]; duplicate {
						t.Error("duplicate attachment was uploaded twice")
						return
					}

					parts[part.FormName()] = content
				}

				if string(parts["photo"]) != payload || string(parts["chat_id"]) != fmt.Sprint(worker+1) {
					t.Error("cross-request form data contamination")
					return
				}

				var media []struct {
					Type    string `json:"type"`
					Media   string `json:"media"`
					Caption string `json:"caption"`
				}
				if err := json.Unmarshal(parts["media"], &media); err != nil {
					t.Error(err)
					return
				}

				if len(media) != 2 {
					t.Error("media array changed")
					return
				}

				for _, item := range media {
					if item.Type != "photo" || item.Media != "attach://photo" || item.Caption != payload {
						t.Error("pooled media data changed")
						return
					}
				}
			}
		})
	}

	wg.Wait()
}

func TestPooledFormFailureDoesNotContaminateNextRequest(t *testing.T) {
	for range 20 {
		writer := multipart.NewWriter(io.Discard)
		bad := &SendMediaGroupParams{ChatID: int64(1), Media: []models.InputMedia{
			&models.InputMediaPhoto{Media: "attach://photo", MediaAttachment: strings.NewReader("partial")},
			nil,
		}}
		if _, err := buildRequestForm(writer, bad); err == nil {
			t.Fatal("expected nil media error")
		}

		writer = multipart.NewWriter(io.Discard)
		good := &SendMediaGroupParams{ChatID: int64(2), Media: []models.InputMedia{
			&models.InputMediaPhoto{Media: "attach://photo", MediaAttachment: strings.NewReader("valid")},
		}}
		if _, err := buildRequestForm(writer, good); err != nil {
			t.Fatalf("failed form contaminated later request: %v", err)
		}

		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRequestReplaySurvivesPoolReuse(t *testing.T) {
	client := &clientMock{}
	bot := &Bot{token: "test", client: client}
	var retained []retainedRequest
	for i := range 32 {
		caption := fmt.Sprintf("request-%d", i)
		if err := bot.rawRequest(context.Background(), "sendMediaGroup", &SendMediaGroupParams{
			ChatID: int64(i + 1),
			Media:  []models.InputMedia{&models.InputMediaPhoto{Media: "file-id", Caption: caption}},
		}, nil); err != nil {
			t.Fatal(err)
		}

		req := client.gotReq
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}

		if err := req.Body.Close(); err != nil {
			t.Fatal(err)
		}

		retained = append(retained, retainedRequest{req: req, body: body})
	}

	runtime.GC()
	for _, saved := range retained {
		for range 2 {
			body, err := saved.req.GetBody()
			if err != nil {
				t.Fatal(err)
			}

			replay, err := io.ReadAll(body)
			if err != nil {
				t.Fatal(err)
			}

			if err := body.Close(); err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(replay, saved.body) || int64(len(replay)) != saved.req.ContentLength {
				t.Fatal("replay corrupted by pool reuse")
			}
		}
	}
}
