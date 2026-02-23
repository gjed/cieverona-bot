package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	baseAvail    = "https://www.comune.verona.it/openpa/data/booking/availabilities"
	baseCalendar = "https://www.comune.verona.it/openpa/data/booking/calendar"
	bookingURL   = "https://www.comune.verona.it/prenota_appuntamento?service_id=5916"
	numMonths    = 3
)

// calendarGroup represents a named set of calendar UUIDs to query together.
type calendarGroup struct {
	name      string
	calendars []string
}

// All calendar groups derived from the provided URLs (deduplicated by calendar set).
var calendarGroups = []calendarGroup{
	{
		name: "Sportello Polifunzionale Adigetto",
		calendars: []string{
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
			"ef73bcf9-755b-439e-a5a0-e824d54260b2",
		},
	},
	{
		name: "3a Circoscrizione – Borgo Milano",
		calendars: []string{
			"55c5baf5-690d-4451-b819-61d40aa58b16",
			"e7a1f60a-f446-415a-9651-1158d802608e",
		},
	},
	{
		name: "7a Circoscrizione – San Michele",
		calendars: []string{
			"71948d3d-e996-4d6b-8061-fbca6829a078",
			"289a3c98-492f-4181-97b0-984dc8c97d13",
		},
	},
	{
		name: "4a Circoscrizione – Golosine",
		calendars: []string{
			"797c76e7-2db3-40ed-9d14-62b92f09b859",
			"5e03e169-7b68-4dc6-9bc6-5080201e008c",
		},
	},
	{
		name: "5a Circoscrizione – S. Croce / Quinzano",
		calendars: []string{
			"95fc8a6f-dd83-4c9d-853a-acac36ab7b09",
			"39278cc0-3ba9-48d7-aa2b-769b2d34c783",
		},
	},
}

// --- API response types ---

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

type calendarInfo struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location"`
}

// --- finding result ---

type finding struct {
	GroupName    string
	CalendarName string
	Location     string
	Date         string
	SlotCount    int
}

// --- calendar name cache ---

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
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var info calendarInfo
	if err := json.Unmarshal(body, &info); err != nil {
		info = calendarInfo{ID: id, Title: id}
	}
	// Strip HTML tags from location.
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
	// Collapse whitespace.
	fields := strings.Fields(s)
	return strings.Join(fields, ", ")
}

// --- availability fetching ---

func fetchAvailabilities(group calendarGroup, month string) ([]finding, error) {
	calParam := strings.Join(group.calendars, ",")
	u := fmt.Sprintf("%s?calendars=%s&month=%s", baseAvail,
		strings.ReplaceAll(calParam, ",", "%2C"), month)

	resp, err := http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var ar availabilityResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("parse response for %s/%s: %w", group.name, month, err)
	}

	var findings []finding
	for _, avail := range ar.Availabilities {
		info := fetchCalendarInfo(avail.CalendarID)
		findings = append(findings, finding{
			GroupName:    group.name,
			CalendarName: info.Title,
			Location:     info.Location,
			Date:         avail.Date,
			SlotCount:    len(avail.Slots),
		})
	}
	return findings, nil
}

// --- main ---

func main() {
	loadDotEnv(".env")
	cfg := loadConfig()

	// Write to stdout without date prefix — Docker adds its own timestamps.
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	log.Printf("Starting daemon, polling every %s", cfg.PollInterval)
	check(cfg) // run immediately on startup

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	// Handle SIGINT / SIGTERM for clean shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			check(cfg)
		case sig := <-quit:
			log.Printf("Received %s, shutting down.", sig)
			return
		}
	}
}

func check(cfg config) {
	now := time.Now()
	var months []string
	for i := 0; i < numMonths; i++ {
		t := now.AddDate(0, i, 0)
		months = append(months, t.Format("2006-01"))
	}

	log.Printf("Checking %d groups for months: %s", len(calendarGroups), strings.Join(months, ", "))

	type result struct {
		findings []finding
		err      error
	}
	ch := make(chan result, len(calendarGroups)*len(months))

	var wg sync.WaitGroup
	for _, g := range calendarGroups {
		for _, m := range months {
			wg.Add(1)
			go func(group calendarGroup, month string) {
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

	var all []finding
	var errs []string
	for r := range ch {
		if r.err != nil {
			errs = append(errs, r.err.Error())
			log.Printf("ERROR: %v", r.err)
			continue
		}
		all = append(all, r.findings...)
	}

	if len(all) == 0 {
		log.Println("No available slots found.")
		return
	}

	// Log each finding individually.
	for _, f := range all {
		log.Printf("FOUND: %s — %s — %s (%d slot(s))", f.Date, f.GroupName, f.CalendarName, f.SlotCount)
	}
	log.Printf("Sending Telegram notification (%d finding(s)).", len(all))

	msg := buildMessage(all, months, errs)
	if err := sendTelegram(cfg, msg); err != nil {
		log.Printf("ERROR: failed to send Telegram message: %v", err)
		return
	}
	log.Println("Telegram message sent successfully.")
}

// --- Telegram message builder ---
// Uses HTML parse mode: <b>, <i>, <a href> are supported; tables are not.

func buildMessage(findings []finding, months []string, errs []string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<b>🆔 CIE Verona – appuntamenti disponibili</b>\n"))
	sb.WriteString(fmt.Sprintf("Mesi: %s\n", strings.Join(months, ", ")))
	sb.WriteString(fmt.Sprintf("Verifica: %s\n\n", time.Now().Format("02/01/2006 15:04")))

	// Group findings by GroupName.
	grouped := map[string][]finding{}
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
		sb.WriteString(fmt.Sprintf("<b>%s</b>\n", tgEscape(gName)))
		for _, f := range grouped[gName] {
			sb.WriteString(fmt.Sprintf(
				"  • %s — %s (%d slot)\n    <i>%s</i>\n",
				tgEscape(f.Date),
				tgEscape(f.CalendarName),
				f.SlotCount,
				tgEscape(f.Location),
			))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("<a href=%q>Prenota appuntamento</a>", bookingURL))

	if len(errs) > 0 {
		sb.WriteString("\n\n<b>Errori:</b>\n")
		for _, e := range errs {
			sb.WriteString(fmt.Sprintf("  • %s\n", tgEscape(e)))
		}
	}

	return sb.String()
}

// tgEscape escapes characters that are significant in Telegram HTML mode.
func tgEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// --- Telegram sender ---

func sendTelegram(cfg config, text string) error {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return fmt.Errorf("init bot: %w", err)
	}

	msg := tgbotapi.NewMessage(cfg.TelegramChatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true

	if _, err := bot.Send(msg); err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

// --- config ---

type config struct {
	TelegramToken  string
	TelegramChatID int64
	PollInterval   time.Duration
}

func loadConfig() config {
	return config{
		TelegramToken:  mustEnv("TELEGRAM_TOKEN"),
		TelegramChatID: mustEnvInt64("TELEGRAM_CHAT_ID"),
		PollInterval:   mustEnvDuration("POLL_INTERVAL", 15*time.Minute),
	}
}

// loadDotEnv reads a .env file and sets any key that is not already set in the
// environment. Real environment variables always take precedence.
// Lines starting with # and blank lines are ignored.
// Supported formats: KEY=value, KEY="value", KEY='value'.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // file absent is not an error
	}
	defer f.Close()

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
		// Strip optional surrounding quotes.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		// Environment takes precedence: only set if not already present.
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
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
