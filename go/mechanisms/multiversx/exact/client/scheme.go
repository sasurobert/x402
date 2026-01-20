package client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/multiversx/mx-sdk-go/blockchain"
	"github.com/multiversx/mx-sdk-go/core"
	"github.com/multiversx/mx-sdk-go/data"

	"github.com/coinbase/x402/go/mechanisms/multiversx"

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/types"
)

// ExactMultiversXScheme implements SchemeNetworkClient
type ExactMultiversXScheme struct {
	signer multiversx.ClientMultiversXSigner
}

func NewExactMultiversXScheme(signer multiversx.ClientMultiversXSigner) *ExactMultiversXScheme {
	return &ExactMultiversXScheme{
		signer: signer,
	}
}

func (s *ExactMultiversXScheme) Scheme() string {
	return multiversx.SchemeExact
}

func (s *ExactMultiversXScheme) CreatePaymentPayload(ctx context.Context, requirements types.PaymentRequirements) (types.PaymentPayload, error) {
	// 1. Validate inputs
	if requirements.PayTo == "" {
		return types.PaymentPayload{}, fmt.Errorf("PayTo is required")
	}

	// STRICT VERIFICATION: PayTo must be a valid Bech32 address
	if _, _, err := multiversx.DecodeBech32(requirements.PayTo); err != nil {
		return types.PaymentPayload{}, fmt.Errorf("invalid PayTo address (must be valid Bech32): %w", err)
	}

	// 2. Prepare Transaction Data
	// Default Gas settings for V1 - can be optimized or made dynamic later
	gasLimit := uint64(50000)
	gasPrice := uint64(1000000000)

	version := uint32(1)
	chainID := "1" // Default to Mainnet "1" if not specified
	apiURL := "https://api.multiversx.com"

	if requirements.Network != "" {
		_, ref, err := x402.Network(requirements.Network).Parse()
		if err == nil {
			chainID = ref
			if chainID == "D" {
				apiURL = "https://devnet-api.multiversx.com"
			} else if chainID == "T" {
				apiURL = "https://testnet-api.multiversx.com"
			}
		}
	}

	sender := s.signer.Address()
	receiver := requirements.PayTo
	value := requirements.Amount // Already big int string

	// FETCH NONCE from Network
	args := blockchain.ArgsProxy{
		ProxyURL:            apiURL,
		Client:              nil, // Use default http client
		SameScState:         false,
		ShouldBeSynced:      false,
		FinalityCheck:       false,
		EntityType:          core.Proxy,
		CacheExpirationTime: time.Minute,
	}
	proxy, err := blockchain.NewProxy(args)
	if err != nil {
		// If proxy creation fails, we fallback to 0 or error?
		// Verification requires correct nonce. Error is safer.
		return types.PaymentPayload{}, fmt.Errorf("failed to create proxy: %w", err)
	}

	// Create address object
	address, err := data.NewAddressFromBech32String(sender)
	if err != nil {
		return types.PaymentPayload{}, fmt.Errorf("invalid sender address: %w", err)
	}

	// Fetch account
	account, err := proxy.GetAccount(ctx, address)
	var nonce uint64
	if err != nil {
		// Log warning but maybe allow fallback if network is down?
		// No, for "Exact", if we sign wrong nonce, it fails.
		return types.PaymentPayload{}, fmt.Errorf("failed to fetch account state: %w", err)
	}
	nonce = account.Nonce

	// ESDT Logic
	dataString := ""
	// Normalize Asset
	asset := requirements.Asset

	if asset != "" && asset != "EGLD" {
		// ESDT Transfer (MultiESDTNFTTransfer)
		receiver = sender
		value = "0"
		gasLimit = 60000000 // Higher gas for SC call

		// Encode Data: MultiESDTNFTTransfer@<DestHex>@01@<TokenHex>@00@<AmountHex>
		// PayTo is already validated as Bech32 above.
		_, decodedBytes, _ := multiversx.DecodeBech32(requirements.PayTo)
		destHex := hex.EncodeToString(decodedBytes)

		tokenHex := hex.EncodeToString([]byte(requirements.Asset))

		amtBig, ok := new(big.Int).SetString(requirements.Amount, 10)
		if !ok {
			return types.PaymentPayload{}, fmt.Errorf("invalid amount: %s", requirements.Amount)
		}
		amtHex := amtBig.Text(16)
		if len(amtHex)%2 != 0 {
			amtHex = "0" + amtHex
		}

		// MultiESDTNFTTransfer format
		// MultiESDTNFTTransfer@<DestHex>@01@<TokenHex>@00@<AmountHex>@<ResourceID>

		// Extract ResourceID from Extra if present
		var resourceIdHex string
		if rid, ok := requirements.Extra["resourceId"].(string); ok && rid != "" {
			resourceIdHex = hex.EncodeToString([]byte(rid))
		}

		if resourceIdHex != "" {
			dataString = fmt.Sprintf("MultiESDTNFTTransfer@%s@01@%s@00@%s@%s", destHex, tokenHex, amtHex, resourceIdHex)
		} else {
			dataString = fmt.Sprintf("MultiESDTNFTTransfer@%s@01@%s@00@%s", destHex, tokenHex, amtHex)
		}

	} else {
		// EGLD
		// If resourceId is present, put it in data?
		// Replicating TS logic:
		if rid, ok := requirements.Extra["resourceId"].(string); ok && rid != "" {
			dataString = rid
		} else {
			dataString = ""
		}
	}

	// 3. Construct Payload Object
	now := time.Now().Unix()
	validAfter := now - 600 // 10 minutes ago
	validBefore := now + int64(requirements.MaxTimeoutSeconds)

	// Note: We sign the standard transaction fields. ValidAfter/Before are x402 metadata.
	txData := struct {
		Nonce    uint64 `json:"nonce"`
		Value    string `json:"value"`
		Receiver string `json:"receiver"`
		Sender   string `json:"sender"`
		GasPrice uint64 `json:"gasPrice"`
		GasLimit uint64 `json:"gasLimit"`
		Data     string `json:"data,omitempty"`
		ChainID  string `json:"chainID"`
		Version  uint32 `json:"version"`
	}{
		Nonce:    nonce,
		Value:    value,
		Receiver: receiver,
		Sender:   sender,
		GasPrice: gasPrice,
		GasLimit: gasLimit,
		Data:     dataString,
		ChainID:  chainID,
		Version:  version,
	}

	// 4. Serialize for Signing (Canonical JSON)
	txBytes, err := json.Marshal(txData)
	if err != nil {
		return types.PaymentPayload{}, err
	}

	// 5. Sign
	sigBytes, err := s.signer.Sign(ctx, txBytes)
	if err != nil {
		return types.PaymentPayload{}, err
	}

	// 6. Build Final Payload
	exactPayload := multiversx.ExactRelayedPayload{
		Scheme: multiversx.SchemeExact,
	}
	// struct copy
	exactPayload.Data.Nonce = txData.Nonce
	exactPayload.Data.Value = txData.Value
	exactPayload.Data.Receiver = txData.Receiver
	exactPayload.Data.Sender = txData.Sender
	exactPayload.Data.GasPrice = txData.GasPrice
	exactPayload.Data.GasLimit = txData.GasLimit
	exactPayload.Data.Data = txData.Data
	exactPayload.Data.ChainID = txData.ChainID
	exactPayload.Data.Version = txData.Version
	exactPayload.Data.Signature = hex.EncodeToString(sigBytes)
	exactPayload.Data.ValidAfter = validAfter
	exactPayload.Data.ValidBefore = validBefore

	// Return Map
	payloadBytes, _ := json.Marshal(exactPayload)
	var finalMap map[string]interface{}
	json.Unmarshal(payloadBytes, &finalMap)

	return types.PaymentPayload{
		X402Version: 2,
		Payload:     finalMap,
	}, nil
}
