package models

type Repository interface {
	AddNewWallet(*Wallet) error
	CheckWalletExists(address string) (bool, error)
	GetWallet(address string) (*Wallet, error)
	IsSubscriptionAddress(address string) (bool, error)

	AddSubscriptionPayment(subscriptionAddress string, amount int64, timestamp int64) error
	GetSubscriptionPayments(subscriptionAddress string) ([]*SubscriptionPayment, error)

	RemoveOldSubscriptionPayments(timestamp int64) error
	RemoveUnpaidSubscriptions(timestamp int64) error
}
