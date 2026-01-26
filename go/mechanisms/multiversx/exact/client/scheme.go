package client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-sdk-go/blockchain"
	"github.com/multiversx/mx-sdk-go/core"
	"github.com/multiversx/mx-sdk-go/data"

	"github.com/coinbase/x402/go/mechanisms/multiversx"

	"github.com/coinbase/x402/go/types"
)

// ExactMultiversXScheme implements SchemeNetworkClient
type ExactMultiversXScheme struct {
	signer multiversx.ClientMultiversXSigner
	proxy  blockchain.Proxy
}

// Option defines functional options for ExactMultiversXScheme
type Option func(*ExactMultiversXScheme)

func WithProxy(proxy blockchain.Proxy) Option {
	return func(s *ExactMultiversXScheme) {
		s.proxy = proxy
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
	if requirements.PayTo == "" {
		return types.PaymentPayload{}, fmt.Errorf("PayTo is required")
	}

	if _, _, err := multiversx.DecodeBech32(requirements.PayTo); err != nil {
		return types.PaymentPayload{}, fmt.Errorf("invalid PayTo address (must be valid Bech32): %w", err)
	}

	gasLimit := uint64(multiversx.GasLimitStandard)
	if gl, ok := requirements.Extra["gasLimit"].(float64); ok {
		gasLimit = uint64(gl)
	} else if gl, ok := requirements.Extra["gasLimit"].(int); ok {
		gasLimit = uint64(gl)
	}
	gasPrice := uint64(multiversx.GasPriceDefault)

	version := uint32(1)
	chainID := multiversx.ChainIDMainnet

	if requirements.Network != "" {
		parts := strings.Split(string(requirements.Network), ":")
		if len(parts) > 1 {
			chainID = parts[1]
		}
	}
	apiURL := multiversx.GetAPIURL(chainID)

	sender := s.signer.Address()
	receiver := requirements.PayTo
	value := requirements.Amount
	dataString := ""

	if s.proxy == nil {
		args := blockchain.ArgsProxy{
			ProxyURL:            apiURL,
			Client:              nil,
			SameScState:         false,
			ShouldBeSynced:      false,
			FinalityCheck:       false,
			EntityType:          core.Proxy,
			CacheExpirationTime: time.Minute,
		}
		var err error
		s.proxy, err = blockchain.NewProxy(args)
		if err != nil {
			return types.PaymentPayload{}, fmt.Errorf("failed to init default proxy: %w", err)
		}
	}

	senderAddr, err := data.NewAddressFromBech32String(sender)
	if err != nil {
		return types.PaymentPayload{}, fmt.Errorf("invalid sender address: %w", err)
	}
	account, err := s.proxy.GetAccount(ctx, senderAddr)
	if err != nil {
		return types.PaymentPayload{}, fmt.Errorf("failed to fetch nonce: %w", err)
	}
	nonce := account.Nonce

	asset := requirements.Asset
	if asset == "" {
		return types.PaymentPayload{}, fmt.Errorf("asset is required")
	}

	if asset != multiversx.NativeTokenTicker {
		receiver = sender
		value = "0"
		gasLimit = uint64(multiversx.GasLimitESDT)

		_, decodedBytes, _ := multiversx.DecodeBech32(requirements.PayTo)
		destHex := hex.EncodeToString(decodedBytes)

		tokenHex := hex.EncodeToString([]byte(asset))

		amtBig, ok := new(big.Int).SetString(requirements.Amount, 10)
		if !ok {
			return types.PaymentPayload{}, fmt.Errorf("invalid amount: %s", requirements.Amount)
		}
		amtHex := amtBig.Text(16)
		if len(amtHex)%2 != 0 {
			amtHex = "0" + amtHex
		}

		var resourceIdHex string
		if rid, ok := requirements.Extra["resourceId"].(string); ok && rid != "" {
			resourceIdHex = hex.EncodeToString([]byte(rid))
		}

		var extraArgs []string
		if argsInterface, ok := requirements.Extra["arguments"].([]interface{}); ok {
			for _, arg := range argsInterface {
				if argStr, ok := arg.(string); ok {
					extraArgs = append(extraArgs, argStr)
				}
			}
		}

		baseData := fmt.Sprintf("MultiESDTNFTTransfer@%s@01@%s@00@%s", destHex, tokenHex, amtHex)

		if resourceIdHex != "" {
			baseData += "@" + resourceIdHex
		}

		if len(extraArgs) > 0 {
			baseData += "@" + strings.Join(extraArgs, "@")
		}

		dataString = baseData

	} else {
		if rid, ok := requirements.Extra["resourceId"].(string); ok && rid != "" {
			dataString = hex.EncodeToString([]byte(rid))
		}

		if argsInterface, ok := requirements.Extra["arguments"].([]interface{}); ok {
			var extraArgs []string
			for _, arg := range argsInterface {
				if argStr, ok := arg.(string); ok {
					extraArgs = append(extraArgs, argStr)
				}
			}
			if len(extraArgs) > 0 {
				if dataString == "" {
					dataString = strings.Join(extraArgs, "@")
				} else {
					dataString += "@" + strings.Join(extraArgs, "@")
				}
			}
		}
	}

	now := time.Now().Unix()
	validAfter := uint64(now - 600)
	validBefore := uint64(now + int64(requirements.MaxTimeoutSeconds))

	options := uint32(0)
	if opt, ok := requirements.Extra["options"].(float64); ok {
		options = uint32(opt)
	} else if opt, ok := requirements.Extra["options"].(int); ok {
		options = uint32(opt)
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
		Options:     options,
		ValidAfter:  validAfter,
		ValidBefore: validBefore,
	}

	sdkTx := txData.ToTransaction()
	txBytes, err := s.serializeTxForSigning(sdkTx)
	if err != nil {
		return types.PaymentPayload{}, err
	}

	sigBytes, err := s.signer.Sign(ctx, txBytes)
	if err != nil {
		return types.PaymentPayload{}, err
	}
	txData.Signature = hex.EncodeToString(sigBytes)

	finalMap := txData.ToMap()

	return types.PaymentPayload{
		X402Version: 2,
		Payload:     finalMap,
	}, nil
}

func (s *ExactMultiversXScheme) serializeTxForSigning(tx transaction.FrontendTransaction) ([]byte, error) {
	return json.Marshal(&tx)
}
