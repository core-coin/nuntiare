package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/core-coin/go-core/v2"
	"github.com/core-coin/go-core/v2/accounts/abi"
	"github.com/core-coin/go-core/v2/accounts/abi/bind"
	"github.com/core-coin/go-core/v2/common"
	"github.com/core-coin/go-core/v2/core/types"
	"github.com/core-coin/go-core/v2/xcbclient"
	"github.com/core-coin/nuntiare/pkg/logger"
)

type Gocore struct {
	logger *logger.Logger

	apiURL       string
	client       *xcbclient.Client
	subscription core.Subscription

	ctnContract *bind.BoundContract
}

// NewGocore creates a new Gocore instance.
func NewGocore(apiURL string, logger *logger.Logger) *Gocore {
	return &Gocore{apiURL: apiURL, logger: logger}
}

func (g *Gocore) Run() error {
	err := g.ConnectToRPC()
	if err != nil {
		g.logger.Fatalf("failed to connect to the core RPC server: %s", err)
	}
	err = g.BuildBindings()
	if err != nil {
		g.logger.Fatalf("failed to build bindings: %s", err)
	}
	return err
}

func (g *Gocore) ConnectToRPC() error {
	client, err := xcbclient.Dial(g.apiURL)
	if err != nil {
		return fmt.Errorf("failed to connect to the core RPC server: %s", err)
	}
	g.client = client
	return nil
}

func (g *Gocore) BuildBindings() error {
	ctnAddress, err := common.HexToAddress(CTNAddress)
	if err != nil {
		g.logger.Fatalf("failed to parse Core Token contract address: %s", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(CTNABI))
	if err != nil {
		g.logger.Fatalf("failed to parse Core Token ABI: %s", err)
	}

	contract := bind.NewBoundContract(ctnAddress, parsedABI, g.client, g.client, g.client)
	g.ctnContract = contract

	return nil
}

func (g *Gocore) NewHeaderSubscription() (<-chan *types.Header, error) {
	channel := make(chan *types.Header)

	subscription, err := g.client.SubscribeNewHead(context.Background(), channel)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to new head: %s", err)
	}
	g.subscription = subscription

	return channel, nil
}

func (g *Gocore) Close() error {
	g.subscription.Unsubscribe()
	g.client.Close()

	return nil
}

func (g *Gocore) GetBlockByNumber(number uint64) (*types.Block, error) {
	block, err := g.client.BlockByNumber(context.Background(), big.NewInt(int64(number)))
	if err != nil {
		return nil, fmt.Errorf("failed to get block by number: %s", err)
	}

	return block, nil
}

func (g *Gocore) GetAddressCTNBalance(wallet string) (*big.Int, error) {
	results := []interface{}{}
	err := g.ctnContract.Call(nil, &results, "balanceOf", wallet)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %s", err)
	}
	balance := results[0].(*big.Int)
	return balance, nil
}
