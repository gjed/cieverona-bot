# Design: charmbracelet/log Migration

**Date:** 2026-03-26  
**Status:** Approved

## Goal

Replace the standard `log` package with `charmbracelet/log` to add colored, leveled, timestamped output to the application logs. Migrate all manual `ERROR:`/`WARN:`/`INFO:` string prefixes to proper log levels.

## Dependency

Add `github.com/charmbracelet/log@v1.0.0` via `go get`. Transitive deps include `charmbracelet/lipgloss`, `muesli/termenv`, `charmbracelet/x/term` — all small, pure-Go.

## Initialization

In `main.go`, replace:

```go
log.SetOutput(os.Stdout)
log.SetFlags(log.Ldate | log.Ltime)
```

With:

```go
import (
    charmlog "github.com/charmbracelet/log"
    "github.com/charmbracelet/x/term"
    "github.com/muesli/termenv"
)

logger := charmlog.NewWithOptions(os.Stdout, charmlog.Options{
    Level:           charmlog.DebugLevel,
    TimeFormat:      time.DateTime,  // "2006-01-02 15:04:05"
    ReportTimestamp: true,
})
if !term.IsTerminal(os.Stdout.Fd()) {
    logger.SetColorProfile(termenv.Ascii) // strip ANSI codes when not a TTY
}
charmlog.SetDefault(logger)
```

Use `charmlog.SetDefault` to set the global logger — appropriate for a single-process daemon. No logger threading through function signatures required.

### TTY detection

When stdout is not a TTY (Docker, systemd journal, CI), ANSI escape codes are stripped via `termenv.Ascii` color profile. Log output remains readable in `docker logs`, `journalctl`, and log aggregators.

## Call Site Migration

All ~25 log calls across 7 files get migrated. The `import "log"` in each file is replaced with `charmlog "github.com/charmbracelet/log"`.

### Level mapping

| Old pattern | New call |
|---|---|
| `log.Printf("ERROR: <msg>: %v", err)` | `charmlog.Error("<msg>", "err", err)` |
| `log.Printf("WARN: <msg>: %v", err)` | `charmlog.Warn("<msg>", "err", err)` |
| `log.Printf("INFO: <msg>")` | `charmlog.Info("<msg>")` |
| `log.Printf("FOUND: %s — %s — %s (%d slot(s))", ...)` | `charmlog.Info("found slot", "date", f.Date, "group", f.GroupName, "calendar", f.CalendarName, "slots", f.SlotCount)` |
| `log.Fatalf("<msg>: %v", err)` | `charmlog.Fatal("<msg>", "err", err)` |
| `log.Println("<msg>")` | `charmlog.Info("<msg>")` |
| `log.Printf("<msg>", args...)` | `charmlog.Info("<msg>", key/value pairs)` |

### Files affected

- `main.go`
- `internal/bot/handler.go`
- `internal/telegram/sender.go`
- `internal/booking/checker.go`
- `internal/booking/client.go`
- `internal/config/config.go`
- `cmd/sendtest/main.go`

## What Does Not Change

- Package structure
- Function signatures
- Error handling logic
- Output destination (stdout)
- Log format: text only (no JSON)

## Output Example (TTY)

```
2026-03-26 10:42:01 INFO Starting daemon, polling every 1m
2026-03-26 10:43:01 INFO found slot date=2026-04-15 group=Milano calendar=Questura slots=3
2026-03-26 10:43:01 WARN send to 123456 failed err="context deadline exceeded"
2026-03-26 10:43:01 ERRO listing subscribers err="sql: database is closed"
2026-03-26 10:43:01 FATA init Telegram bot err="unauthorized"
```

Colors: INFO=cyan, WARN=yellow, ERRO=red, FATA=red+bold, timestamp=dim.  
Note: `charmbracelet/log` abbreviates level names to 4 chars (`ERRO`, `FATA`).

## Output Example (non-TTY / Docker)

Same as above, no ANSI escape codes.
