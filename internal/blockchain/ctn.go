package blockchain

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/core-coin/go-core/v2/common"
	"github.com/core-coin/go-core/v2/core/types"
)

// CTNABI is the ABI of the Core Token contract (CBC20 standard)
const CTNABI = `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"spender","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Transfer","type":"event"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"spender","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address[]","name":"recipients","type":"address[]"},{"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"name":"batchTransfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"subtractedValue","type":"uint256"}],"name":"decreaseAllowance","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"addedValue","type":"uint256"}],"name":"increaseAllowance","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"sender","type":"address"},{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transferFrom","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"}]`

// ABI encoding offsets and lengths (all values in hex characters, not bytes)
// Standard Ethereum/Core ABI encoding uses 32-byte (64 hex char) slots
const (
	// Method selector is the first 4 bytes (8 hex chars) of the Keccak-256 hash of the function signature
	methodSelectorLength = 8

	// Standard ABI slot size: 32 bytes = 64 hex characters
	abiSlotSize = 64

	// Address encoding offsets
	// Format: 4 bytes method selector + 12 bytes padding + 20 bytes address = 44 bytes = 88 hex chars per address
	addressStartOffset = 28  // Skip method selector (8) + padding (20) = 28
	addressEndOffset   = 72  // Start (28) + address (44) = 72

	// Amount/value encoding offsets (second parameter slot)
	amountStartOffset = 72  // After first address slot
	amountEndOffset   = 136 // 72 + 64 = 136

	// Count/third parameter offset (for batch transfers)
	countStartOffset = 136 // After two slots (method + address + amount)
	countEndOffset   = 200 // 136 + 64 = 200

	// transferFrom "from" address offset (first parameter)
	transferFromFromStart = 28 // Same as regular address
	transferFromFromEnd   = 72

	// transferFrom "to" address offset (second parameter)
	transferFromToStart = 92  // After first slot + 20 padding
	transferFromToEnd   = 136 // 92 + 44 = 136

	// Minimum input lengths for validation
	minTransferInputLength     = 136 // method(8) + to_address(64) + amount(64) = 136
	minBatchTransferLength     = 200 // method(8) + recipients_offset(64) + amounts_offset(64) + count(64) = 200
	minTransferFromInputLength = 200 // method(8) + from(64) + to(64) + amount(64) = 200
)

// CBC721ABI is the ABI for CBC721 (ERC721) tokens
const CBC721ABI = `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":true,"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"Transfer","type":"event"}]`

// CBC721 Transfer event signature: keccak256("Transfer(address,address,uint256)")
// Core blockchain uses: 0xc17a9d92b89f27cb79cc390f23a1a5d302fefab8c7911075ede952ac2b5607a1
const cbc721TransferEventSignature = "c17a9d92b89f27cb79cc390f23a1a5d302fefab8c7911075ede952ac2b5607a1"

const (
	// transfer(address,uint256)
	transfer = "4b40e901"
	// batchTransfer(address[],uint256[])
	batchTransfer = "e86e7c5f"
	// transferFrom(address,address,uint256)
	transferFrom = "31f2e679"
)

type Transfer struct {
	From         string
	To           string
	Amount       float64
	TokenAddress string // Contract address for the token
	TokenSymbol  string // Token symbol (e.g., CTN, USDT)
	TokenType    string // Token type (CBC20, CBC721)
	TokenID      string // For CBC721 NFTs
	TxHash       string // Transaction hash
	NetworkID    int64  // Network ID (1 for mainnet, 3 for devnet)
}

// CheckForCTNTransfer checks if a transaction is a CTN transfer
// This is kept for backward compatibility and subscription payment detection
func CheckForCTNTransfer(tx *types.Transaction, CTNAddress string, networkID int64) ([]*Transfer, error) {
	return CheckForCBC20Transfer(tx, CTNAddress, "CTN", 18, networkID)
}

