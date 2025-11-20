package nuntiare

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/core-coin/go-core/v2/core/types"

	"github.com/core-coin/nuntiare/internal/blockchain"
	"github.com/core-coin/nuntiare/internal/config"
	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
)

const (
	// MaxConcurrentNotifications limits the number of concurrent notification goroutines
	MaxConcurrentNotifications = 100

	// Cleanup intervals
	UnpaidSubscriptionCleanupInterval = 5 * time.Minute
	UnpaidSubscriptionGracePeriod     = 10 * time.Minute
	LockCleanupInterval               = 1 * time.Minute

	// Blockchain connection retry settings
	InitialBackoff      = 1 * time.Second
	MaxBackoff          = 60 * time.Second
	ConnectionBackoff   = 5 * time.Second
	BlockProcessLockTTL = 30 // seconds

	// Timeouts
	BlockFetchTimeout      = 10 * time.Second
	ReceiptFetchTimeout    = 10 * time.Second
	ChannelDrainTimeout    = 5 * time.Second
)

// TokenCache interface for getting cached tokens
type TokenCache interface {
	GetAllTokens() []*models.Token
}

// Nuntiare is the main struct for the Nuntiare application
// It contains all the necessary components to run the application
// and serves all business logic
type Nuntiare struct {
	logger     *logger.Logger
	config     *config.Config
	instanceID string // Unique identifier for this instance (for HA distributed locking)

	repo        models.Repository
	gocore      models.BlockchainService
	notificator models.NotificationService
	tokenCache  TokenCache

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Semaphore to limit concurrent notification goroutines (prevents goroutine explosion)
	notificationSem chan struct{}
}

// generateInstanceID creates a unique identifier for this instance
func generateInstanceID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("instance-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
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
	instanceID := generateInstanceID()
	logger.Info("Initializing Nuntiare instance", "instance_id", instanceID)

	ctx, cancel := context.WithCancel(context.Background())

	return &Nuntiare{
		repo:            repo,
		gocore:          gocore,
		logger:          logger,
		notificator:     notificator,
		tokenCache:      tokenCache,
		config:          config,
		instanceID:      instanceID,
		ctx:             ctx,
		cancel:          cancel,
		notificationSem: make(chan struct{}, MaxConcurrentNotifications),
	}
}

// Stop gracefully stops the Nuntiare instance
func (n *Nuntiare) Stop() {
	n.logger.Info("Stopping Nuntiare instance", "instance_id", n.instanceID)
	n.cancel() // Signal all goroutines to stop
	n.wg.Wait() // Wait for all goroutines to finish
	n.logger.Info("Nuntiare instance stopped", "instance_id", n.instanceID)
}

// safeGo runs a function in a goroutine with panic recovery and semaphore-based limiting
func (n *Nuntiare) safeGo(fn func(), description string) {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done() // Always decrement WaitGroup counter when goroutine exits
		defer func() {
			if r := recover(); r != nil {
				n.logger.Error("Goroutine panicked",
					"description", description,
					"panic", r,
					"stack", string(debug.Stack()))
			}
		}()

		// Acquire semaphore slot (blocks if at limit)
		select {
		case n.notificationSem <- struct{}{}:
			defer func() { <-n.notificationSem }() // Release slot when done
			fn()
		case <-n.ctx.Done():
			// Context cancelled, don't start the goroutine
			n.logger.Debug("Goroutine cancelled before execution", "description", description)
			return
		}
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

	// Check if wallet is active (not cancelled)
	if !wallet.Active {
		n.logger.Debug("Wallet notifications are cancelled", "address", address)
		return wallet, false, nil
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
	// Start a goroutine to clean up unpaid subscriptions
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		ticker := time.NewTicker(UnpaidSubscriptionCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n.logger.Debug("Cleaning up unpaid subscriptions")
				gracePeriod := time.Now().Unix() - int64(UnpaidSubscriptionGracePeriod.Seconds())
				err := n.repo.RemoveUnpaidSubscriptions(gracePeriod)
				if err != nil {
					n.logger.Error("Failed to remove unpaid subscriptions", "error", err)
				}
			case <-n.ctx.Done():
				n.logger.Debug("Unpaid subscription cleanup stopped")
				return
			}
		}
	}()

	// HA: Start a goroutine to cleanup expired locks
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		ticker := time.NewTicker(LockCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n.logger.Debug("Cleaning up expired locks")
				if err := n.repo.CleanupExpiredLocks(); err != nil {
					n.logger.Error("Failed to cleanup expired locks", "error", err)
				}
			case <-n.ctx.Done():
				n.logger.Debug("Lock cleanup stopped")
				return
			}
		}
	}()

	// Start watching for new transactions (handles connection retries internally)
	n.wg.Add(1)
	go n.WatchTransfers()
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

