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

	gasLimit := uint64(multiversx.GasLimitStandard)
	if gl, ok := requirements.Extra["gasLimit"].(uint64); ok {
		gasLimit = gl
	} else if gl, ok := requirements.Extra["gasLimit"].(float64); ok {
		gasLimit = uint64(gl)
	} else if gl, ok := requirements.Extra["gasLimit"].(int); ok {
		gasLimit = uint64(gl)
	}
	version := uint32(2)
	chainID := s.chainID

	sender := s.signer.Address()
	receiver := requirements.PayTo
	value := requirements.Amount
	dataString := ""
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

	// Extract SC function and arguments early to avoid duplication
	scFunction, _ := requirements.Extra["scFunction"].(string)
	var arguments []string
	if argsInterface, ok := requirements.Extra["arguments"].([]interface{}); ok {
		for _, arg := range argsInterface {
			if argStr, ok := arg.(string); ok {
				arguments = append(arguments, argStr)
			}
		}
	}
	relayer, _ := requirements.Extra["relayer"].(string)
	if rl, ok := requirements.Extra["relayer"].(string); ok {
		relayer = rl
	}

	asset := requirements.Asset
	if asset == "" {
		return types.PaymentPayload{}, fmt.Errorf("asset is required")
	}

	if asset != multiversx.NativeTokenTicker {
		receiver = sender
		value = "0"
		gasLimit = uint64(multiversx.GasLimitESDT)

		payToAddr, _ := data.NewAddressFromBech32String(requirements.PayTo)
		destHex := hex.EncodeToString(payToAddr.AddressBytes())

		tokenHex := hex.EncodeToString([]byte(asset))

		amtBig, ok := new(big.Int).SetString(requirements.Amount, 10)
		if !ok {
			return types.PaymentPayload{}, fmt.Errorf("invalid amount: %s", requirements.Amount)
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
		}

		if len(arguments) > 0 {
			parts = append(parts, arguments...)
		}

		dataString = strings.Join(parts, "@")

	} else {
		parts := []string{}
		if scFunction != "" {
			parts = append(parts, scFunction)
		}
		if len(arguments) > 0 {
			parts = append(parts, arguments...)
		}
		dataString = strings.Join(parts, "@")
	}

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
