package main

import (
	"bytes"
	"context"
	"fmt"
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
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(os.Getenv("EXAMPLE_TELEGRAM_BOT_TOKEN"), opts...)
	if err != nil {

		panic(err)
	}

	b.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	fileData, errReadFile := os.ReadFile("./examples/send_document/demo.txt")
	if errReadFile != nil {
		fmt.Printf("error read file, %v\n", errReadFile)
		return
	}

	params := &bot.SendDocumentParams{
		ChatID:   update.Message.Chat.ID,
		Document: &models.InputFileUpload{Filename: "demo.txt", Data: bytes.NewReader(fileData)},
		Caption:  "Document",
	}

	if _, err := b.SendDocument(ctx, params); err != nil {
		log.Printf("SendDocument: %v", err)
	}
}
