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
		bot.WithDebug(),
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(os.Getenv("EXAMPLE_TELEGRAM_BOT_TOKEN"), opts...)
	if err != nil {

		panic(err)
	}

	b.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	params := &bot.SendPhotoParams{
		ChatID:  update.Message.Chat.ID,
		Photo:   &models.InputFileString{Data: "AgACAgIAAxkDAAIBOWJimnCJHQJiJ4P3aasQCPNyo6mlAALDuzEbcD0YSxzjB-vmkZ6BAQADAgADbQADJAQ"},
		Caption: "Preloaded Facebook logo",
	}

	if _, err := b.SendPhoto(ctx, params); err != nil {
		log.Printf("SendPhoto: %v", err)
	}
}
