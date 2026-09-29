package bot

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ad3n/telegram-bot/models"
)

const (
	defaultPollTimeout      = time.Minute
	defaultUpdatesChanCap   = 1024
	defaultCheckInitTimeout = time.Second * 5
	defaultWorkers          = 1
)

type (
	HttpClient interface {
		Do(*http.Request) (*http.Response, error)
	}

	ErrorsHandler func(err error)

	DebugHandler func(format string, args ...any)

	Middleware func(next HandlerFunc) HandlerFunc

	HandlerFunc func(ctx context.Context, bot *Bot, update *models.Update)

	MatchFunc func(update *models.Update) bool

	Bot struct {
		lastUpdateID int64

		url                string
		token              string
		pollTimeout        time.Duration
		skipGetMe          bool
		webhookSecretToken string
		testEnvironment    bool
		workers            int
		notAsyncHandlers   bool

		defaultHandlerFunc HandlerFunc

		errorsHandler ErrorsHandler
		debugHandler  DebugHandler

		middlewares []Middleware

		handlersMx sync.RWMutex
		handlers   []handler

		client           HttpClient
		isDebug          bool
		checkInitTimeout time.Duration

		allowedUpdates AllowedUpdates

		updates chan *models.Update
	}
)

func New(token string, options ...Option) (*Bot, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("empty token")
	}

	b := &Bot{
		url:         "https://api.telegram.org",
		token:       token,
		pollTimeout: defaultPollTimeout,
		client: &http.Client{
			Timeout: defaultPollTimeout,
		},
		defaultHandlerFunc: defaultHandler,
		errorsHandler:      defaultErrorsHandler,
		debugHandler:       defaultDebugHandler,
		checkInitTimeout:   defaultCheckInitTimeout,
		workers:            defaultWorkers,

		updates: make(chan *models.Update, defaultUpdatesChanCap),
	}

	for _, o := range options {
		o(b)
	}

	ctx, cancel := context.WithTimeout(context.Background(), b.checkInitTimeout)
	defer cancel()

	if !b.skipGetMe {
		_, err := b.GetMe(ctx)
		if err != nil {
			return nil, fmt.Errorf("error call getMe, %w", err)
		}
	}

	return b, nil
}

func (b *Bot) ID() int64 {
	i, _ := strconv.ParseInt(strings.Split(b.token, ":")[0], 10, 64)
	return i
}

func (b *Bot) SetToken(token string) {
	b.token = token
}

func (b *Bot) Token() string {
	return b.token
}

func (b *Bot) StartWebhook(ctx context.Context) {
	wg := sync.WaitGroup{}

	wg.Add(b.workers)
	for range b.workers {
		go b.waitUpdates(ctx, &wg)
	}

	wg.Wait()
}

func (b *Bot) Start(ctx context.Context) {
	wg := sync.WaitGroup{}

	wg.Add(1)
	go b.getUpdates(ctx, &wg)

	wg.Add(b.workers)
	for range b.workers {
		go b.waitUpdates(ctx, &wg)
	}

	wg.Wait()
}

func defaultErrorsHandler(err error) {
	log.Printf("[TGBOT] [ERROR] %v", err)
}

func defaultDebugHandler(format string, args ...any) {
	log.Printf("[TGBOT] [DEBUG] "+format, args...)
}

func defaultHandler(_ context.Context, _ *Bot, update *models.Update) {
	log.Printf("[TGBOT] [UPDATE] %+v", update)
}

func (b *Bot) error(format string, args ...any) {
	b.errorsHandler(fmt.Errorf(format, args...))
}

func True() *bool {
	return new(true)
}

func False() *bool {
	return new(false)
}

func (b *Bot) FileDownloadLink(f *models.File) string {
	return fmt.Sprintf("%s/file/bot%s/%s", b.url, b.token, f.FilePath)
}
