package repository

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
)

type PostgresDB struct {
	logger *logger.Logger

	Conn *gorm.DB
}

func NewPostgresDB(user, password, dbname, host string, port int, logger *logger.Logger) (models.Repository, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		host, user, password, dbname, port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %s", err)
	}

	logger.Info("Successfully connected to PostgreSQL!")
	return &PostgresDB{Conn: db, logger: logger}, nil
}

func (db *PostgresDB) Close() error {
	sqlDB, err := db.Conn.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %s", err)
	}
	return sqlDB.Close()
}

func (db *PostgresDB) AddNewWallet(wallet *models.Wallet) error {
	if err := db.Conn.Create(wallet).Error; err != nil {
		return fmt.Errorf("failed to create new wallet: %s", err)
	}

	return nil
}

func (db *PostgresDB) CheckWalletExists(address string) (bool, error) {
	var wallet models.Wallet
	if err := db.Conn.Where("address = ?", address).First(&wallet).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if wallet exists: %s", err)
	}

	return true, nil
}

func (db *PostgresDB) GetWallet(address string) (*models.Wallet, error) {
	var wallet models.Wallet
	if err := db.Conn.Where("address = ?", wallet.Address).First(&wallet).Error; err != nil {
		return nil, fmt.Errorf("failed to get wallet: %s", err)
	}

	return &wallet, nil
}

func (db *PostgresDB) AddSubscriptionPayment(subscriptionAddress string, amount int64, timestamp int64) error {
	payment := models.SubscriptionPayment{
		Address:   subscriptionAddress,
		Amount:    amount,
		Timestamp: timestamp,
	}
	if err := db.Conn.Create(&payment).Error; err != nil {
		return fmt.Errorf("failed to add subscription payment: %s", err)
	}
	return nil
}

func (db *PostgresDB) GetSubscriptionPayments(subscriptionAddress string) ([]*models.SubscriptionPayment, error) {
	var payments []*models.SubscriptionPayment
	if err := db.Conn.Where("address = ?", subscriptionAddress).Find(&payments).Error; err != nil {
		return nil, fmt.Errorf("failed to get subscription payments: %s", err)
	}

	return payments, nil
}

func (db *PostgresDB) IsSubscriptionAddress(subscriptionAddress string) (bool, error) {
	var wallet models.Wallet
	if err := db.Conn.Where("subscription_address = ?", subscriptionAddress).First(&wallet).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if wallet is subscription address: %s", err)
	}

	return true, nil
}

func (db *PostgresDB) RemoveOldSubscriptionPayments(timestamp int64) error {
	if err := db.Conn.Where("timestamp < ?", timestamp).Delete(&models.SubscriptionPayment{}).Error; err != nil {
		return fmt.Errorf("failed to remove old subscription payments: %s", err)
	}

	return nil
}

func (db *PostgresDB) RemoveUnpaidSubscriptions(timestamp int64) error {
	if err := db.Conn.Where("created_at < ? AND paid = ?", timestamp, false).Delete(&models.Wallet{}).Error; err != nil {
		return fmt.Errorf("failed to remove unpaid subscriptions: %s", err)
	}

	return nil
}