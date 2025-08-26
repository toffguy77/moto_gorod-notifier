package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	ChatID    int64  `json:"chat_id,omitempty"`
	ServiceID int    `json:"service_id,omitempty"`
	StaffID   int    `json:"staff_id,omitempty"`
	Time      string `json:"time,omitempty"`
}

func main() {
	var (
		logFile = flag.String("logs", "", "Path to log file")
		dbPath  = flag.String("db", "/data/notifier.db", "Path to SQLite database")
	)
	flag.Parse()

	if *logFile == "" {
		log.Fatal("Usage: go run migrate_from_logs.go -logs=bot.log [-db=/data/notifier.db]")
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := createTables(db); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	file, err := os.Open(*logFile)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	var (
		subscribers = make(map[int64]bool)
		seenSlots   = make(map[string]time.Time)
	)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			if entry.Message == "User subscribed" && entry.ChatID != 0 {
				subscribers[entry.ChatID] = true
			}
			if entry.Message == "New slot found" && entry.ServiceID != 0 && entry.StaffID != 0 && entry.Time != "" {
				key := fmt.Sprintf("svc=%d|staff=%d|dt=%s", entry.ServiceID, entry.StaffID, entry.Time)
				if timestamp, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
					seenSlots[key] = timestamp
				}
			}
		} else {
			parseTextLog(line, subscribers, seenSlots)
		}
	}

	subscriberCount := 0
	for chatID := range subscribers {
		if err := insertSubscriber(db, chatID); err == nil {
			subscriberCount++
		}
	}

	slotCount := 0
	for slotKey, timestamp := range seenSlots {
		if err := insertSeenSlot(db, slotKey, timestamp); err == nil {
			slotCount++
		}
	}

	fmt.Printf("Migration completed:\n- Subscribers: %d\n- Seen slots: %d\n", subscriberCount, slotCount)
}

func createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS subscribers (chat_id INTEGER PRIMARY KEY, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS seen_slots (slot_key TEXT PRIMARY KEY, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	}
	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

func insertSubscriber(db *sql.DB, chatID int64) error {
	_, err := db.Exec("INSERT OR IGNORE INTO subscribers (chat_id) VALUES (?)", chatID)
	return err
}

func insertSeenSlot(db *sql.DB, slotKey string, timestamp time.Time) error {
	_, err := db.Exec("INSERT OR IGNORE INTO seen_slots (slot_key, created_at) VALUES (?, ?)", slotKey, timestamp)
	return err
}

func parseTextLog(line string, subscribers map[int64]bool, seenSlots map[string]time.Time) {
	if strings.Contains(line, "User subscribed") {
		if re := regexp.MustCompile(`chat_id[":]\s*(\d+)`); re != nil {
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				if chatID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
					subscribers[chatID] = true
				}
			}
		}
	}

	if strings.Contains(line, "New slot found") {
		var serviceID, staffID int
		var timeStr string

		if re := regexp.MustCompile(`service_id[":]\s*(\d+)`); re != nil {
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				serviceID, _ = strconv.Atoi(matches[1])
			}
		}

		if re := regexp.MustCompile(`staff_id[":]\s*(\d+)`); re != nil {
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				staffID, _ = strconv.Atoi(matches[1])
			}
		}

		if re := regexp.MustCompile(`time[":]\s*"([^"]+)"`); re != nil {
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				timeStr = matches[1]
			}
		}

		if serviceID != 0 && staffID != 0 && timeStr != "" {
			key := fmt.Sprintf("svc=%d|staff=%d|dt=%s", serviceID, staffID, timeStr)
			seenSlots[key] = time.Now()
		}
	}
}