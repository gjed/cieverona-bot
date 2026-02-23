package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	log.Printf("Starting daemon, polling every %s", cfg.PollInterval)
	run(cfg, groups) // run immediately on startup

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			run(cfg, groups)
		case sig := <-quit:
			log.Printf("Received %s, shutting down.", sig)
			return
		}
	}
}

func run(cfg config.Config, groups []booking.CalendarGroup) {
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
	tgCfg := telegram.Config{Token: cfg.TelegramToken, ChatID: cfg.TelegramChatID}
	if err := telegram.Send(tgCfg, msg); err != nil {
		log.Printf("ERROR: failed to send Telegram message: %v", err)
		return
	}
	log.Println("Telegram message sent successfully.")
}
