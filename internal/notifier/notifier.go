package notifier

import (
	"context"
	"bytes"
	"fmt"
	"time"
	"text/template"

	"github.com/thatguy/moto_gorod-notifier/internal/bot"
	"github.com/thatguy/moto_gorod-notifier/internal/logger"
	"github.com/thatguy/moto_gorod-notifier/internal/yclients"
)

type Options struct {
	Interval time.Duration
	Timezone string
	LocationID int
	ServiceIDs []int
}

type Notifier struct {
	bot       *bot.Bot
	yc        *yclients.Client
	opts      Options
	templates map[string]*template.Template
	log       *logger.Logger
	storage   Storage
}

type Storage interface {
	IsSlotSeen(slotKey string) (bool, error)
	MarkSlotSeen(slotKey string) error
	CleanOldSlots(olderThan time.Duration) error
}

func New(b *bot.Bot, yc *yclients.Client, opts Options, storage Storage, log *logger.Logger) *Notifier {
	if opts.Interval <= 0 {
		opts.Interval = 30 * time.Second
	}
	n := &Notifier{
		bot:       b,
		yc:        yc,
		opts:      opts,
		templates: make(map[string]*template.Template),
		log:       log,
		storage:   storage,
	}
	
	// Parse all templates
	templateFiles := []string{
		"templates/slot_message.tmpl",
		"templates/welcome_message.tmpl",
		"templates/current_slots.tmpl",
		"templates/no_slots.tmpl",
		"templates/goodbye_message.tmpl",
	}
	
	for _, file := range templateFiles {
		t, err := template.ParseFS(templateFS, file)
		if err != nil {
			n.log.WithError(err).ErrorWithFields("Failed to parse template", logger.Fields{"file": file})
		} else {
			n.templates[file] = t
		}
	}
	
	n.log.InfoWithFields("Templates loaded", logger.Fields{"count": len(n.templates)})
	
	n.log.InfoWithFields("Notifier initialized", logger.Fields{
		"interval":    opts.Interval.String(),
		"timezone":    opts.Timezone,
		"location_id": opts.LocationID,
		"service_ids": opts.ServiceIDs,
	})
	
	return n
}

func (n *Notifier) Run(ctx context.Context) {
	n.log.InfoWithFields("Starting notifier polling loop", logger.Fields{
		"interval": n.opts.Interval.String(),
	})
	
	ticker := time.NewTicker(n.opts.Interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			n.log.Info("Context canceled, stopping notifier")
			return
		case <-ticker.C:
			n.checkAndNotify(ctx)
		}
	}
}

func (n *Notifier) checkAndNotify(ctx context.Context) {
	start := time.Now()
	n.log.Debug("Starting slot availability check")
	
	if len(n.opts.ServiceIDs) == 0 || n.opts.LocationID == 0 {
		n.log.WarnWithFields("Configuration incomplete, skipping check", logger.Fields{
			"location_id": n.opts.LocationID,
			"service_ids": n.opts.ServiceIDs,
		})
		return
	}

	loc, err := time.LoadLocation(n.opts.Timezone)
	if err != nil {
		n.log.WithError(err).WarnWithFields("Failed to load timezone, using fallback", logger.Fields{
			"timezone": n.opts.Timezone,
			"fallback": "UTC+3",
		})
		loc = time.FixedZone("UTC+3", 3*3600)
	}
	
	today := time.Now().In(loc).Format("2006-01-02")
	const farFuture = "9999-01-01"
	
	newSlotsFound := 0
	totalChecks := 0
	
	for _, serviceID := range n.opts.ServiceIDs {
		n.log.DebugWithFields("Checking service", logger.Fields{
			"service_id": serviceID,
		})
		
		staffIDs, err := n.yc.GetBookableStaffIDs(ctx, n.opts.LocationID, serviceID)
		if err != nil {
			n.log.WithError(err).ErrorWithFields("Failed to get staff IDs", logger.Fields{
				"service_id": serviceID,
			})
			continue
		}
		
		if len(staffIDs) == 0 {
			n.log.DebugWithFields("No bookable staff found", logger.Fields{
				"service_id": serviceID,
			})
			continue
		}
		
		n.log.DebugWithFields("Found bookable staff", logger.Fields{
			"service_id": serviceID,
			"staff_ids":  staffIDs,
		})
		
		for _, staffID := range staffIDs {
			sid := staffID
			dates, err := n.yc.GetBookableDates(ctx, n.opts.LocationID, serviceID, today, farFuture, &sid)
			if err != nil {
				n.log.WithError(err).ErrorWithFields("Failed to get bookable dates", logger.Fields{
					"service_id": serviceID,
					"staff_id":   staffID,
				})
				continue
			}
			
			for _, date := range dates {
				times, err := n.yc.GetBookableTimeslots(ctx, n.opts.LocationID, serviceID, date, staffID)
				if err != nil {
					n.log.WithError(err).ErrorWithFields("Failed to get timeslots", logger.Fields{
						"service_id": serviceID,
						"staff_id":   staffID,
						"date":       date,
					})
					continue
				}
				
				for _, t := range times {
					totalChecks++
					key := n.buildKey(serviceID, staffID, t)
					seen, err := n.storage.IsSlotSeen(key)
					if err != nil {
						n.log.WithError(err).Error("Failed to check if slot seen")
						continue
					}
					if seen {
						continue
					}
					
					if err := n.storage.MarkSlotSeen(key); err != nil {
						n.log.WithError(err).Error("Failed to mark slot as seen")
					}
					newSlotsFound++
					
					n.log.InfoWithFields("New slot found", logger.Fields{
						"service_id": serviceID,
						"staff_id":   staffID,
						"date":       date,
						"time":       t,
					})
					
					// Notify subscribers
					msg := n.formatSlotMessage(serviceID, staffID, t)
					subscribers := n.bot.Subscribers()
					
					for _, chatID := range subscribers {
						if err := n.bot.Notify(chatID, msg); err != nil {
							n.log.WithError(err).ErrorWithFields("Failed to notify subscriber", logger.Fields{
								"chat_id": chatID,
							})
						}
					}
					
					n.log.InfoWithFields("Notified subscribers about new slot", logger.Fields{
						"subscribers_count": len(subscribers),
						"service_id":        serviceID,
						"staff_id":          staffID,
					})
				}
			}
		}
	}
	
	duration := time.Since(start)
	// Clean old slots (older than 7 days)
	if err := n.storage.CleanOldSlots(7 * 24 * time.Hour); err != nil {
		n.log.WithError(err).Warn("Failed to clean old slots")
	}
	
	n.log.InfoWithFields("Slot availability check completed", logger.Fields{
		"duration":        duration.String(),
		"new_slots_found": newSlotsFound,
		"total_checks":    totalChecks,
	})
}

