package notificator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
	"github.com/go-telegram/bot"
	tgModels "github.com/go-telegram/bot/models"
)

type TelegramNotificator struct {
	logger      *logger.Logger
	bot         *bot.Bot
	db          models.Repository
	webhookMode bool
}

func NewTelegramNotificator(logger *logger.Logger, token string, db models.Repository, webhookMode bool) *TelegramNotificator {
	provider := &TelegramNotificator{
		logger:      logger,
		db:          db,
		webhookMode: webhookMode,
	}

	// If no token provided, return provider with nil bot (disabled)
	if token == "" {
		logger.Warn("Telegram bot token not provided, Telegram notifications will be disabled")
		return provider
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(provider.handler),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		logger.Error("Failed to initialize Telegram bot, Telegram notifications will be disabled", "error", err)
		return provider
	}

	// Only start polling if not in webhook mode
	if !webhookMode {
		go b.Start(context.Background())
		logger.Info("Telegram bot initialized successfully (polling mode)")
	} else {
		logger.Info("Telegram bot initialized successfully (webhook mode)")
	}

	provider.bot = b
	return provider
}

func (t *TelegramNotificator) SendNotification(chatId, message string) {
	if t.bot == nil {
		t.logger.Warn("Telegram bot unavailable, skipping notification")
		return
	}

	params := &bot.SendMessageParams{
		ChatID: chatId,
		Text:   message,
	}
	_, err := t.bot.SendMessage(context.Background(), params)
	if err != nil {
		t.logger.Error("Failed to send notification: ", err)
	}
}

func (t *TelegramNotificator) handler(ctx context.Context, b *bot.Bot, update *tgModels.Update) {
	if update.Message == nil {
		t.logger.Debug("Telegram update without message payload received")
		return
	}
	t.logger.Debug("Telegram update: ", update.Message.From.Username, " ", update.Message.Text)
	user := update.Message.From
	if user == nil {
		t.logger.Error("User is nil")
		return
	}
	if update.Message.Text == "/start" {
		providers, err := t.db.GetNotificationProvidersByTelegramUsername(user.Username)
		if err != nil {
			t.logger.Error("Failed to get notification provider by telegram username: ", err, " username: ", user.Username)
			return
		}
		if len(providers) == 0 {
			t.logger.Error("Notification providers not found for username: ", user.Username)
			return
		}
		t.logger.Info("Telegram providers found: ", len(providers))
		chatID := fmt.Sprint(update.Message.Chat.ID)
		if err := t.db.AddTelegramProviderChatID(user.Username, chatID); err != nil {
			t.logger.Error("Failed to add telegram provider chat ID: ", err)
			return
		}
		t.logger.Info("Telegram provider chat ID added successfully")
		addresses := make([]string, 0, len(providers))
		for _, provider := range providers {
			addresses = append(addresses, provider.Address)
		}
		message := "You have successfully subscribed to notifications."
		if len(addresses) > 0 {
			message = fmt.Sprintf("%s Addresses: %s", message, strings.Join(addresses, ", "))
		}
		t.SendNotification(chatID, message)
	}
}

// SetWebhook configures the Telegram webhook URL with retry logic and exponential backoff
func (t *TelegramNotificator) SetWebhook(webhookURL string) error {
	if t.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	ctx := context.Background()
	maxRetries := 5
	baseBackoff := 2 // Start with 2 seconds as Telegram suggests "retry after 1"

	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err := t.bot.SetWebhook(ctx, &bot.SetWebhookParams{
			URL: webhookURL,
		})
		if err == nil {
			t.logger.Info("Telegram webhook configured successfully", "url", webhookURL)
			return nil
		}

		// Check if it's a rate limit error
		errMsg := err.Error()
		if !containsRateLimit(errMsg) {
			// Not a rate limit error, fail immediately
			return fmt.Errorf("failed to set webhook: %w", err)
		}

		// Calculate backoff with exponential increase
		backoff := baseBackoff * (1 << attempt) // 2s, 4s, 8s, 16s, 32s
		if backoff > 60 {
			backoff = 60 // Cap at 60 seconds
		}

		if attempt < maxRetries-1 {
			t.logger.Warn("Rate limited by Telegram API, retrying",
				"attempt", attempt+1,
				"max_retries", maxRetries,
				"backoff_seconds", backoff,
				"error", err)
			time.Sleep(time.Duration(backoff) * time.Second)
		}
	}

	return fmt.Errorf("failed to set webhook after %d retries due to rate limiting", maxRetries)
}

// containsRateLimit checks if an error message indicates a rate limit
func containsRateLimit(errMsg string) bool {
	rateLimitIndicators := []string{
		"too many requests",
		"retry after",
		"rate limit",
		"Too Many Requests",
	}
	for _, indicator := range rateLimitIndicators {
		if len(errMsg) > 0 && len(indicator) > 0 &&
			(errMsg == indicator || len(errMsg) > len(indicator) &&
			(errMsg[:len(indicator)] == indicator || contains(errMsg, indicator))) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ProcessUpdate processes a webhook update
func (t *TelegramNotificator) ProcessUpdate(update *tgModels.Update) error {
	if t.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	// Process the update using the existing handler
	t.handler(context.Background(), t.bot, update)
	return nil
}
