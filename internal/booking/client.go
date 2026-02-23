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

var (
	calCache   = map[string]calendarInfo{}
	calCacheMu sync.Mutex
	httpClient = &http.Client{Timeout: 10 * time.Second}
)

func fetchCalendarInfo(id string) calendarInfo {
	calCacheMu.Lock()
	if info, ok := calCache[id]; ok {
		calCacheMu.Unlock()
		return info
	}
	calCacheMu.Unlock()

	resp, err := httpClient.Get(fmt.Sprintf("%s/%s", baseCalendar, id))
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

	calCacheMu.Lock()
	calCache[id] = info
	calCacheMu.Unlock()
	return info
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}
