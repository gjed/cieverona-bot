# Refactor main.go Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all linting errors and split `main.go` into internal sub-packages for maintainability.

**Architecture:** Move config, booking, and telegram logic into `internal/{config,booking,telegram}` packages. Replace hardcoded calendar groups with `calendars.json`. Add `.gitignore`.

**Tech Stack:** Go 1.25, `github.com/go-telegram-bot-api/telegram-bot-api/v5`, golangci-lint

---

### Task 1: Add `.gitignore`

**Files:**
- Create: `.gitignore`

**Step 1: Create the file**

```gitignore
# Binaries
cie-verona
*.exe

# Go build cache
/vendor/

# Environment secrets (never commit)
.env

# Editor/IDE
.vscode/
.idea/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
```

**Step 2: Verify**

Run: `git status`
Expected: `.gitignore` appears as untracked, `.env` does NOT appear.

**Step 3: Commit**

```bash
git add .gitignore
git commit -m "chore: add .gitignore"
```

---

### Task 2: Create `internal/config/config.go`

Move all config/env logic out of `main.go` into this package.

**Files:**
- Create: `internal/config/config.go`

**Step 1: Write the file**

```go
package config

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration loaded from the environment.
type Config struct {
	TelegramToken  string
	TelegramChatID int64
	PollInterval   time.Duration
	CalendarsFile  string // path to calendars.json; defaults to "calendars.json"
}

// Load reads environment variables (after LoadDotEnv) and returns a Config.
// It calls log.Fatalf on any missing required value.
func Load() Config {
	return Config{
		TelegramToken:  mustEnv("TELEGRAM_TOKEN"),
		TelegramChatID: mustEnvInt64("TELEGRAM_CHAT_ID"),
		PollInterval:   mustEnvDuration("POLL_INTERVAL", 15*time.Minute),
		CalendarsFile:  getEnv("CALENDARS_FILE", "calendars.json"),
	}
}

// LoadDotEnv reads a .env file and sets any key not already in the environment.
// Real environment variables always take precedence.
// Lines starting with # and blank lines are ignored.
// Supported formats: KEY=value, KEY="value", KEY='value'.
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // file absent is not an error
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("WARN: closing %s: %v", path, err)
		}
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, val); err != nil {
				log.Printf("WARN: os.Setenv(%q): %v", key, err)
			}
		}
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %q is not set", key)
	}
	return v
}

func mustEnvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("environment variable %q is not a valid duration (e.g. 15m, 1h): %v", key, err)
	}
	return d
}

func mustEnvInt64(key string) int64 {
	v := mustEnv(key)
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Fatalf("environment variable %q must be an integer: %v", key, err)
	}
	return n
}
```

**Step 2: Build check**

Run: `go build ./internal/config/`
Expected: no errors.

**Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): extract config and dotenv into internal/config"
```

---

### Task 3: Create `calendars.json` and `internal/booking/calendars.go`

**Files:**
- Create: `calendars.json`
- Create: `internal/booking/calendars.go`

**Step 1: Write `calendars.json`**

```json
[
  {
    "name": "Sportello Polifunzionale Adigetto",
    "calendars": [
      "3c76ca92-c4a7-4568-84aa-cbf8d409b019",
      "1d86bb6d-a4cb-41da-bbf8-606df733072e",
      "f618e6c1-3595-4833-ad09-8622bae666b7",
      "bb2ad109-4127-447d-a1cd-4313a683efae",
      "5cd6646a-e173-4cad-9022-4d23cbd4a4e6",
      "f07d6009-d464-4b78-93e3-816436648dca",
      "c1226a53-e1bb-46d6-98fd-38cc6d08d865",
      "b4c51c8b-5957-4413-a817-a7d78b593ac4",
      "a4564a1d-d917-430a-99e0-5cba11491ef7",
      "e5907a73-4e58-4a7c-b343-dad3faea6692",
      "d984c932-7a3f-4e79-92c5-361858dde0e0",
      "ef73bcf9-755b-439e-a5a0-e824d54260b2"
    ]
  },
  {
    "name": "3a Circoscrizione – Borgo Milano",
    "calendars": [
      "55c5baf5-690d-4451-b819-61d40aa58b16",
      "e7a1f60a-f446-415a-9651-1158d802608e"
    ]
  },
  {
    "name": "7a Circoscrizione – San Michele",
    "calendars": [
      "71948d3d-e996-4d6b-8061-fbca6829a078",
      "289a3c98-492f-4181-97b0-984dc8c97d13"
    ]
  },
  {
    "name": "4a Circoscrizione – Golosine",
    "calendars": [
      "797c76e7-2db3-40ed-9d14-62b92f09b859",
      "5e03e169-7b68-4dc6-9bc6-5080201e008c"
    ]
  },
  {
    "name": "5a Circoscrizione – S. Croce / Quinzano",
    "calendars": [
      "95fc8a6f-dd83-4c9d-853a-acac36ab7b09",
      "39278cc0-3ba9-48d7-aa2b-769b2d34c783"
    ]
  }
]
```

**Step 2: Write `internal/booking/calendars.go`**

```go
package booking