// UpdateNotificationProvider updates notification providers for an existing wallet
func (n *Nuntiare) UpdateNotificationProvider(address, telegram, email string) error {
	return n.repo.UpdateNotificationProvider(address, telegram, email)
}

// UpdateNotificationProviderAndReactivate updates notification providers and reactivates wallet
func (n *Nuntiare) UpdateNotificationProviderAndReactivate(address, telegram, email string) error {
	// Update notification providers
	if err := n.repo.UpdateNotificationProvider(address, telegram, email); err != nil {
		return err
	}

	// Reactivate wallet (in case it was cancelled)
	if err := n.repo.SetWalletActive(address, true); err != nil {
		return err
	}

	return nil
}

// CancelWallet deactivates notifications while keeping subscription active
func (n *Nuntiare) CancelWallet(address string) error {
	return n.repo.SetWalletActive(address, false)
}

// IsRegistered checks if the given address is registered
func (n *Nuntiare) IsRegistered(address string) (bool, error) {
	return n.repo.CheckWalletExists(address)
}

// initializeBlockchain initializes the blockchain service connection
func (n *Nuntiare) initializeBlockchain() error {
	return n.gocore.Run()
}

// WatchTransfers starts watching for new transfers inside blockchain
// If tx receiver is a registered wallet, it sends a notification if wallet has subscribtion
func (n *Nuntiare) WatchTransfers() {
	defer n.wg.Done()

	backoff := InitialBackoff
	maxBackoff := MaxBackoff
	connectionBackoff := ConnectionBackoff

	// First, ensure blockchain connection is established
	n.logger.Info("Initializing blockchain connection for transfer monitoring...")

	for {
		// Try to connect to blockchain
		if err := n.initializeBlockchain(); err != nil {
			n.logger.Warn("Failed to initialize blockchain connection, will retry",
				"error", err,
				"retry_in", connectionBackoff)
			time.Sleep(connectionBackoff)
			continue
		}

		n.logger.Info("Successfully connected to blockchain service")
		break
	}

	// Now start watching for transfers
	for {
		subscription, channel, err := n.gocore.NewHeaderSubscription()
		if err != nil {
			n.logger.Error("Failed to subscribe to new head, will retry", "error", err, "retry_in", backoff)
			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			// Try to reinitialize blockchain connection
			if err := n.initializeBlockchain(); err != nil {
				n.logger.Debug("Failed to reinitialize blockchain", "error", err)
			}
			continue
		}

		// Reset backoff on successful connection
		backoff = InitialBackoff
		n.logger.Info("Successfully subscribed to blockchain headers")

		// Process headers with proper cleanup
		func() {
			defer subscription.Unsubscribe()

			for {
				select {
				case header, ok := <-channel:
					if !ok {
						// Channel closed, break inner loop to retry subscription
						n.logger.Warn("Header channel closed, will restart subscription")
						return
					}

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

				case err := <-subscription.Err():
					// Subscription error (connection dropped, etc.)
					n.logger.Error("Blockchain subscription error, will restart", "error", err)
					return

				case <-n.ctx.Done():
					// Context cancelled, clean up and exit
					n.logger.Info("WatchTransfers stopped while processing headers")
					// Drain the channel with timeout to prevent goroutine leak
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), ChannelDrainTimeout)
						defer cancel()
						for {
							select {
							case _, ok := <-channel:
								if !ok {
									return
								}
							case <-ctx.Done():
								return
							}
						}
					}()
					return
				}
			}
		}()

		// If we reach here, channel was closed, retry after backoff
		select {
		case <-time.After(backoff):
			n.logger.Info("Retrying blockchain subscription after channel close")
			continue
		case <-n.ctx.Done():
			n.logger.Info("WatchTransfers stopped during retry backoff")
			return
		}
	}

}

