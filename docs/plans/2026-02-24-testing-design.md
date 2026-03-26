# Testing Design

**Date:** 2026-02-24
**Scope:** Add behaviour-describing tests across all packages; introduce minimal refactoring to enable testability of HTTP-dependent and Telegram-dependent code.

---

## Goals

- All packages have tests that describe behaviour, not implementation details.
- No third-party test libraries — stdlib `testing` only (consistent with existing tests).
- Minimal production code changes: only what is necessary to make code injectable/testable.

---

## Section 1 — Refactoring to enable testability

### 1a. `Sender` interface in `internal/telegram`

Extract a minimal interface:

```go
type Sender interface {
    Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}
```

- `Send()` and `SendAll()` change their first parameter from `*tgbotapi.BotAPI` to `Sender`.
- `*tgbotapi.BotAPI` satisfies this interface with no changes.
- Call sites in `main.go` and `bot/handler.go` pass the same `*tgbotapi.BotAPI` value — no behaviour change.

### 1b. `http.Client` injection in `internal/booking`

Replace the package-level `httpClient` with a `Client` struct:

```go
type Client struct {
    http *http.Client
}

func NewClient(hc *http.Client) *Client { ... }
```

- `fetchAvailabilities` and `fetchCalendarInfo` become methods on `Client`.
- `Check` becomes a method on `Client`.
- The calendar info cache moves from package-level globals to a field on `Client`, fixing test isolation.
- `nil` http argument falls back to `http.DefaultClient`.
- Call site in `main.go` uses `booking.NewClient(nil).Check(...)`.

---

## Section 2 — Pure function tests

No refactoring required. All use `t.TempDir()` for file fixtures and `t.Setenv` for env isolation.

| File | Functions | Key cases |
|---|---|---|
| `telegram/message_test.go` | `escape`, `BuildMessage` | `&`, `<`, `>` escaping; grouping by `GroupName` preserving insertion order; empty findings; errors section present/absent |
| `booking/checker_test.go` | `Months` | Returns exactly 3 months; first is current month; correct `YYYY-MM` format |
| `booking/client_test.go` | `stripHTMLTags` | Tags stripped; HTML entities unescaped; multiple spaces collapsed; empty string |
| `config/config_test.go` | `LoadDotEnv`, `mustEnvDuration` | Missing file is silent; bare `KEY=value`; quoted values; comments/blanks ignored; env var takes precedence; valid/invalid/missing duration |

---

## Section 3 — HTTP-level tests for `booking.Client`

Tests use `httptest.NewServer` to serve canned JSON responses against the injected `*http.Client`.

### `Client.Check` cases
- Happy path: server returns valid availabilities JSON → correct `Finding` values returned.
- Non-200 from server → error collected in `errs`, no panic.
- Malformed JSON → error collected, other goroutines unaffected.
- Partial failure: one group/month fails, others succeed → findings from successful ones returned alongside errors.

### `Client.fetchCalendarInfo` cases
- Cache hit: second call does not make an HTTP request (verified by request counter on test server).
- Non-200 fallback: returns `calendarInfo{ID: id, Title: id}`.
- HTML in `location` field: stripped via `stripHTMLTags`.

---

## Section 4 — `telegram` and `bot` tests using the `Sender` interface

### `telegram/sender_test.go`

A `mockSender` captures sent messages:

```go
type mockSender struct {
    sent []tgbotapi.Chattable
    err  error
}
func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) { ... }
```

Cases:
- `Send`: HTML parse mode set on outbound message; error propagated to caller.
- `SendAll`: all IDs receive a message; one failure is logged but does not abort the loop.

### `bot/handler_test.go`

`handleCommand` called directly (not via `StartListener`). Uses `mockSender` for Telegram replies and a real `store.Store` backed by a temp-dir SQLite DB (same pattern as `store_test.go`).

Cases:
- `/subscribe` → subscription stored, `msgSubscribed` reply sent.
- `/subscribe` twice → idempotent, second call still sends `msgSubscribed`.
- `/unsubscribe` → subscription removed, `msgUnsubscribed` reply sent.
- `/status` when subscribed → `msgStatusActive` reply sent.
- `/status` when not subscribed → `msgStatusInactive` reply sent.
- Unknown command → `msgHelp` reply sent.
