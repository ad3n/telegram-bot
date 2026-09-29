package bot

import (
	"context"
	"io"
	"mime/multipart"
	"testing"

	"github.com/go-telegram/bot/models"
	"regexp"
)

var (
	benchmarkString string

	benchmarkMatch bool
)

func BenchmarkHandlerRegexp(b *testing.B) {
	h := handler{handlerType: HandlerTypeMessageText, matchType: matchTypeRegexp, re: regexp.MustCompile(`^/start`)}
	upd := &models.Update{Message: &models.Message{Text: "/start hello world"}}
	b.ReportAllocs()
	for b.Loop() {
		benchmarkMatch = h.match(upd)
	}
}

func BenchmarkEscapeMarkdown(b *testing.B) {
	for _, s := range []string{"hello world telegram bot", "hello *world* telegram_bot!", "こんにちは世界🙂", `already \*escaped\*`, "\xff*\xfe"} {
		b.Run(s, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchmarkString = EscapeMarkdown(s)
			}
		})
	}
}

func BenchmarkRequestForm(b *testing.B) {
	for name, params := range map[string]any{
		"message": &SendMessageParams{ChatID: int64(123), Text: "hello world"},
		"media": &SendMediaGroupParams{ChatID: int64(123), Media: []models.InputMedia{
			&models.InputMediaPhoto{Media: "first", Caption: "one"},
			&models.InputMediaPhoto{Media: "second", Caption: "two"},
			&models.InputMediaPhoto{Media: "third", Caption: "three"},
		}},
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				w := multipart.NewWriter(io.Discard)
				if _, err := buildRequestForm(w, params); err != nil {
					b.Fatal(err)
				}

				if err := w.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkProcessUpdate(b *testing.B) {
	bot := &Bot{defaultHandlerFunc: func(context.Context, *Bot, *models.Update) {}, notAsyncHandlers: true}
	upd := &models.Update{}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		bot.ProcessUpdate(ctx, upd)
	}
}

func TestEscapeMarkdownNoAllocation(t *testing.T) {
	for _, s := range []string{"hello world", "こんにちは🙂", ""} {
		if n := testing.AllocsPerRun(100, func() { benchmarkString = EscapeMarkdown(s) }); n != 0 {
			t.Fatalf("EscapeMarkdown(%q): %v allocations", s, n)
		}

		if n := testing.AllocsPerRun(100, func() { benchmarkString = EscapeMarkdownUnescaped(s) }); n != 0 {
			t.Fatalf("EscapeMarkdownUnescaped(%q): %v allocations", s, n)
		}
	}
}

func TestRegexHandlerNoAllocation(t *testing.T) {
	h := handler{handlerType: HandlerTypeMessageText, matchType: matchTypeRegexp, re: regexp.MustCompile(`^/start`)}
	upd := &models.Update{Message: &models.Message{Text: "/start hello"}}
	if n := testing.AllocsPerRun(100, func() { benchmarkMatch = h.match(upd) }); n != 0 {
		t.Fatalf("regex handler: %v allocations", n)
	}
}
