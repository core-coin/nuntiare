package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/core-coin/go-core/v2"
	"github.com/core-coin/go-core/v2/accounts/abi"
	"github.com/core-coin/go-core/v2/accounts/abi/bind"
	"github.com/core-coin/go-core/v2/common"
	"github.com/core-coin/go-core/v2/core/types"
	"github.com/core-coin/go-core/v2/xcbclient"
	"github.com/core-coin/nuntiare/internal/config"
	"github.com/core-coin/nuntiare/pkg/logger"
)

const (
	// BlockHeaderChannelBuffer is the buffer size for the block header channel
	// Sized to handle ~1.5 minute of blocks assuming ~7s block time
	BlockHeaderChannelBuffer = 15
)

type Gocore struct {
	logger       *logger.Logger
	config       *config.Config
	apiURL       string
	client       *xcbclient.Client

	mu           sync.RWMutex
	subscription core.Subscription

	ctnContract *bind.BoundContract
}

// NewGocore creates a new Gocore instance.
func NewGocore(apiURL string, logger *logger.Logger, config *config.Config) *Gocore {
	return &Gocore{apiURL: apiURL, logger: logger, config: config}
}

func (g *Gocore) Run() error {
	err := g.ConnectToRPC()
	if err != nil {
		return fmt.Errorf("failed to connect to the core RPC server: %w", err)
	}
	err = g.BuildBindings()
	if err != nil {
		return fmt.Errorf("failed to build bindings: %w", err)
	}
	return nil
}

func (g *Gocore) ConnectToRPC() error {
	client, err := xcbclient.Dial(g.apiURL)
	if err != nil {
		return fmt.Errorf("failed to connect to the core RPC server: %w", err)
	}
	g.client = client
	return nil
}

func (g *Gocore) BuildBindings() error {
	ctnAddress, err := common.HexToAddress(g.config.SmartContractAddress)
	if err != nil {
		return fmt.Errorf("failed to parse Core Token contract address: %w", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(CTNABI))
	if err != nil {
		return fmt.Errorf("failed to parse Core Token ABI: %w", err)
	}

	contract := bind.NewBoundContract(ctnAddress, parsedABI, g.client, g.client, g.client)
	g.ctnContract = contract

	return nil
}

func (g *Gocore) NewHeaderSubscription() (core.Subscription, <-chan *types.Header, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Unsubscribe from previous subscription if it exists to prevent resource leak
	if g.subscription != nil {
		g.subscription.Unsubscribe()
		g.subscription = nil
	}

	channel := make(chan *types.Header, BlockHeaderChannelBuffer)

	subscription, err := g.client.SubscribeNewHead(context.Background(), channel)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to subscribe to new head: %w", err)
	}
	g.subscription = subscription

	return subscription, channel, nil
}

func (g *Gocore) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.subscription != nil {
		g.subscription.Unsubscribe()
		g.subscription = nil
	}
	if g.client != nil {
		g.client.Close()
	}

	return nil
}

func (g *Gocore) GetBlockByNumber(number uint64) (*types.Block, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	block, err := g.client.BlockByNumber(ctx, big.NewInt(int64(number)))
	if err != nil {
		return nil, fmt.Errorf("failed to get block by number: %w", err)
	}

	return block, nil
}

func (g *Gocore) GetAddressCTNBalance(wallet string) (*big.Int, error) {
	results := []interface{}{}
	err := g.ctnContract.Call(nil, &results, "balanceOf", wallet)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}
	balance := results[0].(*big.Int)
	return balance, nil
}

func (g *Gocore) GetTransactionReceipt(txHash string) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hash := common.HexToHash(txHash)
	receipt, err := g.client.TransactionReceipt(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}
	return receipt, nil
}
