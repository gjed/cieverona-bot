package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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
	return err
}

// Unsubscribe removes chatID from subscribers. No-op if not present.
func (s *Store) Unsubscribe(chatID int64) error {
	_, err := s.db.Exec(`DELETE FROM subscribers WHERE chat_id = ?`, chatID)
	return err
}

// ListSubscribers returns all subscribed chat IDs.
func (s *Store) ListSubscribers() ([]int64, error) {
	rows, err := s.db.Query(`SELECT chat_id FROM subscribers`)
	if err != nil {
		return nil, fmt.Errorf("query subscribers: %w", err)
	}
	defer rows.Close()

	var ids []int64
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