import (
	"encoding/json"
	"fmt"
	"os"
)

// CalendarGroup is a named set of calendar UUIDs to query together.
type CalendarGroup struct {
	Name      string   `json:"name"`
	Calendars []string `json:"calendars"`
}

// LoadCalendarGroups reads calendar groups from a JSON file.
func LoadCalendarGroups(path string) ([]CalendarGroup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var groups []CalendarGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("%s contains no calendar groups", path)
	}
	return groups, nil
}
```

**Step 3: Build check**

Run: `go build ./internal/booking/`
Expected: no errors.

**Step 4: Commit**

```bash
git add calendars.json internal/booking/calendars.go
git commit -m "feat(booking): add calendars.json and LoadCalendarGroups"
```

---

### Task 4: Create `internal/booking/client.go`

Move HTTP calendar-info fetching and HTML stripping here.

**Files:**
- Create: `internal/booking/client.go`

**Step 1: Write the file**

```go
package booking

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

const baseCalendar = "https://www.comune.verona.it/openpa/data/booking/calendar"

type calendarInfo struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location"`
}

var (
	calCache   = map[string]calendarInfo{}
	calCacheMu sync.Mutex
)

func fetchCalendarInfo(id string) calendarInfo {
	calCacheMu.Lock()
	if info, ok := calCache[id]; ok {
		calCacheMu.Unlock()
		return info
	}
	calCacheMu.Unlock()

	resp, err := http.Get(fmt.Sprintf("%s/%s", baseCalendar, id))
	if err != nil {
		return calendarInfo{ID: id, Title: id}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("WARN: closing calendar response body: %v", err)
		}
	}()

	body, _ := io.ReadAll(resp.Body)
	var info calendarInfo
	if err := json.Unmarshal(body, &info); err != nil {
		info = calendarInfo{ID: id, Title: id}
	}
	info.Location = stripHTMLTags(info.Location)

	calCacheMu.Lock()
	calCache[id] = info
	calCacheMu.Unlock()
	return info
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), ", ")
}
```

**Step 2: Build check**

Run: `go build ./internal/booking/`
Expected: no errors.

**Step 3: Commit**

```bash
git add internal/booking/client.go
git commit -m "feat(booking): add calendar HTTP client with cache"
```

---

### Task 5: Create `internal/booking/checker.go`

Move availability fetching and the `Check` orchestrator here.

**Files:**
- Create: `internal/booking/checker.go`

**Step 1: Write the file**

```go
package booking

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	baseAvail = "https://www.comune.verona.it/openpa/data/booking/availabilities"
	numMonths = 3
)

// Finding is a single available slot group found for a calendar on a date.
type Finding struct {
	GroupName    string
	CalendarName string
	Location     string
	Date         string
	SlotCount    int
}

type availabilityResponse struct {
	From           string         `json:"from"`
	To             string         `json:"to"`
	Availabilities []availability `json:"availabilities"`
}

type availability struct {
	Date       string `json:"date"`
	CalendarID string `json:"calendar_id"`
	Slots      []slot `json:"slots"`
}

type slot struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Check queries all groups for the next numMonths months and returns findings
// and any non-fatal errors encountered.
func Check(groups []CalendarGroup) (findings []Finding, errs []string) {
	now := time.Now()
	var months []string
	for i := 0; i < numMonths; i++ {
		months = append(months, now.AddDate(0, i, 0).Format("2006-01"))
	}

	log.Printf("Checking %d groups for months: %s", len(groups), strings.Join(months, ", "))

	type result struct {
		findings []Finding
		err      error
	}
	ch := make(chan result, len(groups)*len(months))

	var wg sync.WaitGroup
	for _, g := range groups {
		for _, m := range months {
			wg.Add(1)
			go func(group CalendarGroup, month string) {
				defer wg.Done()
				f, err := fetchAvailabilities(group, month)
				ch <- result{f, err}
			}(g, m)
		}
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	for r := range ch {
		if r.err != nil {
			errs = append(errs, r.err.Error())
			log.Printf("ERROR: %v", r.err)
			continue
		}
		findings = append(findings, r.findings...)
	}
	return findings, errs
}

// Months returns the list of YYYY-MM strings for the next numMonths months.
func Months() []string {
	now := time.Now()
	months := make([]string, numMonths)
	for i := range months {
		months[i] = now.AddDate(0, i, 0).Format("2006-01")
	}
	return months
}

func fetchAvailabilities(group CalendarGroup, month string) ([]Finding, error) {
	calParam := strings.Join(group.Calendars, ",")
	u := fmt.Sprintf("%s?calendars=%s&month=%s", baseAvail,
		strings.ReplaceAll(calParam, ",", "%2C"), month)

	resp, err := http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("WARN: closing availability response body: %v", err)
		}
	}()

	body, _ := io.ReadAll(resp.Body)
	var ar availabilityResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("parse response for %s/%s: %w", group.Name, month, err)
	}

	var findings []Finding
	for _, avail := range ar.Availabilities {
		info := fetchCalendarInfo(avail.CalendarID)
		findings = append(findings, Finding{
			GroupName:    group.Name,
			CalendarName: info.Title,
			Location:     info.Location,
			Date:         avail.Date,
			SlotCount:    len(avail.Slots),
		})
	}
	return findings, nil
}
```

**Step 2: Build check**

Run: `go build ./internal/booking/`
Expected: no errors.

**Step 3: Commit**

```bash
git add internal/booking/checker.go
git commit -m "feat(booking): add Check orchestrator and availability fetcher"
```

---

### Task 6: Create `internal/telegram/message.go` and `internal/telegram/sender.go`

**Files:**
- Create: `internal/telegram/message.go`
- Create: `internal/telegram/sender.go`

**Step 1: Write `internal/telegram/message.go`**

```go
package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/gjed/cie-verona/internal/booking"
)

