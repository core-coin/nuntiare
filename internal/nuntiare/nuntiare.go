package nuntiare

import (
	"math/big"
	"time"

	"github.com/core-coin/go-core/v2/core/types"

	"github.com/core-coin/nuntiare/internal/blockchain"
	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
)

// Nuntiare is the main struct for the Nuntiare application
// It contains all the necessary components to run the application
// and serves all business logic
type Nuntiare struct {
	logger *logger.Logger

	repo        models.Repository
	gocore      models.BlockchainService
	notificator models.NotificationService

	minimumBalanceForNotification *big.Int
}

// NewNuntiare creates a new Nuntiare instance
func NewNuntiare(
	repo models.Repository,
	gocore models.BlockchainService,
	notificator models.NotificationService,
	logger *logger.Logger,
	minimalBalance *big.Int,
) models.NuntiareI {
	return &Nuntiare{
		repo:                          repo,
		gocore:                        gocore,
		logger:                        logger,
		notificator:                   notificator,
		minimumBalanceForNotification: minimalBalance}
}

// Start starts the Nuntiare application
func (n *Nuntiare) Start() {
	// Start a goroutine to remove old subscription payments
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			err := n.RemoveOldSubscriptionPayments(time.Now().Unix() - int64(30*24*time.Hour.Seconds())) // 1 month
			if err != nil {
				n.logger.Error("Failed to remove old subscription payments", "error", err)
			}
			err = n.RemoveUnpaidSubscriptions(time.Now().Unix() - int64(24*time.Hour.Seconds()))
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

func (n *Nuntiare) IsSubscriptionAddress(address string) (bool, error) {
	return n.repo.IsSubscriptionAddress(address)
}

func (n *Nuntiare) RemoveOldSubscriptionPayments(timestamp int64) error {
	return n.repo.RemoveOldSubscriptionPayments(timestamp)
}

func (n *Nuntiare) RemoveUnpaidSubscriptions(timestamp int64) error {
	return n.repo.RemoveUnpaidSubscriptions(timestamp)
}

// WatchTransfers starts watching for new transfers inside blockchain
// If tx receiver is a registered wallet, it sends a notification if wallet has subscribtion
func (n *Nuntiare) WatchTransfers() {
	channel, err := n.gocore.NewHeaderSubscription()
	if err != nil {
		n.logger.Fatal("Failed to subscribe to new head:", err)
	}

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

}

func (n *Nuntiare) checkBlock(block *types.Block) {
	for _, tx := range block.Body().Transactions {
		// check if is CTN transfer
		transfers, err := blockchain.CheckForCTNTransfer(tx)
		if err != nil {
			n.logger.Error("Failed to check for CTN transfer", "error", err)
		}
		if len(transfers) > 0 {
			go n.processCTNTransfers(transfers)
		} else {
			go n.processXCBTransfer(tx)
		}

	}
}

func (n *Nuntiare) processCTNTransfers(transfers []*blockchain.Transfer) {
	for _, transfer := range transfers {
		exists, err := n.IsRegistered(transfer.To)
		if err != nil {
			n.logger.Error("Failed to check if wallet exists", "error", err, "currency", "CTN")
			continue
		}

		if exists {
			n.logger.Info("Transaction to registered wallet detected", "tx", transfer)
			wallet, err := n.repo.GetWallet(transfer.To)
			if err != nil {
				n.logger.Error("Failed to get wallet", "error", err, "currency", "CTN")
				continue
			}
			// todo:error2215 check if wallet has a subscription only if it's not whitelisted
			// so we don't make unnecessary requests to the DB
			subscribed, err := n.CheckWalletSubscription(wallet)
			if err != nil {
				n.logger.Error("Failed to check wallet subscription", "error", err, "currency", "CTN")
				continue
			}
			if wallet.Whitelisted || subscribed {
				notification := &models.Notification{
					Wallet:   transfer.To,
					Amount:   &transfer.Amount,
					Currency: "CTN",
				}
				n.logger.Debug("Sending notification", "notification", notification, "currency", "CTN")
				go n.notificator.SendNotification(wallet.SubscriptionAddress, notification)
			}
		}

		subsciptionPayment, err := n.IsSubscriptionAddress(transfer.To)
		if err != nil {
			n.logger.Error("Failed to check if wallet is subscription address", "error", err, "currency", "CTN")
			continue
		}
		if subsciptionPayment {
			err = n.repo.AddSubscriptionPayment(transfer.To, transfer.Amount.Int64(), time.Now().Unix())
			if err != nil {
				n.logger.Error("Failed to add subscription payment", "error", err)
			}
		}
	}
}

func (n *Nuntiare) processXCBTransfer(tx *types.Transaction) {
	exists, err := n.IsRegistered(tx.To().String())
	if err != nil {
		n.logger.Error("Failed to check if wallet exists", "error", err)
		return
	}

	if exists {
		n.logger.Info("Transaction to registered wallet detected", "tx", tx.Hash().String())
		wallet, err := n.repo.GetWallet(tx.To().String())
		if err != nil {
			n.logger.Error("Failed to get wallet", "error", err)
			return
		}
		// todo:error2215 check if wallet has a subscription only if it's not whitelisted
		// so we don't make unnecessary requests to the DB
		subscribed, err := n.CheckWalletSubscription(wallet)
		if err != nil {
			n.logger.Error("Failed to check wallet subscription", "error", err)
			return
		}
		if wallet.Whitelisted || subscribed {
			notification := &models.Notification{
				Wallet:   tx.To().String(),
				Amount:   tx.Value(),
				Currency: "XCB",
			}
			n.logger.Debug("Sending notification", "notification", notification)
			go n.notificator.SendNotification(wallet.SubscriptionAddress, notification)
		}
	}
}

// CheckWalletSubscription check at the moment of call the CTN balance of the wallet.
// If the balance is > 0, it adds a subscription payment to the repository.
func (n *Nuntiare) CheckWalletInitialSubscription(subscriptionAddress string) error {
	balance, err := n.gocore.GetAddressCTNBalance(subscriptionAddress)
	if err != nil {
		n.logger.Error("Failed to check wallet initial balance", "error", err)
		return err
	}
	n.logger.Debug("Wallet initial subscription checked", "subscriptionAddress", subscriptionAddress, "balance", balance)
	if balance.Cmp(big.NewInt(0)) > 0 {
		err = n.repo.AddSubscriptionPayment(subscriptionAddress, balance.Int64(), time.Now().Unix())
		if err != nil {
			n.logger.Error("Failed to add subscription payment", "error", err)
			return err
		}
	}
	return nil
}

// CheckWalletSubscription checks if the wallet is subscribed
// It takes all subscription payments for specified address from the repository
// and checks if the total amount of payments is >= 200 CTN
func (n *Nuntiare) CheckWalletSubscription(wallet *models.Wallet) (bool, error) {
	payments, err := n.repo.GetSubscriptionPayments(wallet.SubscriptionAddress)
	if err != nil {
		n.logger.Error("Failed to get subscription payments", "error", err)
		return false, err
	}
	total := int64(0)
	for _, payment := range payments {
		total += payment.Amount
	}
	n.logger.Debug("Wallet subscription checked", "subscriptionAddress", wallet.SubscriptionAddress, "total", total)
	return total >= n.minimumBalanceForNotification.Int64(), nil
}
