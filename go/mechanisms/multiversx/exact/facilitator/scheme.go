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

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-sdk-go/blockchain"
	"github.com/multiversx/mx-sdk-go/core"
	"github.com/multiversx/mx-sdk-go/data"

	"github.com/coinbase/x402/go/mechanisms/multiversx"

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/types"
)

// ProxyWithStatus extends the base Proxy interface with processing status check
type ProxyWithStatus interface {
	blockchain.Proxy
	ProcessTransactionStatus(ctx context.Context, txHash string) (transaction.TxStatus, error)
}

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
	proxy, _ := blockchain.NewProxy(args)

	return &ExactMultiversXScheme{
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
	relayedPayloadPtr, err := multiversx.PayloadFromMap(payload.Payload)
	if err != nil {
		return nil, x402.NewVerifyError(x402.ErrCodeInvalidPayment, "", "multiversx", fmt.Errorf("invalid payload format: %v", err))
	}
	relayedPayload := *relayedPayloadPtr

	isValid, err := multiversx.VerifyPayment(ctx, relayedPayload, requirements, s.verifyViaSimulation)
	if err != nil {
		return nil, err
	}
	if !isValid {
		return nil, x402.NewVerifyError(x402.ErrCodeSignatureInvalid, relayedPayload.Sender, "multiversx", nil)
	}

	now := uint64(time.Now().Unix())
	if relayedPayload.ValidBefore > 0 && now > relayedPayload.ValidBefore {
		return nil, fmt.Errorf("payment expired (validBefore: %d, now: %d)", relayedPayload.ValidBefore, now)
	}
	if relayedPayload.ValidAfter > 0 && now < relayedPayload.ValidAfter {
		return nil, fmt.Errorf("payment not yet valid (validAfter: %d, now: %d)", relayedPayload.ValidAfter, now)
	}

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
		if txData.Receiver != expectedReceiver {
			return nil, fmt.Errorf("receiver mismatch: expected %s, got %s", expectedReceiver, txData.Receiver)
		}
		if !multiversx.CheckBigInt(txData.Value, expectedAmount) {
			return nil, fmt.Errorf("amount mismatch: expected %s, got %s", expectedAmount, txData.Value)
		}
	} else {
		parts := strings.Split(txData.Data, "@")
		if len(parts) < 6 || parts[0] != "MultiESDTNFTTransfer" {
			return nil, errors.New("invalid ESDT transfer data format (expected MultiESDTNFTTransfer)")
		}

		destHex := parts[1]
		if !multiversx.IsValidHex(destHex) {
			return nil, fmt.Errorf("invalid receiver hex")
		}

		expectedAddr, err := data.NewAddressFromBech32String(expectedReceiver)
		if err != nil {
			return nil, fmt.Errorf("invalid expected receiver format: %v", err)
		}
		expectedHex := hex.EncodeToString(expectedAddr.AddressBytes())

		if destHex != expectedHex {
			return nil, fmt.Errorf("receiver mismatch: encoded destination %s does not match requirement %s", destHex, expectedReceiver)
		}

		tokenBytes, err := hex.DecodeString(parts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid token hex")
		}
		if string(tokenBytes) != reqAsset {
			return nil, fmt.Errorf("asset mismatch: expected %s, got %s", reqAsset, string(tokenBytes))
		}

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
	relayedPayloadPtr, err := multiversx.PayloadFromMap(payload.Payload)
	if err != nil {
		return nil, x402.NewSettleError("invalid_payload", "", "multiversx", "", err)
	}
	relayedPayload := *relayedPayloadPtr

	tx := relayedPayload.ToTransaction()

	hash, err := s.proxy.SendTransaction(ctx, &tx)
	if err != nil {
		return nil, x402.NewSettleError("broadcast_failed", relayedPayload.Sender, "multiversx", "", err)
	}

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
				continue
			}
		}
	}
}

// getTransactionStatus fetches status via the proxy engine
func (s *ExactMultiversXScheme) getTransactionStatus(ctx context.Context, txHash string) (string, error) {
	proxyWithStatus, ok := s.proxy.(ProxyWithStatus)
	if !ok {
		return "", fmt.Errorf("proxy implementation does not support status checking")
	}

	status, err := proxyWithStatus.ProcessTransactionStatus(ctx, txHash)
	if err != nil {
		return "", err
	}

	return string(status), nil
}

func (s *ExactMultiversXScheme) verifyViaSimulation(payload multiversx.ExactRelayedPayload) (string, error) {
	tx := payload.ToTransaction()

	url := fmt.Sprintf("%s/transaction/simulate", s.config.ApiUrl)
	txBytes, _ := json.Marshal(&tx)

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
				Status string `json:"status"`
				Hash   string `json:"hash"`
			} `json:"result"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", errors.New(res.Error)
	}

	if res.Code == "successful" {
		hash := res.Data.Result.Hash
		if hash == "" {
			hash = "simulated"
		}
		return hash, nil
	}

	if res.Data.Result.Status != "success" && res.Data.Result.Status != "successful" {
		return "", fmt.Errorf("simulation status not success: %s (code: %s)", res.Data.Result.Status, res.Code)
	}

	return res.Data.Result.Hash, nil
}
