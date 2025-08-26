package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application configuration loaded from environment variables.
// Required: TELEGRAM_TOKEN, YCLIENTS_LOGIN, YCLIENTS_PASSWORD, YCLIENTS_PARTNER_TOKEN, YCLIENTS_FORM_ID
// Optional: YCLIENTS_COMPANY_ID (default 780413), TIMEZONE (default Europe/Moscow), CHECK_INTERVAL_SECONDS (default 60s)

type Config struct {
	TelegramToken        string
	YClientsLogin        string
	YClientsPassword     string
	YClientsPartnerToken string
	YClientsCompanyID    string
	YClientsFormID       string
	Timezone             string
	ServiceIDs           []int
	PollInterval         time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load() // ignore error if .env doesn't exist

	cfg := Config{
		TelegramToken:        os.Getenv("TELEGRAM_TOKEN"),
		YClientsLogin:        os.Getenv("YCLIENTS_LOGIN"),
		YClientsPassword:     os.Getenv("YCLIENTS_PASSWORD"),
		YClientsPartnerToken: os.Getenv("YCLIENTS_PARTNER_TOKEN"),
		YClientsCompanyID:    firstNonEmpty(os.Getenv("YCLIENTS_COMPANY_ID"), "780413"),
		YClientsFormID:       os.Getenv("YCLIENTS_FORM_ID"),
		Timezone:             firstNonEmpty(os.Getenv("TIMEZONE"), "Europe/Moscow"),
		PollInterval:         60 * time.Second,
	}

	if s := strings.TrimSpace(os.Getenv("YCLIENTS_SERVICE_IDS")); s != "" {
		parts := strings.Split(s, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if n, err := strconv.Atoi(p); err == nil {
				cfg.ServiceIDs = append(cfg.ServiceIDs, n)
			} else {
				// Log invalid service ID but continue
				fmt.Printf("Warning: invalid service ID '%s' ignored\n", p)
			}
		}
	}

	if s := strings.TrimSpace(os.Getenv("CHECK_INTERVAL_SECONDS")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			cfg.PollInterval = time.Duration(n) * time.Second
		}
	}

	if cfg.TelegramToken == "" || cfg.YClientsLogin == "" || cfg.YClientsPassword == "" || cfg.YClientsPartnerToken == "" || cfg.YClientsFormID == "" {
		return Config{}, errors.New("missing required env vars: TELEGRAM_TOKEN, YCLIENTS_LOGIN, YCLIENTS_PASSWORD, YCLIENTS_PARTNER_TOKEN, YCLIENTS_FORM_ID")
	}
	return cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (c Config) String() string {
	// mask tokens for logs
	mask := func(s string) string {
		if len(s) <= 6 {
			return "***"
		}
		return s[:3] + "***" + s[len(s)-3:]
	}
	return fmt.Sprintf("Config{Telegram:%s, YClientsLogin:%s, PartnerToken:%s, CompanyID:%s, FormID:%s, TZ:%s, Interval:%s, ServiceIDs:%v}",
		mask(c.TelegramToken), mask(c.YClientsLogin), mask(c.YClientsPartnerToken), c.YClientsCompanyID, c.YClientsFormID, c.Timezone, c.PollInterval, c.ServiceIDs,
	)
}
