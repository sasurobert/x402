package facilitator

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/multiversx/mx-sdk-go/blockchain"
	"github.com/multiversx/mx-sdk-go/core"

	"github.com/coinbase/x402/go/mechanisms/multiversx"

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/types"
)

// ExactMultiversXScheme implements SchemeNetworkFacilitator
type ExactMultiversXScheme struct {
	config multiversx.NetworkConfig
	proxy  blockchain.Proxy
}

func NewExactMultiversXScheme(apiUrl string) *ExactMultiversXScheme {
	args := blockchain.ArgsProxy{
		ProxyURL:            apiUrl,
		Client:              nil,
		SameScState:         false,
		ShouldBeSynced:      false,
		FinalityCheck:       false,
		EntityType:          core.Proxy,
		CacheExpirationTime: time.Minute,
	}
	proxy, _ := blockchain.NewProxy(args) // Error ignored as it mainly checks client (nil) or cache

	return &ExactMultiversXScheme{
		// Casing fixed: ApiUrl matches types.go
		config: multiversx.NetworkConfig{ApiUrl: apiUrl},
		proxy:  proxy,
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
	// TODO: If the facilitator holds a wallet to pay gas, return its address here.
	return []string{}
}

func (s *ExactMultiversXScheme) Verify(ctx context.Context, payload types.PaymentPayload, requirements types.PaymentRequirements) (*x402.VerifyResponse, error) {
	// 1. Unmarshal directly to ExactRelayedPayload using json mapping
	// Optimization: Avoid double marshal/unmarshal if possible, but map->struct usually requires it or mapstructure.
	// Using generic JSON roundtrip for simplicity and correctness with struct tags.
	var relayedPayload multiversx.ExactRelayedPayload
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		return nil, fmt.Errorf("payload marshal failed: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, &relayedPayload); err != nil {
		return nil, x402.NewVerifyError(x402.ErrCodeInvalidPayment, "", "multiversx", fmt.Errorf("invalid payload format: %v", err))
	}

	// 2. Perform Verification using Universal logic
	isValid, err := multiversx.VerifyPayment(ctx, relayedPayload, requirements, s.verifyViaSimulation)
	if err != nil {
		return nil, err
	}
	if !isValid {
		return nil, x402.NewVerifyError(x402.ErrCodeSignatureInvalid, relayedPayload.Sender, "multiversx", nil)
	}

	// 2.1 Enforce Validity Windows
	now := uint64(time.Now().Unix())
	if relayedPayload.ValidBefore > 0 && now > relayedPayload.ValidBefore {
		return nil, fmt.Errorf("payment expired (validBefore: %d, now: %d)", relayedPayload.ValidBefore, now)
	}
	if relayedPayload.ValidAfter > 0 && now < relayedPayload.ValidAfter {
		return nil, fmt.Errorf("payment not yet valid (validAfter: %d, now: %d)", relayedPayload.ValidAfter, now)
	}

	// 3. Validate Requirements
	expectedReceiver := requirements.PayTo
	expectedAmount := requirements.Amount
	if expectedAmount == "" {
		return nil, errors.New("requirement amount is empty")
	}

	reqAsset := requirements.Asset
	if reqAsset == "" {
		return nil, errors.New("requirement asset is required")
	}

	txData := relayedPayload
	transferMethod, _ := requirements.Extra["assetTransferMethod"].(string)

	if reqAsset == multiversx.NativeTokenTicker && transferMethod != multiversx.TransferMethodESDT {
		// Case A: Direct EGLD OR Smart Contract Call paying EGLD
		if txData.Receiver != expectedReceiver {
			return nil, fmt.Errorf("receiver mismatch: expected %s, got %s", expectedReceiver, txData.Receiver)
		}
		if !multiversx.CheckBigInt(txData.Value, expectedAmount) {
			return nil, fmt.Errorf("amount mismatch: expected %s, got %s", expectedAmount, txData.Value)
		}
		// Data field check is loose for Direct unless specified otherwise (e.g. strict data match).
	} else {
		// Case B: ESDT Transfer (MultiESDTNFTTransfer)
		// Or Case C: EGLD via Smart Contract (wrapped/unwrapped logic not typical here, usually Direct)
		parts := strings.Split(txData.Data, "@")
		if len(parts) < 6 || parts[0] != "MultiESDTNFTTransfer" {
			return nil, errors.New("invalid ESDT transfer data format (expected MultiESDTNFTTransfer)")
		}

		// Decode Destination (Receiver of tokens)
		destHex := parts[1]
		if !multiversx.IsValidHex(destHex) {
			return nil, fmt.Errorf("invalid receiver hex")
		}

		// STRICT VERIFICATION: Ensure destHex matches expectedReceiver (PayTo)
		_, pubKeyBytes, err := multiversx.DecodeBech32(expectedReceiver)
		if err != nil {
			return nil, fmt.Errorf("invalid expected receiver format: %v", err)
		}
		expectedHex := hex.EncodeToString(pubKeyBytes)

		if destHex != expectedHex {
			return nil, fmt.Errorf("receiver mismatch: encoded destination %s does not match requirement %s", destHex, expectedReceiver)
		}

		// Token Identifier
		tokenBytes, err := hex.DecodeString(parts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid token hex")
		}
		if string(tokenBytes) != reqAsset {
			return nil, fmt.Errorf("asset mismatch: expected %s, got %s", reqAsset, string(tokenBytes))
		}

		// Amount
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
	// 1. Recover RelayedPayload
	var relayedPayload multiversx.ExactRelayedPayload
	payloadBytes, err := json.Marshal(payload.Payload) // Optimization: type assertion if possible
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, &relayedPayload); err != nil {
		return nil, x402.NewSettleError("invalid_payload", "", "multiversx", "", err)
	}

	// 2. Prepare Transaction
	tx := relayedPayload.ToTransaction()

	// 3. Broadcast
	hash, err := s.proxy.SendTransaction(ctx, &tx)
	if err != nil {
		return nil, x402.NewSettleError("broadcast_failed", relayedPayload.Sender, "multiversx", "", err)
	}

	// 4. Wait for Completion
	if err := s.waitForTx(ctx, hash); err != nil {
		return nil, x402.NewSettleError("tx_failed", relayedPayload.Sender, "multiversx", hash, err)
	}

	return &x402.SettleResponse{
		Success:     true,
		Transaction: hash,
	}, nil
}

// waitForTx polls the transaction status using direct API usage since Proxy interface lacks it
func (s *ExactMultiversXScheme) waitForTx(ctx context.Context, txHash string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Wait up to 60 seconds
	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for tx %s", txHash)
		case <-ticker.C:
			// status: "success", "pending", etc.
			status, err := s.getTransactionStatus(ctx, txHash)
			if err != nil {
				continue // retry on transient errors
			}

			switch status {
			case "success", "successful":
				return nil
			case "fail", "invalid":
				return fmt.Errorf("transaction failed with status: %s", status)
			case "pending", "processing", "received":
				continue
			default:
				// Unknown status, assume pending
				continue
			}
		}
	}
}

