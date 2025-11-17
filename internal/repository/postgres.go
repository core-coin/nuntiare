package repository

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"

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

	// Configure GORM logger to suppress "record not found" messages
	gormLogger := gormLogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // Use standard logger
		gormLogger.Config{
			SlowThreshold:             200 * time.Millisecond, // Log queries slower than this
			LogLevel:                  gormLogger.Warn,        // Only log warnings or errors
			IgnoreRecordNotFoundError: true,                   // Suppress "record not found" errors
			Colorful:                  true,                   // Enable colorful logs
		},
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormLogger})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Configure connection pool for production
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(25)                  // Maximum number of open connections
	sqlDB.SetMaxIdleConns(5)                   // Maximum number of idle connections
	sqlDB.SetConnMaxLifetime(5 * time.Minute)  // Maximum lifetime of a connection
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // Maximum idle time of a connection

	if err := db.AutoMigrate(&models.Wallet{}, &models.SubscriptionPayment{}, &models.NotificationProvider{}, &models.TelegramProvider{}, &models.EmailProvider{}, &models.AppLock{}); err != nil {
		return nil, fmt.Errorf("failed to auto-migrate models: %w", err)
	}
	logger.Info("Successfully connected to PostgreSQL with connection pool configured!")
	return &PostgresDB{Conn: db, logger: logger}, nil
}

func (db *PostgresDB) Close() error {
	sqlDB, err := db.Conn.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	return sqlDB.Close()
}

func (db *PostgresDB) AddNewWallet(wallet *models.Wallet) error {
	if err := db.Conn.Create(wallet).Error; err != nil {
		return fmt.Errorf("failed to create new wallet: %w", err)
	}

	return nil
}

func (db *PostgresDB) CheckWalletExists(address string) (bool, error) {
	var wallet models.Wallet
	if err := db.Conn.Where("address = ?", address).First(&wallet).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if wallet exists: %w", err)
	}

	return true, nil
}

func (db *PostgresDB) GetWallet(address string) (*models.Wallet, error) {
	var wallet models.Wallet
	if err := db.Conn.Where("address = ?", address).First(&wallet).Error; err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}

	return &wallet, nil
}

func (db *PostgresDB) AddSubscriptionPayment(subscriptionAddress string, amount float64, timestamp int64) error {
	payment := models.SubscriptionPayment{
		Address:   subscriptionAddress,
		Amount:    amount,
		Timestamp: timestamp,
	}
	db.logger.Debug("Adding subscription payment ", "payment ", payment)
	if err := db.Conn.Create(&payment).Error; err != nil {
		return fmt.Errorf("failed to add subscription payment: %w", err)
	}
	return nil
}

func (db *PostgresDB) GetSubscriptionPayments(subscriptionAddress string) ([]*models.SubscriptionPayment, error) {
	var payments []*models.SubscriptionPayment
	if err := db.Conn.Where("address = ?", subscriptionAddress).Find(&payments).Error; err != nil {
		return nil, fmt.Errorf("failed to get subscription payments: %w", err)
	}

	return payments, nil
}

func (db *PostgresDB) IsSubscriptionAddress(subscriptionAddress string) (bool, error) {
	var wallet models.Wallet
	if err := db.Conn.Where("subscription_address = ?", subscriptionAddress).First(&wallet).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if wallet is subscription address: %w", err)
	}

	return true, nil
}

func (db *PostgresDB) RemoveOldSubscriptionPayments(timestamp int64) error {
	if err := db.Conn.Where("timestamp < ?", timestamp).Delete(&models.SubscriptionPayment{}).Error; err != nil {
		return fmt.Errorf("failed to remove old subscription payments: %w", err)
	}

	return nil
}

func (db *PostgresDB) RemoveUnpaidSubscriptions(timestamp int64) error {
	if err := db.Conn.Where("created_at < ? AND paid = ?", timestamp, false).Delete(&models.Wallet{}).Error; err != nil {
		return fmt.Errorf("failed to remove unpaid subscriptions: %w", err)
	}

	return nil
}

func (db *PostgresDB) UpdateWalletPaidStatus(address string, paid bool) error {
	var wallet models.Wallet
	if err := db.Conn.Where("address = ?", address).First(&wallet).Error; err != nil {
		return fmt.Errorf("failed to get wallet: %w", err)
	}

	wallet.Paid = paid
	if err := db.Conn.Save(&wallet).Error; err != nil {
		return fmt.Errorf("failed to update wallet paid status: %w", err)
	}

	return nil
}

