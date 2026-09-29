package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/ad3n/telegram-bot"
	"github.com/ad3n/telegram-bot/models"
)

var (
	currentOptions = []bool{false, false, false}
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithMessageTextHandler("/select", bot.MatchTypeExact, commandHandler),
		bot.WithCallbackQueryDataHandler("btn_", bot.MatchTypePrefix, callbackHandler),
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

	switch update.CallbackQuery.Data {
	case "btn_opt1":
		currentOptions[0] = !currentOptions[0]
	case "btn_opt2":
		currentOptions[1] = !currentOptions[1]
	case "btn_opt3":
		currentOptions[2] = !currentOptions[2]
	case "btn_select":
		if _, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
		}); err != nil {
			log.Printf("DeleteMessage: %v", err)
		}

		if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   fmt.Sprintf("Selected options: %v", currentOptions),
		}); err != nil {
			log.Printf("SendMessage: %v", err)
		}

		return
	}

	if _, err := b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		ReplyMarkup: buildKeyboard(),
	}); err != nil {
		log.Printf("EditMessageReplyMarkup: %v", err)
	}
}

func buildKeyboard() models.ReplyMarkup {
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: buttonText("Option 1", currentOptions[0]), CallbackData: "btn_opt1"},
				{Text: buttonText("Option 1", currentOptions[1]), CallbackData: "btn_opt2"},
				{Text: buttonText("Option 1", currentOptions[2]), CallbackData: "btn_opt3"},
			}, {
				{Text: "Select", CallbackData: "btn_select"},
			},
		},
	}

	return kb
}

func buttonText(text string, opt bool) string {
	if opt {
		return "✅ " + text
	}

	return "❌ " + text
}

func commandHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "Select multiple options",
		ReplyMarkup: buildKeyboard(),
	}); err != nil {
		log.Printf("SendMessage: %v", err)
	}
}
