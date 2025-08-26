package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/thatguy/moto_gorod-notifier/internal/logger"
)

type Storage struct {
	db  *sql.DB
	log *logger.Logger
}

func New(dbPath string, log *logger.Logger) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	s := &Storage{
		db:  db,
		log: log,
	}

	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return s, nil
}

func (s *Storage) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS subscribers (
			chat_id INTEGER PRIMARY KEY,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS seen_slots (
			slot_key TEXT PRIMARY KEY,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	s.log.Info("Database migrated successfully")
	return nil
}

func (s *Storage) AddSubscriber(chatID int64) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO subscribers (chat_id) VALUES (?)", chatID)
	return err
}

func (s *Storage) RemoveSubscriber(chatID int64) error {
	_, err := s.db.Exec("DELETE FROM subscribers WHERE chat_id = ?", chatID)
	return err
}

func (s *Storage) GetSubscribers() ([]int64, error) {
	rows, err := s.db.Query("SELECT chat_id FROM subscribers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscribers []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			continue
		}
		subscribers = append(subscribers, chatID)
	}
	return subscribers, nil
}

func (s *Storage) IsSlotSeen(slotKey string) (bool, error) {
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM seen_slots WHERE slot_key = ?)", slotKey).Scan(&exists)
	return exists, err
}

func (s *Storage) MarkSlotSeen(slotKey string) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO seen_slots (slot_key) VALUES (?)", slotKey)
	return err
}

func (s *Storage) CleanOldSlots(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	_, err := s.db.Exec("DELETE FROM seen_slots WHERE created_at < ?", cutoff)
	return err
}

func (s *Storage) Close() error {
	return s.db.Close()
}