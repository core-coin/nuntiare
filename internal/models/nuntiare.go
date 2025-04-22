package models

type NuntiareI interface {
	// Start starts the application
	Start()

	// RegisterNewWallet adds a new wallet to the repository
	RegisterNewWallet(*Wallet) error
	// GetWallet returns a wallet from the repository
	GetWallet(address string) (*Wallet, error)

	// NewHeaderSubscription creates a new header subscription
	WatchTransfers()

	// // CheckWalletSubscription check at the moment of call the CTN balance of the wallet.
	// // If the balance is > 0, it adds a subscriptio
	// CheckWalletInitialSubscription(subscriptionAddress string) error

	// CheckWalletSubscription checks if the wallet is subscribed.
	// Data is taken from the repository.
	CheckWalletSubscription(wallet *Wallet) (bool, error)
}
