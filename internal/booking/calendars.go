package booking

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

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
	for _, g := range groups {
		if g.Name == "" {
			return nil, fmt.Errorf("%s: group has empty name", path)
		}
		if len(g.Calendars) == 0 {
			return nil, fmt.Errorf("%s: group %q has no calendars", path, g.Name)
		}
		for _, uid := range g.Calendars {
			if !uuidRe.MatchString(uid) {
				return nil, fmt.Errorf("%s: group %q has invalid UUID %q", path, g.Name, uid)
			}
		}
	}
	return groups, nil
}
