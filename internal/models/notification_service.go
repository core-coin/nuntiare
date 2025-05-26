package models

import "fmt"

type NotificationService interface {
	SendNotification(notification *Notification)
}

type Notification struct {
	Wallet   string  `json:"wallet"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

func (n *Notification) String() string {
	return fmt.Sprintf("Received %v %v on address %v", n.Amount, n.Currency, n.Wallet)
}
