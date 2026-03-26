package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	charmlog "github.com/charmbracelet/log"
	_ "modernc.org/sqlite"
)

// Store persists Telegram subscriber chat IDs in SQLite.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path.
// The parent directory is created if it does not exist.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Subscribe adds chatID to subscribers. Idempotent.
func (s *Store) Subscribe(chatID int64) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO subscribers (chat_id) VALUES (?)`, chatID,
	)
	if err != nil {
		return fmt.Errorf("subscribe %d: %w", chatID, err)
	}
	return nil
}

// Unsubscribe removes chatID from subscribers. No-op if not present.
func (s *Store) Unsubscribe(chatID int64) error {
	_, err := s.db.Exec(`DELETE FROM subscribers WHERE chat_id = ?`, chatID)
	if err != nil {
		return fmt.Errorf("unsubscribe %d: %w", chatID, err)
	}
	return nil
}

// IsSubscribed reports whether chatID is currently subscribed.
func (s *Store) IsSubscribed(chatID int64) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM subscribers WHERE chat_id = ?`, chatID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("is subscribed %d: %w", chatID, err)
	}
	return count > 0, nil
}

// ListSubscribers returns all subscribed chat IDs.
func (s *Store) ListSubscribers() ([]int64, error) {
	rows, err := s.db.Query(`SELECT chat_id FROM subscribers`)
	if err != nil {
		return nil, fmt.Errorf("query subscribers: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			charmlog.Error("closing query rows", "err", closeErr)
		}
	}()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan subscriber: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return fmt.Errorf("set WAL mode: %w", err)
	}
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS subscribers (
			chat_id       INTEGER PRIMARY KEY,
			subscribed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
