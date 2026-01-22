package facilitator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/coinbase/x402/go/mechanisms/multiversx"

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/types"
)

// ExactMultiversXScheme implements SchemeNetworkFacilitator
type ExactMultiversXScheme struct {
	config multiversx.NetworkConfig
	client *http.Client
}

func NewExactMultiversXScheme(apiUrl string) *ExactMultiversXScheme {
	return &ExactMultiversXScheme{
		config: multiversx.NetworkConfig{APIUrl: apiUrl},
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *ExactMultiversXScheme) Scheme() string {
	return multiversx.SchemeExact
}

func (s *ExactMultiversXScheme) CaipFamily() string {
	return "multiversx:*"
}

func (s *ExactMultiversXScheme) GetExtra(network x402.Network) map[string]interface{} {
	return nil
}

func (s *ExactMultiversXScheme) GetSigners(network x402.Network) []string {
	return []string{}
}

func (s *ExactMultiversXScheme) Verify(ctx context.Context, payload types.PaymentPayload, requirements types.PaymentRequirements) (*x402.VerifyResponse, error) {
	// 1. Unmarshal directly to ExactRelayedPayload
	var relayedPayload multiversx.ExactRelayedPayload

	// Convert map to struct via JSON (easiest way without mapstructure dependency)
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(payloadBytes, &relayedPayload); err != nil {
		return nil, fmt.Errorf("invalid payload format: %v", err)
	}

	// 2. Perform Verification using Universal logic
	isValid, err := multiversx.VerifyPayment(ctx, relayedPayload, requirements, s.verifyViaSimulation)
	if err != nil {
		return nil, err // Returns invalid reason wrapped
	}
	if !isValid {
		return nil, fmt.Errorf("verification failed")
	}

	// 2.1 Enforce Validity Windows
	now := time.Now().Unix()
	if relayedPayload.Data.ValidBefore > 0 && now > relayedPayload.Data.ValidBefore {
		return nil, fmt.Errorf("payment expired (validBefore: %d, now: %d)", relayedPayload.Data.ValidBefore, now)
	}
	if relayedPayload.Data.ValidAfter > 0 && now < relayedPayload.Data.ValidAfter {
		return nil, fmt.Errorf("payment not yet valid (validAfter: %d, now: %d)", relayedPayload.Data.ValidAfter, now)
	}

	// 3. Validate Requirements (Specific Fields)
	expectedReceiver := requirements.PayTo
	expectedAmount := requirements.Amount
	if expectedAmount == "" {
		return nil, errors.New("requirement amount is empty")
	}

	reqAsset := requirements.Asset
	if reqAsset == "" {
		reqAsset = "EGLD"
	}

	txData := relayedPayload.Data

	if reqAsset == "EGLD" {
		// Case A: Direct EGLD
		if txData.Receiver != expectedReceiver {
			return nil, fmt.Errorf("receiver mismatch: expected %s, got %s", expectedReceiver, txData.Receiver)
		}
		if !multiversx.CheckBigInt(txData.Value, expectedAmount) {
			return nil, fmt.Errorf("amount mismatch: expected %s, got %s", expectedAmount, txData.Value)
		}
	} else {
		// Case B: ESDT Transfer
		parts := strings.Split(txData.Data, "@")
		if len(parts) < 6 || parts[0] != "MultiESDTNFTTransfer" {
			return nil, errors.New("invalid ESDT transfer data format")
		}

		// Decode Receiver (parts[1]) - Hex (Destination)
		destHex := parts[1]
		if !multiversx.IsValidHex(destHex) {
			return nil, fmt.Errorf("invalid receiver hex")
		}

		// STRICT VERIFICATION: Ensure destHex matches expectedReceiver (PayTo)
		// expectedReceiver is Bech32 (erd1...). We must decode it to get the pubkey hex.
		_, pubKeyBytes, err := multiversx.DecodeBech32(expectedReceiver)
		if err != nil {
			return nil, fmt.Errorf("invalid expected receiver format (not bech32): %v", err)
		}
		expectedHex := hex.EncodeToString(pubKeyBytes)

		if destHex != expectedHex {
			return nil, fmt.Errorf("receiver mismatch: encoded destination %s does not match requirement %s (%s)", destHex, expectedReceiver, expectedHex)
		}

		// Token Hex
		tokenBytes, err := hex.DecodeString(parts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid token hex")
		}
		if string(tokenBytes) != reqAsset {
			return nil, fmt.Errorf("asset mismatch: expected %s, got %s", reqAsset, string(tokenBytes))
		}

		// Amount Hex
		amountBytes, err := hex.DecodeString(parts[5])
		if err != nil {
			return nil, fmt.Errorf("invalid amount hex")
		}
		amountBig := new(big.Int).SetBytes(amountBytes)
		expectedBig, ok := new(big.Int).SetString(expectedAmount, 10)
		if !ok {
			return nil, fmt.Errorf("invalid expected amount: %s", expectedAmount)
		}
		if amountBig.Cmp(expectedBig) < 0 {
			return nil, fmt.Errorf("amount too low or invalid")
		}
	}

	return &x402.VerifyResponse{
		IsValid: true,
	}, nil
}

func (s *ExactMultiversXScheme) Settle(ctx context.Context, payload types.PaymentPayload, requirements types.PaymentRequirements) (*x402.SettleResponse, error) {
	// 1. Recover ExactRelayedPayload
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var relayedPayload multiversx.ExactRelayedPayload
	if err := json.Unmarshal(payloadBytes, &relayedPayload); err != nil {
		return nil, fmt.Errorf("invalid payload format: %w", err)
	}

	// 2. Prepare Transaction for Broadcasting
	// multiversx-api expects base64 encoded data for /transaction/simulate, likely same for /transaction/send
	// or it accepts the raw string which it handles. Using base64 for consistency with verify.
	reqBody := struct {
		Nonce     uint64 `json:"nonce"`
		Value     string `json:"value"`
		Receiver  string `json:"receiver"`
		Sender    string `json:"sender"`
		GasPrice  uint64 `json:"gasPrice"`
		GasLimit  uint64 `json:"gasLimit"`
		Data      string `json:"data,omitempty"`
		Signature string `json:"signature"`
		ChainID   string `json:"chainID"`
		Version   uint32 `json:"version"`
	}{
		Nonce:     relayedPayload.Data.Nonce,
		Value:     relayedPayload.Data.Value,
		Receiver:  relayedPayload.Data.Receiver,
		Sender:    relayedPayload.Data.Sender,
		GasPrice:  relayedPayload.Data.GasPrice,
		GasLimit:  relayedPayload.Data.GasLimit,
		Data:      base64.StdEncoding.EncodeToString([]byte(relayedPayload.Data.Data)),
		Signature: relayedPayload.Data.Signature,
		ChainID:   relayedPayload.Data.ChainID,
		Version:   relayedPayload.Data.Version,
	}

	// 3. Broadcast to /transaction/send
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tx request: %w", err)
	}

	url := fmt.Sprintf("%s/transaction/send", s.config.APIUrl)
	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast transaction: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var bodyErr map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&bodyErr)
		return nil, fmt.Errorf("broadcast API returned error: %d %v", resp.StatusCode, bodyErr)
	}

	var txResp struct {
		Data struct {
			TxHash string `json:"txHash"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&txResp); err != nil {
		return nil, fmt.Errorf("failed to decode broadcast response: %w", err)
	}

	if txResp.Error != "" {
		return nil, fmt.Errorf("broadcast error: %s", txResp.Error)
	}

	return &x402.SettleResponse{
		Success:     true,
		Transaction: txResp.Data.TxHash,
	}, nil
}

func (s *ExactMultiversXScheme) verifyViaSimulation(payload multiversx.ExactRelayedPayload) (string, error) {
	reqBody := multiversx.SimulationRequest{
		Nonce:     payload.Data.Nonce,
		Value:     payload.Data.Value,
		Receiver:  payload.Data.Receiver,
		Sender:    payload.Data.Sender,
		GasPrice:  payload.Data.GasPrice,
		GasLimit:  payload.Data.GasLimit,
		Data:      base64.StdEncoding.EncodeToString([]byte(payload.Data.Data)),
		ChainID:   payload.Data.ChainID,
		Version:   payload.Data.Version,
		Signature: payload.Data.Signature,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal simulation request: %v", err)
	}

	url := fmt.Sprintf("%s/transaction/simulate", s.config.APIUrl)
	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to send simulation request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var bodyErr map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&bodyErr)
		return "", fmt.Errorf("simulation API returned non-200/201 status: %d Body: %v", resp.StatusCode, bodyErr)
	}

	var simResp multiversx.SimulationResponse
	if err := json.NewDecoder(resp.Body).Decode(&simResp); err != nil {
		return "", fmt.Errorf("failed to decode simulation response: %v", err)
	}

	if simResp.Error != "" {
		return "", fmt.Errorf("simulation returned error: %s (code: %s)", simResp.Error, simResp.Code)
	}

	if simResp.Data.Result.Status != "success" {
		return "", fmt.Errorf("simulation status not success: %s", simResp.Data.Result.Status)
	}

	return simResp.Data.Result.Hash, nil
}
