package multiversx

import "math/big"

// RelayedPayload matches the JSON sent by the Client
type RelayedPayload struct {
	Scheme string `json:"scheme"`
	Data   struct {
		Nonce     uint64 `json:"nonce"`
		Value     string `json:"value"`
		Receiver  string `json:"receiver"`
		Sender    string `json:"sender"`
		GasPrice  uint64 `json:"gasPrice"`
		GasLimit  uint64 `json:"gasLimit"`
		Data      string `json:"data"`
		ChainID   string `json:"chainID"`
		Version   uint32 `json:"version"`
		Options   uint32 `json:"options"`
		Signature string `json:"signature"` // Hex encoded
	} `json:"data"`
}

// PaymentDetails represents the decoded x402 payment token (TxHash)
type PaymentDetails struct {
	TxHash string
}

// TransactionAPIResponse models the MultiversX API response
type TransactionAPIResponse struct {
	Hash      string `json:"hash"`
	Sender    string `json:"sender"`
	Receiver  string `json:"receiver"`
	Value     string `json:"value"` // Atomic units
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
	Data      string `json:"data"` // "pay@..."
	Action    struct {
		Arguments struct {
			Transfers []struct {
				Token      string `json:"token"`
				Identifier string `json:"tokenIdentifier"`
				Value      string `json:"value"`
			} `json:"transfers"`
		} `json:"arguments"`
		Receiver string `json:"receiver"` // sometimes useful for ESDT
	} `json:"action"`
	Logs struct {
		Events []struct {
			Address    string   `json:"address"`
			Identifier string   `json:"identifier"`
			Topics     []string `json:"topics"`
			Data       string   `json:"data"`
		} `json:"events"`
	} `json:"logs"`
}

// Helper to check big int logic if needed
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
