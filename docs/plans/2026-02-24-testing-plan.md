# Testing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add behaviour-describing tests across all packages, with minimal refactoring to enable testability of HTTP-dependent and Telegram-dependent code.

**Architecture:** Extract a `Sender` interface in `internal/telegram` so bot and sender tests can use a mock; refactor `internal/booking` to a `Client` struct with an injected `*http.Client` so HTTP responses can be intercepted via `httptest.NewServer`; then add pure-function tests for `config`, `telegram/message`, and `booking` helpers.

**Tech Stack:** Go stdlib `testing`, `net/http/httptest`, `encoding/json`. No third-party test libraries.

---

### Task 1: Extract `Sender` interface in `internal/telegram`

**Files:**
- Modify: `internal/telegram/sender.go`
- Modify: `internal/bot/handler.go`
- Modify: `main.go`

**Step 1: Add the `Sender` interface and update `Send`/`SendAll` signatures**

In `internal/telegram/sender.go`, add the interface above `NewBot` and change `Send` and `SendAll` to accept `Sender`:

```go
// Sender is the subset of tgbotapi.BotAPI used for sending messages.
type Sender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// Send sends text (HTML-formatted) to a Telegram chat.
func Send(s Sender, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	if _, err := s.Send(msg); err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

// SendAll sends text to every subscriber chat ID.
// Errors for individual recipients are logged but do not abort the loop.
func SendAll(s Sender, chatIDs []int64, text string) {
	for _, id := range chatIDs {
		if err := Send(s, id, text); err != nil {
			log.Printf("WARN: send to %d failed: %v", id, err)
		}
	}
}
```

**Step 2: Update `internal/bot/handler.go`**

Change `handleCommand` and `StartListener` to use `telegram.Sender` instead of `*tgbotapi.BotAPI`:

```go
import (
	"log"

	"github.com/gjed/cie-verona/internal/store"
	"github.com/gjed/cie-verona/internal/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StartListener starts a goroutine that long-polls for Telegram updates.
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

func handleCommand(sender telegram.Sender, s *store.Store, msg *tgbotapi.Message) {
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
	case "status":
		ok, err := s.IsSubscribed(chatID)
		if err != nil {
			log.Printf("ERROR: status %d: %v", chatID, err)
			text = "Errore interno. Riprova più tardi."
		} else if ok {
			text = msgStatusActive
		} else {
			text = msgStatusInactive
		}
	default:
		text = msgHelp
	}

	reply := tgbotapi.NewMessage(chatID, text)
	if _, err := sender.Send(reply); err != nil {
		log.Printf("WARN: reply to %d failed: %v", chatID, err)
	}
}
```

**Step 3: Verify `main.go` compiles unchanged**

`tgBot` is `*tgbotapi.BotAPI` which satisfies `telegram.Sender` — no changes needed.

**Step 4: Run tests and build**

```
go build ./...
go test ./...
```

Expected: existing tests still pass, build succeeds.

**Step 5: Commit**

```
git add internal/telegram/sender.go internal/bot/handler.go
git commit -m "refactor(telegram): extract Sender interface to enable testing"
```

---

### Task 2: Refactor `internal/booking` to `Client` struct with injected `http.Client`

**Files:**
- Modify: `internal/booking/client.go`
- Modify: `internal/booking/checker.go`
- Modify: `main.go`

**Step 1: Rewrite `internal/booking/client.go`**

Replace package-level globals with a `Client` struct:

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
	"time"
)

const baseCalendar = "https://www.comune.verona.it/openpa/data/booking/calendar"

type calendarInfo struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location"`
}

// Client queries the Comune di Verona booking API.
type Client struct {
	http       *http.Client
	calCache   map[string]calendarInfo
	calCacheMu sync.Mutex
}

// NewClient creates a Client. If hc is nil, http.DefaultClient is used.
func NewClient(hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{
		http:     hc,
		calCache: map[string]calendarInfo{},
	}
}

