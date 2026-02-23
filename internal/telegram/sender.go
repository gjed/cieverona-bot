package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// NewBot creates and validates a Telegram bot from the given token.
// Call this once at startup.
func NewBot(token string) (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("init bot: %w", err)
	}
	return bot, nil
}

// Send sends text (HTML-formatted) to the configured Telegram chat.
func Send(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true

	if _, err := bot.Send(msg); err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}
