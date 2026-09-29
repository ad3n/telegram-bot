package bot

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/ad3n/telegram-bot/models"
)

func referenceEscapeMarkdown(s string, preserveEscapes bool) string {
	var result []rune
	var escaped bool
	for _, r := range s {
		if preserveEscapes && r == '\\' {
			escaped = !escaped
			result = append(result, r)
			continue
		}

		if strings.ContainsRune("_*[]()~`>#+-=|{}.!", r) && !escaped {
			result = append(result, '\\')
		}

		escaped = false
		result = append(result, r)
	}

	return string(result)
}

func FuzzEscapeMarkdownParity(f *testing.F) {
	for _, s := range []string{"", "hello world", "こんにちは🙂", "_*[]()~`>#+-=|{}.!", `\!\\!\\\!`, "\xff\xfe*", "\xef\xbf\xbd", "\xc0\xaf"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		if got, want := EscapeMarkdown(s), referenceEscapeMarkdown(s, false); got != want {
			t.Fatalf("EscapeMarkdown(%q) = %q, want %q", s, got, want)
		}

		if got, want := EscapeMarkdownUnescaped(s), referenceEscapeMarkdown(s, true); got != want {
			t.Fatalf("EscapeMarkdownUnescaped(%q) = %q, want %q", s, got, want)
		}
	})
}

func FuzzRegexHandlerParity(f *testing.F) {
	patterns := []string{`^/start`, `^/start(?: |$)`, `^[a-z]+`, `(?i)^hello`, `(?m)^/help$`, `(?s)a.*b`, `\bcat\b`, `\p{L}+`, `\P{N}+`, `^.$`, `^..$`, `\x{FFFD}`, `foo|bar|baz`, `[a-z]+[0-9]+`, `a{2,5}`, `[^a]`, `.*`, `$`, `^$`}
	regexes := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		regexes[i] = regexp.MustCompile(pattern)
	}

	for _, s := range []string{"", "/start", "/start hello", "abc123", "HELLO", "a\nb", " cat ", "日本語🙂", "é", "\xff", "\xc0\xaf", "\xef\xbf\xbd", "\x00foo"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		upd := &models.Update{Message: &models.Message{Text: s, Caption: s}, CallbackQuery: &models.CallbackQuery{Data: s, GameShortName: s}}
		for _, re := range regexes {
			for _, kind := range []HandlerType{HandlerTypeMessageText, HandlerTypePhotoCaption, HandlerTypeCallbackQueryData, HandlerTypeCallbackQueryGameShortName} {
				h := handler{handlerType: kind, matchType: matchTypeRegexp, re: re}
				if got, want := h.match(upd), re.Match([]byte(s)); got != want {
					t.Fatalf("pattern %q input %q type %d: got %v, want %v", re.String(), s, kind, got, want)
				}
			}
		}
	})
}

func TestUnregisterHandlerReleasesReferences(t *testing.T) {
	b := &Bot{}
	noop := func(context.Context, *Bot, *models.Update) {}
	first := b.RegisterHandlerRegexp(HandlerTypeMessageText, regexp.MustCompile(`first`), noop)
	second := b.RegisterHandlerRegexp(HandlerTypeMessageText, regexp.MustCompile(`second`), noop)
	b.UnregisterHandler(first)
	if len(b.handlers) != 1 || b.handlers[0].id != second {
		t.Fatal("unregister changed handler order")
	}

	backing := b.handlers[:cap(b.handlers)]
	if !reflect.ValueOf(backing[1]).IsZero() {
		t.Fatal("removed slot retains references")
	}

	b.UnregisterHandler(second)
	if len(b.handlers) != 0 || !reflect.ValueOf(backing[0]).IsZero() {
		t.Fatal("last removed slot retains references")
	}
}

func TestConcurrentRegexDispatchAndRegistration(t *testing.T) {
	noop := func(context.Context, *Bot, *models.Update) {}
	b := &Bot{defaultHandlerFunc: noop, notAsyncHandlers: true}
	re := regexp.MustCompile(`(?i)^(start|help)[0-9]+$`)
	b.RegisterHandlerRegexp(HandlerTypeMessageText, re, noop)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			upd := &models.Update{Message: &models.Message{Text: "START123"}}
			for range 200 {
				id := b.RegisterHandlerRegexp(HandlerTypeMessageText, re, noop)
				b.ProcessUpdate(context.Background(), upd)
				if !re.MatchString(upd.Message.Text) || re.MatchString("invalid") {
					t.Error("concurrent matching changed result")
					return
				}

				b.UnregisterHandler(id)
			}
		})
	}

	wg.Wait()
	if len(b.handlers) != 1 {
		t.Fatalf("temporary handlers retained: %d", len(b.handlers))
	}
}