func (db *PostgresDB) UpdateWalletSubscriptionExpiration(address string, expiresAt int64) error {
	var wallet models.Wallet
	if err := db.Conn.Where("address = ?", address).First(&wallet).Error; err != nil {
		return fmt.Errorf("failed to get wallet: %w", err)
	}

	wallet.SubscriptionExpiresAt = expiresAt
	if err := db.Conn.Save(&wallet).Error; err != nil {
		return fmt.Errorf("failed to update wallet subscription expiration: %w", err)
	}

	return nil
}

func (db *PostgresDB) GetWalletBySubscriptionAddress(subscriptionAddress string) (*models.Wallet, error) {
	var wallet models.Wallet
	if err := db.Conn.Where("subscription_address = ?", subscriptionAddress).First(&wallet).Error; err != nil {
		return nil, fmt.Errorf("failed to get wallet by subscription address: %w", err)
	}

	return &wallet, nil
}

func (db *PostgresDB) GetWalletsNotificationProvider(address string) (*models.NotificationProvider, error) {
	var notificationProvider models.NotificationProvider
	if err := db.Conn.Preload("TelegramProvider").Preload("EmailProvider").Where("address = ?", address).First(&notificationProvider).Error; err != nil {
		return nil, fmt.Errorf("failed to get wallet's notification provider: %w", err)
	}

	return &notificationProvider, nil
}

func (db *PostgresDB) AddTelegramProviderChatID(username, chatID string) error {
	if err := db.Conn.Model(&models.TelegramProvider{}).Where("username = ?", username).Update("chat_id", chatID).Error; err != nil {
		return fmt.Errorf("failed to add telegram provider chat ID: %w", err)
	}
	return nil
}

func (db *PostgresDB) GetNotificationProvidersByTelegramUsername(username string) ([]*models.NotificationProvider, error) {
	var notificationProviders []*models.NotificationProvider
	if err := db.Conn.Joins("JOIN telegram_providers ON telegram_providers.notification_provider_id = notification_providers.id").
		Where("telegram_providers.username = ?", username).
		Preload("TelegramProvider").
		Preload("EmailProvider").
		Find(&notificationProviders).Error; err != nil {
		return nil, fmt.Errorf("failed to get notification providers by telegram username: %w", err)
	}

	return notificationProviders, nil
}

// TryAcquireLock attempts to acquire a distributed lock
// Returns true if lock was acquired, false if another instance holds it
func (db *PostgresDB) TryAcquireLock(lockName, instanceID string, ttlSeconds int) (bool, error) {
	now := time.Now().Unix()
	expiresAt := now + int64(ttlSeconds)

	// First, try to delete any expired locks for this lock name
	if err := db.Conn.Where("lock_name = ? AND expires_at < ?", lockName, now).Delete(&models.AppLock{}).Error; err != nil {
		db.logger.Error("Failed to cleanup expired lock", "lock", lockName, "error", err)
	}

	lock := &models.AppLock{
		LockName:   lockName,
		InstanceID: instanceID,
		AcquiredAt: now,
		ExpiresAt:  expiresAt,
	}

	// Try to insert the lock (will fail if lock already exists and not expired)
	// Use a session with silent logger to avoid logging expected duplicate key errors
	silentLogger := gormLogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormLogger.Config{LogLevel: gormLogger.Silent},
	)
	result := db.Conn.Session(&gorm.Session{Logger: silentLogger}).Create(lock)
	if result.Error != nil {
		// Lock already exists (someone else holds it or not expired yet)
		if strings.Contains(result.Error.Error(), "duplicate key") ||
			strings.Contains(result.Error.Error(), "UNIQUE constraint failed") {
			db.logger.Debug("Lock already held by another instance", "lock", lockName)
			return false, nil
		}
		return false, fmt.Errorf("failed to acquire lock: %w", result.Error)
	}

	db.logger.Debug("Lock acquired", "lock", lockName, "instance", instanceID, "ttl", ttlSeconds)
	return true, nil
}

// ReleaseLock releases a lock held by this instance
func (db *PostgresDB) ReleaseLock(lockName, instanceID string) error {
	result := db.Conn.Where("lock_name = ? AND instance_id = ?", lockName, instanceID).Delete(&models.AppLock{})
	if result.Error != nil {
		return fmt.Errorf("failed to release lock: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		db.logger.Debug("Lock released", "lock", lockName, "instance", instanceID)
	}

	return nil
}

// CleanupExpiredLocks removes all expired locks from the database
func (db *PostgresDB) CleanupExpiredLocks() error {
	now := time.Now().Unix()
	result := db.Conn.Where("expires_at < ?", now).Delete(&models.AppLock{})
	if result.Error != nil {
		return fmt.Errorf("failed to cleanup expired locks: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		db.logger.Debug("Cleaned up expired locks", "count", result.RowsAffected)
	}

	return nil
}

