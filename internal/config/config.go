package config

import (
	"math/big"
	"os"
	"strconv"

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
	SmartContractAddress string
	BlockchainServiceURL string
	NetworkID            *big.Int

	// SMTP configuration
	SMTPHost            string
	SMTPPort            int
	SMTPAlternativePort int
	SMTPUser            string
	SMTPPassword        string
	SMTPSender          string

	// Notification configuration
	TelegramBotToken string
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
		SMTPHost:             getEnv("SMTP_HOST", "smtp.example.com"),
		SMTPPort:             getEnvAsInt("SMTP_PORT", 587),
		SMTPAlternativePort:  getEnvAsInt("SMTP_ALTERNATIVE_PORT", 465),
		SMTPUser:             getEnv("SMTP_USER", ""),
		SMTPPassword:         getEnv("SMTP_PASSWORD", ""),
		SMTPSender:           getEnv("SMTP_SENDER", ""),

		APIPort: getEnvAsInt("API_PORT", 6532),
	}

	return cfg, nil
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