func (n *Notifier) buildKey(serviceID, staffID int, datetime string) string {
	return fmt.Sprintf("svc=%d|staff=%d|dt=%s", serviceID, staffID, datetime)
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

func (n *Notifier) formatSlotMessage(serviceID, staffID int, datetime string) string {
	// Try to parse RFC3339 datetime and present it nicely in configured timezone
	loc, err := time.LoadLocation(n.opts.Timezone)
	if err != nil {
		n.log.WithError(err).WarnWithFields("Failed to load timezone for message formatting", logger.Fields{
			"timezone": n.opts.Timezone,
		})
		loc = time.FixedZone("UTC+3", 3*3600)
	}
	
	t, err := time.Parse(time.RFC3339, datetime)
	var date, clock, zone, weekday string
	if err == nil {
		tt := t.In(loc)
		date = tt.Format("02.01.2006")
		clock = tt.Format("15:04")
		zone = tt.Format("MST")
		weekday = getRussianWeekday(tt.Weekday())
	} else {
		n.log.WithError(err).WarnWithFields("Failed to parse datetime, using raw value", logger.Fields{
			"datetime": datetime,
		})
		clock = datetime
	}

	// Resolve human-friendly names
	comp := fmt.Sprintf("%d", n.opts.LocationID)
	svc := fmt.Sprintf("%d", serviceID)
	companyName, ok := CompanyName(comp)
	if !ok {
		companyName = "#" + comp
		n.log.DebugWithFields("Company name not found, using ID", logger.Fields{
			"company_id": comp,
		})
	}
	serviceName, ok := ServiceName(svc)
	if !ok {
		serviceName = "#" + svc
		n.log.DebugWithFields("Service name not found, using ID", logger.Fields{
			"service_id": svc,
		})
	}

	// Render via template if available
	if tmpl, ok := n.templates["templates/slot_message.tmpl"]; ok {
		var buf bytes.Buffer
		err := tmpl.Execute(&buf, struct {
			CompanyName string
			ServiceName string
			StaffID     int
			Date        string
			Time        string
			Zone        string
			Weekday     string
		}{CompanyName: companyName, ServiceName: serviceName, StaffID: staffID, Date: date, Time: clock, Zone: zone, Weekday: weekday})
		
		if err != nil {
			n.log.WithError(err).Error("Failed to execute message template, using fallback")
		} else {
			return buf.String()
		}
	}

	// Fallback template
	if date != "" {
		return fmt.Sprintf("🟢 Доступно окно записи\n\nКомпания: %s\nУслуга: %s\nСотрудник: #%d\nДата: %s (%s)\nВремя: %s %s\n", companyName, serviceName, staffID, date, weekday, clock, zone)
	}
	return fmt.Sprintf("🟢 Доступно окно записи\n\nКомпания: %s\nУслуга: %s\nСотрудник: #%d\nВремя: %s\n", companyName, serviceName, staffID, clock)
}

func (n *Notifier) RenderTemplate(templateName string, data interface{}) string {
	tmpl, ok := n.templates[templateName]
	if !ok {
		n.log.WarnWithFields("Template not found", logger.Fields{"template": templateName})
		return "Template not found"
	}
	
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, data)
	if err != nil {
		n.log.WithError(err).ErrorWithFields("Failed to execute template", logger.Fields{"template": templateName})
		return "Template error"
	}
	
	return buf.String()
}

func (n *Notifier) GetWelcomeMessage() string {
	return n.RenderTemplate("templates/welcome_message.tmpl", nil)
}

func (n *Notifier) GetGoodbyeMessage() string {
	return n.RenderTemplate("templates/goodbye_message.tmpl", nil)
}

func (n *Notifier) GetCurrentSlotsMessage(slots []string) string {
	if len(slots) == 0 {
		return n.RenderTemplate("templates/no_slots.tmpl", nil)
	}
	return n.RenderTemplate("templates/current_slots.tmpl", struct{ Slots []string }{Slots: slots})
}
