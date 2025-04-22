package models

type NotificationService interface {
	SendNotification(url string, notification *Notification)
}

type Notification struct {
	Wallet   string `json:"wallet"`
	Amount   float64 `json:"amount"`
	Currency string `json:"currency"`
}
