package nuntiare

import (
	"fmt"
	"math/big"
	"runtime/debug"
	"time"

	"github.com/core-coin/go-core/v2/core/types"

	"github.com/core-coin/nuntiare/internal/blockchain"
	"github.com/core-coin/nuntiare/internal/config"
	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
)

const (
	// minimalBalance is the minimum balance for a wallet to be notified
	minimalBalance = 200 // 200 CTN
)

// TokenCache interface for getting cached tokens
type TokenCache interface {
	GetAllTokens() []*models.Token
}

// Nuntiare is the main struct for the Nuntiare application
// It contains all the necessary components to run the application
// and serves all business logic
type Nuntiare struct {
	logger *logger.Logger
	config *config.Config

	repo        models.Repository
	gocore      models.BlockchainService
	notificator models.NotificationService
	tokenCache  TokenCache
}

// NewNuntiare creates a new Nuntiare instance
func NewNuntiare(
	repo models.Repository,
	gocore models.BlockchainService,
	notificator models.NotificationService,
	tokenCache TokenCache,
	logger *logger.Logger,
	config *config.Config,
) models.NuntiareI {
	return &Nuntiare{
		repo:        repo,
		gocore:      gocore,
		logger:      logger,
		notificator: notificator,
		tokenCache:  tokenCache,
		config:      config,
	}
}

// safeGo runs a function in a goroutine with panic recovery
func (n *Nuntiare) safeGo(fn func(), context string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				n.logger.Error("Goroutine panicked",
					"context", context,
					"panic", r,
					"stack", string(debug.Stack()))
			}
		}()
		fn()
	}()
}

// shouldNotifyWallet checks if a wallet should receive notifications
// Returns the wallet and whether it should be notified
// Optimization: Skips subscription check for whitelisted wallets
func (n *Nuntiare) shouldNotifyWallet(address string) (*models.Wallet, bool, error) {
	exists, err := n.IsRegistered(address)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check registration: %w", err)
	}
	if !exists {
		return nil, false, nil
	}

	wallet, err := n.repo.GetWallet(address)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get wallet: %w", err)
	}

	// Optimization: Skip subscription check for whitelisted wallets (saves DB query)
	if wallet.Whitelisted {
		return wallet, true, nil
	}

	subscribed, err := n.CheckWalletSubscription(wallet)
	if err != nil {
		return wallet, false, fmt.Errorf("failed to check subscription: %w", err)
	}

	return wallet, subscribed, nil
}

// weiToXCB converts Wei to XCB (1 XCB = 10^18 Wei)
func weiToXCB(wei *big.Int) float64 {
	weiFloat := new(big.Float).SetInt(wei)
	xcbFloat := new(big.Float).Quo(weiFloat, big.NewFloat(1e18))
	amount, _ := xcbFloat.Float64()
	return amount
}

// Start starts the Nuntiare application
func (n *Nuntiare) Start() {
	// Start a goroutine to remove old subscription payments
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			n.logger.Debug("Removing old subscription payments")
			err := n.repo.RemoveOldSubscriptionPayments(time.Now().Unix() - int64(30*24*time.Hour.Seconds())) // every payment is 200 CTN and it is subscription for 1 month
			if err != nil {
				n.logger.Error("Failed to remove old subscription payments", "error", err)
			}
			err = n.repo.RemoveUnpaidSubscriptions(time.Now().Unix() - int64(10*time.Minute.Seconds())) // if user doesn't pay for subscription in 10 minutes, remove it
			if err != nil {
				n.logger.Error("Failed to remove unpaid subscriptions", "error", err)
			}
		}
	}()
	// Start watching for new transactions
	n.WatchTransfers()
}

// RegisterNewWallet adds a new wallet to the repository
func (n *Nuntiare) RegisterNewWallet(wallet *models.Wallet) error {
	// err := n.CheckWalletInitialSubscription(wallet.SubscriptionAddress)
	// if err != nil {
	// 	n.logger.Error("failed to check wallet initial subscription", "error", err)
	// 	return fmt.Errorf("failed to check wallet initial subscription: %s", err) // todo:error2215 do we need to terminate the registration process if the initial subscription check fails?
	// }

	return n.repo.AddNewWallet(wallet)
}

// IsRegistered checks if the given address is registered
func (n *Nuntiare) IsRegistered(address string) (bool, error) {
	return n.repo.CheckWalletExists(address)
}

