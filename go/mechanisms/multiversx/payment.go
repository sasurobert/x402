package multiversx

import "math/big"

// PaymentDetails represents the decoded x402 payment token (TxHash)
type PaymentDetails struct {
	TxHash string
}

// TransactionAPIResponse models the MultiversX API response
type TransactionAPIResponse struct {
	Hash     string `json:"hash"`
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Value    string `json:"value"` // Atomic units
	Status   string `json:"status"`
	Data     string `json:"data"` // "pay@..."
	Action   struct {
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
