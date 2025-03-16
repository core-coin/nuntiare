package models

import (
	"math/big"

	"github.com/core-coin/go-core/v2/core/types"
)

// BlockchainService represents a service that interacts with a blockchain.
type BlockchainService interface {
	NewHeaderSubscription() (<-chan *types.Header, error)
	GetBlockByNumber(number uint64) (*types.Block, error)
	GetAddressCTNBalance(address string) (*big.Int, error)
}
