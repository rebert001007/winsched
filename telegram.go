package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"time"
)

// TelegramNotifier sends task notifications to a Telegram chat.
type TelegramNotifier struct {
	botToken string
	chatID   string
	enabled  bool
	logger   *Logger
	client   *http.Client
}

// NewTelegramNotifier creates a notifier. Returns nil if config is invalid.
func NewTelegramNotifier(cfg TelegramConfig, proxy ProxyConfig, logger *Logger) *TelegramNotifier {
	if !cfg.Enabled {
		return nil
	}
	if cfg.BotToken == "" || cfg.ChatID == "" {
		logger.Warn("Telegram enabled but bot_token or chat_id is empty — disabling")
		return nil
	}

	transport := &http.Transport{}
	if proxy.Host != "" && proxy.Port != 0 {
		proxyURL, _ := url.Parse(fmt.Sprintf("http://%s:%d", proxy.Host, proxy.Port))
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	return &TelegramNotifier{
		botToken: cfg.BotToken,
		chatID:   cfg.ChatID,
		enabled:  cfg.Enabled,
		logger:   logger,
		client:   &http.Client{Timeout: 10 * time.Second, Transport: transport},
	}
}

func (n *TelegramNotifier) SendStart(taskName, cronExpr string) {
	n.send(FormatStartMessage(taskName, cronExpr, time.Now().In(beijingLoc)))
}

func (n *TelegramNotifier) SendSuccess(taskName, duration, output string) {
	n.send(FormatSuccessMessage(taskName, duration, output, time.Now().In(beijingLoc)))
}

func (n *TelegramNotifier) SendFailure(taskName, duration, status, errMsg string) {
	n.send(FormatFailureMessage(taskName, duration, status, errMsg, time.Now().In(beijingLoc)))
}

func (n *TelegramNotifier) send(text string) {
	if n == nil || !n.enabled {
		return
	}

	body := map[string]string{
		"chat_id":    n.chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	data, _ := json.Marshal(body)

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.botToken)
	resp, err := n.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		n.logger.Warn("Telegram send failed: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		n.logger.Warn("Telegram send returned %d", resp.StatusCode)
	}
}

// FormatStartMessage builds the HTML message for task start.
func FormatStartMessage(taskName, cronExpr string, now time.Time) string {
	ts := now.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("▶️ <b>WinSched</b> — Task Started\n\n"+
		"Task: <code>%s</code>\n"+
		"Cron: <code>%s</code>\n"+
		"Started: <code>%s</code>",
		html.EscapeString(taskName), html.EscapeString(cronExpr), ts)
}

// FormatSuccessMessage builds the HTML message for task success.
func FormatSuccessMessage(taskName, duration, output string, now time.Time) string {
	ts := now.Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("✅ <b>WinSched</b> — Task Completed\n\n"+
		"Task: <code>%s</code>\n"+
		"Duration: %s\n"+
		"Finished: <code>%s</code>",
		html.EscapeString(taskName), duration, ts)
	if output != "" {
		out := truncate(output, 1024)
		msg += fmt.Sprintf("\n\nOutput:\n<code>%s</code>", html.EscapeString(out))
	}
	return msg
}

// FormatFailureMessage builds the HTML message for task failure/timeout.
func FormatFailureMessage(taskName, duration, status, errMsg string, now time.Time) string {
	icon := "❌"  // cross mark
	label := "failed"
	if status == "timeout" {
		icon = "⏰" // alarm clock
		label = "timeout"
	}

	ts := now.Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("%s <b>WinSched</b> — Task Failed (%s)\n\n"+
		"Task: <code>%s</code>\n"+
		"Duration: %s\n"+
		"Finished: <code>%s</code>",
		icon, label, html.EscapeString(taskName), duration, ts)
	if errMsg != "" {
		err := truncate(errMsg, 1024)
		msg += fmt.Sprintf("\n\nError:\n<code>%s</code>", html.EscapeString(err))
	}
	return msg
}
