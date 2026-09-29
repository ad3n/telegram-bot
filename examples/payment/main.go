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

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
		bot.WithMessageTextHandler("/payment", bot.MatchTypeExact, handlerPaymentCommand),
	}

	b, _ := bot.New(os.Getenv("EXAMPLE_TELEGRAM_BOT_TOKEN"), opts...)

	b.Start(ctx)
}

func handlerPaymentCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	_, err := b.SendInvoice(ctx, &bot.SendInvoiceParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: 0,
		Title:           "Invoice Title",
		Description:     "Invoice description",
		Payload:         "xxx",
		ProviderToken:   os.Getenv("EXAMPLE_TELEGRAM_PAYMENT_TOKEN"),
		Currency:        "RUB",
		Prices: []models.LabeledPrice{
			{Label: "Price 1", Amount: 12425},
			{Label: "Price 2", Amount: 32454},
		},
	})
	if err != nil {
		fmt.Printf("error sending invoice: %s\n", err)
	}
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.PreCheckoutQuery != nil {
		fmt.Printf("get PreCheckoutQuery for invoce payload: %s\n", update.PreCheckoutQuery.InvoicePayload)

		if _, err := b.AnswerPreCheckoutQuery(ctx, &bot.AnswerPreCheckoutQueryParams{
			PreCheckoutQueryID: update.PreCheckoutQuery.ID,
			OK:                 true,
			ErrorMessage:       "",
		}); err != nil {
			log.Printf("AnswerPreCheckoutQuery: %v", err)
		}

		return
	}

	if update.Message != nil {
		if update.Message.SuccessfulPayment != nil {
			if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    update.Message.Chat.ID,
				Text:      fmt.Sprintf("Payment was successful with payment payload: *%s*", update.Message.SuccessfulPayment.InvoicePayload),
				ParseMode: models.ParseModeMarkdown,
			}); err != nil {
				log.Printf("SendMessage: %v", err)
			}

			return
		}
	}
}
