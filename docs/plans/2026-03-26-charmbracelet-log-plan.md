# charmbracelet/log Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the standard `log` package with `charmbracelet/log` across the entire codebase for colored, leveled, timestamped output.

**Architecture:** Set `charmbracelet/log` as the global default logger in `main.go` with TTY detection. Replace all `log.Printf/Println/Fatalf` calls across 7 files with leveled equivalents (`Error`, `Warn`, `Info`, `Fatal`), removing manual `ERROR:`/`WARN:`/`INFO:` string prefixes.

**Tech Stack:** `github.com/charmbracelet/log v1.0.0`, `github.com/charmbracelet/x/term`, `github.com/muesli/termenv`

---

### Task 1: Initialize charmbracelet/log in main.go

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Replace log initialization**

Replace:
```go
log.SetOutput(os.Stdout)
log.SetFlags(log.Ldate | log.Ltime)
```
With:
```go
logger := charmlog.NewWithOptions(os.Stdout, charmlog.Options{
    Level:           charmlog.DebugLevel,
    TimeFormat:      time.DateTime,
    ReportTimestamp: true,
})
if !term.IsTerminal(os.Stdout.Fd()) {
    logger.SetColorProfile(termenv.Ascii)
}
charmlog.SetDefault(logger)
```

- [ ] **Step 2: Update imports in main.go**

Replace `"log"` with:
```go
charmlog "github.com/charmbracelet/log"
"github.com/charmbracelet/x/term"
"github.com/muesli/termenv"
```

- [ ] **Step 3: Migrate log calls in main.go**

| Old | New |
|-----|-----|
| `log.Fatalf("ERROR: loading calendar groups: %v", err)` | `charmlog.Fatal("loading calendar groups", "err", err)` |
| `log.Fatalf("ERROR: opening subscriber store: %v", err)` | `charmlog.Fatal("opening subscriber store", "err", err)` |
| `log.Printf("ERROR: closing subscriber store: %v", err)` | `charmlog.Error("closing subscriber store", "err", err)` |
| `log.Fatalf("ERROR: init Telegram bot: %v", err)` | `charmlog.Fatal("init Telegram bot", "err", err)` |
| `log.Printf("Starting daemon, polling every %s", cfg.PollInterval)` | `charmlog.Info("starting daemon", "poll_interval", cfg.PollInterval)` |
| `log.Printf("Received %s, shutting down.", sig)` | `charmlog.Info("shutting down", "signal", sig)` |
| `log.Println("No available slots found.")` | `charmlog.Info("no available slots found")` |
| `log.Printf("FOUND: %s — %s — %s (%d slot(s))", f.Date, f.GroupName, f.CalendarName, f.SlotCount)` | `charmlog.Info("found slot", "date", f.Date, "group", f.GroupName, "calendar", f.CalendarName, "slots", f.SlotCount)` |
| `log.Printf("ERROR: listing subscribers: %v", err)` | `charmlog.Error("listing subscribers", "err", err)` |
| `log.Println("No subscribers — skipping Telegram notification.")` | `charmlog.Info("no subscribers, skipping notification")` |
| `log.Printf("Sending Telegram notification to %d subscriber(s) (%d finding(s)).", len(subscribers), len(findings))` | `charmlog.Info("sending notifications", "subscribers", len(subscribers), "findings", len(findings))` |
| `log.Println("Telegram notifications sent.")` | `charmlog.Info("notifications sent")` |

- [ ] **Step 4: Verify it compiles**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add main.go go.mod go.sum
git commit -m "feat: initialize charmbracelet/log in main.go"
```

---

### Task 2: Migrate internal/bot/handler.go

**Files:**
- Modify: `internal/bot/handler.go`

- [ ] **Step 1: Replace import**

Replace `"log"` with `charmlog "github.com/charmbracelet/log"`.

- [ ] **Step 2: Migrate log calls**

| Old | New |
|-----|-----|
| `log.Printf("ERROR: subscribe %d: %v", chatID, err)` | `charmlog.Error("subscribe failed", "chat_id", chatID, "err", err)` |
| `log.Printf("INFO: %d subscribed", chatID)` | `charmlog.Info("subscribed", "chat_id", chatID)` |
| `log.Printf("ERROR: unsubscribe %d: %v", chatID, err)` | `charmlog.Error("unsubscribe failed", "chat_id", chatID, "err", err)` |
| `log.Printf("INFO: %d unsubscribed", chatID)` | `charmlog.Info("unsubscribed", "chat_id", chatID)` |
| `log.Printf("ERROR: status %d: %v", chatID, err)` | `charmlog.Error("status check failed", "chat_id", chatID, "err", err)` |
| `log.Printf("WARN: reply to %d failed: %v", chatID, err)` | `charmlog.Warn("reply failed", "chat_id", chatID, "err", err)` |

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/bot/handler.go
git commit -m "feat: migrate bot/handler to charmbracelet/log"
```

