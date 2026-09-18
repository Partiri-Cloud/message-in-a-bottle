package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
)

const (
	telegramAPIBaseURL = "https://api.telegram.org"

	// telegramMaxTextUnits is the Bot API limit on a message's text. Telegram
	// counts UTF-16 code units, so an emoji costs two.
	telegramMaxTextUnits = 4096

	// telegramMaxDescription bounds how much of Telegram's error description is
	// copied into the error (and from there into the activity log).
	telegramMaxDescription = 200
)

// Bot tokens are "<bot id>:<secret>". Checking the shape keeps the token from
// rewriting the request path, and keeps it whole in any error that quotes it.
var telegramBotTokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)

// A chat ID is either numeric (negative for groups and supergroups) or a public
// channel's @username, which Telegram restricts to 5-32 characters.
var telegramChatIDPattern = regexp.MustCompile(`^(-?[0-9]{1,20}|@[A-Za-z][A-Za-z0-9_]{4,31})$`)

// ValidateTelegramChatID reports whether id is a chat ID the Bot API accepts.
func ValidateTelegramChatID(id string) error {
	if !telegramChatIDPattern.MatchString(id) {
		return fmt.Errorf("must be a numeric chat ID or a public channel @username")
	}
	return nil
}

type TelegramProvider struct {
	token   string
	baseURL string
	client  *http.Client
}

func NewTelegramProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	var c model.TelegramCreds
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("invalid telegram credentials: %w", err)
	}
	if c.BotToken == "" {
		return nil, fmt.Errorf("invalid telegram credentials: botToken is required")
	}
	if !telegramBotTokenPattern.MatchString(c.BotToken) {
		return nil, fmt.Errorf("invalid telegram credentials: botToken is not a bot token")
	}
	return &TelegramProvider{token: c.BotToken, baseURL: telegramAPIBaseURL, client: WebhookClient}, nil
}

func (p *TelegramProvider) ID() string      { return "telegram_bot" }
func (p *TelegramProvider) Channel() string { return "telegram" }

// Send delivers opts.Content as plain text. The subject is ignored: Telegram
// messages have no subject line.
func (p *TelegramProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	if opts.To == "" {
		return SendResult{}, fmt.Errorf("subscriber has no telegram chat id configured")
	}
	if err := ValidateTelegramChatID(opts.To); err != nil {
		return SendResult{}, fmt.Errorf("invalid telegram chat id: %w", err)
	}
	result, err := p.send(ctx, opts.To, truncateTelegramText(opts.Content))
	return result, p.redact(err)
}

func (p *TelegramProvider) send(ctx context.Context, chatID, text string) (SendResult, error) {
	body, err := json.Marshal(map[string]any{"chat_id": chatID, "text": text})
	if err != nil {
		return SendResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/bot"+p.token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return SendResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return SendResult{}, err
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Result      struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err := json.Unmarshal(raw, &result); err != nil || !result.OK {
		// Redact before cutting: a token split by the cut would no longer match.
		desc := strings.ReplaceAll(result.Description, p.token, "<redacted>")
		if r := []rune(desc); len(r) > telegramMaxDescription {
			desc = string(r[:telegramMaxDescription])
		}
		if desc == "" {
			return SendResult{}, fmt.Errorf("telegram error: status %d", resp.StatusCode)
		}
		return SendResult{}, fmt.Errorf("telegram error: status %d: %s", resp.StatusCode, desc)
	}

	return SendResult{ProviderMessageID: strconv.FormatInt(result.Result.MessageID, 10)}, nil
}

// redact keeps the bot token out of err. The token is part of the request path,
// and a transport failure comes back as a *url.Error quoting the full URL —
// which would otherwise land in the activity log, readable over the API.
func (p *TelegramProvider) redact(err error) error {
	if err == nil {
		return nil
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = fmt.Errorf("telegram request failed: %w", ue.Err)
	}
	msg := err.Error()
	if !strings.Contains(msg, p.token) {
		return err
	}
	return errors.New(strings.ReplaceAll(msg, p.token, "<redacted>"))
}

// truncateTelegramText cuts text to the Bot API limit, ending it with an
// ellipsis, without splitting a character.
func truncateTelegramText(text string) string {
	if len(utf16.Encode([]rune(text))) <= telegramMaxTextUnits {
		return text
	}
	const ellipsis = '…'
	budget := telegramMaxTextUnits - utf16.RuneLen(ellipsis)
	var b strings.Builder
	for _, r := range text {
		n := utf16.RuneLen(r)
		if budget-n < 0 {
			break
		}
		budget -= n
		b.WriteRune(r)
	}
	b.WriteRune(ellipsis)
	return b.String()
}