func (n *Nuntiare) checkBlock(block *types.Block) {
	// HA: Try to acquire distributed lock for this block processing
	// Lock name includes block number to allow different instances to process different blocks
	// TTL is 30 seconds - if processing takes longer, another instance can take over
	lockName := fmt.Sprintf("block_processor_%d", block.NumberU64())
	acquired, err := n.repo.TryAcquireLock(lockName, n.instanceID, 30)
	if err != nil {
		n.logger.Error("Failed to acquire lock for block processing", "block", block.NumberU64(), "error", err)
		return
	}
	if !acquired {
		// Another instance is processing this block, skip it
		n.logger.Debug("Block already being processed by another instance", "block", block.NumberU64())
		return
	}

	// Release lock when done
	defer func() {
		if err := n.repo.ReleaseLock(lockName, n.instanceID); err != nil {
			n.logger.Error("Failed to release lock", "block", block.NumberU64(), "error", err)
		}
	}()

	n.logger.Debug("Processing block", "block", block.NumberU64(), "instance", n.instanceID)

	// Get all watched tokens from in-memory cache
	tokens := n.tokenCache.GetAllTokens()

	// Build address->token map for O(1) lookup instead of O(n) iteration
	// Normalize all addresses to lowercase for consistent lookups
	tokensByAddress := make(map[string]*models.Token, len(tokens))
	for _, token := range tokens {
		tokensByAddress[strings.ToLower(token.Address)] = token
	}

	for _, tx := range block.Body().Transactions {
		// Skip contract creation transactions
		if tx.To() == nil {
			continue
		}

		receiver := tx.To().Hex()
		// Normalize receiver address for lookups (remove 0x prefix and lowercase)
		receiverNormalized := receiver
		if len(receiverNormalized) > 2 && receiverNormalized[:2] == "0x" {
			receiverNormalized = receiverNormalized[2:]
		}
		receiverNormalized = strings.ToLower(receiverNormalized)

		n.logger.Debug("Processing transaction", "tx", tx.Hash().String(), "to", receiverNormalized)
		var allTransfers []*blockchain.Transfer
		// Use cached normalized address for efficient comparison
		isCTNContract := receiverNormalized == n.config.SmartContractAddressNormalized

		// Check for CTN transfers (for subscription payments)
		if isCTNContract {
			ctnTransfers, err := blockchain.CheckForCTNTransfer(tx, n.config.SmartContractAddress)
			if err != nil {
				n.logger.Error("Failed to check for CTN transfer", "error", err)
			} else if len(ctnTransfers) > 0 {
				n.logger.Debug("CTN transfer detected", "tx", tx.Hash().String())
				allTransfers = append(allTransfers, ctnTransfers...)
			}
		}

		// O(1) lookup for token by address instead of O(n) iteration
		// Skip if already processed as CTN contract to avoid duplicate notifications
		if !isCTNContract {
			if token, exists := tokensByAddress[receiverNormalized]; exists {
				n.logger.Debug("Token found in cache", "token", token.Symbol, "type", token.Type, "address", token.Address)
				var transfers []*blockchain.Transfer
				var err error

				if token.Type == "CBC20" {
					transfers, err = blockchain.CheckForCBC20Transfer(tx, token.Address, token.Symbol, token.Decimals)
				} else if token.Type == "CBC721" {
					n.logger.Debug("Fetching receipt for CBC721 transfer", "tx", tx.Hash().String())
					// CBC721 transfers emit events, so we need to fetch the receipt
					receipt, receiptErr := n.gocore.GetTransactionReceipt(tx.Hash().Hex())
					if receiptErr != nil {
						n.logger.Error("Failed to get transaction receipt", "tx", tx.Hash().String(), "error", receiptErr)
					} else {
						n.logger.Debug("Receipt fetched, parsing events", "tx", tx.Hash().String(), "logs", len(receipt.Logs))
						transfers, err = blockchain.CheckForCBC721TransferFromReceipt(receipt, token.Address, token.Symbol)
						n.logger.Debug("CBC721 parsing complete", "tx", tx.Hash().String(), "transfers", len(transfers))
					}
				}

				if err != nil {
					n.logger.Error("Failed to check for token transfer", "token", token.Symbol, "error", err)
				} else if len(transfers) > 0 {
					n.logger.Debug("Token transfer detected", "token", token.Symbol, "type", token.Type, "tx", tx.Hash().String())
					allTransfers = append(allTransfers, transfers...)
				} else {
					n.logger.Debug("No transfers found", "token", token.Symbol, "type", token.Type)
				}
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
	n.logger.Debug("Processing user notification", "to", transfer.To, "token", transfer.TokenSymbol, "type", transfer.TokenType)

	wallet, shouldNotify, err := n.shouldNotifyWallet(transfer.To)
	if err != nil {
		n.logger.Error("Wallet check failed", "error", err, "address", transfer.To, "token", transfer.TokenSymbol)
		return
	}

	if !shouldNotify {
		n.logger.Debug("Wallet should not be notified", "address", transfer.To, "registered", wallet != nil)
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

// processSubscriptionPayment handles CTN payments to the shared RECEIVING_ADDRESS
// All subscription payments go TO RECEIVING_ADDRESS FROM subscriber addresses
func (n *Nuntiare) processSubscriptionPayment(transfer *blockchain.Transfer) {
	// Only CTN token can be used for subscriptions
	if transfer.TokenAddress != n.config.SmartContractAddress {
		return
	}

	// Normalize addresses for comparison (lowercase, no 0x prefix)
	transferToNormalized := strings.ToLower(strings.TrimPrefix(transfer.To, "0x"))
	receivingAddrNormalized := n.config.ReceivingAddressNormalized

	// Check if payment is TO the shared RECEIVING_ADDRESS
	if transferToNormalized != receivingAddrNormalized {
		return
	}

	n.logger.Debug("Payment to RECEIVING_ADDRESS detected, checking sender",
		"from", transfer.From,
		"to", transfer.To,
		"amount", transfer.Amount)

	// Look up wallet by subscriber address (the FROM address)
	// GetWalletBySubscriptionAddress looks up by subscription_address field
	wallet, err := n.repo.GetWalletBySubscriptionAddress(transfer.From)
	if err != nil {
		n.logger.Debug("No registered wallet found for subscriber address",
			"subscriber", transfer.From,
			"error", err)
		return
	}

	n.logger.Info("Subscription payment detected",
		"subscriber", transfer.From,
		"destination_wallet", wallet.Address,
		"amount", transfer.Amount)

	if err := n.AddSubscriptionPaymentAndUpdatePaidStatus(wallet, transfer.Amount, time.Now().Unix()); err != nil {
		n.logger.Error("Failed to process subscription payment",
			"error", err,
			"wallet", wallet.Address,
			"subscriber", transfer.From)
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
// It checks if the subscription expiration date is in the future
func (n *Nuntiare) CheckWalletSubscription(wallet *models.Wallet) (bool, error) {
	now := time.Now().Unix()

	n.logger.Debug("Wallet subscription checked",
		"subscriptionAddress", wallet.SubscriptionAddress,
		"expiresAt", wallet.SubscriptionExpiresAt,
		"now", now)

	if wallet.SubscriptionExpiresAt > now {
		// Subscription is still active
		return true, nil
	}

	// Subscription has expired, update paid status to false
	if wallet.Paid {
		err := n.repo.UpdateWalletPaidStatus(wallet.Address, false)
		if err != nil {
			n.logger.Error("Failed to update wallet paid status", "error", err)
			return false, err
		}
	}

	return false, nil
}

func (n *Nuntiare) GetWallet(address string) (*models.Wallet, error) {
	wallet, err := n.repo.GetWallet(address)
	if err != nil {
		// Only log as error if it's not a "not found" error
		if !strings.Contains(err.Error(), "record not found") {
			n.logger.Error("Failed to get wallet", "error", err, "address", address)
		}
		return nil, err
	}
	return wallet, nil
}

func (n *Nuntiare) AddSubscriptionPaymentAndUpdatePaidStatus(
	wallet *models.Wallet,
	amount float64,
	timestamp int64,
) error {
	// Add payment record for tracking
	err := n.repo.AddSubscriptionPayment(wallet.SubscriptionAddress, amount, timestamp)
	if err != nil {
		n.logger.Error("Failed to add subscription payment", "error", err)
		return err
	}

	// Calculate how many months this payment covers
	monthsToAdd := amount / n.config.SubscriptionMonthCost
	secondsToAdd := int64(monthsToAdd * n.config.SubscriptionMonthDuration)

	now := time.Now().Unix()
	var newExpiresAt int64

	// If subscription is still active, extend it from current expiration
	// Otherwise, start from now
	if wallet.SubscriptionExpiresAt > now {
		newExpiresAt = wallet.SubscriptionExpiresAt + secondsToAdd
		n.logger.Info("Extending active subscription",
			"address", wallet.Address,
			"amount", amount,
			"months", monthsToAdd,
			"currentExpires", wallet.SubscriptionExpiresAt,
			"newExpires", newExpiresAt)
	} else {
		newExpiresAt = now + secondsToAdd
		n.logger.Info("Starting new subscription",
			"address", wallet.Address,
			"amount", amount,
			"months", monthsToAdd,
			"expiresAt", newExpiresAt)
	}

	// Update wallet's expiration date and paid status
	err = n.repo.UpdateWalletSubscriptionExpiration(wallet.Address, newExpiresAt)
	if err != nil {
		n.logger.Error("Failed to update wallet subscription expiration", "error", err)
		return err
	}

	err = n.repo.UpdateWalletPaidStatus(wallet.Address, true)
	if err != nil {
		n.logger.Error("Failed to update wallet paid status", "error", err)
		return err
	}

	// Update the wallet object with new expiration
	wallet.SubscriptionExpiresAt = newExpiresAt
	wallet.Paid = true

	return nil
}

// ProcessTelegramWebhook processes a Telegram webhook update
func (n *Nuntiare) ProcessTelegramWebhook(update interface{}) error {
	n.logger.Debug("Received Telegram webhook update", "update", update)
	// Webhook processing will be handled by the Telegram bot API
	// This is a placeholder for now - actual implementation depends on bot library
	return nil
}