func (c *Client) fetchCalendarInfo(id string) calendarInfo {
	c.calCacheMu.Lock()
	if info, ok := c.calCache[id]; ok {
		c.calCacheMu.Unlock()
		return info
	}
	c.calCacheMu.Unlock()

	resp, err := c.http.Get(fmt.Sprintf("%s/%s", baseCalendar, id))
	if err != nil {
		return calendarInfo{ID: id, Title: id}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("WARN: closing calendar response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		log.Printf("WARN: calendar API returned %d for %s", resp.StatusCode, id)
		return calendarInfo{ID: id, Title: id}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WARN: reading calendar response for %s: %v", id, err)
		return calendarInfo{ID: id, Title: id}
	}
	var info calendarInfo
	if err := json.Unmarshal(body, &info); err != nil {
		info = calendarInfo{ID: id, Title: id}
	}
	info.Location = stripHTMLTags(info.Location)

	c.calCacheMu.Lock()
	c.calCache[id] = info
	c.calCacheMu.Unlock()
	return info
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}
```

**Step 2: Rewrite `internal/booking/checker.go`**

Make `Check` and `fetchAvailabilities` methods on `Client`. Keep `baseAvail` as a field so tests can override it:

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
	defaultBaseAvail = "https://www.comune.verona.it/openpa/data/booking/availabilities"
	defaultBaseCalendar = "https://www.comune.verona.it/openpa/data/booking/calendar"
	numMonths        = 3
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
func (c *Client) Check(now time.Time, groups []CalendarGroup) (findings []Finding, errs []string) {
	months := Months(now)

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
				f, err := c.fetchAvailabilities(group, month)
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
func Months(now time.Time) []string {
	months := make([]string, numMonths)
	for i := range months {
		months[i] = now.AddDate(0, i, 0).Format("2006-01")
	}
	return months
}

func (c *Client) fetchAvailabilities(group CalendarGroup, month string) ([]Finding, error) {
	calParam := strings.Join(group.Calendars, ",")
	u := fmt.Sprintf("%s?calendars=%s&month=%s", c.baseAvail,
		strings.ReplaceAll(calParam, ",", "%2C"), month)

	resp, err := c.http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("WARN: closing availability response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("availability API returned %d for %s/%s", resp.StatusCode, group.Name, month)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading availability response for %s/%s: %w", group.Name, month, err)
	}

	var ar availabilityResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("parse response for %s/%s: %w", group.Name, month, err)
	}

	var findings []Finding
	for _, avail := range ar.Availabilities {
		if len(avail.Slots) == 0 {
			continue
		}
		info := c.fetchCalendarInfo(avail.CalendarID)
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

Note: `baseAvail` and `baseCalendar` need to be fields on `Client` (not constants) so tests can point them at `httptest.Server`. Add them to the struct and `NewClient`:

```go
type Client struct {
	http         *http.Client
	baseAvail    string
	baseCalendar string
	calCache     map[string]calendarInfo
	calCacheMu   sync.Mutex
}

func NewClient(hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{
		http:         hc,
		baseAvail:    defaultBaseAvail,
		baseCalendar: defaultBaseCalendar,
		calCache:     map[string]calendarInfo{},
	}
}
```

**Step 3: Update `main.go` call site**

Replace `booking.Check(now, groups)` with:

```go
bookingClient := booking.NewClient(nil)
// ...
findings, errs := bookingClient.Check(now, groups)
```

Pass `bookingClient` into `run` (or construct it once in `main` and close over it).

**Step 4: Build and verify**

```
go build ./...
go test ./...
```

Expected: builds and all existing tests pass.

**Step 5: Commit**

```
git add internal/booking/client.go internal/booking/checker.go main.go
git commit -m "refactor(booking): introduce Client struct with injected http.Client"
```

---

### Task 3: Pure function tests — `booking` helpers

**Files:**
- Modify: `internal/booking/checker_test.go` (new file in existing package)
- Modify: `internal/booking/client_test.go` (new file in existing package)

**Step 1: Write failing tests for `Months`**

In `internal/booking/checker_test.go`:

```go
package booking

import (
	"testing"
	"time"
)

func TestMonths_ReturnsThreeMonths(t *testing.T) {
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	got := Months(now)
	if len(got) != 3 {
		t.Fatalf("expected 3 months, got %d", len(got))
	}
}

func TestMonths_FirstIsCurrentMonth(t *testing.T) {
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	got := Months(now)
	if got[0] != "2026-02" {
		t.Errorf("expected first month to be 2026-02, got %q", got[0])
	}
}

func TestMonths_CorrectSequence(t *testing.T) {
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	got := Months(now)
	want := []string{"2026-11", "2026-12", "2027-01"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("month[%d]: expected %q, got %q", i, w, got[i])
		}
	}
}
```

**Step 2: Run to verify they fail**

```
go test ./internal/booking/ -run TestMonths -v
```

Expected: FAIL (function exists but test file is new — they should pass immediately since `Months` is already implemented; if so, that's correct).

**Step 3: Write failing tests for `stripHTMLTags`**

In `internal/booking/client_test.go`:

```go
package booking

