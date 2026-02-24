package bot

import (
	"log"

	"github.com/gjed/cie-verona/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	msgSubscribed     = "✅ Iscritto! Riceverai notifiche sugli appuntamenti disponibili."
	msgUnsubscribed   = "❌ Disiscritto."
	msgStatusActive   = "✅ Sei iscritto alle notifiche."
	msgStatusInactive = "🔕 Non sei iscritto. Usa /subscribe per ricevere notifiche."
	msgHelp           = "Comandi disponibili:\n/subscribe – ricevi notifiche\n/unsubscribe – smetti di ricevere notifiche\n/status – controlla se sei iscritto"
)

// StartListener starts a goroutine that long-polls for Telegram updates
// and handles /subscribe and /unsubscribe commands.
// It returns immediately; the goroutine runs until the process exits.
func StartListener(bot *tgbotapi.BotAPI, s *store.Store) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	go func() {
		for update := range updates {
			if update.Message == nil || !update.Message.IsCommand() {
				continue
			}
			handleCommand(bot, s, update.Message)
		}
	}()
}

func handleCommand(bot *tgbotapi.BotAPI, s *store.Store, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	var text string

	switch msg.Command() {
	case "subscribe":
		if err := s.Subscribe(chatID); err != nil {
			log.Printf("ERROR: subscribe %d: %v", chatID, err)
			text = "Errore interno. Riprova più tardi."
		} else {
			log.Printf("INFO: %d subscribed", chatID)
			text = msgSubscribed
		}
	case "unsubscribe":
		if err := s.Unsubscribe(chatID); err != nil {
			log.Printf("ERROR: unsubscribe %d: %v", chatID, err)
			text = "Errore interno. Riprova più tardi."
		} else {
			log.Printf("INFO: %d unsubscribed", chatID)
			text = msgUnsubscribed
		}
	case "status":
		ok, err := s.IsSubscribed(chatID)
		if err != nil {
			log.Printf("ERROR: status %d: %v", chatID, err)
			text = "Errore interno. Riprova più tardi."
		} else if ok {
			text = msgStatusActive
		} else {
			text = msgStatusInactive
		}
	default:
		text = msgHelp
	}

	reply := tgbotapi.NewMessage(chatID, text)
	if _, err := bot.Send(reply); err != nil {
		log.Printf("WARN: reply to %d failed: %v", chatID, err)
	}
}
