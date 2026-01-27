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

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/mechanisms/multiversx"

	"github.com/coinbase/x402/go/types"
)

// ExactMultiversXScheme implements SchemeNetworkClient
type ExactMultiversXScheme struct {
	signer  multiversx.ClientMultiversXSigner
	network x402.Network
	chainID string
	proxy   blockchain.Proxy
}

// Option defines functional options for ExactMultiversXScheme
type Option func(*ExactMultiversXScheme)

// WithProxy configures a custom blockchain proxy
func WithProxy(proxy blockchain.Proxy) Option {
	return func(s *ExactMultiversXScheme) {
		s.proxy = proxy
	}
}

// NewExactMultiversXScheme creates a new client scheme instance
func NewExactMultiversXScheme(signer multiversx.ClientMultiversXSigner, network x402.Network, opts ...Option) (*ExactMultiversXScheme, error) {
	chainID, err := multiversx.GetMultiversXChainId(string(network))
	if err != nil {
		return nil, err
	}

	s := &ExactMultiversXScheme{
		signer:  signer,
		network: network,
		chainID: chainID,
	}
	for _, opt := range opts {
		opt(s)
	}

	if s.proxy == nil {
		apiURL := multiversx.GetAPIURL(s.chainID)
		args := blockchain.ArgsProxy{
			ProxyURL:            apiURL,
			Client:              nil,
			SameScState:         false,
			ShouldBeSynced:      false,
			FinalityCheck:       false,
			EntityType:          core.Proxy,
			CacheExpirationTime: time.Minute,
		}
		s.proxy, err = blockchain.NewProxy(args)
		if err != nil {
			return nil, fmt.Errorf("failed to init proxy for %s: %w", network, err)
		}
	}

	return s, nil
}

// Scheme returns the scheme identifier
func (s *ExactMultiversXScheme) Scheme() string {
	return multiversx.SchemeExact
}

// CreatePaymentPayload constructs the payment payload for a given requirement
func (s *ExactMultiversXScheme) CreatePaymentPayload(ctx context.Context, requirements types.PaymentRequirements) (types.PaymentPayload, error) {
	if requirements.PayTo == "" {
		return types.PaymentPayload{}, fmt.Errorf("PayTo is required")
	}

	if _, err := data.NewAddressFromBech32String(requirements.PayTo); err != nil {
		return types.PaymentPayload{}, fmt.Errorf("invalid PayTo address (must be valid Bech32): %w", err)
	}

	transferMethod, _ := requirements.Extra["assetTransferMethod"].(string)

	version := uint32(2)
	// If explicitly set to direct, use version 1, otherwise default to version 2 (relayed)
	if transferMethod == multiversx.TransferMethodDirect {
		version = 1
	}

	chainID := s.chainID

	sender := s.signer.Address()
	gasPrice := uint64(multiversx.GasPriceDefault)

	senderAddr, err := data.NewAddressFromBech32String(sender)
	if err != nil {
		return types.PaymentPayload{}, fmt.Errorf("invalid sender address: %w", err)
	}
	account, err := s.proxy.GetAccount(ctx, senderAddr)
	if err != nil {
		return types.PaymentPayload{}, fmt.Errorf("failed to fetch nonce: %w", err)
	}
	nonce := account.Nonce

	// Extract relayer info
	relayer, _ := requirements.Extra["relayer"].(string)

	asset := requirements.Asset
	if asset == "" {
		return types.PaymentPayload{}, fmt.Errorf("asset is required")
	}

	// Construct transaction data and determine value/receiver
	dataString, receiver, value, err := s.constructTransferData(requirements, sender)
	if err != nil {
		return types.PaymentPayload{}, err
	}

	gasLimit := s.calculateGasLimit(requirements, dataString)

	now := time.Now().Unix()
	validAfter := uint64(now - 600)
	validBefore := uint64(now + 600) // Default 10 min buffer
	if requirements.MaxTimeoutSeconds > 0 {
		validBefore = uint64(now + int64(requirements.MaxTimeoutSeconds))
	}

	txData := multiversx.ExactRelayedPayload{
		Nonce:       nonce,
		Value:       value,
		Receiver:    receiver,
		Sender:      sender,
		GasPrice:    gasPrice,
		GasLimit:    gasLimit,
		Data:        dataString,
		ChainID:     chainID,
		Version:     version,
		Options:     0,
		Relayer:     relayer,
		ValidAfter:  validAfter,
		ValidBefore: validBefore,
	}

	// Sign transaction using SDK builder
	cryptoHolder, err := multiversx.NewSimpleCryptoHolderFromBytes(s.signer.PrivateKey())
	if err != nil {
		return types.PaymentPayload{}, fmt.Errorf("failed to create crypto holder: %w", err)
	}

	tx := txData.ToTransaction()
	if err := multiversx.SignTransactionWithBuilder(cryptoHolder, &tx, false); err != nil {
		return types.PaymentPayload{}, fmt.Errorf("failed to sign transaction: %w", err)
	}
	txData.Signature = tx.Signature

	finalMap := txData.ToMap()

	return types.PaymentPayload{
		X402Version: 2,
		Payload:     finalMap,
	}, nil
}