// getTransactionStatus fetches status via HTTP since Proxy interface doesn't expose it
func (s *ExactMultiversXScheme) getTransactionStatus(ctx context.Context, txHash string) (string, error) {
	url := fmt.Sprintf("%s/transaction/%s/status", s.config.ApiUrl, txHash)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api error: %s", resp.Status)
	}

	var res struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", errors.New(res.Error)
	}
	return res.Data.Status, nil
}

func (s *ExactMultiversXScheme) verifyViaSimulation(payload multiversx.ExactRelayedPayload) (string, error) {
	// Call /transaction/simulate
	tx := payload.ToTransaction()

	// Create simulation struct expected by API
	// Similar to transaction.FrontendTransaction but might wrapped
	// We send the tx directly as FrontendTransaction JSON

	url := fmt.Sprintf("%s/transaction/simulate", s.config.ApiUrl)
	txBytes, _ := json.Marshal(&tx) // FrontendTransaction marshals correctly

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(txBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("simulation api error: %s - %s", resp.Status, string(body))
	}

	var res struct {
		Data struct {
			Result struct {
				Status string `json:"status"` // Might be used in some versions?
				Hash   string `json:"hash"`
			} `json:"result"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"` // Top level code
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", errors.New(res.Error)
	}

	// Check Code first
	if res.Code == "successful" {
		hash := res.Data.Result.Hash
		if hash == "" {
			hash = "simulated"
		}
		return hash, nil
	}

	// Fallback check
	if res.Data.Result.Status != "success" && res.Data.Result.Status != "successful" {
		return "", fmt.Errorf("simulation status not success: %s (code: %s)", res.Data.Result.Status, res.Code)
	}

	// If simulate returns a hash (it might not be the TX hash but simulation hash)
	// We usually return the Tx Hash we computed or the simulation receipt hash.
	// For verification purpose, success is enough.
	return res.Data.Result.Hash, nil
}
