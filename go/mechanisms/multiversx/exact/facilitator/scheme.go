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

// Proxy defines the interface for interacting with MultiversX blockchain
type Proxy interface {
	GetTransactionStatus(ctx context.Context, hash string) (string, error)
	GetTransactionInfo(ctx context.Context, hash string) (*data.TransactionInfo, error)
	GetTransactionInfoWithResults(ctx context.Context, hash string) (*data.TransactionInfo, error)
	GetAccount(ctx context.Context, address core.AddressHandler) (*data.Account, error)
	SendTransaction(ctx context.Context, tx *transaction.FrontendTransaction) (string, error)
}

// ExactMultiversXScheme implements SchemeNetworkFacilitator
type ExactMultiversXScheme struct {
	config multiversx.NetworkConfig
	proxy  Proxy
	signer multiversx.FacilitatorMultiversXSigner
}

// NewExactMultiversXScheme creates a new facilitator scheme instance
func NewExactMultiversXScheme(apiUrl string, signer multiversx.FacilitatorMultiversXSigner) (*ExactMultiversXScheme, error) {
	args := blockchain.ArgsProxy{
		ProxyURL:            apiUrl,
		Client:              nil,
		SameScState:         false,
		ShouldBeSynced:      false,
		FinalityCheck:       false,
		EntityType:          core.Proxy,
		CacheExpirationTime: time.Minute,
	}
	proxy, err := blockchain.NewProxy(args)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy: %w", err)
	}

	p, ok := interface{}(proxy).(Proxy)
	if !ok {
		return nil, fmt.Errorf("proxy does not implement the required interface")
	}

	return &ExactMultiversXScheme{
		config: multiversx.NetworkConfig{ApiUrl: apiUrl},
		proxy:  p,
		signer: signer,
	}, nil
}

// Scheme returns the scheme identifier ("exact")
func (s *ExactMultiversXScheme) Scheme() string {
	return multiversx.SchemeExact
}

// CaipFamily returns the CAIP network family ("multiversx:*")
func (s *ExactMultiversXScheme) CaipFamily() string {
	return "multiversx:*"
}

// GetExtra returns any extra configuration (none for this scheme)
func (s *ExactMultiversXScheme) GetExtra(network x402.Network) map[string]interface{} {
	return nil
}

// GetSigners returns the addresses of available signers
func (s *ExactMultiversXScheme) GetSigners(network x402.Network) []string {
	if s.signer != nil {
		return s.signer.GetAddresses()
	}
	return []string{}
}

// Verify validates a payment payload against requirements
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

// Settle executes the payment defined in the payload
// It handles both Direct and Relayed V3 transactions
func (s *ExactMultiversXScheme) Settle(ctx context.Context, payload types.PaymentPayload, requirements types.PaymentRequirements) (*x402.SettleResponse, error) {
	relayedPayloadPtr, err := multiversx.PayloadFromMap(payload.Payload)
	if err != nil {
		return nil, x402.NewSettleError("invalid_payload", "", "multiversx", "", err)
	}
	relayedPayload := *relayedPayloadPtr

	tx := relayedPayload.ToTransaction()

	var hash string

	// Default to relayed unless explicit "direct" transfer method is requested
	transferMethod, _ := requirements.Extra["assetTransferMethod"].(string)

	if transferMethod != multiversx.TransferMethodDirect {
		// RELAYED TRANSFER (Relayed V3) - Default
		// Strictly require a signer for relayed transactions
		if s.signer == nil {
			return nil, x402.NewSettleError("configuration_error", relayedPayload.Sender, "multiversx", "", fmt.Errorf("signer required for relayed translation"))
		}

		addresses := s.signer.GetAddresses()
		if len(addresses) == 0 {
			return nil, x402.NewSettleError("no_signer_address", relayedPayload.Sender, "multiversx", "", errors.New("signer has no addresses"))
		}
		facilitatorAddr := addresses[0]

		tx.RelayerAddr = facilitatorAddr
		tx.Version = 2 // Relayed V3 uses version 2

		// Store signature in temporary error variable to avoid shadowing 'err'
		var sig string
		var signErr error
		sig, signErr = s.signer.Sign(ctx, &tx)
		if signErr != nil {
			return nil, x402.NewSettleError("signing_failed", relayedPayload.Sender, "multiversx", "", signErr)
		}
		tx.RelayerSignature = sig
	}

	hash, err = s.proxy.SendTransaction(ctx, &tx)

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

// waitForTx polls the transaction status using the proxy
func (s *ExactMultiversXScheme) waitForTx(ctx context.Context, txHash string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Wait up to 120 seconds
	timeout := time.After(120 * time.Second)

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
			case "success", "successful", "executed":
				return nil
			case "fail", "failed", "invalid":
				return fmt.Errorf("transaction failed with status: %s", status)
			case "pending", "processing", "received", "partially-executed":
				continue
			default:
				// t.Logf("Unknown transaction status: %s", status)
				continue
			}
		}
	}
}

// getTransactionStatus fetches status via the proxy engine
func (s *ExactMultiversXScheme) getTransactionStatus(ctx context.Context, txHash string) (string, error) {
	status, err := s.proxy.GetTransactionStatus(ctx, txHash)
	if err != nil {
		return "", err
	}

	if status == "fail" || status == "failed" || status == "invalid" {
		txInfo, err := s.proxy.GetTransactionInfo(ctx, txHash)
		if err == nil && txInfo.Error != "" {

			return fmt.Sprintf("%s (error: %s)", status, txInfo.Error), nil
		}
	}

	return status, nil
}

func (s *ExactMultiversXScheme) verifyViaSimulation(payload multiversx.ExactRelayedPayload) (string, error) {
	tx := payload.ToTransaction()
	if tx.Version >= 2 && tx.RelayerAddr != "" && s.signer != nil {
		// Attempt to sign as relayer if we hold the key
		addresses := s.signer.GetAddresses()
		for _, addr := range addresses {
			if addr == tx.RelayerAddr {
				// We are the relayer
				sig, err := s.signer.Sign(context.TODO(), &tx)
				if err != nil {
					return "", fmt.Errorf("failed to sign as relayer: %w", err)
				}
				if tx.RelayerSignature == "" {
					tx.RelayerSignature = sig
				}
				break
			}
		}
	}

	txBytes, err := json.Marshal(tx)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/transaction/simulate", s.config.ApiUrl)

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
