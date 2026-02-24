# Subscriptions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `/subscribe` and `/unsubscribe` Telegram commands backed by SQLite so any user can opt into slot notifications.

**Architecture:** New `internal/store` package owns a SQLite DB with a single `subscribers` table. New `internal/bot` package runs a long-polling goroutine for command handling. `main.go` wires everything; `telegram.SendAll` fans out to all subscribers. `TELEGRAM_CHAT_ID` is removed.

**Tech Stack:** Go 1.25, `modernc.org/sqlite` (pure Go SQLite driver), `github.com/go-telegram-bot-api/telegram-bot-api/v5`

---

### Task 1: Add `modernc.org/sqlite` dependency

**Files:**
- Modify: `go.mod`, `go.sum`

**Step 1: Add the dependency**

```bash
go get modernc.org/sqlite
```

**Step 2: Verify**

```bash
go build ./...
```
Expected: clean, no errors.

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add modernc.org/sqlite dependency"
```

---

### Task 2: Create `internal/store/store.go`

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`

**Step 1: Write the failing tests**

```go
package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gjed/cie-verona/internal/store"
)

func TestSubscribeAndList(t *testing.T) {
	s := openTestStore(t)
	if err := s.Subscribe(111); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := s.Subscribe(222); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	ids, err := s.ListSubscribers()
	if err != nil {
		t.Fatalf("ListSubscribers: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 subscribers, got %d", len(ids))
	}
}

func TestSubscribeIdempotent(t *testing.T) {
	s := openTestStore(t)
	if err := s.Subscribe(111); err != nil {
		t.Fatalf("first Subscribe: %v", err)
	}
	if err := s.Subscribe(111); err != nil {
		t.Fatalf("second Subscribe (idempotent): %v", err)
	}
	ids, _ := s.ListSubscribers()
	if len(ids) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(ids))
	}
}

func TestUnsubscribe(t *testing.T) {
	s := openTestStore(t)
	_ = s.Subscribe(111)
	if err := s.Unsubscribe(111); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	ids, _ := s.ListSubscribers()
	if len(ids) != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe, got %d", len(ids))
	}
}

func TestUnsubscribeNotSubscribed(t *testing.T) {
	s := openTestStore(t)
	// Should not error on unsubscribing a non-existent user.
	if err := s.Unsubscribe(999); err != nil {
		t.Fatalf("Unsubscribe of unknown id: %v", err)
	}
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
```

**Step 2: Run to verify they fail**

```bash
go test ./internal/store/
```
Expected: FAIL — `store` package does not exist yet.

**Step 3: Write the implementation**

```go
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store persists Telegram subscriber chat IDs in SQLite.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path.
// The parent directory is created if it does not exist.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Subscribe adds chatID to subscribers. Idempotent.
func (s *Store) Subscribe(chatID int64) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO subscribers (chat_id) VALUES (?)`, chatID,
	)
	return err
}

// Unsubscribe removes chatID from subscribers. No-op if not present.
func (s *Store) Unsubscribe(chatID int64) error {
	_, err := s.db.Exec(`DELETE FROM subscribers WHERE chat_id = ?`, chatID)
	return err
}

