package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/ad3n/telegram-bot"
	"github.com/ad3n/telegram-bot/models"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
	}

	b, err := bot.New(os.Getenv("EXAMPLE_TELEGRAM_BOT_TOKEN"), opts...)
	if err != nil {

		panic(err)
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/hello", bot.MatchTypeExact, helloHandler)

	b.Start(ctx)
}

func helloHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      "Hello, *" + bot.EscapeMarkdown(update.Message.From.FirstName) + "*",
		ParseMode: models.ParseModeMarkdown,
	}); err != nil {
		log.Printf("SendMessage: %v", err)
	}
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Say /hello",
	}); err != nil {
		log.Printf("SendMessage: %v", err)
	}
}
