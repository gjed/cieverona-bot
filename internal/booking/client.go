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
