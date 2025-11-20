package models

type Repository interface {
	AddNewWallet(*Wallet) error
	CheckWalletExists(address string) (bool, error)
	GetWallet(address string) (*Wallet, error)
	GetWalletBySubscriptionAddress(subscriptionAddress string) (*Wallet, error)
	UpdateWalletPaidStatus(address string, paid bool) error
	UpdateWalletSubscriptionExpiration(address string, expiresAt int64) error

	AddSubscriptionPayment(subscriptionAddress string, amount float64, timestamp int64) error
	GetSubscriptionPayments(subscriptionAddress string) ([]*SubscriptionPayment, error)

	RemoveOldSubscriptionPayments(timestamp int64) error
	RemoveUnpaidSubscriptions(timestamp int64) error

	GetWalletsNotificationProvider(address string) (*NotificationProvider, error)
	UpdateNotificationProvider(address, telegram, email string) error
	UpdateWalletMetadata(address, os, lang string) error
	SetWalletActive(address string, active bool) error

	AddTelegramProviderChatID(username, chatID string) error
	GetNotificationProvidersByTelegramUsername(username string) ([]*NotificationProvider, error)

	// Distributed lock methods for HA
	TryAcquireLock(lockName, instanceID string, ttlSeconds int) (bool, error)
	ReleaseLock(lockName, instanceID string) error
	CleanupExpiredLocks() error

	// Lifecycle management
	Close() error
}
