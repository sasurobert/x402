package multiversx

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/multiversx/mx-sdk-go/data"
)

var tokenIDRegex = regexp.MustCompile(`^[A-Z0-9]{3,8}-[0-9a-fA-F]{6}$`)

// IsValidTokenID checks if the token ID follows the MultiversX ESDT format (Ticker-Nonce)
func IsValidTokenID(tokenID string) bool {
	return tokenIDRegex.MatchString(tokenID)
}

// GetMultiversXChainId returns the chain ID for a given network string
// Supports "multiversx:1", "multiversx:D", "multiversx:T", or legacy short names
func GetMultiversXChainId(network string) (string, error) {
	net := network

	// Map common aliases
	switch net {
	case "mainnet", "multiversx-mainnet":
		return "1", nil
	case "devnet", "multiversx-devnet":
		return "D", nil
	case "testnet", "multiversx-testnet":
		return "T", nil
	}

	if strings.HasPrefix(net, "multiversx:") {
		ref := strings.TrimPrefix(net, "multiversx:")
		if ref == "1" || ref == "D" || ref == "T" {
			return ref, nil
		}

	}

	return "", fmt.Errorf("unsupported network format: %s", network)
}

// GetAPIURL returns the MultiversX API URL for a given Chain ID
func GetAPIURL(chainID string) string {
	switch chainID {
	case ChainIDDevnet:
		return "https://devnet-api.multiversx.com"
	case ChainIDTestnet:
		return "https://testnet-api.multiversx.com"
	case ChainIDMainnet:
		return "https://api.multiversx.com"
	default:
		return "https://api.multiversx.com"
	}
}

func IsValidAddress(address string) bool {
	if len(address) != 62 {
		return false
	}

	_, err := data.NewAddressFromBech32String(address)
	return err == nil
}

// IsValidHex checks if string is valid hex
func IsValidHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

// BytesToHex helper
func BytesToHex(b []byte) string {
	return hex.EncodeToString(b)
}

// CheckAmount verifies decimal amount string
func CheckAmount(amount string) (*big.Int, error) {
	i, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount: %s", amount)
	}

	return i, nil
}

func CalculateGasLimit(data []byte, numTransfers int) uint64 {
	const BaseCost = 50000
	const GasPerByte = 1500
	const MultiTransferCost = 200000
	const RelayedCost = 50000

	return BaseCost +
		(GasPerByte * uint64(len(data))) +
		(MultiTransferCost * uint64(numTransfers)) +
		RelayedCost
}