import "testing"

func TestStripHTMLTags_RemovesTags(t *testing.T) {
	got := stripHTMLTags("<p>Hello</p>")
	if got != "Hello" {
		t.Errorf("expected %q, got %q", "Hello", got)
	}
}

func TestStripHTMLTags_UnescapesEntities(t *testing.T) {
	got := stripHTMLTags("Citt&agrave; di Verona")
	if got != "Città di Verona" {
		t.Errorf("expected %q, got %q", "Città di Verona", got)
	}
}

func TestStripHTMLTags_CollapsesWhitespace(t *testing.T) {
	got := stripHTMLTags("<p>  foo  </p>  <p>  bar  </p>")
	if got != "foo bar" {
		t.Errorf("expected %q, got %q", "foo bar", got)
	}
}

func TestStripHTMLTags_EmptyString(t *testing.T) {
	got := stripHTMLTags("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestStripHTMLTags_NoTags(t *testing.T) {
	got := stripHTMLTags("plain text")
	if got != "plain text" {
		t.Errorf("expected %q, got %q", "plain text", got)
	}
}
```

**Step 4: Run to verify**

```
go test ./internal/booking/ -run TestStripHTMLTags -v
```

Expected: PASS.

**Step 5: Commit**

```
git add internal/booking/checker_test.go internal/booking/client_test.go
git commit -m "test(booking): add tests for Months and stripHTMLTags"
```

---

### Task 4: Pure function tests — `config`

**Files:**
- Create: `internal/config/config_test.go`

**Step 1: Write failing tests**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDotEnv_MissingFileIsNoop(t *testing.T) {
	LoadDotEnv("/nonexistent/.env") // must not panic or error
}

func TestLoadDotEnv_SetsKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("TEST_KEY_PLAIN=hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_KEY_PLAIN", "")
	os.Unsetenv("TEST_KEY_PLAIN")
	LoadDotEnv(path)
	if got := os.Getenv("TEST_KEY_PLAIN"); got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestLoadDotEnv_DoubleQuotedValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(`TEST_KEY_DQ="quoted value"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv("TEST_KEY_DQ")
	LoadDotEnv(path)
	if got := os.Getenv("TEST_KEY_DQ"); got != "quoted value" {
		t.Errorf("expected %q, got %q", "quoted value", got)
	}
}

func TestLoadDotEnv_SingleQuotedValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("TEST_KEY_SQ='single quoted'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv("TEST_KEY_SQ")
	LoadDotEnv(path)
	if got := os.Getenv("TEST_KEY_SQ"); got != "single quoted" {
		t.Errorf("expected %q, got %q", "single quoted", got)
	}
}

func TestLoadDotEnv_CommentsAndBlanksIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# this is a comment\n\nTEST_KEY_COMMENT=set\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv("TEST_KEY_COMMENT")
	LoadDotEnv(path)
	if got := os.Getenv("TEST_KEY_COMMENT"); got != "set" {
		t.Errorf("expected %q, got %q", "set", got)
	}
}