func TestHandlerMalformedInput(t *testing.T) {
	h := handler{handlerType: HandlerTypeMessageText, matchType: matchTypeRegexp}
	if h.match(nil) || h.match(&models.Update{Message: &models.Message{Text: "test"}}) {
		t.Fatal("nil update or regex matched")
	}

	h.matchType = MatchTypeCommand
	for _, entity := range []models.MessageEntity{
		{Type: models.MessageEntityTypeBotCommand, Offset: -1, Length: 4},
		{Type: models.MessageEntityTypeBotCommand, Offset: 100, Length: 4},
		{Type: models.MessageEntityTypeBotCommand, Offset: 0, Length: -1},
		{Type: models.MessageEntityTypeBotCommand, Offset: 1, Length: int(^uint(0) >> 1)},
	} {
		if h.match(&models.Update{Message: &models.Message{Text: "/foo", Entities: []models.MessageEntity{entity}}}) {
			t.Fatal("invalid entity matched")
		}
	}
}

func TestNilFormValues(t *testing.T) {
	for _, params := range []any{
		&struct {
			File *models.InputFileString `json:"file"`
		}{},
		&struct {
			Rich *models.InputRichMessage `json:"rich"`
		}{},
		&struct {
			Results []models.InlineQueryResult `json:"results"`
		}{Results: []models.InlineQueryResult{nil}},
		&struct {
			Results []models.InlineQueryResult `json:"results"`
		}{Results: []models.InlineQueryResult{(*models.InlineQueryResultArticle)(nil)}},
		&struct {
			Media []models.InputMedia `json:"media"`
		}{Media: []models.InputMedia{(*models.InputMediaPhoto)(nil)}},
	} {
		w := multipart.NewWriter(io.Discard)
		if _, err := buildRequestForm(w, params); err == nil {
			t.Fatalf("expected nil value error for %T", params)
		}
	}
}

func TestEmptyFormArrays(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	params := &struct {
		Media    []models.InputMedia        `json:"media"`
		Paid     []models.InputPaidMedia    `json:"paid"`
		Results  []models.InlineQueryResult `json:"results"`
		Stickers []models.InputSticker      `json:"stickers"`
	}{}
	if _, err := buildRequestForm(w, params); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := multipart.NewReader(&buf, w.Boundary())
	for _, name := range []string{"media", "paid", "results", "stickers"} {
		part, err := r.NextPart()
		if err != nil {
			t.Fatal(err)
		}

		value, err := io.ReadAll(part)
		if err != nil || part.FormName() != name || string(value) != "[]" {
			t.Fatalf("field %s: name=%s value=%q err=%v", name, part.FormName(), value, err)
		}
	}
}

func TestGlobalMiddlewareStateRemainsPerUpdate(t *testing.T) {
	constructions := 0
	var calls []int
	b := &Bot{
		notAsyncHandlers:   true,
		defaultHandlerFunc: func(context.Context, *Bot, *models.Update) {},
		middlewares: []Middleware{func(next HandlerFunc) HandlerFunc {
			constructions++
			count := 0
			return func(ctx context.Context, bot *Bot, upd *models.Update) {
				count++
				calls = append(calls, count)
				next(ctx, bot, upd)
			}
		}},
	}
	b.ProcessUpdate(context.Background(), &models.Update{})
	b.ProcessUpdate(context.Background(), &models.Update{})
	if constructions != 2 || !reflect.DeepEqual(calls, []int{1, 1}) {
		t.Fatalf("middleware lifetime changed: constructions=%d calls=%v", constructions, calls)
	}
}

func FuzzCommandEntityBounds(f *testing.F) {
	f.Add("/foo", 0, 4)
	f.Add("/foo", -1, -1)
	f.Add("/foo", 1, int(^uint(0)>>1))
	f.Fuzz(func(t *testing.T, s string, offset, length int) {
		upd := &models.Update{Message: &models.Message{
			Text:     s,
			Entities: []models.MessageEntity{{Type: models.MessageEntityTypeBotCommand, Offset: offset, Length: length}},
		}}
		for _, matchType := range []MatchType{MatchTypeCommand, MatchTypeCommandStartOnly} {
			h := handler{handlerType: HandlerTypeMessageText, matchType: matchType, pattern: "foo"}
			got := h.match(upd)
			valid := offset >= 0 && offset < len(s) && length > 0 && length <= len(s)-offset
			if !valid {
				if got {
					t.Fatal("invalid entity matched")
				}

				continue
			}

			want := s[offset+1:offset+length] == "foo" && (matchType == MatchTypeCommand || offset == 0)
			if got != want {
				t.Fatal("valid entity behavior changed")
			}
		}
	})
}

func TestMatchFuncReceivesNilUpdate(t *testing.T) {
	called := false
	h := handler{matchType: matchTypeFunc, matchFunc: func(upd *models.Update) bool {
		called = true
		return upd == nil
	}}
	if !h.match(nil) || !called {
		t.Fatal("custom matcher no longer receives nil updates")
	}
}

func TestEscapeMarkdownAllBytes(t *testing.T) {
	for first := range 256 {
		for second := range 256 {
			s := string([]byte{byte(first), byte(second)})
			if got, want := EscapeMarkdown(s), referenceEscapeMarkdown(s, false); got != want {
				t.Fatalf("EscapeMarkdown(%q) = %q, want %q", s, got, want)
			}

			if got, want := EscapeMarkdownUnescaped(s), referenceEscapeMarkdown(s, true); got != want {
				t.Fatalf("EscapeMarkdownUnescaped(%q) = %q, want %q", s, got, want)
			}
		}
	}
}
