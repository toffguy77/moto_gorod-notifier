package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/thatguy/moto_gorod-notifier/internal/bot"
	"github.com/thatguy/moto_gorod-notifier/internal/config"
	"github.com/thatguy/moto_gorod-notifier/internal/logger"
	"github.com/thatguy/moto_gorod-notifier/internal/metrics"
	"github.com/thatguy/moto_gorod-notifier/internal/notifier"
	"github.com/thatguy/moto_gorod-notifier/internal/storage"
	"github.com/thatguy/moto_gorod-notifier/internal/yclients"
)

func main() {
	// Initialize structured logger
	log := logger.New()
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		log = log.WithLevel(logger.LogLevel(level))
	}

	log.Info("Starting Moto Gorod Slot Notifier")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.WithError(err).Error("Failed to load configuration")
		os.Exit(1)
	}

	log.InfoWithFields("Configuration loaded successfully", logger.Fields{
		"telegram_token_set": cfg.TelegramToken != "",
		"yclients_login_set":  cfg.YClientsLogin != "",
		"company_id":          cfg.YClientsCompanyID,
		"form_id":             cfg.YClientsFormID,
		"timezone":            cfg.Timezone,
		"poll_interval":       cfg.PollInterval.String(),
		"service_ids":         cfg.ServiceIDs,
	})

	// Root context with graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Convert company ID to int for availability payloads
	companyIDInt, err := strconv.Atoi(cfg.YClientsCompanyID)
	if err != nil {
		log.WithError(err).WithField("company_id", cfg.YClientsCompanyID).Error("Invalid YCLIENTS_COMPANY_ID")
		os.Exit(1)
	}

	// Initialize YCLIENTS client
	yc := yclients.New(cfg.YClientsLogin, cfg.YClientsPassword, cfg.YClientsPartnerToken, cfg.YClientsCompanyID, cfg.YClientsFormID)
	st := yc.GetStatus(ctx)
	log.InfoWithFields("YCLIENTS client initialized", logger.Fields{
		"auth_configured": st.AuthConfigured,
		"company_id":      st.CompanyID,
		"form_id":         st.FormID,
		"notes":           st.Notes,
	})
	
	// Test authentication immediately if we have service IDs
	if len(cfg.ServiceIDs) > 0 {
		log.Info("Testing YCLIENTS authentication...")
		if _, err := yc.GetBookableStaffIDs(ctx, companyIDInt, cfg.ServiceIDs[0]); err != nil {
			log.WithError(err).Error("YCLIENTS authentication test failed")
			os.Exit(1)
		}
		log.Info("YCLIENTS authentication successful")
	} else {
		log.Warn("No service IDs configured, skipping authentication test")
	}

	// Initialize storage
	store, err := storage.New("/data/notifier.db", log.WithField("component", "storage"))
	if err != nil {
		log.WithError(err).Error("Failed to initialize storage")
		os.Exit(1)
	}
	defer store.Close()

	// Show startup statistics
	subscriberCount, seenSlotsCount, err := store.GetStats()
	if err != nil {
		log.WithError(err).Warn("Failed to get startup statistics")
	} else {
		log.InfoWithFields("Database statistics", logger.Fields{
			"subscribers": subscriberCount,
			"seen_slots":  seenSlotsCount,
		})
	}

	// Initialize metrics
	metrics := metrics.New()

	// Initialize Telegram bot
	tg, err := bot.New(cfg.TelegramToken, store, log.WithField("component", "telegram_bot"))
	if err != nil {
		log.WithError(err).Error("Failed to initialize Telegram bot")
		os.Exit(1)
	}
	tg.SetMetrics(metrics)

	// Update interface for all users on startup
	if subscriberCount > 0 {
		log.InfoWithFields("Updating bot interface for existing users", logger.Fields{
			"users_to_update": subscriberCount,
		})
		tg.UpdateInterfaceForAll()
	} else {
		log.Info("No existing users to update")
	}

	// Set current slots handler
	tg.SetCurrentSlotsHandler(func() ([]string, error) {
		return getCurrentSlots(ctx, yc, companyIDInt, cfg.ServiceIDs, cfg.Timezone)
	})

	// Initialize notifier
	n := notifier.New(tg, yc, notifier.Options{
		Interval:   cfg.PollInterval,
		Timezone:   cfg.Timezone,
		LocationID: companyIDInt,
		ServiceIDs: cfg.ServiceIDs,
	}, store, log.WithField("component", "notifier"))
	n.SetMetrics(metrics)

	// Set initial metrics from database stats
	metrics.SetActiveSubscribers(float64(subscriberCount))
	metrics.SetSeenSlotsTotal(float64(seenSlotsCount))

	// Start metrics HTTP server
	go func() {
		http.Handle("/metrics", metrics.Handler())
		log.Info("Starting metrics server on :9090")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.WithError(err).Error("Metrics server failed")
		}
	}()

	// Set template renderer for bot
	tg.SetTemplateRenderer(n)

	// Start components with proper error handling and graceful shutdown
	var wg sync.WaitGroup
	
	log.Info("Starting Telegram bot")
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.WithField("panic", r).Error("Telegram bot panicked")
			}
		}()
		tg.Run(ctx)
		log.Info("Telegram bot stopped")
	}()

	log.Info("Starting notifier")
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.WithField("panic", r).Error("Notifier panicked")
			}
		}()
		n.Run(ctx)
		log.Info("Notifier stopped")
	}()

	log.Info("Moto Gorod Slot Notifier started successfully")
	<-ctx.Done()
	log.Info("Received shutdown signal, stopping gracefully...")
	
	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Wait for all goroutines to finish or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		log.Info("All components stopped gracefully")
	case <-shutdownCtx.Done():
		log.Warn("Shutdown timeout reached, forcing exit")
	}
}

func getCurrentSlots(ctx context.Context, yc *yclients.Client, locationID int, serviceIDs []int, timezone string) ([]string, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.FixedZone("UTC+3", 3*3600)
	}
	
	today := time.Now().In(loc).Format("2006-01-02")
	const farFuture = "9999-01-01"
	
	var allSlots []string
	
	for _, serviceID := range serviceIDs {
		staffIDs, err := yc.GetBookableStaffIDs(ctx, locationID, serviceID)
		if err != nil {
			continue
		}
		
		for _, staffID := range staffIDs {
			sid := staffID
			dates, err := yc.GetBookableDates(ctx, locationID, serviceID, today, farFuture, &sid)
			if err != nil {
				continue
			}
			
			for _, date := range dates {
				times, err := yc.GetBookableTimeslots(ctx, locationID, serviceID, date, staffID)
				if err != nil {
					continue
				}
				
				for _, timeSlot := range times {
					t, err := time.Parse(time.RFC3339, timeSlot)
					if err == nil {
						tt := t.In(loc)
						date := tt.Format("02.01.2006")
						clock := tt.Format("15:04")
						weekday := getRussianWeekday(tt.Weekday())
						slot := fmt.Sprintf("📅 %s (%s) в %s - Сотрудник #%d", date, weekday, clock, staffID)
						allSlots = append(allSlots, slot)
					}
				}
			}
		}
	}
	
	return allSlots, nil
}

func getRussianWeekday(wd time.Weekday) string {
	switch wd {
	case time.Monday:
		return "понедельник"
	case time.Tuesday:
		return "вторник"
	case time.Wednesday:
		return "среда"
	case time.Thursday:
		return "четверг"
	case time.Friday:
		return "пятница"
	case time.Saturday:
		return "суббота"
	case time.Sunday:
		return "воскресенье"
	default:
		return ""
	}
}
