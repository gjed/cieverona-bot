package booking

import (
	"encoding/json"
	"fmt"
	"os"
)

// CalendarGroup is a named set of calendar UUIDs to query together.
type CalendarGroup struct {
	Name      string   `json:"name"`
	Calendars []string `json:"calendars"`
}

// LoadCalendarGroups reads calendar groups from a JSON file.
func LoadCalendarGroups(path string) ([]CalendarGroup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var groups []CalendarGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("%s contains no calendar groups", path)
	}
	return groups, nil
}
