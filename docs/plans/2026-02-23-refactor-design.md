# Refactor Design: lint fixes + internal package split

**Date:** 2026-02-23

## Goal

Fix all linting errors in `main.go` and split the single-file codebase into proper
sub-packages under `internal/` to improve maintainability. Calendar groups are moved
from hardcoded Go to a committed `calendars.json` file.

## Package Structure

```text
cie-verona/
├── main.go                     # daemon loop only (~40 lines)
├── calendars.json              # committed default calendar groups
├── .gitignore
├── go.mod / go.sum
├── Dockerfile / docker-compose.yml
└── internal/
    ├── config/
    │   └── config.go           # loadDotEnv, loadConfig, env helpers
    ├── booking/
    │   ├── calendars.go        # CalendarGroup type, LoadCalendarGroups(path)
    │   ├── client.go           # HTTP fetching, calendarInfo cache
    │   └── checker.go          # Check(), fetchAvailabilities(), Finding type
    └── telegram/
        ├── sender.go           # SendMessage()
        └── message.go          # BuildMessage(), tgEscape()
```

## Lint Fixes

| Location | Issue | Fix |
|---|---|---|
| `fetchCalendarInfo` | `resp.Body.Close` unchecked | log error in defer |
| `fetchAvailabilities` | `resp.Body.Close` unchecked | log error in defer |
| `loadDotEnv` | `f.Close` unchecked | log error in defer |
| `loadDotEnv` | `os.Setenv` unchecked | log.Printf on error |
| `buildMessage` | unnecessary `fmt.Sprintf` on string literal | direct `sb.WriteString` |
| `getEnv` | unused function | delete |

## Data Flow

```
main.go
  └── config.LoadConfig()         → config.Config
  └── booking.LoadCalendarGroups() → []booking.CalendarGroup
  └── booking.Check(cfg, groups)
        ├── fetchAvailabilities() → []Finding  (parallel, per group×month)
        ├── client.fetchCalendarInfo()  (cached)
        └── telegram.SendMessage(cfg, findings, months, errs)
              └── message.BuildMessage() → string
```

## `.gitignore`

Standard Go gitignore: binaries, `vendor/`, `.env` (secrets), IDE files.
`calendars.json` is intentionally committed (not ignored).
