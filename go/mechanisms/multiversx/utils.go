package multiversx

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

var tokenIDRegex = regexp.MustCompile(`^[A-Z0-9]{3,8}-[0-9a-fA-F]{6}$`)

// IsValidTokenID checks if the token ID follows the MultiversX ESDT format
// Format: Ticker-Nonce (Ticker: 3-8 alphanumeric uppercase, Nonce: 6 hex chars)
// Regex enforces: ^[A-Z0-9]{3,8}-[0-9a-fA-F]{6}$
func IsValidTokenID(tokenID string) bool {
	return tokenIDRegex.MatchString(tokenID)
}

// GetMultiversXChainId returns the chain ID for a given network string
// Supports "multiversx:1", "multiversx:D", "multiversx:T", or legacy short names
func GetMultiversXChainId(network string) (string, error) {
	// Normalize
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

	// Parse CAIP-2 or custom format "multiversx:Ref"
	if strings.HasPrefix(net, "multiversx:") {
		ref := strings.TrimPrefix(net, "multiversx:")
		// Ref must be 1, D, T usually
		if ref == "1" || ref == "D" || ref == "T" {
			return ref, nil
		}
		// Allow custom
		return ref, nil
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
		// Fallback to mainnet if unknown? Or explicit empty?
		// Usually Mainnet is default.
		return "https://api.multiversx.com"
	}
}

// IsValidAddress checks if addres is valid Bech32 with Checksum
func IsValidAddress(address string) bool {
	// 1. Basic length check (erd1... is 62)
	if len(address) != 62 {
		return false
	}

	// 2. Full Bech32 Decode & Checksum Verify
	hrp, _, err := DecodeBech32(address)
	if err != nil {
		return false
	}

	// 3. Check HRP is "erd"
	if hrp != "erd" {
		return false
	}

	return true
}

// IsValidHex checks if string is valid hex (length check optional?)
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

// CalculateGasLimit estimates the gas limit for a transaction
// Formula: 50k base + 1.5k * data + 200k * transfers + 50k relayed
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

// SerializeTransaction creates the bytes to be signed
func SerializeTransaction(tx transaction.FrontendTransaction) ([]byte, error) {
	// Standard JSON serialization of the map of fields
	// We use a map to relying on encoding/json to sort keys mostly.
	// Keys: chainID, data, gasLimit, gasPrice, nonce, options, receiver, sender, value, version.
	// This ALPHABETICAL order is standard for MultiversX (canonical JSON).
	m := map[string]interface{}{
		"nonce":    tx.Nonce,
		"value":    tx.Value,
		"receiver": tx.Receiver,
		"sender":   tx.Sender,
		"gasPrice": tx.GasPrice,
		"gasLimit": tx.GasLimit,
		"data":     string(tx.Data), // Convert []byte to string for proper JSON string representation
		"chainID":  tx.ChainID,
		"version":  tx.Version,
		"options":  tx.Options,
	}

	return json.Marshal(m)
}