func TestLoadDotEnv_EnvVarTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("TEST_KEY_PREC=from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_KEY_PREC", "from-env")
	LoadDotEnv(path)
	if got := os.Getenv("TEST_KEY_PREC"); got != "from-env" {
		t.Errorf("env var should take precedence; expected %q, got %q", "from-env", got)
	}
}

func TestMustEnvDuration_ValidDuration(t *testing.T) {
	t.Setenv("TEST_DUR", "30m")
	got := mustEnvDuration("TEST_DUR", 5*time.Minute)
	if got != 30*time.Minute {
		t.Errorf("expected 30m, got %v", got)
	}
}

func TestMustEnvDuration_MissingReturnsDefault(t *testing.T) {
	os.Unsetenv("TEST_DUR_MISSING")
	got := mustEnvDuration("TEST_DUR_MISSING", 5*time.Minute)
	if got != 5*time.Minute {
		t.Errorf("expected 5m default, got %v", got)
	}
}
```

Note: `mustEnvDuration` calls `log.Fatalf` on invalid input — that case is not unit-testable without process isolation; skip it.

**Step 2: Run to verify**

```
go test ./internal/config/ -v
```

Expected: PASS.

**Step 3: Commit**

```
git add internal/config/config_test.go
git commit -m "test(config): add tests for LoadDotEnv and mustEnvDuration"
```

---

### Task 5: Tests for `telegram/message.go`

**Files:**
- Create: `internal/telegram/message_test.go`

**Step 1: Write tests**

```go
package telegram

import (
	"strings"
	"testing"

	"github.com/gjed/cie-verona/internal/booking"
)

func TestEscape_Ampersand(t *testing.T) {
	if got := escape("a & b"); got != "a &amp; b" {
		t.Errorf("expected %q, got %q", "a &amp; b", got)
	}
}

func TestEscape_LessThan(t *testing.T) {
	if got := escape("a < b"); got != "a &lt; b" {
		t.Errorf("expected %q, got %q", "a &lt; b", got)
	}
}

func TestEscape_GreaterThan(t *testing.T) {
	if got := escape("a > b"); got != "a &gt; b" {
		t.Errorf("expected %q, got %q", "a &gt; b", got)
	}
}

func TestEscape_NoSpecialChars(t *testing.T) {
	if got := escape("hello world"); got != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", got)
	}
}

func TestBuildMessage_EmptyFindings(t *testing.T) {
	msg := BuildMessage(nil, []string{"2026-02"}, nil)
	if !strings.Contains(msg, "CIE Verona") {
		t.Error("message should contain header even with no findings")
	}
	if !strings.Contains(msg, "Prenota appuntamento") {
		t.Error("message should contain booking link even with no findings")
	}
}

func TestBuildMessage_GroupsPreserveInsertionOrder(t *testing.T) {
	findings := []booking.Finding{
		{GroupName: "Zeta", CalendarName: "Cal1", Date: "2026-02-01", SlotCount: 1},
		{GroupName: "Alpha", CalendarName: "Cal2", Date: "2026-02-02", SlotCount: 2},
		{GroupName: "Zeta", CalendarName: "Cal3", Date: "2026-02-03", SlotCount: 3},
	}
	msg := BuildMessage(findings, []string{"2026-02"}, nil)
	posZeta := strings.Index(msg, "Zeta")
	posAlpha := strings.Index(msg, "Alpha")
	if posZeta == -1 || posAlpha == -1 {
		t.Fatal("expected both group names in message")
	}
	if posZeta > posAlpha {
		t.Error("Zeta should appear before Alpha (insertion order)")
	}
}

func TestBuildMessage_SlotCountInOutput(t *testing.T) {
	findings := []booking.Finding{
		{GroupName: "G", CalendarName: "C", Date: "2026-02-01", SlotCount: 5},
	}
	msg := BuildMessage(findings, []string{"2026-02"}, nil)
	if !strings.Contains(msg, "5 slot") {
		t.Errorf("expected slot count in message, got: %s", msg)
	}
}