const bookingURL = "https://www.comune.verona.it/prenota_appuntamento?service_id=5916"

// BuildMessage formats findings into a Telegram HTML message.
func BuildMessage(findings []booking.Finding, months []string, errs []string) string {
	var sb strings.Builder

	sb.WriteString("<b>🆔 CIE Verona – appuntamenti disponibili</b>\n")
	sb.WriteString(fmt.Sprintf("Mesi: %s\n", strings.Join(months, ", ")))
	sb.WriteString(fmt.Sprintf("Verifica: %s\n\n", time.Now().Format("02/01/2006 15:04")))

	// Group findings by GroupName, preserving order of first appearance.
	grouped := map[string][]booking.Finding{}
	var order []string
	seen := map[string]bool{}
	for _, f := range findings {
		if !seen[f.GroupName] {
			order = append(order, f.GroupName)
			seen[f.GroupName] = true
		}
		grouped[f.GroupName] = append(grouped[f.GroupName], f)
	}

	for _, gName := range order {
		sb.WriteString(fmt.Sprintf("<b>%s</b>\n", escape(gName)))
		for _, f := range grouped[gName] {
			sb.WriteString(fmt.Sprintf(
				"  • %s — %s (%d slot)\n    <i>%s</i>\n",
				escape(f.Date),
				escape(f.CalendarName),
				f.SlotCount,
				escape(f.Location),
			))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("<a href=%q>Prenota appuntamento</a>", bookingURL))

	if len(errs) > 0 {
		sb.WriteString("\n\n<b>Errori:</b>\n")
		for _, e := range errs {
			sb.WriteString(fmt.Sprintf("  • %s\n", escape(e)))
		}
	}

	return sb.String()
}

// escape escapes characters significant in Telegram HTML mode.
func escape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
```

**Step 2: Write `internal/telegram/sender.go`**

```go
package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Config holds the Telegram credentials needed to send a message.
type Config struct {
	Token  string
	ChatID int64
}

// Send sends text (HTML-formatted) to the configured Telegram chat.
func Send(cfg Config, text string) error {
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return fmt.Errorf("init bot: %w", err)
	}

	msg := tgbotapi.NewMessage(cfg.ChatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true

	if _, err := bot.Send(msg); err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}
```

**Step 3: Build check**

Run: `go build ./internal/telegram/`
Expected: no errors.

**Step 4: Commit**

```bash
git add internal/telegram/message.go internal/telegram/sender.go
git commit -m "feat(telegram): add message builder and sender"
```

---

### Task 7: Rewrite `main.go`

Replace the monolithic file with a lean daemon loop that wires up the packages.

**Files:**
- Modify: `main.go`

**Step 1: Replace `main.go` entirely**

```go
package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
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
	months := booking.Months()
	findings, errs := booking.Check(groups)

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
```

**Step 2: Build the whole project**

Run: `go build ./...`
Expected: no errors, binary `cie-verona` produced.

**Step 3: Run linter**

Run: `golangci-lint run`
Expected: 0 issues.

**Step 4: Commit**

```bash
git add main.go
git commit -m "refactor: rewrite main.go as lean daemon, wire internal packages"
```

---

### Task 8: Final verification

**Step 1: Full build**

Run: `go build ./...`
Expected: clean.

**Step 2: Lint**

Run: `golangci-lint run`
Expected: 0 issues.

**Step 3: Commit docs**

```bash
git add docs/
git commit -m "docs: add refactor design and implementation plan"
```