// CheckForCBC20Transfer checks if a transaction is a CBC20 token transfer
func CheckForCBC20Transfer(tx *types.Transaction, tokenAddress, tokenSymbol string, decimals int, networkID int64) ([]*Transfer, error) {
	txHash := tx.Hash().String()
	signer := types.NewNucleusSigner(big.NewInt(int64(common.DefaultNetworkID)))

	receiver := tx.To().Hex()
	sender, err := signer.Sender(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender: %w", err)
	}
	input := common.Bytes2Hex(tx.Data())
	if receiver != tokenAddress {
		return nil, nil
	}

	// Validate minimum input length for method selector
	if len(input) < methodSelectorLength {
		return nil, nil // Not enough data for method selector
	}

	// Calculate the divisor based on decimals
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))

	switch input[:methodSelectorLength] {
	case transfer:
		if len(input) < minTransferInputLength {
			return nil, fmt.Errorf("invalid transfer input length: %d, expected at least %d", len(input), minTransferInputLength)
		}
		// Parse: transfer(address to, uint256 amount)
		recipientAddr := input[addressStartOffset:addressEndOffset]
		amountHex := input[amountStartOffset:amountEndOffset]
		amount, _ := big.NewFloat(0).Quo(new(big.Float).SetInt(big.NewInt(0).SetBytes(common.Hex2Bytes(amountHex))), divisor).Float64()
		return []*Transfer{
			{
				From:         sender.Hex(),
				To:           recipientAddr,
				Amount:       amount,
				TokenAddress: tokenAddress,
				TokenSymbol:  tokenSymbol,
				TokenType:    "CBC20",
				TxHash:       txHash,
				NetworkID:    networkID,
			},
		}, nil
	case batchTransfer:
		if len(input) < minBatchTransferLength {
			return nil, fmt.Errorf("invalid batchTransfer input length: %d, expected at least %d", len(input), minBatchTransferLength)
		}
		transfers := []*Transfer{}
		offset := countStartOffset
		count, ok := new(big.Int).SetString(input[countStartOffset:countEndOffset], 16)
		if !ok {
			return nil, fmt.Errorf("cannot convert batch transfer count to big.Int: %s", input[countStartOffset:countEndOffset])
		}

		// Validate count to prevent out-of-bounds access
		countInt := int(count.Int64())
		if countInt < 0 || countInt > 1000 {
			return nil, fmt.Errorf("invalid batch transfer count: %d (must be between 0 and 1000)", countInt)
		}

		// Validate that we have enough data for all transfers
		requiredLength := offset + 192 + countInt*64 + countInt*64
		if len(input) < requiredLength {
			return nil, fmt.Errorf("insufficient data for batch transfer: got %d, need %d for %d transfers", len(input), requiredLength, countInt)
		}

		for i := 0; i < countInt; i++ {
			toStart := offset + 84 + i*64
			toEnd := offset + 128 + i*64
			valueStart := offset + 128 + countInt*64 + i*64
			valueEnd := offset + 192 + countInt*64 + i*64

			// Additional bounds check for safety
			if toEnd > len(input) || valueEnd > len(input) {
				return nil, fmt.Errorf("array index out of bounds in batch transfer at index %d", i)
			}

			to := input[toStart:toEnd]
			value := input[valueStart:valueEnd]
			amount, _ := big.NewFloat(0).Quo(new(big.Float).SetInt(big.NewInt(0).SetBytes(common.Hex2Bytes(value))), divisor).Float64()
			transfers = append(transfers, &Transfer{
				From:         sender.Hex(),
				To:           to,
				Amount:       amount,
				TokenAddress: tokenAddress,
				TokenSymbol:  tokenSymbol,
				TokenType:    "CBC20",
				TxHash:       txHash,
				NetworkID:    networkID,
			})
		}
		return transfers, nil
	case transferFrom:
		if len(input) < minTransferFromInputLength {
			return nil, fmt.Errorf("invalid transferFrom input length: %d, expected at least %d", len(input), minTransferFromInputLength)
		}
		// Parse: transferFrom(address from, address to, uint256 amount)
		fromAddr := input[transferFromFromStart:transferFromFromEnd]
		toAddr := input[transferFromToStart:transferFromToEnd]
		amountHex := input[amountEndOffset:countEndOffset]
		amount, _ := big.NewFloat(0).Quo(new(big.Float).SetInt(big.NewInt(0).SetBytes(common.Hex2Bytes(amountHex))), divisor).Float64()
		return []*Transfer{
			{
				From:         fromAddr,
				To:           toAddr,
				Amount:       amount,
				TokenAddress: tokenAddress,
				TokenSymbol:  tokenSymbol,
				TokenType:    "CBC20",
				TxHash:       txHash,
				NetworkID:    networkID,
			},
		}, nil
	}

	return nil, nil
}