func (s *ExactMultiversXScheme) calculateGasLimit(requirements types.PaymentRequirements, dataString string) uint64 {
	if gl, ok := requirements.Extra["gasLimit"].(uint64); ok {
		return gl
	}

	asset := requirements.Asset

	// Fallback calculation using utils
	// Count number of transfers - currently strictly 1 for this flow
	// Add 10M buffer if SC call (dataString not empty implies potential SC or ESDT transfer)
	// For standard EGLD transfer dataString is empty

	// Base gas limit
	gasLimit := multiversx.CalculateGasLimit([]byte(dataString), 1)

	// Check for SC call indicator (if any extra arguments or SC function passed)
	scFunction, _ := requirements.Extra["scFunction"].(string)
	isScCall := scFunction != "" || (asset != multiversx.NativeTokenTicker)

	if isScCall {
		gasLimit += 10_000_000
	}

	return gasLimit
}

func (s *ExactMultiversXScheme) constructTransferData(requirements types.PaymentRequirements, sender string) (string, string, string, error) {
	asset := requirements.Asset
	scFunction, _ := requirements.Extra["scFunction"].(string)

	var arguments []string
	if argsInterface, ok := requirements.Extra["arguments"].([]string); ok {
		arguments = argsInterface
	}

	if asset != multiversx.NativeTokenTicker {
		// Token Transfer (ESDT)
		receiver := sender
		value := "0"

		payToAddr, _ := data.NewAddressFromBech32String(requirements.PayTo)
		destHex := hex.EncodeToString(payToAddr.AddressBytes())

		tokenHex := hex.EncodeToString([]byte(asset))

		amtBig, ok := new(big.Int).SetString(requirements.Amount, 10)
		if !ok {
			return "", "", "", fmt.Errorf("invalid amount: %s", requirements.Amount)
		}
		amtHex := hex.EncodeToString(amtBig.Bytes())

		parts := []string{
			"MultiESDTNFTTransfer",
			destHex,
			"01",
			tokenHex,
			"00",
			amtHex,
		}

		if scFunction != "" {
			parts = append(parts, hex.EncodeToString([]byte(scFunction)))
			if len(arguments) > 0 {
				parts = append(parts, arguments...)
			}
		}

		return strings.Join(parts, "@"), receiver, value, nil
	}

	// Native EGLD Transfer
	receiver := requirements.PayTo
	value := requirements.Amount

	var parts []string
	if scFunction != "" {
		parts = append(parts, scFunction)
		if len(arguments) > 0 {
			parts = append(parts, arguments...)
		}
	}

	return strings.Join(parts, "@"), receiver, value, nil
}
