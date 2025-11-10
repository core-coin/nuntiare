package models

import (
	"fmt"
	"math/big"
	"strings"
)

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
		// Convert hex token ID to decimal for better readability
		tokenID := n.TokenID
		tokenIDStr := strings.TrimPrefix(tokenID, "0x")
		if tokenIDBig, ok := new(big.Int).SetString(tokenIDStr, 16); ok {
			tokenID = tokenIDBig.String() // Decimal representation
		}
		return fmt.Sprintf("Received %v NFT (ID: %v) on address %v", n.Currency, tokenID, n.Wallet)
	}
	// Format amount to avoid scientific notation and strip trailing zeros
	amountStr := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.18f", n.Amount), "0"), ".")
	return fmt.Sprintf("Received %v %v on address %v", amountStr, n.Currency, n.Wallet)
}
