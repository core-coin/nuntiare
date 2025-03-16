package models

// / Wallet represents a wallet in the system.
type Wallet struct {
	// Origin is the company name who is issuing it
	Origin string `json:"origin" gorm:"column:origin"`
	// Address is the address of the wallet.
	Address string `json:"address" gorm:"column:address;primaryKey"`
	// SubscriptionAddress is the address which we use to check if user has subscription.
	SubscriptionAddress string `json:"subscription_address" gorm:"column:subscription_address"`
	// WebHookURL is the URL to send notifications to.
	HookURL string `json:"hook_url" gorm:"column:hook_url"`
	// WalletType is the type of the wallet. (ican, iban etc.)
	WalletType string `json:"wallet_type" gorm:"column:wallet_type"`
	// Network is the network the wallet is on. (xcb, btc etc.)
	Network string `json:"network" gorm:"column:network"`
	// CreatedAt is the date when the wallet was created.
	CreatedAt int64 `json:"created_at" gorm:"column:created_at"`
	// Whitelisted is a flag indicating if the wallet is whitelisted.
	Whitelisted bool `json:"whitelisted" gorm:"column:whitelisted"`
	// Paid is a flag indicating if the wallet has paid for the subscription.
	Paid bool `json:"paid" gorm:"column:paid"`
}

type SubscriptionPayment struct {
	// Address is the address of the wallet.
	Address string `json:"address" gorm:"column:address;primaryKey"`
	// Amount is the amount of CTN paid for the subscription.
	Amount int64 `json:"amount" gorm:"column:amount"`
	// Timestamp is the date when the payment was made.
	Timestamp int64 `json:"timestamp" gorm:"column:timestamp"`
}
