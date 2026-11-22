package bot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/ad3n/telegram-bot/models"
)

func (b *Bot) WebhookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if b.webhookSecretToken != "" && req.Header.Get("X-Telegram-Bot-Api-Secret-Token") != b.webhookSecretToken {
			b.error("invalid webhook secret token received from update")
			return
		}

		body, errReadBody := io.ReadAll(req.Body)
		if errReadBody != nil {
			b.error("error read request body, %w", errReadBody)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		update := &models.Update{}

		errDecode := json.Unmarshal(body, update)
		if errDecode != nil {
			b.error("error decode request body, %s, %w", body, errDecode)
			return
		}

		if b.isDebug {
			b.debugHandler("webhook request '%s'", body)
		}

		if !b.queueWebhookUpdate(req.Context(), update) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}
}

func (b *Bot) queueWebhookUpdate(ctx context.Context, update *models.Update) bool {
	b.updatesMx.Lock()
	run := b.updatesRun
	b.updatesMx.Unlock()

	if run == nil {
		b.error("webhook update rejected: StartWebhook is not running")
		return false
	}

	if !run.webhook || run.ctx.Err() != nil || ctx.Err() != nil {
		return false
	}

	dispatched := make(chan struct{})
	requestDone := ctx.Done()
	select {
	case run.queue <- queuedUpdate{update: update, dispatched: dispatched, requestDone: requestDone}:
	case <-requestDone:
		return false
	case <-run.ctx.Done():
		return false
	}

	select {
	case <-dispatched:
		return true
	case <-requestDone:
		return false
	case <-run.ctx.Done():
		return false
	}
}
