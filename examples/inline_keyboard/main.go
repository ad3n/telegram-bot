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
		bot.WithCallbackQueryDataHandler("button", bot.MatchTypePrefix, callbackHandler),
	}

	b, err := bot.New(os.Getenv("EXAMPLE_TELEGRAM_BOT_TOKEN"), opts...)
	if err != nil {

		panic(err)
	}

	b.Start(ctx)
}

func callbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {

	if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	}); err != nil {
		log.Printf("AnswerCallbackQuery: %v", err)
	}

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.CallbackQuery.Message.Message.Chat.ID,
		Text:   "You selected the button: " + update.CallbackQuery.Data,
	}); err != nil {
		log.Printf("SendMessage: %v", err)
	}
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Button 1", CallbackData: "button_1"},
				{Text: "Button 2", CallbackData: "button_2"},
			}, {
				{Text: "Button 3", CallbackData: "button_3"},
			},
		},
	}

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "Click by button",
		ReplyMarkup: kb,
	}); err != nil {
		log.Printf("SendMessage: %v", err)
	}
}
