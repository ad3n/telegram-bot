package main

import (
	"bytes"
	"context"
	"fmt"
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
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(os.Getenv("EXAMPLE_TELEGRAM_BOT_TOKEN"), opts...)
	if err != nil {

		panic(err)
	}

	b.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	fileData, errReadFile := os.ReadFile("./examples/send_photo_upload/facebook.png")
	if errReadFile != nil {
		fmt.Printf("error read file, %v\n", errReadFile)
		return
	}

	params := &bot.SendPhotoParams{
		ChatID:  update.Message.Chat.ID,
		Photo:   &models.InputFileUpload{Filename: "facebook.png", Data: bytes.NewReader(fileData)},
		Caption: "New uploaded Facebook logo",
	}

	if _, err := b.SendPhoto(ctx, params); err != nil {
		log.Printf("SendPhoto: %v", err)
	}
}
