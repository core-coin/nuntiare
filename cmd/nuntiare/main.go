package main

import (
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/core-coin/nuntiare/internal/blockchain"
	"github.com/core-coin/nuntiare/internal/config"
	"github.com/core-coin/nuntiare/internal/http_api"
	"github.com/core-coin/nuntiare/internal/notificator"
	"github.com/core-coin/nuntiare/internal/nuntiare"
	"github.com/core-coin/nuntiare/internal/repository"
	"github.com/core-coin/nuntiare/pkg/logger"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "nuntiare",
		Usage: "Nuntiare is a blockchain notification service",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "postgres-user", Aliases: []string{"u"}, Usage: "Postgres user"},
			&cli.StringFlag{Name: "postgres-password", Aliases: []string{"p"}, Usage: "Postgres password"},
			&cli.StringFlag{Name: "postgres-host", Aliases: []string{"t"}, Usage: "Postgres host"},
			&cli.IntFlag{Name: "postgres-port", Aliases: []string{"P"}, Usage: "Postgres port"},
			&cli.StringFlag{Name: "postgres-db", Aliases: []string{"d"}, Usage: "Postgres database name"},
			&cli.StringFlag{Name: "blockchain-service-url", Aliases: []string{"b"}, Usage: "Blockchain service URL"},
			&cli.StringFlag{Name: "smart-contract-address", Aliases: []string{"s"}, Usage: "Smart contract address"},
			&cli.BoolFlag{Name: "development", Aliases: []string{"D"}, Usage: "Development mode"},
			&cli.StringFlag{Name: "minimal-balance-for-notif", Aliases: []string{"m"}, Usage: "Minimal balance for notification"},
		},
		Action: func(c *cli.Context) error {
			return run(c)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	// Load configuration from environment variables
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Override with flags if set
	if c.IsSet("postgres-user") {
		cfg.PostgresUser = c.String("postgres-user")
	}
	if c.IsSet("postgres-password") {
		cfg.PostgresPassword = c.String("postgres-password")
	}
	if c.IsSet("postgres-host") {
		cfg.PostgresHost = c.String("postgres-host")
	}
	if c.IsSet("postgres-port") {
		cfg.PostgresPort = c.Int("postgres-port")
	}
	if c.IsSet("postgres-db") {
		cfg.PostgresDB = c.String("postgres-db")
	}
	if c.IsSet("blockchain-service-url") {
		cfg.BlockchainServiceURL = c.String("blockchain-service-url")
	}
	if c.IsSet("smart-contract-address") {
		cfg.SmartContractAddress = c.String("smart-contract-address")
	}
	if c.IsSet("development") {
		cfg.Development = c.Bool("development")
	}
	if c.IsSet("minimal-balance-for-notif") {
		minimalBalance, ok := new(big.Int).SetString(c.String("minimal-balance-for-notif"), 10)
		if ok {
			cfg.MinimalBalanceForNotif = minimalBalance
		}
	}

	// Initialize logger
	log, err := logger.NewLogger(cfg.Development)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %v", err)
	}

	// Initialize database
	db, err := repository.NewPostgresDB(cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresDB, cfg.PostgresHost, cfg.PostgresPort, log)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	// Initialize blockchain service
	blockchainService := blockchain.NewGocore(cfg.BlockchainServiceURL, log)
	// Initialize notificator
	notificator := notificator.NewNotificator(log)
	// Initialize API server
	// Create Nuntiare instance
	nuntiareApp := nuntiare.NewNuntiare(db, blockchainService, notificator, log, cfg.MinimalBalanceForNotif)

	apiServer := http_api.NewHTTPServer(nuntiareApp, cfg.BlockchainServiceURL)

	go apiServer.Start()
	// Start the application
	nuntiareApp.Start()

	return nil
}
