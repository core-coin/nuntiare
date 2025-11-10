package models

// Token represents a token contract that is being watched for transfers
type Token struct {
	// Address is the contract address of the token
	Address string `json:"address"`
	// Name is the full name of the token
	Name string `json:"name"`
	// Symbol is the short symbol of the token (e.g., CTN, USDT)
	Symbol string `json:"symbol"`
	// Decimals is the number of decimals the token uses
	Decimals int `json:"decimals"`
	// Type is the token type (CBC20, CBC721, etc.)
	Type string `json:"type"`
	// Network is the network the token is on (mainnet, devin, etc.)
	Network string `json:"network"`
	// UpdatedAt is the timestamp when the token info was last updated
	UpdatedAt int64 `json:"updated_at"`
}
