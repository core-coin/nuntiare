package validation

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// ValidateAddress validates a blockchain address format
func ValidateAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("address cannot be empty")
	}

	// Remove 0x prefix if present
	normalized := strings.TrimPrefix(addr, "0x")
	normalized = strings.TrimPrefix(normalized, "0X")

	// Check length (44 hex characters = 22 bytes)
	if len(normalized) != 44 {
		return fmt.Errorf("invalid address length: expected 44 characters (without 0x), got %d", len(normalized))
	}

	// Validate hex format
	if _, err := hex.DecodeString(normalized); err != nil {
		return fmt.Errorf("invalid hex address: %w", err)
	}

	return nil
}

// NormalizeAddress converts an address to lowercase without 0x prefix
func NormalizeAddress(addr string) string {
	addr = strings.TrimPrefix(addr, "0x")
	addr = strings.TrimPrefix(addr, "0X")
	return strings.ToLower(addr)
}

// ValidateAndNormalizeAddress validates an address and returns its normalized form
func ValidateAndNormalizeAddress(addr string) (string, error) {
	if err := ValidateAddress(addr); err != nil {
		return "", err
	}
	return NormalizeAddress(addr), nil
}
