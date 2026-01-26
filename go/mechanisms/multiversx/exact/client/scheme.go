package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/multiversx/mx-sdk-go/blockchain"
	"github.com/multiversx/mx-sdk-go/core"
	"github.com/multiversx/mx-sdk-go/data"

	"github.com/coinbase/x402/go/mechanisms/multiversx"

	"github.com/coinbase/x402/go/types"
)

// NetworkProvider abstracts the network interactions
type NetworkProvider interface {
	GetNonce(ctx context.Context, address string) (uint64, error)
}

// ProxyNetworkProvider implements NetworkProvider using mx-sdk-go
type ProxyNetworkProvider struct {
	proxyURL string
}

func NewProxyNetworkProvider(url string) *ProxyNetworkProvider {
	return &ProxyNetworkProvider{proxyURL: url}
}

func (p *ProxyNetworkProvider) GetNonce(ctx context.Context, addrStr string) (uint64, error) {
	args := blockchain.ArgsProxy{
		ProxyURL:            p.proxyURL,
		Client:              nil,
		SameScState:         false,
		ShouldBeSynced:      false,
		FinalityCheck:       false,
		EntityType:          core.Proxy,
		CacheExpirationTime: time.Minute,
	}
	proxy, err := blockchain.NewProxy(args)
	if err != nil {
		return 0, err
	}

	address, err := data.NewAddressFromBech32String(addrStr)
	if err != nil {
		return 0, err
	}

	account, err := proxy.GetAccount(ctx, address)
	if err != nil {
		return 0, err
	}
	return account.Nonce, nil
}

// ExactMultiversXScheme implements SchemeNetworkClient
type ExactMultiversXScheme struct {
	signer          multiversx.ClientMultiversXSigner
	networkProvider NetworkProvider
}

// Option defines functional options for ExactMultiversXScheme
type Option func(*ExactMultiversXScheme)

func WithNetworkProvider(np NetworkProvider) Option {
	return func(s *ExactMultiversXScheme) {
		s.networkProvider = np
	}
}

func NewExactMultiversXScheme(signer multiversx.ClientMultiversXSigner, opts ...Option) *ExactMultiversXScheme {
	s := &ExactMultiversXScheme{
		signer: signer,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
	gasLimit := uint64(multiversx.GasLimitStandard)
	gasPrice := uint64(multiversx.GasPriceDefault)

	version := uint32(1)
	chainID := multiversx.ChainIDMainnet
	apiURL := "https://api.multiversx.com"

	if requirements.Network != "" {
		// Parse network identifier cleanly
		parts := strings.Split(string(requirements.Network), ":")
		if len(parts) > 1 {
			chainID = parts[1]
			switch chainID {
			case multiversx.ChainIDDevnet:
				apiURL = "https://devnet-api.multiversx.com"
			case multiversx.ChainIDTestnet:
				apiURL = "https://testnet-api.multiversx.com"
			}
		}
	}

	sender := s.signer.Address()
	receiver := requirements.PayTo
	value := requirements.Amount

	// FETCH NONCE from Network
	var nonce uint64

	// Use injected provider or create default
	provider := s.networkProvider
	if provider == nil {
		provider = NewProxyNetworkProvider(apiURL)
	}

	var err error
	nonce, err = provider.GetNonce(ctx, sender)
	if err != nil {
		return types.PaymentPayload{}, fmt.Errorf("failed to fetch nonce: %w", err)
	}

	// ESDT Logic
	dataString := ""
	asset := requirements.Asset

	if asset != "" && asset != "EGLD" {
		// ESDT Transfer (MultiESDTNFTTransfer)
		receiver = sender
		value = "0"
		gasLimit = uint64(multiversx.GasLimitESDT) // Higher gas for SC call

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

	// Fetch Options from Extra if present
	options := uint32(0)
	if opt, ok := requirements.Extra["options"].(float64); ok {
		options = uint32(opt)
	} else if opt, ok := requirements.Extra["options"].(int); ok {
		options = uint32(opt)
	}

	txData := multiversx.TransactionData{
		Nonce:    nonce,
		Value:    value,
		Receiver: receiver,
		Sender:   sender,
		GasPrice: gasPrice,
		GasLimit: gasLimit,
		Data:     dataString,
		ChainID:  chainID,
		Version:  version,
		Options:  options,
	}

	// 4. Serialize for Signing (Canonical JSON - alphabetical sorting)
	txBytes, err := multiversx.SerializeTransaction(txData)
	if err != nil {
		return types.PaymentPayload{}, err
	}

	// 5. Sign
	sigBytes, err := s.signer.Sign(ctx, txBytes)
	if err != nil {
		return types.PaymentPayload{}, err
	}

	// 6. Build Final Payload Map directly
	finalMap := map[string]interface{}{
		"nonce":       txData.Nonce,
		"value":       txData.Value,
		"receiver":    txData.Receiver,
		"sender":      txData.Sender,
		"gasPrice":    txData.GasPrice,
		"gasLimit":    txData.GasLimit,
		"data":        txData.Data,
		"chainID":     txData.ChainID,
		"version":     txData.Version,
		"signature":   hex.EncodeToString(sigBytes),
		"validAfter":  validAfter,
		"validBefore": validBefore,
		"options":     txData.Options,
	}

	return types.PaymentPayload{
		X402Version: 2,
		Payload:     finalMap,
	}, nil
}
