package main

import (
	"bytes"
	"context"
	"fmt"
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

	b.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	fileData, errReadFile := os.ReadFile("./examples/send_video/examle_video.mp4")
	if errReadFile != nil {
		fmt.Printf("error read file, %v\n", errReadFile)
		return
	}

	params := &bot.SendVideoParams{
		ChatID:  update.Message.Chat.ID,
		Video:   &models.InputFileUpload{Filename: "examle_video.mp4", Data: bytes.NewReader(fileData)},
		Caption: "New uploaded example video",
	}

	if _, err := b.SendVideo(ctx, params); err != nil {
		fmt.Printf("error send video, %v\n", err)
	}
}
