# CIE Verona – appointment availability checker

Polls the Comune di Verona booking API for available CIE (Carta d'Identità Elettronica)
appointment slots across **all five office groups** for the **current month and the next two**.

If any slot is open, a Telegram message is sent to all subscribers with a summary and a
direct link to the booking page. Users subscribe with `/subscribe` and unsubscribe with
`/unsubscribe` directly in the bot chat.

## Build

```sh
go build -o cie-verona .
```

## Configuration

Copy `.env.example` to `.env` and fill it in. The binary reads `.env` automatically from
the working directory. Real environment variables always take precedence over `.env`.

```sh
cp .env.example .env
$EDITOR .env
./cie-verona
```

| Variable         | Required | Default                  | Description                                        |
|------------------|----------|--------------------------|----------------------------------------------------|
| `TELEGRAM_TOKEN` | yes      | –                        | Bot token from @BotFather                          |
| `POLL_INTERVAL`  | no       | `15m`                    | How often to check. Go duration syntax: `15m`, `1h`, `30s` |
| `DB_PATH`        | no       | `data/subscribers.db`    | Path to SQLite subscriber database                 |

## Telegram setup

### 1. Create a bot

1. Open Telegram and start a chat with [@BotFather](https://t.me/BotFather).
2. Send `/newbot`, follow the prompts.
3. Copy the token (format: `123456789:AAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`) → `TELEGRAM_TOKEN`.

### 2. Subscribe to notifications

Start a chat with your bot and send `/subscribe`. The bot stores your chat ID in SQLite and
will notify you whenever a slot opens. Send `/unsubscribe` to stop. Any other command shows
the help message.

## Docker

```sh
cp .env.example .env
$EDITOR .env
docker compose up -d
```

The container runs as a daemon and restarts automatically. Logs:

```sh
docker compose logs -f
```

To change the polling interval without rebuilding, set `POLL_INTERVAL` in `.env`:

```sh
POLL_INTERVAL=30m
```

Valid units: `s`, `m`, `h` (e.g. `30s`, `15m`, `1h`). Default is `15m`.

## Run without Docker

```sh
./cie-verona
```

The process loops forever and handles `SIGINT`/`SIGTERM` cleanly. To run as a systemd
service, create `/etc/systemd/system/cie-verona.service`:

```ini
[Unit]
Description=CIE Verona availability checker
After=network-online.target

[Service]
WorkingDirectory=/path/to/cie-verona
ExecStart=/path/to/cie-verona/cie-verona
Restart=on-failure
EnvironmentFile=/path/to/cie-verona/.env

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now cie-verona
```

## What is checked

| Group                                    | Address                     | Calendars |
|------------------------------------------|-----------------------------|-----------|
| Sportello Polifunzionale Adigetto        | Via Pallone 13, 37122       | 12        |
| 3a Circoscrizione – Borgo Milano         | Via Sogare 3, 37138         | 2         |
| 4a Circoscrizione – Golosine             | Via Tevere 38, 37136        | 2         |
| 5a Circoscrizione – S. Croce / Quinzano  | Via Benedetti 77, 37135     | 2         |
| 7a Circoscrizione – San Michele          | Piazza del Popolo 15, 37132 | 2         |

Calendar names and addresses are resolved live from the API at each run.

## Booking link

<https://www.comune.verona.it/prenota_appuntamento?service_id=5916>
