package notificator

import (
	"encoding/json"
	"fmt"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
)

type Notificator struct {
	logger *logger.Logger
}

func NewNotificator(logger *logger.Logger) *Notificator {
	return &Notificator{logger: logger}
}

func (n *Notificator) SendNotification(url string, notification *models.Notification) {
	data, err := json.Marshal(notification)
	if err != nil {
		n.logger.Error("Failed to marshal notification data: ", err)
		return
	}

	// resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	// if err != nil {
	// 	n.logger.Error("Failed to send notification: ", err)
	// 	return
	// }
	// defer resp.Body.Close()

	// if resp.StatusCode != http.StatusOK {
	// 	n.logger.Error("Received non-OK response: ", resp.Status)
	// }
	fmt.Println("Notification sent: ", string(data))
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