func TestBuildMessage_ErrorsSection(t *testing.T) {
	msg := BuildMessage(nil, []string{"2026-02"}, []string{"something went wrong"})
	if !strings.Contains(msg, "Errori") {
		t.Error("expected errors section in message")
	}
	if !strings.Contains(msg, "something went wrong") {
		t.Error("expected error text in message")
	}
}

func TestBuildMessage_NoErrorsSectionWhenNone(t *testing.T) {
	msg := BuildMessage(nil, []string{"2026-02"}, nil)
	if strings.Contains(msg, "Errori") {
		t.Error("should not contain errors section when no errors")
	}
}

func TestBuildMessage_EscapesSpecialCharsInFindings(t *testing.T) {
	findings := []booking.Finding{
		{GroupName: "G & H", CalendarName: "C < D", Location: "L > M", Date: "2026-02-01", SlotCount: 1},
	}
	msg := BuildMessage(findings, []string{"2026-02"}, nil)
	if strings.Contains(msg, "G & H") {
		t.Error("group name should have & escaped")
	}
	if !strings.Contains(msg, "G &amp; H") {
		t.Error("group name should contain &amp;")
	}
}
```

**Step 2: Run to verify**

```
go test ./internal/telegram/ -run "TestEscape|TestBuildMessage" -v
```

Expected: PASS.

**Step 3: Commit**

```
git add internal/telegram/message_test.go
git commit -m "test(telegram): add tests for escape and BuildMessage"
```

---

### Task 6: HTTP-level tests for `booking.Client`

**Files:**
- Create: `internal/booking/booking_client_test.go` (package `booking`, white-box)

These tests need a helper to build a `Client` pointing at a test HTTP server. Because `baseAvail` and `baseCalendar` are struct fields, the test sets them directly after construction.

**Step 1: Write tests**

```go
package booking

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(srv.Client())
	c.baseAvail = srv.URL
	c.baseCalendar = srv.URL
	return c, srv
}

func TestCheck_HappyPath(t *testing.T) {
	availBody := availabilityResponse{
		Availabilities: []availability{
			{
				Date:       "2026-02-10",
				CalendarID: "cal-1",
				Slots:      []slot{{From: "09:00", To: "09:30"}},
			},
		},
	}
	calBody := calendarInfo{ID: "cal-1", Title: "Sportello A", Location: "Via Roma"}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cal-1" {
			json.NewEncoder(w).Encode(calBody)
			return
		}
		json.NewEncoder(w).Encode(availBody)
	})

	c, _ := newTestClient(t, mux)
	groups := []CalendarGroup{{Name: "Group A", Calendars: []string{"cal-1"}}}
	findings, errs := c.Check(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), groups)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
	if len(findings) != 3 { // 3 months × 1 slot
		t.Fatalf("expected 3 findings (one per month), got %d", len(findings))
	}
	if findings[0].CalendarName != "Sportello A" {
		t.Errorf("expected CalendarName %q, got %q", "Sportello A", findings[0].CalendarName)
	}
}

func TestCheck_Non200CollectedAsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.Client())
	c.baseAvail = srv.URL
	c.baseCalendar = srv.URL
	groups := []CalendarGroup{{Name: "Group A", Calendars: []string{"cal-1"}}}
	_, errs := c.Check(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), groups)

	if len(errs) == 0 {
		t.Fatal("expected errors for non-200, got none")
	}
}

func TestCheck_MalformedJSONCollectedAsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not valid`))
	}))
	defer srv.Close()

	c := NewClient(srv.Client())
	c.baseAvail = srv.URL
	groups := []CalendarGroup{{Name: "Group A", Calendars: []string{"cal-1"}}}
	_, errs := c.Check(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), groups)

	if len(errs) == 0 {
		t.Fatal("expected errors for malformed JSON, got none")
	}
}

func TestCheck_PartialFailure(t *testing.T) {
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Fail the first availability call; succeed on calendar info
		if r.URL.Path != "/cal-1" && calls <= 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/cal-1" {
			json.NewEncoder(w).Encode(calendarInfo{ID: "cal-1", Title: "Sportello A"})
			return
		}
		json.NewEncoder(w).Encode(availabilityResponse{
			Availabilities: []availability{
				{Date: "2026-02-10", CalendarID: "cal-1", Slots: []slot{{From: "09:00", To: "09:30"}}},
			},
		})
	})

	c, _ := newTestClient(t, mux)
	groups := []CalendarGroup{{Name: "Group A", Calendars: []string{"cal-1"}}}
	findings, errs := c.Check(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), groups)

	if len(errs) == 0 {
		t.Fatal("expected at least one error from partial failure")
	}
	if len(findings) == 0 {
		t.Fatal("expected findings from successful requests alongside errors")
	}
}

func TestFetchCalendarInfo_CachePreventsDuplicateRequests(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		json.NewEncoder(w).Encode(calendarInfo{ID: "cal-1", Title: "Cached"})
	}))
	defer srv.Close()

	c := NewClient(srv.Client())
	c.baseCalendar = srv.URL

	c.fetchCalendarInfo("cal-1")
	c.fetchCalendarInfo("cal-1")

	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request due to caching, got %d", requestCount)
	}
}

func TestFetchCalendarInfo_Non200ReturnsFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.Client())
	c.baseCalendar = srv.URL

	info := c.fetchCalendarInfo("unknown-id")
	if info.ID != "unknown-id" || info.Title != "unknown-id" {
		t.Errorf("expected fallback calendarInfo, got %+v", info)
	}
}
```

**Step 2: Run to verify**

```
go test ./internal/booking/ -run "TestCheck|TestFetchCalendar" -v
```

Expected: PASS.

**Step 3: Commit**

```
git add internal/booking/booking_client_test.go
git commit -m "test(booking): add HTTP-level tests for Client.Check and fetchCalendarInfo"
```

---

### Task 7: Tests for `telegram.Send` and `telegram.SendAll`

**Files:**
- Create: `internal/telegram/sender_test.go`

**Step 1: Write tests**

```go
package telegram

import (
	"errors"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type mockSender struct {
	sent []tgbotapi.Chattable
	err  error
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sent = append(m.sent, c)
	return tgbotapi.Message{}, m.err
}

func TestSend_SetsHTMLParseMode(t *testing.T) {
	mock := &mockSender{}
	if err := Send(mock, 123, "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(mock.sent))
	}
	msg, ok := mock.sent[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatal("expected tgbotapi.MessageConfig")
	}
	if msg.ParseMode != tgbotapi.ModeHTML {
		t.Errorf("expected HTML parse mode, got %q", msg.ParseMode)
	}
}

func TestSend_PropagatesError(t *testing.T) {
	mock := &mockSender{err: errors.New("telegram down")}
	err := Send(mock, 123, "hello")
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
}

func TestSendAll_SendsToAllRecipients(t *testing.T) {
	mock := &mockSender{}
	SendAll(mock, []int64{111, 222, 333}, "test message")
	if len(mock.sent) != 3 {
		t.Errorf("expected 3 messages sent, got %d", len(mock.sent))
	}
}

func TestSendAll_ContinuesAfterOneFailure(t *testing.T) {
	callCount := 0
	// Fail only the first send
	mock := &mockSender{}
	_ = callCount
	// Use a custom mock that fails on first call
	failing := &failOnFirstSender{}
	SendAll(failing, []int64{111, 222, 333}, "test")
	if failing.callCount != 3 {
		t.Errorf("expected all 3 sends attempted, got %d", failing.callCount)
	}
}

type failOnFirstSender struct {
	callCount int
}

func (f *failOnFirstSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.callCount++
	if f.callCount == 1 {
		return tgbotapi.Message{}, errors.New("first send fails")
	}
	return tgbotapi.Message{}, nil
}
```

**Step 2: Run to verify**

```
go test ./internal/telegram/ -run "TestSend" -v
```

Expected: PASS.

**Step 3: Commit**

```
git add internal/telegram/sender_test.go
git commit -m "test(telegram): add tests for Send and SendAll"
```

---

### Task 8: Tests for `bot.handleCommand`

**Files:**
- Create: `internal/bot/handler_test.go`

**Step 1: Write tests**

