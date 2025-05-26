package notificator

import (
	"context"

	"fmt"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
	"github.com/go-telegram/bot"
	tgModels "github.com/go-telegram/bot/models"
)

type TelegramNotificator struct {
	logger *logger.Logger
	bot    *bot.Bot

	db models.Repository
}

func NewTelegramNotificator(logger *logger.Logger, token string, db models.Repository) *TelegramNotificator {
	provider := &TelegramNotificator{
		logger: logger,
		db:     db,
	}
	opts := []bot.Option{
		bot.WithDefaultHandler(provider.handler),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		panic(err)
	}
	go b.Start(context.Background())
	provider.bot = b

	return provider
}

func (t *TelegramNotificator) SendNotification(chatId, message string) {
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
	t.logger.Debug("Telegram update: ", update.Message.From.Username, " ", update.Message.Text)
	user := update.Message.From
	if user == nil {
		t.logger.Error("User is nil")
		return
	}
	if update.Message.Text == "/start" {
		provider, err := t.db.GetNotificationProviderByTelegramUsername(user.Username)
		if err != nil {
			t.logger.Error("Failed to get notification provider by telegram username: ", err, " username: ", user.Username)
			return
		}
		if provider == nil {
			t.logger.Error("Notification provider is nil")
			return
		}
		t.logger.Info("Telegram provider found: ", provider)
		if err := t.db.AddTelegramProviderChatID(provider.Address, fmt.Sprint(update.Message.Chat.ID)); err != nil {
			t.logger.Error("Failed to add telegram provider chat ID: ", err)
			return
		}
		t.logger.Info("Telegram provider chat ID added successfully")
		t.SendNotification(fmt.Sprint(update.Message.Chat.ID), "You have successfully subscribed to notifications. Address: "+provider.Address)
	}
}
