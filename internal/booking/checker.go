package booking

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	charmlog "github.com/charmbracelet/log"
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
func Check(now time.Time, groups []CalendarGroup) (findings []Finding, errs []string) {
	var months []string
	for i := 0; i < numMonths; i++ {
		months = append(months, now.AddDate(0, i, 0).Format("2006-01"))
	}

	charmlog.Info("checking availability", "groups", len(groups), "months", strings.Join(months, ", "))

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
			charmlog.Error("availability check failed", "err", r.err)
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

func fetchAvailabilities(group CalendarGroup, month string) ([]Finding, error) {
	calParam := strings.Join(group.Calendars, ",")
	u := fmt.Sprintf("%s?calendars=%s&month=%s", baseAvail,
		strings.ReplaceAll(calParam, ",", "%2C"), month)

	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			charmlog.Warn("closing availability response body", "err", err)
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
