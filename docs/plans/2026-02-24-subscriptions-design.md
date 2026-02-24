# Subscriptions Design

**Date:** 2026-02-24

## Goal

Allow any Telegram user to subscribe to slot notifications via `/subscribe` and
`/unsubscribe` commands. Subscribers are persisted in SQLite on a Docker volume.

## Architecture

```
internal/
  store/
    store.go        # SQLite wrapper: Subscribe, Unsubscribe, ListSubscribers
  bot/
    handler.go      # long-polling goroutine, command dispatch
```

**Changes to existing packages:**
- `internal/config/config.go` — add `DBPath string` (default `"data/subscribers.db"`), remove `TelegramChatID`
- `internal/telegram/sender.go` — add `SendAll(bot, []int64, text)` fan-out
- `main.go` — open DB, start listener, pass subscribers to `run()`
- `docker-compose.yml` — add named volume at `/data`

## SQLite Schema

```sql
CREATE TABLE IF NOT EXISTS subscribers (
    chat_id      INTEGER PRIMARY KEY,
    subscribed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

Driver: `modernc.org/sqlite` (pure Go, no cgo required).

## Data Flow

```
main()
 ├── store.Open(cfg.DBPath)
 ├── telegram.NewBot(cfg.Token)
 ├── bot.StartListener(bot, store)   ← goroutine
 └── ticker → run(cfg, groups, bot, store)
               ├── booking.Check(now, groups)
               ├── store.ListSubscribers()
               └── telegram.SendAll(bot, subscribers, msg)
```

## Bot Commands

| Command | Response |
|---|---|
| `/subscribe` | "✅ Iscritto! Riceverai notifiche sugli appuntamenti disponibili." |
| `/unsubscribe` | "❌ Disiscritto." |
| anything else | "Comandi disponibili:\n/subscribe – ricevi notifiche\n/unsubscribe – smetti di ricevere notifiche" |

## Docker

`docker-compose.yml` gains a named volume `cie-data` mounted at `/data`.
`scratch` image works fine — Docker creates the mount point at runtime.

## Removed

`TELEGRAM_CHAT_ID` env var removed entirely.