// CheckForCBC721Transfer checks if a transaction is a CBC721 (NFT) transfer
// This function is kept for backward compatibility and for detecting transfers from input data
// For proper event-based detection, use CheckForCBC721TransferFromReceipt instead
func CheckForCBC721Transfer(tx *types.Transaction, tokenAddress, tokenSymbol string, networkID int64) ([]*Transfer, error) {
	txHash := tx.Hash().String()
	receiver := tx.To().Hex()
	if receiver != tokenAddress {
		return nil, nil
	}

	// Parse input data for transferFrom calls
	input := common.Bytes2Hex(tx.Data())

	// Validate minimum input length for method selector
	if len(input) < methodSelectorLength {
		return nil, nil // Not enough data for method selector
	}

	// For CBC721, we look for transferFrom or safeTransferFrom
	// transferFrom(address from, address to, uint256 tokenId) = 0x31f2e679
	// safeTransferFrom would have a different signature
	switch input[:methodSelectorLength] {
	case transferFrom:
		if len(input) < minTransferFromInputLength {
			return nil, fmt.Errorf("invalid CBC721 transferFrom input length: %d, expected at least %d", len(input), minTransferFromInputLength)
		}
		// For NFTs, the third parameter is tokenId (not amount)
		fromAddr := input[transferFromFromStart:transferFromFromEnd]
		toAddr := input[transferFromToStart:transferFromToEnd]
		tokenID := input[amountEndOffset:countEndOffset] // TokenID is in the amount slot
		return []*Transfer{
			{
				From:         fromAddr,
				To:           toAddr,
				Amount:       1, // NFTs are always 1 unit
				TokenAddress: tokenAddress,
				TokenSymbol:  tokenSymbol,
				TokenType:    "CBC721",
				TokenID:      tokenID,
				TxHash:       txHash,
				NetworkID:    networkID,
			},
		}, nil
	}

	return nil, nil
}

// CheckForCBC721TransferFromReceipt parses transaction receipt logs for CBC721 Transfer events
// This is the proper way to detect NFT transfers as they emit Transfer events
func CheckForCBC721TransferFromReceipt(receipt *types.Receipt, tokenAddress, tokenSymbol string, txHash string, networkID int64) ([]*Transfer, error) {
	if receipt == nil {
		return nil, nil
	}

	transfers := []*Transfer{}

	// Parse logs for Transfer events
	for _, log := range receipt.Logs {
		// Check if log is from the token contract
		// Compare by matching the raw address bytes (last N chars of token address)
		logAddr := strings.TrimPrefix(strings.ToLower(log.Address.Hex()), "0x")
		tokenAddr := strings.ToLower(tokenAddress)

		// Compare raw address: if token address is longer, compare with its suffix
		tokenAddrToCompare := tokenAddr
		if len(tokenAddr) > len(logAddr) {
			tokenAddrToCompare = tokenAddr[len(tokenAddr)-len(logAddr):]
		}

		if logAddr != tokenAddrToCompare {
			continue
		}

		// CBC721 Transfer events have 4 topics:
		// topics[0]: event signature
		// topics[1]: from address (indexed)
		// topics[2]: to address (indexed)
		// topics[3]: tokenId (indexed)
		if len(log.Topics) != 4 {
			continue
		}

		// Check if this is a Transfer event
		eventSig := log.Topics[0].Hex()
		expectedSig := "0x" + cbc721TransferEventSignature
		if eventSig != expectedSig {
			continue
		}

		// Extract from, to, and tokenId from topics
		// Topics are 32 bytes (64 hex chars), Core addresses are 22 bytes (44 hex chars)
		// Addresses are right-aligned in topics, so extract last 44 hex chars
		fromAddrFull := log.Topics[1].Hex() // 0x + 64 hex chars
		toAddrFull := log.Topics[2].Hex()
		tokenIDHex := log.Topics[3].Hex()

		// Extract last 44 hex chars for Core addresses
		// Remove 0x prefix first: fromAddrFull has "0x" + 64 chars = 66 total
		fromAddrRaw := strings.TrimPrefix(fromAddrFull, "0x")
		toAddrRaw := strings.TrimPrefix(toAddrFull, "0x")

		fromAddr := strings.ToLower(fromAddrRaw[len(fromAddrRaw)-44:])
		toAddr := strings.ToLower(toAddrRaw[len(toAddrRaw)-44:])

		// Remove 0x prefix from tokenID
		tokenIDHex = strings.TrimPrefix(tokenIDHex, "0x")

		transfers = append(transfers, &Transfer{
			From:         fromAddr,
			To:           toAddr,
			Amount:       1, // NFTs are always 1 unit
			TokenAddress: tokenAddress,
			TokenSymbol:  tokenSymbol,
			TokenType:    "CBC721",
			TokenID:      tokenIDHex,
			TxHash:       txHash,
			NetworkID:    networkID,
		})
	}

	return transfers, nil
}
