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
		if _, ok := os.LookupEnv(key); !ok {
			if err := os.Setenv(key, val); err != nil {
				log.Printf("WARN: os.Setenv(%q): %v", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("WARN: reading %s: %v", path, err)
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
