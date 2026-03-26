// sendtest manually triggers a notification to all subscribers with a fake finding.
// Use it to verify the Telegram integration end-to-end without waiting for a real slot.
//
// Usage:
//
//	go run ./cmd/sendtest/
package main

import (
	"time"

	charmlog "github.com/charmbracelet/log"
	"github.com/gjed/cie-verona/internal/booking"
	"github.com/gjed/cie-verona/internal/config"
	"github.com/gjed/cie-verona/internal/store"
	"github.com/gjed/cie-verona/internal/telegram"
)

func main() {
	config.LoadDotEnv(".env")
	cfg := config.Load()

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		charmlog.Fatal("open store", "err", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			charmlog.Error("close store", "err", err)
		}
	}()

	bot, err := telegram.NewBot(cfg.TelegramToken)
	if err != nil {
		charmlog.Fatal("init bot", "err", err)
	}

	subscribers, err := db.ListSubscribers()
	if err != nil {
		charmlog.Fatal("list subscribers", "err", err)
	}
	if len(subscribers) == 0 {
		charmlog.Fatal("no subscribers, send /subscribe to the bot first")
	}

	findings := []booking.Finding{
		{
			Date:         "2026-03-01",
			GroupName:    "Sportello Polifunzionale Adigetto",
			CalendarName: "Sportello 1",
			SlotCount:    3,
		},
	}
	months := booking.Months(time.Now())
	msg := telegram.BuildMessage(findings, months, nil)
	telegram.SendAll(bot, subscribers, msg)
	charmlog.Info("sent", "subscribers", len(subscribers))
}
