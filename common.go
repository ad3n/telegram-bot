package bot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = 6
	letterIdxMask = 1<<letterIdxBits - 1
	letterIdxMax  = 63 / letterIdxBits
)

var (
	randSrc = rand.NewSource(time.Now().UnixNano())

	randSrcMx sync.Mutex
)

type (
	WebAppUser struct {
		ID                    int64  `json:"id"`
		IsBot                 bool   `json:"is_bot"`
		FirstName             string `json:"first_name"`
		LastName              string `json:"last_name"`
		Username              string `json:"username"`
		LanguageCode          string `json:"language_code"`
		IsPremium             bool   `json:"is_premium"`
		AddedToAttachmentMenu bool   `json:"added_to_attachment_menu"`
		AllowsWriteToPM       bool   `json:"allows_write_to_pm"`
		PhotoURL              string `json:"photo_url"`
	}
)

func EscapeMarkdown(s string) string {
	return escapeMarkdown(s, false)
}

func EscapeMarkdownUnescaped(s string) string {
	return escapeMarkdown(s, true)
}

func escapeMarkdown(s string, preserveEscapes bool) string {
	size := 0
	escaped := false
	changed := false
	for i, r := range s {
		size += utf8.RuneLen(r)
		if r == utf8.RuneError && s[i] >= utf8.RuneSelf {
			_, width := utf8.DecodeRuneInString(s[i:])
			changed = changed || width == 1
		}

		if preserveEscapes && r == '\\' {
			escaped = !escaped
			continue
		}

		if !escaped && markdownNeedsEscape(r) {
			size++
			changed = true
		}

		escaped = false
	}

	if !changed {
		return s
	}

	var result strings.Builder
	result.Grow(size)
	escaped = false
	start := 0
	for i, r := range s {
		if preserveEscapes && r == '\\' {
			escaped = !escaped
			continue
		}

		if !escaped && markdownNeedsEscape(r) {
			result.WriteString(s[start:i])
			result.WriteByte('\\')
			start = i
		}

		if r == utf8.RuneError {
			_, width := utf8.DecodeRuneInString(s[i:])
			if width == 1 {
				result.WriteString(s[start:i])
				result.WriteRune(utf8.RuneError)
				start = i + 1
			}
		}

		escaped = false
	}

	result.WriteString(s[start:])

	return result.String()
}

func markdownNeedsEscape(r rune) bool {
	switch r {
	case '_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!':
		return true
	}

	return false
}

func RandomString(n int) string {
	b := make([]byte, n)

	randSrcMx.Lock()
	ch := randSrc.Int63()
	randSrcMx.Unlock()
	for i, remain := n-1, letterIdxMax; i >= 0; {
		if remain == 0 {
			randSrcMx.Lock()
			ch, remain = randSrc.Int63(), letterIdxMax
			randSrcMx.Unlock()
		}

		if idx := int(ch & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}

		ch >>= letterIdxBits
		remain--
	}

	return string(b)
}

func ValidateWebappRequest(values url.Values, token string) (user *WebAppUser, ok bool) {
	h := values.Get("hash")
	values.Del("hash")

	var vals []string

	var u WebAppUser

	for k, v := range values {
		vv, _ := url.QueryUnescape(v[0])
		vals = append(vals, k+"="+vv)
		if k == "user" {
			errDecodeUser := json.Unmarshal([]byte(vv), &u)
			if errDecodeUser != nil {
				return nil, false
			}
		}
	}

	slices.Sort(vals)

	hmac1 := hmac.New(sha256.New, []byte("WebAppData"))
	hmac1.Write([]byte(token))
	r1 := hmac1.Sum(nil)

	data := []byte(strings.Join(vals, "\n"))

	hmac2 := hmac.New(sha256.New, r1)
	hmac2.Write(data)
	r2 := hmac2.Sum(nil)

	if h != fmt.Sprintf("%x", r2) {
		return nil, false
	}

	return &u, true
}
