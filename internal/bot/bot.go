package bot

import (
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/thatguy/moto_gorod-notifier/internal/logger"
)

// Bot wraps Telegram bot operations and stores subscriptions in database.
type Bot struct {
	api          *tgbotapi.BotAPI
	log          *logger.Logger
	currentSlotsFn func() ([]string, error)
	bookingURL   string
	templateRenderer TemplateRenderer
	storage      Storage
}

type Storage interface {
	AddSubscriber(chatID int64) error
	RemoveSubscriber(chatID int64) error
	GetSubscribers() ([]int64, error)
}

type TemplateRenderer interface {
	GetWelcomeMessage() string
	GetGoodbyeMessage() string
	GetCurrentSlotsMessage(slots []string) string
}

func New(token string, storage Storage, log *logger.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	
	bot := &Bot{
		api:         api,
		log:         log,
		bookingURL:  "https://n841217.yclients.com/",
		storage:     storage,
	}
	
	bot.log.InfoWithFields("Telegram bot initialized", logger.Fields{
		"bot_username": api.Self.UserName,
		"bot_id":       api.Self.ID,
	})
	
	return bot, nil
}

func (b *Bot) Run(ctx context.Context) {
	b.log.Info("Starting Telegram bot updates loop")
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 10 // Shorter timeout for better responsiveness
	updates := b.api.GetUpdatesChan(u)

	defer func() {
		b.api.StopReceivingUpdates()
		b.log.Info("Telegram bot updates loop stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			b.log.Info("Context canceled, stopping Telegram bot updates loop")
			return
		case upd, ok := <-updates:
			if !ok {
				b.log.Info("Updates channel closed")
				return
			}
			if upd.Message != nil {
				b.handleMessage(upd.Message)
			} else if upd.CallbackQuery != nil {
				b.handleCallbackQuery(upd.CallbackQuery)
			}
		}
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if !msg.IsCommand() {
		return
	}

	chatID := msg.Chat.ID
	command := msg.Command()
	username := msg.From.UserName
	firstName := msg.From.FirstName

	b.log.InfoWithFields("Received command", logger.Fields{
		"command":    command,
		"chat_id":    chatID,
		"username":   username,
		"first_name": firstName,
	})

	switch command {
	case "start":
		b.addSubscriber(chatID)
		subsCount := len(b.Subscribers())
		b.log.InfoWithFields("User subscribed", logger.Fields{
			"chat_id":           chatID,
			"username":          username,
			"total_subscribers": subsCount,
		})
		b.sendWelcomeMessage(chatID)
	case "current":
		b.handleCurrentSlots(chatID)
	case "stop":
		b.removeSubscriber(chatID)
		subsCount := len(b.Subscribers())
		b.log.InfoWithFields("User unsubscribed", logger.Fields{
			"chat_id":           chatID,
			"username":          username,
			"total_subscribers": subsCount,
		})
		b.sendGoodbyeMessage(chatID)
	default:
		b.log.WarnWithFields("Unknown command received", logger.Fields{
			"command":  command,
			"chat_id":  chatID,
			"username": username,
		})
		b.sendHelpMessage(chatID)
	}
}

func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		b.log.WithError(err).WithFields(logger.Fields{
			"chat_id": chatID,
			"message": text,
		}).Error("Failed to send message")
	} else {
		b.log.DebugWithFields("Message sent successfully", logger.Fields{
			"chat_id": chatID,
			"message": text,
		})
	}
}

func (b *Bot) addSubscriber(chatID int64) {
	if err := b.storage.AddSubscriber(chatID); err != nil {
		b.log.WithError(err).Error("Failed to add subscriber")
	}
}

func (b *Bot) removeSubscriber(chatID int64) {
	if err := b.storage.RemoveSubscriber(chatID); err != nil {
		b.log.WithError(err).Error("Failed to remove subscriber")
	}
}

func (b *Bot) Subscribers() []int64 {
	subscribers, err := b.storage.GetSubscribers()
	if err != nil {
		b.log.WithError(err).Error("Failed to get subscribers")
		return []int64{}
	}
	return subscribers
}

func (b *Bot) Notify(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.api.Send(msg)
	if err != nil {
		b.log.WithError(err).WithFields(logger.Fields{
			"chat_id": chatID,
			"message": text,
		}).Error("Failed to send notification")
	} else {
		b.log.InfoWithFields("Notification sent", logger.Fields{
			"chat_id": chatID,
		})
	}
	return err
}

func (b *Bot) SetCurrentSlotsHandler(fn func() ([]string, error)) {
	b.currentSlotsFn = fn
}

func (b *Bot) SetTemplateRenderer(renderer TemplateRenderer) {
	b.templateRenderer = renderer
}

func (b *Bot) sendWelcomeMessage(chatID int64) {
	var text string
	if b.templateRenderer != nil {
		text = b.templateRenderer.GetWelcomeMessage()
	} else {
		text = "🚗 Привет! Я бот автошколы Мото Город."
	}
	keyboard := b.createMainKeyboard()
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) sendGoodbyeMessage(chatID int64) {
	var text string
	if b.templateRenderer != nil {
		text = b.templateRenderer.GetGoodbyeMessage()
	} else {
		text = "👋 Подписка отменена."
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.api.Send(msg)
}

func (b *Bot) sendHelpMessage(chatID int64) {
	text := "ℹ️ Доступные команды:\n\n/start - подписаться на уведомления\n/current - показать текущие слоты\n/stop - отписаться от уведомлений"
	keyboard := b.createMainKeyboard()
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleCurrentSlots(chatID int64) {
	if b.currentSlotsFn == nil {
		b.reply(chatID, "⚠️ Функция проверки слотов недоступна")
		return
	}

	slots, err := b.currentSlotsFn()
	if err != nil {
		b.log.WithError(err).Error("Failed to get current slots")
		b.reply(chatID, "❌ Ошибка при получении информации о слотах")
		return
	}

	var text string
	if b.templateRenderer != nil {
		text = b.templateRenderer.GetCurrentSlotsMessage(slots)
	} else if len(slots) == 0 {
		text = "😔 В данный момент свободных слотов нет"
	} else {
		text = "📅 Доступные слоты:\n\n" + strings.Join(slots, "\n")
	}
	b.reply(chatID, text)
}

func (b *Bot) createMainKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Текущие слоты", "current"),
			tgbotapi.NewInlineKeyboardButtonURL("📝 Записаться", b.bookingURL),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔕 Отписаться", "stop"),
		),
	)
}

func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	data := callback.Data

	b.log.InfoWithFields("Received callback", logger.Fields{
		"data":    data,
		"chat_id": chatID,
	})

	// Answer callback to remove loading state
	callbackAnswer := tgbotapi.NewCallback(callback.ID, "")
	b.api.Request(callbackAnswer)

	switch data {
	case "current":
		b.handleCurrentSlots(chatID)
	case "stop":
		b.removeSubscriber(chatID)
		subsCount := len(b.Subscribers())
		b.log.InfoWithFields("User unsubscribed via button", logger.Fields{
			"chat_id":           chatID,
			"total_subscribers": subsCount,
		})
		b.sendGoodbyeMessage(chatID)
	default:
		b.log.WarnWithFields("Unknown callback data", logger.Fields{
			"data":    data,
			"chat_id": chatID,
		})
	}
}
