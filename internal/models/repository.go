package models

type Repository interface {
	AddNewWallet(*Wallet) error
	CheckWalletExists(address string) (bool, error)
	GetWallet(address string) (*Wallet, error)
	GetWalletBySubscriptionAddress(subscriptionAddress string) (*Wallet, error)
	IsSubscriptionAddress(address string) (bool, error)
	UpdateWalletPaidStatus(address string, paid bool) error

	AddSubscriptionPayment(subscriptionAddress string, amount float64, timestamp int64) error
	GetSubscriptionPayments(subscriptionAddress string) ([]*SubscriptionPayment, error)

	RemoveOldSubscriptionPayments(timestamp int64) error
	RemoveUnpaidSubscriptions(timestamp int64) error
}
