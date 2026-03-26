package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	charmlog "github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/muesli/termenv"

	"github.com/gjed/cie-verona/internal/booking"
	"github.com/gjed/cie-verona/internal/bot"
	"github.com/gjed/cie-verona/internal/config"
	"github.com/gjed/cie-verona/internal/store"
	"github.com/gjed/cie-verona/internal/telegram"
)

func main() {
	config.LoadDotEnv(".env")
	cfg := config.Load()

	logger := charmlog.NewWithOptions(os.Stdout, charmlog.Options{
		Level:           charmlog.DebugLevel,
		TimeFormat:      time.DateTime,
		ReportTimestamp: true,
	})
	if !term.IsTerminal(os.Stdout.Fd()) {
		logger.SetColorProfile(termenv.Ascii)
	}
	charmlog.SetDefault(logger)

	groups, err := booking.LoadCalendarGroups(cfg.CalendarsFile)
	if err != nil {
		charmlog.Fatal("loading calendar groups", "err", err)
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		charmlog.Fatal("opening subscriber store", "err", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			charmlog.Error("closing subscriber store", "err", err)
		}
	}()

	tgBot, err := telegram.NewBot(cfg.TelegramToken)
	if err != nil {
		charmlog.Fatal("init Telegram bot", "err", err)
	}

	bot.StartListener(tgBot, db)

	charmlog.Info("starting daemon", "poll_interval", cfg.PollInterval)
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
			charmlog.Info("shutting down", "signal", sig)
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
		charmlog.Info("no available slots found")
		return
	}

	for _, f := range findings {
		charmlog.Info("found slot", "date", f.Date, "group", f.GroupName, "calendar", f.CalendarName, "slots", f.SlotCount)
	}

	subscribers, err := db.ListSubscribers()
	if err != nil {
		charmlog.Error("listing subscribers", "err", err)
		return
	}
	if len(subscribers) == 0 {
		charmlog.Info("no subscribers, skipping notification")
		return
	}

	charmlog.Info("sending notifications", "subscribers", len(subscribers), "findings", len(findings))
	msg := telegram.BuildMessage(findings, months, errs)
	telegram.SendAll(tgBot, subscribers, msg)
	charmlog.Info("notifications sent")
}
