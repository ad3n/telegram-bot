package bot

import (
	"context"
	"sync"

	"github.com/ad3n/telegram-bot/models"
)

type (
	queuedUpdate struct {
		update      *models.Update
		claim       *pollClaim
		dispatched  chan<- struct{}
		requestDone <-chan struct{}
	}

	updatesRun struct {
		ctx     context.Context
		queue   chan queuedUpdate
		done    chan struct{}
		webhook bool
	}
)

func (b *Bot) start(ctx context.Context, webhook bool) {
	if ctx.Err() != nil {
		return
	}

	if b.workers <= 0 {
		go b.error("cannot start bot without update workers")
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	run := &updatesRun{ctx: runCtx, webhook: webhook, done: make(chan struct{})}
	if !b.beginUpdates(run) {
		return
	}
	defer b.endUpdates()

	var polling, workers, handlers sync.WaitGroup
	if !webhook {
		polling.Add(1)
		go func() {
			defer func() {
				cancel()
				polling.Done()
			}()

			b.getUpdates(runCtx, run.queue)
		}()
	}

	workers.Add(b.workers)
	for range b.workers {
		go func() {
			defer workers.Done()

			b.waitUpdates(run, &handlers)
		}()
	}

	workers.Wait()
	cancel()
	polling.Wait()
	handlers.Wait()
}

func (b *Bot) beginUpdates(run *updatesRun) bool {
	for run.ctx.Err() == nil {
		b.updatesMx.Lock()
		previous := b.updatesRun
		if previous == nil {
			run.queue = make(chan queuedUpdate, b.updatesChanCap)
			b.updatesRun = run
			b.updatesMx.Unlock()
			return true
		}

		b.updatesMx.Unlock()
		select {
		case <-previous.done:
		case <-run.ctx.Done():
			return false
		}
	}

	return false
}

func (b *Bot) endUpdates() {
	b.updatesMx.Lock()
	run := b.updatesRun
	b.updatesRun = nil
	b.updatesMx.Unlock()
	close(run.done)
}

func (b *Bot) waitUpdates(run *updatesRun, handlers *sync.WaitGroup) {
	for {
		select {
		case <-run.ctx.Done():
			return
		case item := <-run.queue:
			handler, ok := b.prepareUpdate(run, item)
			if !ok {
				continue
			}

			update := item.update
			if b.notAsyncHandlers {
				handler(run.ctx, b, update)
				continue
			}

			handlers.Add(1)
			go func() {
				defer handlers.Done()

				handler(run.ctx, b, update)
			}()
		}
	}
}

func (b *Bot) prepareUpdate(run *updatesRun, item queuedUpdate) (HandlerFunc, bool) {
	switch {
	case run.webhook:
		select {
		case item.dispatched <- struct{}{}:
		case <-run.ctx.Done():
			return nil, false
		case <-item.requestDone:
			return nil, false
		}
	case run.ctx.Err() != nil || !item.claim.take():
		return nil, false
	}

	return applyMiddlewares(b.findHandler(item.update), b.middlewares...), true
}
