package bot

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"github.com/ad3n/telegram-bot/models"
)

const (
	maxTimeoutAfterError = time.Second * 5
)

const (
	claimPending int32 = iota
	claimTaken
	claimAbandoned
)

var (
	errMissingUpdateID = errors.New("missing update_id")
)

type (
	getUpdatesParams struct {
		Offset         int64          `json:"offset,omitempty"`
		Timeout        int            `json:"timeout,omitempty"`
		AllowedUpdates AllowedUpdates `json:"allowed_updates,omitempty"`
	}

	AllowedUpdates []string

	batchUpdate struct {
		claim pollClaim
		id    int64
	}

	pollClaim struct {
		dispatched chan struct{}
		state      int32
	}
)

func (b *Bot) getUpdates(ctx context.Context, queue chan<- queuedUpdate) {
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
			Timeout:        int((b.pollTimeout - time.Second).Seconds()),
			Offset:         atomic.LoadInt64(&b.lastUpdateID) + 1,
			AllowedUpdates: b.allowedUpdates,
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
		batch := make([]batchUpdate, len(updates))
		batchLen := 0

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
			if params.Offset < 0 && batchLen == 0 {
				atomic.StoreInt64(&b.lastUpdateID, upd.ID-1)
			}

			entry := &batch[batchLen]
			entry.id = upd.ID

			if errDecode != nil {
				b.error("error decode update %d, skipped, %s, %w", upd.ID, raw, errDecode)
				batchLen++
				continue
			}

			entry.claim.dispatched = make(chan struct{})
			select {
			case <-ctx.Done():
				b.confirmDispatched(ctx, batch[:batchLen])
				return
			case queue <- queuedUpdate{update: upd, claim: &entry.claim}:
				batchLen++
			}
		}

		if !b.confirmDispatched(ctx, batch[:batchLen]) {
			return
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

	*upd = models.Update{ID: head.ID}
	return upd, errDecode
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

func (c *pollClaim) take() bool {
	if !atomic.CompareAndSwapInt32(&c.state, claimPending, claimTaken) {
		return false
	}

	close(c.dispatched)
	return true
}

func (c *pollClaim) abandon() bool {
	return atomic.CompareAndSwapInt32(&c.state, claimPending, claimAbandoned)
}

func (b *Bot) confirmDispatched(ctx context.Context, batch []batchUpdate) bool {
	settled := 0
	for settled < len(batch) && batch[settled].wait(ctx) {
		settled++
	}

	if settled > 0 {
		atomic.StoreInt64(&b.lastUpdateID, batch[settled-1].id)
	}

	return settled == len(batch)
}

func (u *batchUpdate) wait(ctx context.Context) bool {
	if u.claim.dispatched == nil {
		return true
	}

	select {
	case <-u.claim.dispatched:
		return true
	case <-ctx.Done():
		return !u.claim.abandon()
	}
}
