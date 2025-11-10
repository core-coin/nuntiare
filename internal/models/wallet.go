package models

// Wallet represents a wallet in the system.
type Wallet struct {
	// Originator is the company name who is issuing it
	Originator string `json:"originator" gorm:"column:originator"`
	// Address is the address of the wallet.
	Address string `json:"address" gorm:"column:address;primaryKey"`
	// SubscriptionAddress is the address which we use to check if user has subscription.
	SubscriptionAddress string `json:"subscription_address" gorm:"column:subscription_address"`
	// Network is the network the wallet is on. (xcb, btc etc.)
	Network string `json:"network" gorm:"column:network"`
	// CreatedAt is the date when the wallet was created.
	CreatedAt int64 `json:"created_at" gorm:"column:created_at"`
	// Whitelisted is a flag indicating if the wallet is whitelisted.
	Whitelisted bool `json:"whitelisted" gorm:"column:whitelisted"`
	// Paid is a flag indicating if the wallet has paid for the subscription.
	Paid bool `json:"paid" gorm:"column:paid"`
	// SubscriptionExpiresAt is the Unix timestamp when the subscription expires.
	SubscriptionExpiresAt int64 `json:"subscription_expires_at" gorm:"column:subscription_expires_at"`
	// NotificationProvider is the associated notification provider for the wallet.
	NotificationProvider NotificationProvider `json:"notification_provider" gorm:"foreignKey:Address;references:Address;constraint:OnDelete:CASCADE"`
}

type SubscriptionPayment struct {
	// ID is the unique identifier for the payment.
	ID int64 `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	// Address is the address of the wallet.
	Address string `json:"address" gorm:"column:address"`
	// Amount is the amount of CTN paid for the subscription.
	Amount float64 `json:"amount" gorm:"column:amount"`
	// Timestamp is the date when the payment was made.
	Timestamp int64 `json:"timestamp" gorm:"column:timestamp"`
}
