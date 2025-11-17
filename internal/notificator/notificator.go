package notificator

import (
	"runtime/debug"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
)

type Notificator struct {
	logger *logger.Logger
	db     models.Repository

	TelegramNotificator *TelegramNotificator
	EmailNotificator    *EmailNotificator
}

func NewNotificator(logger *logger.Logger, db models.Repository, telNotif *TelegramNotificator, emailNotif *EmailNotificator) *Notificator {
	return &Notificator{logger: logger, db: db, TelegramNotificator: telNotif, EmailNotificator: emailNotif}
}

// safeCall runs a function with panic recovery (synchronous, no goroutine spawning)
func (n *Notificator) safeCall(fn func(), context string) {
	defer func() {
		if r := recover(); r != nil {
			n.logger.Error("Function panicked",
				"context", context,
				"panic", r,
				"stack", string(debug.Stack()))
		}
	}()
	fn()
}

func (n *Notificator) SendNotification(notification *models.Notification) {
	notificationProvider, err := n.db.GetWalletsNotificationProvider(notification.Wallet)
	if err != nil {
		n.logger.Error("Failed to get notification provider: ", err)
		return
	}
	if notificationProvider == nil {
		n.logger.Error("Notification provider not found for wallet: ", notification.Wallet)
		return
	}

	// Send notifications synchronously (we're already in a goroutine from nuntiare.safeGo)
	// This prevents untracked goroutine spawning
	if notificationProvider.TelegramProvider.ChatID != "" {
		chatID := notificationProvider.TelegramProvider.ChatID
		message := notification.String()
		n.safeCall(func() { n.TelegramNotificator.SendNotification(chatID, message) }, "telegramNotification")
	}
	if notificationProvider.EmailProvider.Email != "" {
		email := notificationProvider.EmailProvider.Email
		message := notification.String()
		n.safeCall(func() { n.EmailNotificator.SendNotification(email, message) }, "emailNotification")
	}
}

/*


type Notificator struct {
    logger *logger.Logger
    client *apns2.Client
}

func NewNotificator(logger *logger.Logger, certPath, certPassword string) (*Notificator, error) {
    cert, err := certificate.FromP12File(certPath, certPassword)
    if err != nil {
        return nil, fmt.Errorf("failed to load APNs certificate: %w", err)
    }

    client := apns2.NewClient(cert).Production()
    return &Notificator{logger: logger, client: client}, nil
}

func (n *Notificator) SendNotification(deviceToken string, notification *models.Notification) {
    data, err := json.Marshal(notification)
    if err != nil {
        n.logger.Error("Failed to marshal notification data: ", err)
        return
    }

    payload := payload.NewPayload().Alert(string(data))
    notification := &apns2.Notification{
        DeviceToken: deviceToken,
        Topic:       "com.yourapp.bundleid", // Replace with your app's bundle ID
        Payload:     payload,
    }

    res, err := n.client.Push(notification)
    if err != nil {
        n.logger.Error("Failed to send notification: ", err)
        return
    }

    if res.Sent() {
        fmt.Println("Notification sent successfully")
    } else {
        n.logger.Error("Failed to send notification: ", res.Reason)
    }
}

*/
