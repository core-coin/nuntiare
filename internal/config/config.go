package config

import (
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/core-coin/go-core/v2/common"
	"github.com/joho/godotenv"
)

type Config struct {
	Development bool
	// API configuration
	APIPort int
	// Postgres configuration
	PostgresUser     string
	PostgresPassword string
	PostgresHost     string
	PostgresPort     int
	PostgresDB       string
	// Blockchain configuration
	SmartContractAddress           string
	SmartContractAddressNormalized string // Cached normalized address (lowercase, no 0x prefix)
	BlockchainServiceURL           string
	NetworkID                      *big.Int

	// SMTP configuration
	SMTPHost            string
	SMTPPort            int
	SMTPAlternativePort int
	SMTPUser            string
	SMTPPassword        string
	SMTPSender          string

	// Notification configuration
	TelegramBotToken   string
	TelegramWebhookURL string

	// Well-known configuration
	WellKnownURL string

	// Subscription configuration
	SubscriptionMonthCost     float64 // Cost in CTN for one month of subscription
	SubscriptionMonthDuration float64 // Duration of one month in seconds
}

// GetNetworkName returns the network name for well-known API based on NetworkID
// NetworkID 1 = xcb (mainnet), NetworkID 3 = xab (devin testnet)
func (c *Config) GetNetworkName() string {
	if c.NetworkID.Cmp(big.NewInt(1)) == 0 {
		return "xcb" // Mainnet
	}
	if c.NetworkID.Cmp(big.NewInt(3)) == 0 {
		return "xab" // Devin testnet
	}
	// Default to xab (testnet) for unknown networks
	return "xab"
}

// LoadConfig loads the configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{
		Development:          getEnvAsBool("DEVELOPMENT", false),
		PostgresUser:         getEnv("POSTGRES_USER", "postgres"),
		PostgresPassword:     getEnv("POSTGRES_PASSWORD", "password"),
		PostgresHost:         getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:         getEnvAsInt("POSTGRES_PORT", 5432),
		PostgresDB:           getEnv("POSTGRES_DB", "nuntiare"),
		SmartContractAddress: getEnv("SMART_CONTRACT_ADDRESS", ""),
		BlockchainServiceURL: getEnv("BLOCKCHAIN_SERVICE_URL", "http://localhost:8545"),
		NetworkID:            getEnvAsBigInt("NETWORK_ID", big.NewInt(1)), // Default to Mainnet ID
		TelegramBotToken:     getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramWebhookURL:   getEnv("TELEGRAM_WEBHOOK_URL", ""),
		SMTPHost:             getEnv("SMTP_HOST", "smtp.example.com"),
		SMTPPort:             getEnvAsInt("SMTP_PORT", 587),
		SMTPAlternativePort:  getEnvAsInt("SMTP_ALTERNATIVE_PORT", 465),
		SMTPUser:             getEnv("SMTP_USER", ""),
		SMTPPassword:         getEnv("SMTP_PASSWORD", ""),
		SMTPSender:           getEnv("SMTP_SENDER", ""),

		APIPort: getEnvAsInt("API_PORT", 6532),

		WellKnownURL: getEnv("WELL_KNOWN_URL", "https://coreblockchain.net"),

		SubscriptionMonthCost:     getEnvAsFloat64("SUBSCRIPTION_MONTH_COST", 200.0),      // 200 CTN per month
		SubscriptionMonthDuration: getEnvAsFloat64("SUBSCRIPTION_MONTH_DURATION", 2592000), // 30 days in seconds
	}

	// Set default network ID before validation (required for address validation)
	common.DefaultNetworkID = common.NetworkID(cfg.NetworkID.Int64())

	// Normalize addresses for efficient comparison
	cfg.SmartContractAddressNormalized = normalizeAddress(cfg.SmartContractAddress)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// normalizeAddress converts an address to lowercase without 0x prefix for efficient comparison
func normalizeAddress(addr string) string {
	addr = strings.TrimPrefix(addr, "0x")
	addr = strings.TrimPrefix(addr, "0X")
	return strings.ToLower(addr)
}

// Validate checks that all required configuration fields are properly set
func (c *Config) Validate() error {
	if c.SmartContractAddress == "" {
		return fmt.Errorf("SMART_CONTRACT_ADDRESS is required")
	}

	// Validate smart contract address format
	if _, err := common.HexToAddress(c.SmartContractAddress); err != nil {
		return fmt.Errorf("invalid SMART_CONTRACT_ADDRESS format: %w", err)
	}

	if c.BlockchainServiceURL == "" {
		return fmt.Errorf("BLOCKCHAIN_SERVICE_URL is required")
	}

	if c.WellKnownURL == "" {
		return fmt.Errorf("WELL_KNOWN_URL is required")
	}

	if c.PostgresDB == "" {
		return fmt.Errorf("POSTGRES_DB is required")
	}

	if c.PostgresHost == "" {
		return fmt.Errorf("POSTGRES_HOST is required")
	}

	// Validate subscription configuration to prevent division by zero
	if c.SubscriptionMonthCost <= 0 {
		return fmt.Errorf("SUBSCRIPTION_MONTH_COST must be greater than 0, got %f", c.SubscriptionMonthCost)
	}

	if c.SubscriptionMonthDuration <= 0 {
		return fmt.Errorf("SUBSCRIPTION_MONTH_DURATION must be greater than 0, got %f", c.SubscriptionMonthDuration)
	}

	return nil
}

// Helper functions to read environment variables
func getEnv(key string, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(name string, defaultValue int) int {
	if valueStr, exists := os.LookupEnv(name); exists {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnvAsBool(name string, defaultValue bool) bool {
	if valueStr, exists := os.LookupEnv(name); exists {
		if value, err := strconv.ParseBool(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnvAsBigInt(name string, defaultValue *big.Int) *big.Int {
	if valueStr, exists := os.LookupEnv(name); exists {
		if value, ok := new(big.Int).SetString(valueStr, 10); ok {
			return value
		}
	}
	return defaultValue
}

func getEnvAsFloat64(name string, defaultValue float64) float64 {
	if valueStr, exists := os.LookupEnv(name); exists {
		if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return value
		}
	}
	return defaultValue
}
