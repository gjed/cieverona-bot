package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/gjed/cie-verona/internal/booking"
	"github.com/gjed/cie-verona/internal/bot"
	"github.com/gjed/cie-verona/internal/config"
	"github.com/gjed/cie-verona/internal/store"
	"github.com/gjed/cie-verona/internal/telegram"
)

func main() {
	config.LoadDotEnv(".env")
	cfg := config.Load()

	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	groups, err := booking.LoadCalendarGroups(cfg.CalendarsFile)
	if err != nil {
		log.Fatalf("ERROR: loading calendar groups: %v", err)
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("ERROR: opening subscriber store: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("ERROR: closing subscriber store: %v", err)
		}
	}()

	tgBot, err := telegram.NewBot(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("ERROR: init Telegram bot: %v", err)
	}

	bot.StartListener(tgBot, db)

	log.Printf("Starting daemon, polling every %s", cfg.PollInterval)
	run(cfg, groups, tgBot, db)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			run(cfg, groups, tgBot, db)
		case sig := <-quit:
			log.Printf("Received %s, shutting down.", sig)
			tgBot.StopReceivingUpdates()
			return
		}
	}
}

func run(cfg config.Config, groups []booking.CalendarGroup, tgBot *tgbotapi.BotAPI, db *store.Store) {
	now := time.Now()
	months := booking.Months(now)
	findings, errs := booking.Check(now, groups)

	if len(findings) == 0 {
		log.Println("No available slots found.")
		return
	}

	for _, f := range findings {
		log.Printf("FOUND: %s — %s — %s (%d slot(s))", f.Date, f.GroupName, f.CalendarName, f.SlotCount)
	}

	subscribers, err := db.ListSubscribers()
	if err != nil {
		log.Printf("ERROR: listing subscribers: %v", err)
		return
	}
	if len(subscribers) == 0 {
		log.Println("No subscribers — skipping Telegram notification.")
		return
	}

	log.Printf("Sending Telegram notification to %d subscriber(s) (%d finding(s)).", len(subscribers), len(findings))
	msg := telegram.BuildMessage(findings, months, errs)
	telegram.SendAll(tgBot, subscribers, msg)
	log.Println("Telegram notifications sent.")
}
