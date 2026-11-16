package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"sync"

	"github.com/ad3n/telegram-bot/models"
)

type (
	webhookReplyKey struct{}

	webhookReply struct {
		mx          sync.Mutex
		closed      bool
		body        []byte
		contentType string
	}
)

func (b *Bot) WebhookReplyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if b.webhookSecretToken != "" && req.Header.Get("X-Telegram-Bot-Api-Secret-Token") != b.webhookSecretToken {
			b.error("invalid webhook secret token received from update")
			w.WriteHeader(http.StatusUnauthorized)
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
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if b.isDebug {
			b.debugHandler("webhook request '%s'", body)
		}

		reply := &webhookReply{}
		ctx := context.WithValue(req.Context(), webhookReplyKey{}, reply)

		h := applyMiddlewares(b.findHandler(update), b.middlewares...)
		h(ctx, b, update)

		reply.mx.Lock()
		defer reply.mx.Unlock()

		reply.closed = true

		if reply.body == nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("Content-Type", reply.contentType)
		w.WriteHeader(http.StatusOK)
		if _, errWrite := w.Write(reply.body); errWrite != nil {
			b.error("error write webhook reply, %w", errWrite)
		}
	}
}

func (b *Bot) WebhookReply(ctx context.Context, method string, params any) error {
	reply, ok := ctx.Value(webhookReplyKey{}).(*webhookReply)
	if !ok {
		return fmt.Errorf("%w, update not handled by WebhookReplyHandler", ErrorWebhookReplyUnavailable)
	}

	reply.mx.Lock()
	defer reply.mx.Unlock()

	if reply.closed {
		return fmt.Errorf("%w, webhook request already answered", ErrorWebhookReplyUnavailable)
	}

	if reply.body != nil {
		return fmt.Errorf("%w, update already replied to", ErrorWebhookReplyUnavailable)
	}

	if params != nil {
		v := reflect.ValueOf(params)
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				params = nil
			}

			if !v.IsNil() && v.Elem().Kind() != reflect.Struct {
				return fmt.Errorf("unsupported params type %T, expected nil or pointer to struct", params)
			}
		default:
			return fmt.Errorf("unsupported params type %T, expected nil or pointer to struct", params)
		}
	}

	var bodyBuf bytes.Buffer
	form := multipart.NewWriter(&bodyBuf)

	if errMethod := form.WriteField("method", method); errMethod != nil {
		return fmt.Errorf("error build webhook reply for method %s, %w", method, errMethod)
	}

	if params != nil {
		if _, errFormData := buildRequestForm(form, params); errFormData != nil {
			return fmt.Errorf("error build webhook reply for method %s, %w", method, errFormData)
		}
	}

	if errFormClose := form.Close(); errFormClose != nil {
		return fmt.Errorf("error form close for method %s, %w", method, errFormClose)
	}

	if b.isDebug {
		replyDebugData, _ := json.Marshal(params)
		b.debugHandler("webhook reply method: %s, payload: %s", method, replyDebugData)
	}

	reply.body = bodyBuf.Bytes()
	reply.contentType = form.FormDataContentType()

	return nil
}