// ListSubscribers returns all subscribed chat IDs.
func (s *Store) ListSubscribers() ([]int64, error) {
	rows, err := s.db.Query(`SELECT chat_id FROM subscribers`)
	if err != nil {
		return nil, fmt.Errorf("query subscribers: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan subscriber: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS subscribers (
			chat_id       INTEGER PRIMARY KEY,
			subscribed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
```

**Step 4: Run tests**

```bash
go test ./internal/store/ -v
```
Expected: 4 tests PASS.

**Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): add SQLite subscriber store"
```

---

### Task 3: Add `telegram.SendAll`

**Files:**
- Modify: `internal/telegram/sender.go`

**Step 1: Add `SendAll` to `sender.go`**

Add after the existing `Send` function:

```go
// SendAll sends text to every subscriber chat ID.
// Errors for individual recipients are logged but do not abort the loop.
func SendAll(bot *tgbotapi.BotAPI, chatIDs []int64, text string) {
	for _, id := range chatIDs {
		if err := Send(bot, id, text); err != nil {
			log.Printf("WARN: send to %d failed: %v", id, err)
		}
	}
}
```

Add `"log"` to the imports in `sender.go`.

**Step 2: Build**

```bash
go build ./internal/telegram/
```
Expected: clean.

**Step 3: Commit**

```bash
git add internal/telegram/sender.go
git commit -m "feat(telegram): add SendAll for fan-out to subscribers"
```

---

### Task 4: Create `internal/bot/handler.go`

**Files:**
- Create: `internal/bot/handler.go`

**Step 1: Write the file**

```go
package bot

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gjed/cie-verona/internal/store"
)

const (
	msgSubscribed   = "✅ Iscritto! Riceverai notifiche sugli appuntamenti disponibili."
	msgUnsubscribed = "❌ Disiscritto."
	msgHelp         = "Comandi disponibili:\n/subscribe – ricevi notifiche\n/unsubscribe – smetti di ricevere notifiche"
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
	default:
		text = msgHelp
	}

	reply := tgbotapi.NewMessage(chatID, text)
	if _, err := bot.Send(reply); err != nil {
		log.Printf("WARN: reply to %d failed: %v", chatID, err)
	}
}
```

**Step 2: Build**

```bash
go build ./internal/bot/
```
Expected: clean.

**Step 3: Commit**

```bash
git add internal/bot/handler.go
git commit -m "feat(bot): add long-polling command handler for subscribe/unsubscribe"
```

---

### Task 5: Update `internal/config/config.go`

Remove `TelegramChatID`, add `DBPath`.

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Update `Config` struct and `Load()`**

In `Config`, replace:
```go
TelegramChatID int64
```
with:
```go
DBPath string // path to SQLite DB; defaults to "data/subscribers.db"
```

In `Load()`, replace:
```go
TelegramChatID: mustEnvInt64("TELEGRAM_CHAT_ID"),
```
with:
```go
DBPath: getEnv("DB_PATH", "data/subscribers.db"),
```

Also delete the `mustEnvInt64` function entirely — it is no longer used.

**Step 2: Build**

```bash
go build ./internal/config/
```
Expected: clean.

**Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "refactor(config): remove TELEGRAM_CHAT_ID, add DBPath"
```

---

### Task 6: Update `main.go`

Wire store, bot listener, and `SendAll`. Remove `TELEGRAM_CHAT_ID` usage.

**Files:**
- Modify: `main.go`

**Step 1: Replace `main.go` entirely**

```go
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
	defer db.Close()

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
```

**Step 2: Build**

```bash
go build ./...
```
Expected: clean, binary `cie-verona` produced.

**Step 3: Lint**

```bash
golangci-lint run
```
Expected: 0 issues.

**Step 4: Commit**

```bash
git add main.go
git commit -m "feat: wire store and bot listener into main, remove TELEGRAM_CHAT_ID"
```

---

### Task 7: Update `docker-compose.yml` and `.env.example`

**Files:**
- Modify: `docker-compose.yml`
- Modify: `.env.example`

**Step 1: Update `docker-compose.yml`**

Replace the file with:

```yaml
services:
  cie-verona:
    build: .
    restart: unless-stopped
    env_file: .env
    environment:
      POLL_INTERVAL: 15s
    volumes:
      - cie-data:/data

volumes:
  cie-data:
```

**Step 2: Update `.env.example`**

Read the current `.env.example` and remove the `TELEGRAM_CHAT_ID` line. Keep all others.

**Step 3: Build and verify**

```bash
go build ./...
golangci-lint run
```

**Step 4: Commit**

```bash
git add docker-compose.yml .env.example
git commit -m "chore: add Docker volume for SQLite, remove TELEGRAM_CHAT_ID from env example"
```

---

### Task 8: Final verification

**Step 1: Full build**

```bash
go build ./...
```
Expected: clean.

**Step 2: All tests**

```bash
go test ./...
```
Expected: all pass (store tests + booking tests).

**Step 3: Lint**

```bash
golangci-lint run
```
Expected: 0 issues.

**Step 4: Check no references to `TELEGRAM_CHAT_ID` remain**

```bash
grep -r TELEGRAM_CHAT_ID .
```
Expected: no matches (only possibly in git history).
