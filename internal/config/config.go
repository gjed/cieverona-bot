package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	charmlog "github.com/charmbracelet/log"
)

// Config holds all runtime configuration loaded from the environment.
type Config struct {
	TelegramToken string
	DBPath        string // path to SQLite DB; defaults to "data/subscribers.db"
	PollInterval  time.Duration
	CalendarsFile string // path to calendars.json; defaults to "calendars.json"
	CheckMonths   int    // number of months to check ahead; defaults to 6
}

// Load reads environment variables (after LoadDotEnv) and returns a Config.
// It calls log.Fatalf on any missing required value.
func Load() Config {
	return Config{
		TelegramToken: mustEnv("TELEGRAM_TOKEN"),
		DBPath:        getEnv("DB_PATH", "data/subscribers.db"),
		PollInterval:  mustEnvDuration("POLL_INTERVAL", 15*time.Minute),
		CalendarsFile: getEnv("CALENDARS_FILE", "calendars.json"),
		CheckMonths:   mustEnvInt("CHECK_MONTHS", 3),
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
			charmlog.Warn("closing file", "path", path, "err", err)
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
		if _, ok := os.LookupEnv(key); !ok {
			if err := os.Setenv(key, val); err != nil {
				charmlog.Warn("setenv failed", "key", key, "err", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		charmlog.Warn("reading file", "path", path, "err", err)
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
		charmlog.Fatal("required env var not set", "key", key)
	}
	return v
}

func mustEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		charmlog.Fatal("invalid integer env var", "key", key, "value", v)
	}
	return n
}

func mustEnvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		charmlog.Fatal("invalid duration env var", "key", key, "err", err)
	}
	return d
}
