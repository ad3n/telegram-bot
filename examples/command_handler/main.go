package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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

	b.RegisterHandler(bot.HandlerTypeMessageText, "foo", bot.MatchTypeCommand, fooHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "bar", bot.MatchTypeCommandStartOnly, barHandler)

	b.Start(ctx)
}

func fooHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      "Caught *foo*",
		ParseMode: models.ParseModeMarkdown,
	}); err != nil {
		log.Printf("SendMessage: %v", err)
	}
}

func barHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      "Caught *bar*",
		ParseMode: models.ParseModeMarkdown,
	}); err != nil {
		log.Printf("SendMessage: %v", err)
	}
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      "Say message with `/foo` anywhere or with `/bar` at start of the message",
		ParseMode: models.ParseModeMarkdown,
	}); err != nil {
		log.Printf("SendMessage: %v", err)
	}
}
