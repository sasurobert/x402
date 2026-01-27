package server

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/coinbase/x402/go/mechanisms/multiversx"

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/types"
)

// ExactMultiversXScheme implements SchemeNetworkServer for MultiversX
type ExactMultiversXScheme struct {
	moneyParsers []x402.MoneyParser
}

// NewExactMultiversXScheme creates a new server scheme instance
func NewExactMultiversXScheme() *ExactMultiversXScheme {
	return &ExactMultiversXScheme{
		moneyParsers: []x402.MoneyParser{},
	}
}

// Scheme returns the scheme identifier
func (s *ExactMultiversXScheme) Scheme() string {
	return multiversx.SchemeExact
}

// RegisterMoneyParser registers a custom money parser
func (s *ExactMultiversXScheme) RegisterMoneyParser(parser x402.MoneyParser) *ExactMultiversXScheme {
	s.moneyParsers = append(s.moneyParsers, parser)
	return s
}

// ParsePrice converts an x402 generic price to a MultiversX-specific AssetAmount
func (s *ExactMultiversXScheme) ParsePrice(price x402.Price, network x402.Network) (x402.AssetAmount, error) {
	if pStruct, ok := price.(x402.AssetAmount); ok {
		if pStruct.Asset == "" {
			return x402.AssetAmount{}, fmt.Errorf("asset is required")
		}
		return pStruct, nil
	}

	if pMap, okMap := price.(map[string]interface{}); okMap {
		amount, _ := pMap["amount"].(string)
		asset, _ := pMap["asset"].(string)

		if asset == "" {
			return x402.AssetAmount{}, fmt.Errorf("asset is required in price map")
		}

		return x402.AssetAmount{
			Asset:  asset,
			Amount: amount,
		}, nil
	}

	decimalAmount, err := s.parseMoneyToDecimal(price)
	if err != nil {
		return x402.AssetAmount{}, err
	}

	for _, parser := range s.moneyParsers {
		result, err := parser(decimalAmount, network)
		if err != nil {
			continue
		}
		if result != nil {
			return *result, nil
		}
	}

	return s.defaultMoneyConversion(decimalAmount)
}

func (s *ExactMultiversXScheme) parseMoneyToDecimal(price x402.Price) (float64, error) {
	switch v := price.(type) {
	case string:
		cleanPrice := strings.TrimSpace(v)
		cleanPrice = strings.TrimPrefix(cleanPrice, "$")
		cleanPrice = strings.TrimSuffix(cleanPrice, " USD")
		cleanPrice = strings.TrimSuffix(cleanPrice, " USDC")
		cleanPrice = strings.TrimSpace(cleanPrice)
		amount, err := strconv.ParseFloat(cleanPrice, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse price string '%s': %w", v, err)
		}
		return amount, nil
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("unsupported price type: %T", price)
	}
}

func (s *ExactMultiversXScheme) defaultMoneyConversion(amount float64) (x402.AssetAmount, error) {
	decimals := 18

	bigFloat := new(big.Float).SetFloat64(amount)
	multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))

	result := new(big.Float).Mul(bigFloat, multiplier)

	finalInt, _ := result.Int(nil)

	return x402.AssetAmount{
		Asset:  multiversx.NativeTokenTicker,
		Amount: finalInt.String(),
	}, nil
}

// EnhancePaymentRequirements enriches the requirements with MultiversX-specific defaults (gas limit, transfer method)
func (s *ExactMultiversXScheme) EnhancePaymentRequirements(
	ctx context.Context,
	requirements types.PaymentRequirements,
	supportedKind types.SupportedKind,
	extensions []string,
) (types.PaymentRequirements, error) {
	// Perform strict validation
	if err := s.ValidatePaymentRequirements(requirements); err != nil {
		return requirements, err
	}

	reqCopy := requirements
	if reqCopy.Extra != nil {
		newExtra := make(map[string]interface{}, len(reqCopy.Extra))
		for k, v := range reqCopy.Extra {
			newExtra[k] = v
		}
		reqCopy.Extra = newExtra
	} else {
		reqCopy.Extra = make(map[string]interface{})
	}

	if _, ok := reqCopy.Extra["assetTransferMethod"]; !ok {
		if reqCopy.Asset == multiversx.NativeTokenTicker {
			reqCopy.Extra["assetTransferMethod"] = multiversx.TransferMethodDirect
		} else {
			reqCopy.Extra["assetTransferMethod"] = multiversx.TransferMethodESDT
		}
	}

	if _, ok := reqCopy.Extra["gasLimit"]; !ok {
		if reqCopy.Extra["assetTransferMethod"] == multiversx.TransferMethodDirect {
			reqCopy.Extra["gasLimit"] = uint64(multiversx.GasLimitStandard)
		} else {
			reqCopy.Extra["gasLimit"] = uint64(multiversx.GasLimitESDT)
		}
	}

	return reqCopy, nil
}

// ValidatePaymentRequirements validates requirements strictly
func (s *ExactMultiversXScheme) ValidatePaymentRequirements(requirements x402.PaymentRequirements) error {
	if !multiversx.IsValidAddress(requirements.PayTo) {
		return x402.NewPaymentError(x402.ErrCodeInvalidPayment, fmt.Sprintf("invalid PayTo address: %s", requirements.PayTo), nil)
	}

	if requirements.Amount == "" {
		return x402.NewPaymentError(x402.ErrCodeInvalidPayment, "amount is required", nil)
	}
	if _, err := multiversx.CheckAmount(requirements.Amount); err != nil {
		return x402.NewPaymentError(x402.ErrCodeInvalidPayment, err.Error(), nil)
	}

	if requirements.Asset == "" {
		return x402.NewPaymentError(x402.ErrCodeInvalidPayment, "asset is required", nil)
	}

	if requirements.Asset != "EGLD" {
		if !multiversx.IsValidTokenID(requirements.Asset) {
			return x402.NewPaymentError(x402.ErrCodeInvalidPayment, fmt.Sprintf("invalid asset TokenID: %s", requirements.Asset), nil)
		}
	}

	return nil
}
