package models

// Wallet represents a wallet in the system.
type Wallet struct {
	// Originator is the company name who is issuing it
	Originator string `json:"originator" gorm:"column:originator"`
	// Address is the destination wallet address that receives notifications.
	Address string `json:"address" gorm:"column:address;primaryKey"`
	// SubscriptionAddress is the subscriber/payer address that sends payment to RECEIVING_ADDRESS.
	// We watch for payments FROM this address TO the shared RECEIVING_ADDRESS (from config).
	// This identifies which wallet's subscription is being paid.
	SubscriptionAddress string `json:"subscription_address" gorm:"column:subscription_address;index;unique"`
	// OriginID is a unique identifier for authentication of update/cancel operations.
	// Format: alphanumeric string, 32 characters (from crypto.randomUUID())
	OriginID string `json:"originid" gorm:"column:originid;index;not null"`
	// Network is the network the wallet is on. (xcb, btc etc.)
	Network string `json:"network" gorm:"column:network"`
	// OS is the operating system of the user (ios, android, web, etc.)
	OS string `json:"os" gorm:"column:os"`
	// Lang is the language preference of the user (en, es, fr, etc.)
	Lang string `json:"lang" gorm:"column:lang"`
	// CreatedAt is the date when the wallet was created.
	CreatedAt int64 `json:"created_at" gorm:"column:created_at;index"`
	// Active indicates if notifications are enabled. User can cancel notifications while keeping subscription.
	Active bool `json:"active" gorm:"column:active;default:true"`
	// Whitelisted is a flag indicating if the wallet is whitelisted.
	Whitelisted bool `json:"whitelisted" gorm:"column:whitelisted"`
	// Paid is a flag indicating if the wallet has paid for the subscription.
	Paid bool `json:"paid" gorm:"column:paid;index"`
	// SubscriptionExpiresAt is the Unix timestamp when the subscription expires.
	SubscriptionExpiresAt int64 `json:"subscription_expires_at" gorm:"column:subscription_expires_at"`
	// NotificationProvider is the associated notification provider for the wallet.
	NotificationProvider NotificationProvider `json:"notification_provider" gorm:"foreignKey:Address;references:Address;constraint:OnDelete:CASCADE"`
}

type SubscriptionPayment struct {
	// ID is the unique identifier for the payment.
	ID int64 `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	// Address is the subscriber/payer address that sent the payment.
	// This matches Wallet.SubscriptionAddress to identify which wallet paid.
	Address string `json:"address" gorm:"column:address;index"`
	// Amount is the amount of CTN paid for the subscription.
	Amount float64 `json:"amount" gorm:"column:amount"`
	// Timestamp is the date when the payment was made.
	Timestamp int64 `json:"timestamp" gorm:"column:timestamp"`
}