```go
package bot

import (
	"path/filepath"
	"testing"

	"github.com/gjed/cie-verona/internal/store"
	"github.com/gjed/cie-verona/internal/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// mockSender captures outbound messages.
type mockSender struct {
	sent []tgbotapi.Chattable
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sent = append(m.sent, c)
	return tgbotapi.Message{}, nil
}

// Ensure mockSender satisfies telegram.Sender.
var _ telegram.Sender = (*mockSender)(nil)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bot_test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("store Close: %v", err)
		}
	})
	return s
}

func fakeMessage(chatID int64, command string) *tgbotapi.Message {
	return &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chatID},
		Text: "/" + command,
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len(command) + 1},
		},
	}
}

func lastReplyText(t *testing.T, mock *mockSender) string {
	t.Helper()
	if len(mock.sent) == 0 {
		t.Fatal("no messages sent")
	}
	msg, ok := mock.sent[len(mock.sent)-1].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatal("last sent message is not a MessageConfig")
	}
	return msg.Text
}

func TestHandleCommand_Subscribe(t *testing.T) {
	mock := &mockSender{}
	s := openTestStore(t)

	handleCommand(mock, s, fakeMessage(42, "subscribe"))

	if text := lastReplyText(t, mock); text != msgSubscribed {
		t.Errorf("expected %q, got %q", msgSubscribed, text)
	}
	ok, _ := s.IsSubscribed(42)
	if !ok {
		t.Error("expected chat 42 to be subscribed after /subscribe")
	}
}

func TestHandleCommand_SubscribeIdempotent(t *testing.T) {
	mock := &mockSender{}
	s := openTestStore(t)

	handleCommand(mock, s, fakeMessage(42, "subscribe"))
	handleCommand(mock, s, fakeMessage(42, "subscribe"))

	ids, _ := s.ListSubscribers()
	if len(ids) != 1 {
		t.Errorf("expected 1 subscriber, got %d", len(ids))
	}
}

func TestHandleCommand_Unsubscribe(t *testing.T) {
	mock := &mockSender{}
	s := openTestStore(t)

	_ = s.Subscribe(42)
	handleCommand(mock, s, fakeMessage(42, "unsubscribe"))

	if text := lastReplyText(t, mock); text != msgUnsubscribed {
		t.Errorf("expected %q, got %q", msgUnsubscribed, text)
	}
	ok, _ := s.IsSubscribed(42)
	if ok {
		t.Error("expected chat 42 to be unsubscribed")
	}
}

func TestHandleCommand_StatusActive(t *testing.T) {
	mock := &mockSender{}
	s := openTestStore(t)

	_ = s.Subscribe(42)
	handleCommand(mock, s, fakeMessage(42, "status"))

	if text := lastReplyText(t, mock); text != msgStatusActive {
		t.Errorf("expected %q, got %q", msgStatusActive, text)
	}
}

func TestHandleCommand_StatusInactive(t *testing.T) {
	mock := &mockSender{}
	s := openTestStore(t)

	handleCommand(mock, s, fakeMessage(42, "status"))

	if text := lastReplyText(t, mock); text != msgStatusInactive {
		t.Errorf("expected %q, got %q", msgStatusInactive, text)
	}
}

func TestHandleCommand_UnknownCommandSendsHelp(t *testing.T) {
	mock := &mockSender{}
	s := openTestStore(t)

	handleCommand(mock, s, fakeMessage(42, "start"))

	if text := lastReplyText(t, mock); text != msgHelp {
		t.Errorf("expected help message, got %q", text)
	}
}
```

**Step 2: Run to verify**

```
go test ./internal/bot/ -v
```

Expected: PASS.

**Step 3: Run all tests**

```
go test ./...
```

Expected: all packages pass.

**Step 4: Commit**

```
git add internal/bot/handler_test.go
git commit -m "test(bot): add tests for handleCommand"
```

---

### Task 9: Final verification

**Step 1: Run full test suite with race detector**

```
go test -race ./...
```

Expected: all pass, no race conditions detected.

**Step 2: Run linter**

```
golangci-lint run ./...
```

Expected: 0 issues.


