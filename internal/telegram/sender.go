package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Config holds the Telegram credentials needed to send a message.
type Config struct {
	Token  string
	ChatID int64
}

// Send sends text (HTML-formatted) to the configured Telegram chat.
func Send(cfg Config, text string) error {
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return fmt.Errorf("init bot: %w", err)
	}

	msg := tgbotapi.NewMessage(cfg.ChatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true

	if _, err := bot.Send(msg); err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}
