package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/ad3n/telegram-bot"
	"github.com/ad3n/telegram-bot/models"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(os.Getenv("EXAMPLE_TELEGRAM_BOT_TOKEN"), opts...)
	if err != nil {

		panic(err)
	}

	if _, err := b.SetWebhook(ctx, &bot.SetWebhookParams{
		URL: "https://example.com/webhook",
	}); err != nil {
		log.Printf("SetWebhook: %v", err)
	}

	go func() {
		if err := http.ListenAndServe(":2000", b.WebhookHandler()); err != nil {
			log.Printf("ListenAndServe: %v", err)
		}
	}()

	b.StartWebhook(ctx)

}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   update.Message.Text,
	}); err != nil {
		log.Printf("SendMessage: %v", err)
	}
}
