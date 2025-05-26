package blockchain

import (
	"fmt"
	"log"
	"math/big"

	"github.com/core-coin/go-core/v2/common"
	"github.com/core-coin/go-core/v2/core/types"
)

// CTNABI is the ABI of the Core Token contract
const CTNABI = `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"spender","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Transfer","type":"event"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"spender","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address[]","name":"recipients","type":"address[]"},{"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"name":"batchTransfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"subtractedValue","type":"uint256"}],"name":"decreaseAllowance","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"addedValue","type":"uint256"}],"name":"increaseAllowance","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"sender","type":"address"},{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transferFrom","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"}]`

const (
	// transfer(address,uint256)
	transfer = "4b40e901"
	// batchTransfer(address[],uint256[])
	batchTransfer = "e86e7c5f"
	// transferFrom(address,address,uint256)
	transferFrom = "31f2e679"
)

type Transfer struct {
	From   string
	To     string
	Amount float64
}

func CheckForCTNTransfer(tx *types.Transaction, CTNAddress string) ([]*Transfer, error) {
	signer := types.NewNucleusSigner(big.NewInt(int64(common.DefaultNetworkID)))

	receiver := tx.To().Hex()
	sender, err := signer.Sender(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender: %s", err)
	}
	input := common.Bytes2Hex(tx.Data())
	if receiver != CTNAddress {
		return nil, nil
	}

	switch input[:8] {
	case transfer:
		amount, _ := big.NewFloat(0).Quo(new(big.Float).SetInt(big.NewInt(0).SetBytes(common.Hex2Bytes(input[72:136]))), big.NewFloat(1e18)).Float64()
		return []*Transfer{
			{
				From:   sender.Hex(),
				To:     input[28:72],
				Amount: amount,
			},
		}, nil
	case batchTransfer:
		transfers := []*Transfer{}
		offset := 136
		count, ok := new(big.Int).SetString(input[136:200], 16)
		if !ok {
			log.Fatalf("cannot convert count to big.Int: %s\n", input[136:200])
		}
		for i := 0; i < int(count.Int64()); i++ {
			to := input[offset+84+i*64 : offset+128+i*64]
			value := input[offset+128+int(count.Int64())*64+i*64 : offset+192+int(count.Int64())*64+i*64]
			amount, _ := big.NewFloat(0).Quo(new(big.Float).SetInt(big.NewInt(0).SetBytes(common.Hex2Bytes(value))), big.NewFloat(1e18)).Float64()
			transfers = append(transfers, &Transfer{
				From:   sender.Hex(),
				To:     to,
				Amount: amount,
			})
		}
		return transfers, nil
	case transferFrom:
		amount, _ := big.NewFloat(0).Quo(new(big.Float).SetInt(big.NewInt(0).SetBytes(common.Hex2Bytes(input[136:200]))), big.NewFloat(1e18)).Float64()
		return []*Transfer{
			{
				From:   input[28:72],
				To:     input[92:136],
				Amount: amount,
			},
		}, nil
	}

	return nil, nil
}
