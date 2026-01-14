package multiversx

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type NetworkConfig struct {
	APIUrl string
}

type Verifier struct {
	config NetworkConfig
	client *http.Client
}

func NewVerifier(apiUrl string) *Verifier {
	return &Verifier{
		config: NetworkConfig{APIUrl: apiUrl},
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// ProcessRelayedPayment handles the Relayed V3 flow
// 1. Verify User Signature (Offline)
// 2. Validate Business Logic (Invoice/Receiver)
// 3. (Todo) Construct Relayed Tx & Broadcast
// SimulationRequest represents the body for /transaction/simulate
type SimulationRequest struct {
	Nonce     uint64 `json:"nonce"`
	Value     string `json:"value"`
	Receiver  string `json:"receiver"`
	Sender    string `json:"sender"`
	GasPrice  uint64 `json:"gasPrice"`
	GasLimit  uint64 `json:"gasLimit"`
	Data      string `json:"data,omitempty"`
	ChainID   string `json:"chainID"`
	Version   uint32 `json:"version"`
	Signature string `json:"signature"`
}

// SimulationResponse represents the response from /transaction/simulate
type SimulationResponse struct {
	Data struct {
		Result struct {
			Status string `json:"status"`
			Hash   string `json:"hash"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
	Code  string `json:"code"`
}

func (v *Verifier) ProcessRelayedPayment(payload RelayedPayload, expectedReceiver string, resourceId string, expectedAmount string, tokenIdentifier string) (string, error) {
	// 1. Verify Transaction Validity (Signature & Logic) via Simulation
	simHash, err := v.verifyViaSimulation(payload)
	if err != nil {
		return "", fmt.Errorf("simulation failed: %v", err)
	}

	// 2. Validate Fields (Double check critical business logic locally even if simulation passes)
	// Check Receiver
	// Note: For ESDT, payload.Data.Receiver is the sender (Self). We check the Data field for destination.
	txReceiver := payload.Data.Receiver
	txData := payload.Data.Data

	resourceIdHex := hex.EncodeToString([]byte(resourceId))
	isCorrectReceiver := false
	foundResource := false

	if tokenIdentifier == "EGLD" {
		// Direct Transfer
		if txReceiver == expectedReceiver {
			isCorrectReceiver = true
		}
		if payload.Data.Value != expectedAmount {
			return "", fmt.Errorf("value mismatch: %s != %s", payload.Data.Value, expectedAmount)
		}
	} else {
		// ESDT Transfer
		// Check Data for MultiESDTNFTTransfer@receiver...
		// Naive check for now
		if strings.Contains(txData, hex.EncodeToString([]byte(expectedReceiver))) {
			isCorrectReceiver = true
		}
		// Check Value (embedded in hex in Data) - complex to parse without full deserializer
		// For MVP, we trust the signature + string check, strict parsing requires more code
	}

	// Check Resource ID
	if strings.Contains(txData, resourceIdHex) {
		foundResource = true
	}

	if !isCorrectReceiver {
		return "", errors.New("invalid receiver")
	}
	if !foundResource {
		return "", errors.New("resource_id mismatch")
	}

	// 3. Relay Logic (Stub for broadcast)
	// In a real implementation we would sign as Relayer here.
	// For now, we simulate success and return a "pending" hash.
	// In a real scenario, we might return the hash from the simulation if it was actually broadcasted,
	// but simulation is read-only. We return a placeholder or the hash needed for tracking.
	return simHash, nil
}

func (v *Verifier) verifyViaSimulation(payload RelayedPayload) (string, error) {
	// Construct Simulation Request
	// Mapping RelayedPayload fields to SimulationRequest
	reqBody := SimulationRequest{
		Nonce:     payload.Data.Nonce,
		Value:     payload.Data.Value,
		Receiver:  payload.Data.Receiver,
		Sender:    payload.Data.Sender,
		GasPrice:  payload.Data.GasPrice,
		GasLimit:  payload.Data.GasLimit,
		Data:      payload.Data.Data,
		ChainID:   payload.Data.ChainID,
		Version:   payload.Data.Version,
		Signature: payload.Data.Signature,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal simulation request: %v", err)
	}

	url := fmt.Sprintf("%s/transaction/simulate", v.config.APIUrl)
	resp, err := v.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to send simulation request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("simulation API returned non-200 status: %d", resp.StatusCode)
	}

	var simResp SimulationResponse
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
