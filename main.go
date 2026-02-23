package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/gjed/cie-verona/internal/booking"
	"github.com/gjed/cie-verona/internal/config"
	"github.com/gjed/cie-verona/internal/telegram"
)

func main() {
	config.LoadDotEnv(".env")
	cfg := config.Load()

	// Write to stdout without date prefix — Docker adds its own timestamps.
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	groups, err := booking.LoadCalendarGroups(cfg.CalendarsFile)
	if err != nil {
		log.Fatalf("ERROR: loading calendar groups: %v", err)
	}

	bot, err := telegram.NewBot(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("ERROR: init Telegram bot: %v", err)
	}

	log.Printf("Starting daemon, polling every %s", cfg.PollInterval)
	run(cfg, groups, bot) // run immediately on startup

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			run(cfg, groups, bot)
		case sig := <-quit:
			log.Printf("Received %s, shutting down.", sig)
			return
		}
	}
}

func run(cfg config.Config, groups []booking.CalendarGroup, bot *tgbotapi.BotAPI) {
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
	log.Printf("Sending Telegram notification (%d finding(s)).", len(findings))

	msg := telegram.BuildMessage(findings, months, errs)
	if err := telegram.Send(bot, cfg.TelegramChatID, msg); err != nil {
		log.Printf("ERROR: failed to send Telegram message: %v", err)
		return
	}
	log.Println("Telegram message sent successfully.")
}
