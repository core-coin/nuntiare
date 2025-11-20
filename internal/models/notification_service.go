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
	Wallet        string  `json:"wallet"` // Recipient address
	From          string  `json:"from"`   // Sender address
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`       // Token symbol (e.g., CTN, USDT, XCB)
	TokenAddress  string  `json:"token_address"`  // Contract address (empty for XCB)
	TokenType     string  `json:"token_type"`     // CBC20, CBC721, or empty for native XCB
	TokenID       string  `json:"token_id"`       // For NFT transfers (CBC721)
	TxHash        string  `json:"tx_hash"`        // Transaction hash
	NetworkID     int64   `json:"network_id"`     // Network ID (1 for mainnet, 3 for devnet)
	CustomMessage string  `json:"custom_message"` // Custom message overrides default formatting
}

func (n *Notification) String() string {
	// If custom message is set, use it instead of default formatting
	if n.CustomMessage != "" {
		return n.CustomMessage
	}

	// Determine explorer base URL based on network ID
	var explorerURL string
	if n.NetworkID == 3 {
		explorerURL = "https://devin.blockindex.net/tx/"
	} else {
		// Default to mainnet (network ID 1)
		explorerURL = "https://blockindex.net/tx/"
	}

	txLink := explorerURL + n.TxHash

	if n.TokenType == "CBC721" {
		// Convert hex token ID to decimal for better readability
		tokenID := n.TokenID
		tokenIDStr := strings.TrimPrefix(tokenID, "0x")
		if tokenIDBig, ok := new(big.Int).SetString(tokenIDStr, 16); ok {
			tokenID = tokenIDBig.String() // Decimal representation
		}
		return fmt.Sprintf("Received NFT %v (ID: %v) from %v to address %v\nTransaction: %v", n.Currency, tokenID, n.From, n.Wallet, txLink)
	}
	// Format amount to avoid scientific notation and strip trailing zeros
	amountStr := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.18f", n.Amount), "0"), ".")
	return fmt.Sprintf("Received %v %v from %v to address %v\nTransaction: %v", amountStr, n.Currency, n.From, n.Wallet, txLink)
}
