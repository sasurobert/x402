package multiversx

import (
	"math/big"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

// SchemeExact is the identifier for the exact payment scheme
const (
	SchemeExact = "exact"

	// Chain IDs
	ChainIDMainnet = "1"
	ChainIDDevnet  = "D"
	ChainIDTestnet = "T"

	// Gas Constants
	GasLimitStandard = 50_000
	GasLimitESDT     = 60_000_000
	GasPriceDefault  = 1_000_000_000

	// Token Constants
	NativeTokenTicker = "EGLD"
	// Transfer Methods
	TransferMethodESDT   = "esdt"
	TransferMethodDirect = "direct"
)

// NetworkConfig holds network-specific configuration
type NetworkConfig struct {
	ChainID     string
	MinGasLimit uint64
	BaseEGLDPay uint64 // e.g., for storage tests or minimums, usually 0 or dust
	MinGasPrice uint64
	GasPerByte  uint64
	ApiUrl      string // URL for MultiversX Proxy/API
	ExplorerUrl string // URL for Explorer (optional)
	NativeToken string // "EGLD"
}

// PaymentPayload is the output of the Scheme
// It matches the requirements for the payment verification and broadcast
type PaymentPayload struct {
	// For Exact, we might return a serialized Transaction to be signed
	// Or a structured object.
	// We'll use a generic struct that can be marshaled to JSON.
	Payload interface{} `json:"payload"`
}

// ExactRelayedPayload defines the structure for a transaction that might be relayed
// This matches the structure expected by MultiversX tools (or our custom facilitator)
type ExactRelayedPayload struct {
	Nonce       uint64 `json:"nonce"`
	Value       string `json:"value"` // BigInt as string
	Receiver    string `json:"receiver"`
	Sender      string `json:"sender"`
	GasPrice    uint64 `json:"gasPrice"`
	GasLimit    uint64 `json:"gasLimit"`
	Data        string `json:"data,omitempty"` // Base64 or plain text? Ideally plain text for transaction construction, but SDK usually handles raw bytes. We use string here.
	ChainID     string `json:"chainID"`
	Version     uint32 `json:"version"`
	Options     uint32 `json:"options,omitempty"`
	Signature   string `json:"signature,omitempty"`   // Hex encoded
	ValidAfter  uint64 `json:"validAfter,omitempty"`  // Timestamp/Nonce
	ValidBefore uint64 `json:"validBefore,omitempty"` // Timestamp/Nonce
}

// ToMap converts the payload to a map for JSON marshaling
func (p *ExactRelayedPayload) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"nonce":       p.Nonce,
		"value":       p.Value,
		"receiver":    p.Receiver,
		"sender":      p.Sender,
		"gasPrice":    p.GasPrice,
		"gasLimit":    p.GasLimit,
		"data":        p.Data,
		"chainID":     p.ChainID,
		"version":     p.Version,
		"options":     p.Options,
		"signature":   p.Signature,
		"validAfter":  p.ValidAfter,
		"validBefore": p.ValidBefore,
	}
}

// PayloadFromMap creates an ExactRelayedPayload from a map
func PayloadFromMap(data map[string]interface{}) (*ExactRelayedPayload, error) {
	p := &ExactRelayedPayload{}

	if val, ok := data["nonce"].(uint64); ok {
		p.Nonce = val
	} else if val, ok := data["nonce"].(float64); ok {
		p.Nonce = uint64(val)
	}

	if val, ok := data["value"].(string); ok {
		p.Value = val
	}

	if val, ok := data["receiver"].(string); ok {
		p.Receiver = val
	}

	if val, ok := data["sender"].(string); ok {
		p.Sender = val
	}

	if val, ok := data["gasPrice"].(uint64); ok {
		p.GasPrice = val
	} else if val, ok := data["gasPrice"].(float64); ok {
		p.GasPrice = uint64(val)
	}

	if val, ok := data["gasLimit"].(uint64); ok {
		p.GasLimit = val
	} else if val, ok := data["gasLimit"].(float64); ok {
		p.GasLimit = uint64(val)
	}

	if val, ok := data["data"].(string); ok {
		p.Data = val
	}

	if val, ok := data["chainID"].(string); ok {
		p.ChainID = val
	}

	if val, ok := data["version"].(uint32); ok {
		p.Version = val
	} else if val, ok := data["version"].(float64); ok {
		p.Version = uint32(val)
	}

	if val, ok := data["options"].(uint32); ok {
		p.Options = val
	} else if val, ok := data["options"].(float64); ok {
		p.Options = uint32(val)
	}

	if val, ok := data["signature"].(string); ok {
		p.Signature = val
	}

	if val, ok := data["validAfter"].(uint64); ok {
		p.ValidAfter = val
	} else if val, ok := data["validAfter"].(float64); ok {
		p.ValidAfter = uint64(val)
	}

	if val, ok := data["validBefore"].(uint64); ok {
		p.ValidBefore = val
	} else if val, ok := data["validBefore"].(float64); ok {
		p.ValidBefore = uint64(val)
	}

	return p, nil
}

// ToTransaction converts the payload to an SDK Transaction struct
// Since ExactRelayedPayload uses string for Data, we convert it to []byte
// Note: Signature is also populated if present
func (p *ExactRelayedPayload) ToTransaction() transaction.FrontendTransaction {
	return transaction.FrontendTransaction{
		Nonce:     p.Nonce,
		Value:     p.Value,
		Receiver:  p.Receiver,
		Sender:    p.Sender,
		GasPrice:  p.GasPrice,
		GasLimit:  p.GasLimit,
		Data:      []byte(p.Data),
		ChainID:   p.ChainID,
		Version:   p.Version,
		Options:   p.Options,
		Signature: p.Signature,
	}
}

// TransactionData was removed in favor of sdkData.Transaction and ExactRelayedPayload.
// If needed for internal utility helpers, we can cast or convert.

// Helper to check big int logic
func CheckBigInt(valStr string, expected string) bool {
	val, ok := new(big.Int).SetString(valStr, 10)
	if !ok {
		return false
	}
	exp, ok := new(big.Int).SetString(expected, 10)
	if !ok {
		return false
	}
	return val.Cmp(exp) >= 0
}
