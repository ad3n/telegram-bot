package main

import (
	"context"
	"net/http"
	"os"

	"github.com/ad3n/telegram-bot"
	"github.com/ad3n/telegram-bot/models"
)

func main() {
	b, err := bot.New(os.Getenv("EXAMPLE_TELEGRAM_BOT_TOKEN"))
	if nil != err {
		panic(err)
	}

	b.SetWebhook(context.Background(), &bot.SetWebhookParams{
		URL: "https://example.com/webhook",
	})

	b.RegisterHandlerInlineQuery(handler)

	http.ListenAndServe(":2000", b.WebhookReplyHandler())

}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	results := []models.InlineQueryResult{
		&models.InlineQueryResultArticle{ID: "1", Title: "Foo 1", InputMessageContent: &models.InputTextMessageContent{MessageText: "foo 1"}},
		&models.InlineQueryResultArticle{ID: "2", Title: "Foo 2", InputMessageContent: &models.InputTextMessageContent{MessageText: "foo 2"}},
		&models.InlineQueryResultArticle{ID: "3", Title: "Foo 3", InputMessageContent: &models.InputTextMessageContent{MessageText: "foo 3"}},
	}

	b.WebhookReply(ctx, "answerInlineQuery", &bot.AnswerInlineQueryParams{
		InlineQueryID: update.InlineQuery.ID,
		Results:       results,
	})
}