// WatchTransfers starts watching for new transfers inside blockchain
// If tx receiver is a registered wallet, it sends a notification if wallet has subscribtion
func (n *Nuntiare) WatchTransfers() {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		channel, err := n.gocore.NewHeaderSubscription()
		if err != nil {
			n.logger.Error("Failed to subscribe to new head, will retry", "error", err, "retry_in", backoff)
			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Reset backoff on successful connection
		backoff = 1 * time.Second
		n.logger.Info("Successfully subscribed to blockchain headers")

		for header := range channel {
			n.logger.Debug("New block header received", "number", header.Number)

			// Check if the block has transactions
			if !header.EmptyBody() {
				n.logger.Debug("Block has transactions")
				block, err := n.gocore.GetBlockByNumber(header.Number.Uint64())
				if err != nil {
					n.logger.Error("Failed to get block by number", "number", header.Number, "error", err)
					continue
				}
				n.checkBlock(block)
			}
		}
		n.logger.Error("Channel closed, restarting subscription")
		time.Sleep(backoff)
	}

}

func (n *Nuntiare) checkBlock(block *types.Block) {
	// Get all watched tokens from in-memory cache
	tokens := n.tokenCache.GetAllTokens()

	// Build address->token map for O(1) lookup instead of O(n) iteration
	tokensByAddress := make(map[string]*models.Token, len(tokens))
	for _, token := range tokens {
		tokensByAddress[token.Address] = token
	}

	for _, tx := range block.Body().Transactions {
		// Skip contract creation transactions
		if tx.To() == nil {
			continue
		}

		receiver := tx.To().Hex()
		var allTransfers []*blockchain.Transfer

		// Check for CTN transfers (for subscription payments)
		if receiver == n.config.SmartContractAddress {
			ctnTransfers, err := blockchain.CheckForCTNTransfer(tx, n.config.SmartContractAddress)
			if err != nil {
				n.logger.Error("Failed to check for CTN transfer", "error", err)
			} else if len(ctnTransfers) > 0 {
				n.logger.Debug("CTN transfer detected", "tx", tx.Hash().String())
				allTransfers = append(allTransfers, ctnTransfers...)
			}
		}

		// O(1) lookup for token by address instead of O(n) iteration
		if token, exists := tokensByAddress[receiver]; exists {
			var transfers []*blockchain.Transfer
			var err error

			if token.Type == "CBC20" {
				transfers, err = blockchain.CheckForCBC20Transfer(tx, token.Address, token.Symbol, token.Decimals)
			} else if token.Type == "CBC721" {
				transfers, err = blockchain.CheckForCBC721Transfer(tx, token.Address, token.Symbol)
			}

			if err != nil {
				n.logger.Error("Failed to check for token transfer", "token", token.Symbol, "error", err)
			} else if len(transfers) > 0 {
				n.logger.Debug("Token transfer detected", "token", token.Symbol, "type", token.Type, "tx", tx.Hash().String())
				allTransfers = append(allTransfers, transfers...)
			}
		}

		// If we found any token transfers, process them
		if len(allTransfers) > 0 {
			transfers := allTransfers // Capture for closure
			n.safeGo(func() { n.processTokenTransfers(transfers) }, "processTokenTransfers")
		} else {
			// If no token transfers found, check if it's an XCB transfer
			if tx.Value().Sign() > 0 {
				n.logger.Debug("XCB transfer detected", "tx", tx.Hash().String())
				transaction := tx // Capture for closure
				n.safeGo(func() { n.processXCBTransfer(transaction) }, "processXCBTransfer")
			}
		}
	}
}

// processTokenTransfers processes all token transfers (CBC20, CBC721, etc.)
func (n *Nuntiare) processTokenTransfers(transfers []*blockchain.Transfer) {
	for _, transfer := range transfers {
		// Handle user notifications
		n.processUserNotification(transfer)

		// Handle subscription payments (CTN only)
		n.processSubscriptionPayment(transfer)
	}
}

// processUserNotification handles notifications for registered wallets
func (n *Nuntiare) processUserNotification(transfer *blockchain.Transfer) {
	wallet, shouldNotify, err := n.shouldNotifyWallet(transfer.To)
	if err != nil {
		n.logger.Error("Wallet check failed", "error", err, "address", transfer.To, "token", transfer.TokenSymbol)
		return
	}

	if !shouldNotify {
		return
	}

	n.logger.Info("Sending notification", "wallet", wallet.Address, "token", transfer.TokenSymbol, "amount", transfer.Amount)

	notification := &models.Notification{
		Wallet:       transfer.To,
		Amount:       transfer.Amount,
		Currency:     transfer.TokenSymbol,
		TokenAddress: transfer.TokenAddress,
		TokenType:    transfer.TokenType,
		TokenID:      transfer.TokenID,
	}

	n.safeGo(func() { n.notificator.SendNotification(notification) }, "sendNotification")
}

