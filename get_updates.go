package bot

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ad3n/telegram-bot/models"
)

const (
	maxTimeoutAfterError = time.Second * 5
)

var (
	errMissingUpdateID = errors.New("missing update_id")
)

type (
	getUpdatesParams struct {
		Offset         int64          `json:"offset,omitempty"`
		Limit          int            `json:"limit,omitempty"`
		Timeout        int            `json:"timeout,omitempty"`
		AllowedUpdates AllowedUpdates `json:"allowed_updates,omitempty"`
	}

	AllowedUpdates []string
)

func (b *Bot) getUpdates(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	var timeoutAfterError time.Duration

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if timeoutAfterError > 0 {
			if b.isDebug {
				b.debugHandler("wait after error, %v", timeoutAfterError)
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(timeoutAfterError):
			}
		}

		params := &getUpdatesParams{
			Timeout: int((b.pollTimeout - time.Second).Seconds()),
			Offset:  atomic.LoadInt64(&b.lastUpdateID) + 1,
		}

		if b.allowedUpdates != nil {
			params.AllowedUpdates = b.allowedUpdates
		}

		var updates []json.RawMessage

		errRequest := b.rawRequest(ctx, "getUpdates", params, &updates)
		if errRequest != nil {
			if errors.Is(errRequest, context.Canceled) {
				return
			}

			b.error("error get updates, %w", errRequest)

			tooManyRequestsErr, ok := errors.AsType[*TooManyRequestsError](errRequest)
			switch {
			case ok && tooManyRequestsErr.RetryAfter > 0:
				timeoutAfterError = time.Duration(tooManyRequestsErr.RetryAfter) * time.Second
			default:
				timeoutAfterError = incErrTimeout(timeoutAfterError)
			}

			continue
		}

		offsetStuck := false

		for _, raw := range updates {
			upd, errDecode := decodeUpdate(raw)
			if upd.ID == 0 {
				if errDecode == nil {
					errDecode = errMissingUpdateID
				}

				b.error("error decode update, %s, %w", raw, errDecode)
				offsetStuck = true
				continue
			}

			offsetStuck = false
			atomic.StoreInt64(&b.lastUpdateID, upd.ID)

			if errDecode != nil {
				b.error("error decode update %d, skipped, %s, %w", upd.ID, raw, errDecode)
				continue
			}

			select {
			case <-ctx.Done():
				b.error("some updates lost, ctx done")
				return
			case b.updates <- upd:
			}
		}

		if offsetStuck {
			timeoutAfterError = incErrTimeout(timeoutAfterError)
			continue
		}

		timeoutAfterError = 0
	}
}

func decodeUpdate(raw json.RawMessage) (*models.Update, error) {
	upd := &models.Update{}
	errDecode := json.Unmarshal(raw, upd)
	if errDecode == nil {
		return upd, nil
	}

	head := struct {
		ID int64 `json:"update_id"`
	}{}
	_ = json.Unmarshal(raw, &head)

	return &models.Update{ID: head.ID}, errDecode
}

func incErrTimeout(timeout time.Duration) time.Duration {
	if timeout == 0 {
		return time.Millisecond * 100
	}

	timeout *= 2
	if timeout > maxTimeoutAfterError {
		return maxTimeoutAfterError
	}

	return timeout
}
