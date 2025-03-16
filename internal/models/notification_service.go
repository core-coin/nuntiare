package models

import "math/big"

type NotificationService interface {
	SendNotification(url string, notification *Notification)
}

type Notification struct {
	Wallet   string   `json:"wallet"`
	Amount   *big.Int `json:"amount"`
	Currency string   `json:"currency"`
}
