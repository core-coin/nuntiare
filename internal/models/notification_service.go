package models

import "fmt"

type NotificationService interface {
	SendNotification(notification *Notification)
}

type Notification struct {
	Wallet       string  `json:"wallet"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`      // Token symbol (e.g., CTN, USDT, XCB)
	TokenAddress string  `json:"token_address"` // Contract address (empty for XCB)
	TokenType    string  `json:"token_type"`    // CBC20, CBC721, or empty for native XCB
	TokenID      string  `json:"token_id"`      // For NFT transfers (CBC721)
}

func (n *Notification) String() string {
	if n.TokenType == "CBC721" {
		return fmt.Sprintf("Received %v NFT (ID: %v) on address %v", n.Currency, n.TokenID, n.Wallet)
	}
	return fmt.Sprintf("Received %v %v on address %v", n.Amount, n.Currency, n.Wallet)
}
