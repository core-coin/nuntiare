package nuntiare

import (
	"math/big"
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

// Nuntiare is the main struct for the Nuntiare application
// It contains all the necessary components to run the application
// and serves all business logic
type Nuntiare struct {
	logger *logger.Logger
	config *config.Config

	repo        models.Repository
	gocore      models.BlockchainService
	notificator models.NotificationService
}

// NewNuntiare creates a new Nuntiare instance
func NewNuntiare(
	repo models.Repository,
	gocore models.BlockchainService,
	notificator models.NotificationService,
	logger *logger.Logger,
	config *config.Config,
) models.NuntiareI {
	return &Nuntiare{
		repo:        repo,
		gocore:      gocore,
		logger:      logger,
		notificator: notificator,
		config:      config,
	}
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
	for {
		channel, err := n.gocore.NewHeaderSubscription()
		if err != nil {
			n.logger.Fatal("Failed to subscribe to new head:", err)
		}

		for header := range channel {
			n.logger.Debug("New block header received ", "number ", header.Number)

			// Check if the block has transactions
			if !header.EmptyBody() {
				n.logger.Debug("Block has transactions")
				block, err := n.gocore.GetBlockByNumber(header.Number.Uint64())
				if err != nil {
					n.logger.Error("Failed to get block by number", "number ", header.Number, "error", err)
					continue
				}
				n.checkBlock(block)
			}
		}
		n.logger.Error("Channel closed, restarting subscription")
		time.Sleep(5 * time.Second) // wait before restarting the subscription
		n.logger.Debug("Restarting subscription to new head")
	}

}

func (n *Nuntiare) checkBlock(block *types.Block) {
	for _, tx := range block.Body().Transactions {
		// check if is CTN transfer
		transfers, err := blockchain.CheckForCTNTransfer(tx, n.config.SmartContractAddress)
		if err != nil {
			n.logger.Error("Failed to check for CTN transfer ", "error ", err)
		}
		if len(transfers) > 0 {
			n.logger.Debug("CTN transfer detected ", "tx ", tx.Hash().String())
			go n.processCTNTransfers(transfers)
		} else {
			n.logger.Debug("XCB transfer detected ", "tx ", tx.Hash().String())
			go n.processXCBTransfer(tx)
		}

	}
}

func (n *Nuntiare) processCTNTransfers(transfers []*blockchain.Transfer) {
	for _, transfer := range transfers {

		exists, err := n.IsRegistered(transfer.To)
		if err != nil {
			n.logger.Error("Failed to check if wallet exists ", "error ", err, "currency ", "CTN")
			continue
		}

		if exists {
			n.logger.Info("Transaction to registered wallet detected ", "tx ", transfer)
			wallet, err := n.repo.GetWallet(transfer.To)
			if err != nil {
				n.logger.Error("Failed to get wallet", "error", err, "currency ", "CTN")
				continue
			}
			// todo:error2215 check if wallet has a subscription only if it's not whitelisted
			// so we don't make unnecessary requests to the DB
			subscribed, err := n.CheckWalletSubscription(wallet)
			if err != nil {
				n.logger.Error("Failed to check wallet subscription", "error", err, "currency ", "CTN")
				continue
			}
			if wallet.Whitelisted || subscribed {
				n.logger.Debug("Wallet is whitelisted or subscribed ", "wallet", wallet, "currency ", "CTN")
				notification := &models.Notification{
					Wallet:   transfer.To,
					Amount:   transfer.Amount,
					Currency: "CTN",
				}
				n.logger.Info("Sending ", "notification ", notification, "currency ", "CTN")
				go n.notificator.SendNotification(notification)
			}
		}

		subscriptionPayment, err := n.repo.IsSubscriptionAddress(transfer.To)
		if err != nil {
			n.logger.Error("Failed to check if wallet is subscription address ", "error ", err, "currency ", "CTN")
			continue
		}
		if subscriptionPayment {
			n.logger.Debug("Transaction to subscription address detected ", "tx ", transfer)
			wallet, err := n.repo.GetWalletBySubscriptionAddress(transfer.To)
			if err != nil {
				n.logger.Error("Failed to get wallet", "error", err, "currency", "CTN")
				continue
			}
			err = n.AddSubscriptionPaymentAndUpdatePaidStatus(wallet, transfer.Amount, time.Now().Unix())
			if err != nil {
				n.logger.Error("Failed to add subscription payment", "error ", err, "currency ", "CTN")
				continue
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
		n.logger.Info("Transaction to registered wallet detected ", "tx ", tx.Hash().String())
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
			n.logger.Debug("Wallet is whitelisted or subscribed ", "wallet ", wallet)
			amount, _ := big.NewFloat(0).Quo(new(big.Float).SetInt(tx.Value()), big.NewFloat(1e18)).Float64()
			notification := &models.Notification{
				Wallet:   tx.To().String(),
				Amount:   amount,
				Currency: "XCB",
			}
			n.logger.Info("Sending ", "notification ", notification)
			go n.notificator.SendNotification(notification)
		}
	}
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