---

### Task 3: Migrate internal/telegram/sender.go

**Files:**
- Modify: `internal/telegram/sender.go`

- [ ] **Step 1: Replace import**

Replace `"log"` with `charmlog "github.com/charmbracelet/log"`.

- [ ] **Step 2: Migrate log calls**

| Old | New |
|-----|-----|
| `log.Printf("WARN: send to %d failed: %v", id, err)` | `charmlog.Warn("send failed", "chat_id", id, "err", err)` |

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/telegram/sender.go
git commit -m "feat: migrate telegram/sender to charmbracelet/log"
```

---

### Task 4: Migrate internal/booking/checker.go

**Files:**
- Modify: `internal/booking/checker.go`

- [ ] **Step 1: Replace import**

Replace `"log"` with `charmlog "github.com/charmbracelet/log"`.

- [ ] **Step 2: Migrate log calls**

| Old | New |
|-----|-----|
| `log.Printf("Checking %d groups for months: %s", len(groups), strings.Join(months, ", "))` | `charmlog.Info("checking availability", "groups", len(groups), "months", strings.Join(months, ", "))` |
| `log.Printf("ERROR: %v", r.err)` | `charmlog.Error("availability check failed", "err", r.err)` |
| `log.Printf("WARN: closing availability response body: %v", err)` | `charmlog.Warn("closing availability response body", "err", err)` |

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/booking/checker.go
git commit -m "feat: migrate booking/checker to charmbracelet/log"
```

---

### Task 5: Migrate internal/booking/client.go

**Files:**
- Modify: `internal/booking/client.go`

- [ ] **Step 1: Replace import**

Replace `"log"` with `charmlog "github.com/charmbracelet/log"`.

- [ ] **Step 2: Migrate log calls**

| Old | New |
|-----|-----|
| `log.Printf("WARN: closing calendar response body: %v", err)` | `charmlog.Warn("closing calendar response body", "err", err)` |
| `log.Printf("WARN: calendar API returned %d for %s", resp.StatusCode, id)` | `charmlog.Warn("calendar API error", "status", resp.StatusCode, "id", id)` |
| `log.Printf("WARN: reading calendar response for %s: %v", id, err)` | `charmlog.Warn("reading calendar response", "id", id, "err", err)` |

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/booking/client.go
git commit -m "feat: migrate booking/client to charmbracelet/log"
```

---

### Task 6: Migrate internal/config/config.go

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Replace import**

Replace `"log"` with `charmlog "github.com/charmbracelet/log"`.

- [ ] **Step 2: Migrate log calls**

| Old | New |
|-----|-----|
| `log.Printf("WARN: closing %s: %v", path, err)` | `charmlog.Warn("closing file", "path", path, "err", err)` |
| `log.Printf("WARN: os.Setenv(%q): %v", key, err)` | `charmlog.Warn("setenv failed", "key", key, "err", err)` |
| `log.Printf("WARN: reading %s: %v", path, err)` | `charmlog.Warn("reading file", "path", path, "err", err)` |
| `log.Fatalf("required environment variable %q is not set", key)` | `charmlog.Fatal("required env var not set", "key", key)` |
| `log.Fatalf("environment variable %q is not a valid duration ...: %v", key, err)` | `charmlog.Fatal("invalid duration env var", "key", key, "err", err)` |

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: migrate config to charmbracelet/log"
```

---

### Task 7: Migrate cmd/sendtest/main.go

**Files:**
- Modify: `cmd/sendtest/main.go`

- [ ] **Step 1: Replace import**

Replace `"log"` with `charmlog "github.com/charmbracelet/log"`.

- [ ] **Step 2: Migrate log calls**

| Old | New |
|-----|-----|
| `log.Fatalf("open store: %v", err)` | `charmlog.Fatal("open store", "err", err)` |
| `log.Printf("close store: %v", err)` | `charmlog.Error("close store", "err", err)` |
| `log.Fatalf("init bot: %v", err)` | `charmlog.Fatal("init bot", "err", err)` |
| `log.Fatalf("list subscribers: %v", err)` | `charmlog.Fatal("list subscribers", "err", err)` |
| `log.Fatal("No subscribers. Send /subscribe to the bot first.")` | `charmlog.Fatal("no subscribers, send /subscribe to the bot first")` |
| `log.Printf("Sent to %d subscriber(s).", len(subscribers))` | `charmlog.Info("sent", "subscribers", len(subscribers))` |

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add cmd/sendtest/main.go
git commit -m "feat: migrate sendtest to charmbracelet/log"
```

---

### Task 8: Final verification

- [ ] **Step 1: Full build**

```bash
go build ./...
```
Expected: no errors, no warnings.

- [ ] **Step 2: Verify no std log imports remain**

```bash
grep -r '"log"' --include="*.go" .
```
Expected: no output (zero remaining `"log"` imports).

- [ ] **Step 3: Push**

```bash
git push origin main
```