// processSubscriptionPayment handles CTN payments to subscription addresses
func (n *Nuntiare) processSubscriptionPayment(transfer *blockchain.Transfer) {
	// Only CTN token can be used for subscriptions
	if transfer.TokenAddress != n.config.SmartContractAddress {
		return
	}

	isSubscriptionAddr, err := n.repo.IsSubscriptionAddress(transfer.To)
	if err != nil {
		n.logger.Error("Failed to check subscription address", "error", err, "address", transfer.To)
		return
	}

	if !isSubscriptionAddr {
		return
	}

	n.logger.Debug("Subscription payment detected", "address", transfer.To, "amount", transfer.Amount)

	wallet, err := n.repo.GetWalletBySubscriptionAddress(transfer.To)
	if err != nil {
		n.logger.Error("Failed to get wallet by subscription address", "error", err, "address", transfer.To)
		return
	}

	if err := n.AddSubscriptionPaymentAndUpdatePaidStatus(wallet, transfer.Amount, time.Now().Unix()); err != nil {
		n.logger.Error("Failed to process subscription payment", "error", err, "wallet", wallet.Address)
	}
}

func (n *Nuntiare) processXCBTransfer(tx *types.Transaction) {
	address := tx.To().String()

	wallet, shouldNotify, err := n.shouldNotifyWallet(address)
	if err != nil {
		n.logger.Error("Wallet check failed", "error", err, "address", address, "tx", tx.Hash().String())
		return
	}

	if !shouldNotify {
		return
	}

	amount := weiToXCB(tx.Value())
	n.logger.Info("Sending notification", "wallet", wallet.Address, "currency", "XCB", "amount", amount, "tx", tx.Hash().String())

	notification := &models.Notification{
		Wallet:   address,
		Amount:   amount,
		Currency: "XCB",
	}

	n.safeGo(func() { n.notificator.SendNotification(notification) }, "sendNotification")
}

// CheckWalletSubscription check at the moment of call the CTN balance of the wallet.
// If the balance is > 0, it adds a subscription payment to the repository.
// func (n *Nuntiare) CheckWalletInitialSubscription(subscriptionAddress string) error {
// 	balance, err := n.gocore.GetAddressCTNBalance(subscriptionAddress)
// 	if err != nil {
// 		n.logger.Error("Failed to check wallet initial balance", "error", err)
// 		return err
// 	}
// 	n.logger.Debug("Wallet initial subscription checked", "subscriptionAddress", subscriptionAddress, "balance", balance)
// 	if balance.Cmp(big.NewInt(0)) > 0 {
// 		err = n.repo.AddSubscriptionPayment(subscriptionAddress, balance.Int64(), time.Now().Unix())
// 		if err != nil {
// 			n.logger.Error("Failed to add subscription payment", "error", err)
// 			return err
// 		}
// 	}
// 	return nil
// }

// CheckWalletSubscription checks if the wallet is subscribed
// It takes all subscription payments for specified address from the repository
// and checks if the total amount of payments is >= 200 CTN
func (n *Nuntiare) CheckWalletSubscription(wallet *models.Wallet) (bool, error) {
	payments, err := n.repo.GetSubscriptionPayments(wallet.SubscriptionAddress)
	if err != nil {
		n.logger.Error("Failed to get subscription payments", "error", err)
		return false, err
	}
	total := float64(0)
	for _, payment := range payments {
		total += payment.Amount
	}
	n.logger.Debug("Wallet subscription checked ", "subscriptionAddress ", wallet.SubscriptionAddress, "total ", total)
	if total >= minimalBalance {
		return true, nil
	}
	// it means that old subscription payments were removed
	// and the wallet is not subscribed anymore
	// so we need to remove the subscription from the repository
	n.repo.UpdateWalletPaidStatus(wallet.Address, false)
	return false, nil
}

func (n *Nuntiare) GetWallet(address string) (*models.Wallet, error) {
	wallet, err := n.repo.GetWallet(address)
	if err != nil {
		n.logger.Error("Failed to get wallet ", "error ", err)
		return nil, err
	}
	return wallet, nil
}

func (n *Nuntiare) AddSubscriptionPaymentAndUpdatePaidStatus(
	wallet *models.Wallet,
	amount float64,
	timestamp int64,
) error {
	err := n.repo.AddSubscriptionPayment(wallet.SubscriptionAddress, amount, timestamp)
	if err != nil {
		n.logger.Error("Failed to add subscription payment ", "error ", err)
		return err
	}
	paid, err := n.CheckWalletSubscription(wallet)
	if err != nil {
		n.logger.Error("Failed to check wallet subscription ", "error ", err)
	}
	if paid {
		err = n.repo.UpdateWalletPaidStatus(wallet.Address, true)
		if err != nil {
			n.logger.Error("Failed to update wallet paid status ", "error ", err)
		}
	}
	return nil
}
