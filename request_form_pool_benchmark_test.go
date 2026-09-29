package bot

import (
	"io"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/ad3n/telegram-bot/models"
)

func BenchmarkRequestFormParallel(b *testing.B) {
	for name, params := range map[string]any{
		"message": &SendMessageParams{ChatID: int64(123), Text: "hello world"},
		"media": &SendMediaGroupParams{ChatID: int64(123), Media: []models.InputMedia{
			&models.InputMediaPhoto{Media: "first", Caption: "one"},
			&models.InputMediaPhoto{Media: "second", Caption: "two"},
			&models.InputMediaPhoto{Media: "third", Caption: "three"},
		}},
		"large-media": &SendMediaGroupParams{ChatID: int64(123), Media: []models.InputMedia{
			&models.InputMediaPhoto{Media: "first", Caption: strings.Repeat("x", 70<<10)},
		}},
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					writer := multipart.NewWriter(io.Discard)
					if _, err := buildRequestForm(writer, params); err != nil {
						b.Error(err)
						return
					}

					if err := writer.Close(); err != nil {
						b.Error(err)
						return
					}
				}
			})
		})
	}
}
