package booking

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCalendarGroups_MissingFile(t *testing.T) {
	_, err := LoadCalendarGroups("/nonexistent/path/calendars.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "/nonexistent/path/calendars.json") {
		t.Errorf("error should contain the path, got: %v", err)
	}
}

func TestLoadCalendarGroups_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCalendarGroups(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestLoadCalendarGroups_EmptyArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCalendarGroups(path)
	if err == nil {
		t.Fatal("expected error for empty array, got nil")
	}
	if !strings.Contains(err.Error(), "no calendar groups") {
		t.Errorf("error should mention 'no calendar groups', got: %v", err)
	}
}

func TestLoadCalendarGroups_EmptyGroupName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noname.json")
	content := `[{"name":"","calendars":["3c76ca92-c4a7-4568-84aa-cbf8d409b019"]}]`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCalendarGroups(path)
	if err == nil {
		t.Fatal("expected error for empty group name, got nil")
	}
}

func TestLoadCalendarGroups_InvalidUUID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baduuid.json")
	content := `[{"name":"Test Group","calendars":["not-a-uuid"]}]`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCalendarGroups(path)
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
	if !strings.Contains(err.Error(), "Test Group") {
		t.Errorf("error should contain group name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not-a-uuid") {
		t.Errorf("error should contain the bad UUID, got: %v", err)
	}
}

func TestLoadCalendarGroups_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "valid.json")
	content := `[
		{
			"name": "Group A",
			"calendars": [
				"3c76ca92-c4a7-4568-84aa-cbf8d409b019",
				"1d86bb6d-a4cb-41da-bbf8-606df733072e"
			]
		},
		{
			"name": "Group B",
			"calendars": [
				"55c5baf5-690d-4451-b819-61d40aa58b16"
			]
		}
	]`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	groups, err := LoadCalendarGroups(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Name != "Group A" {
		t.Errorf("expected 'Group A', got %q", groups[0].Name)
	}
	if len(groups[0].Calendars) != 2 {
		t.Errorf("expected 2 calendars in Group A, got %d", len(groups[0].Calendars))
	}
	if groups[1].Name != "Group B" {
		t.Errorf("expected 'Group B', got %q", groups[1].Name)
	}
	if len(groups[1].Calendars) != 1 {
		t.Errorf("expected 1 calendar in Group B, got %d", len(groups[1].Calendars))
	}
}
