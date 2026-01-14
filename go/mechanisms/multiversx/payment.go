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
