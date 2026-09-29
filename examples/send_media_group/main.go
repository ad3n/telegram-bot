package main

import (
	"bytes"
	"context"
	"embed"
	"log"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var (
	images embed.FS
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
	fileDataFacebook, _ := images.ReadFile("images/facebook.png")
	fileDataYoutube, _ := images.ReadFile("images/youtube.png")

	media1 := &models.InputMediaPhoto{
		Media:   "https://telegram.org/img/t_logo.png",
		Caption: "Telegram Logo",
	}

	media2 := &models.InputMediaPhoto{
		Media:           "attach://facebook.png",
		Caption:         "Facebook Logo",
		MediaAttachment: bytes.NewReader(fileDataFacebook),
	}

	media3 := &models.InputMediaPhoto{
		Media:           "attach://youtube.png",
		Caption:         "Youtube Logo",
		MediaAttachment: bytes.NewReader(fileDataYoutube),
	}

	params := &bot.SendMediaGroupParams{
		ChatID: update.Message.Chat.ID,
		Media: []models.InputMedia{
			media1,
			media2,
			media3,
		},
	}

	if _, err := b.SendMediaGroup(ctx, params); err != nil {
		log.Printf("SendMediaGroup: %v", err)
	}
}
