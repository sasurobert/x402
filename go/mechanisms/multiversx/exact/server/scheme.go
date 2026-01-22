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

func NewExactMultiversXScheme() *ExactMultiversXScheme {
	return &ExactMultiversXScheme{
		moneyParsers: []x402.MoneyParser{},
	}
}

func (s *ExactMultiversXScheme) Scheme() string {
	return multiversx.SchemeExact
}

// RegisterMoneyParser registers a custom money parser
func (s *ExactMultiversXScheme) RegisterMoneyParser(parser x402.MoneyParser) *ExactMultiversXScheme {
	s.moneyParsers = append(s.moneyParsers, parser)
	return s
}

func (s *ExactMultiversXScheme) ParsePrice(price x402.Price, network x402.Network) (x402.AssetAmount, error) {
	// 1. Try casting to AssetAmount struct (already parsed)
	if pStruct, ok := price.(x402.AssetAmount); ok {
		if pStruct.Asset == "" {
			pStruct.Asset = "EGLD"
		}
		return pStruct, nil
	}

	// 2. Try casting to Map (raw pass-through)
	if pMap, okMap := price.(map[string]interface{}); okMap {
		amount, _ := pMap["amount"].(string)
		asset, _ := pMap["asset"].(string)

		if asset == "" {
			asset = "EGLD"
		}

		// If it's a map, we assume it's already formatted/raw or we accept it as is.
		// EVM implementation returns directly here too.
		return x402.AssetAmount{
			Asset:  asset,
			Amount: amount,
		}, nil
	}

	// 3. Parse simple Money (string/float/int) -> Decimal
	decimalAmount, err := s.parseMoneyToDecimal(price)
	if err != nil {
		return x402.AssetAmount{}, err
	}

	// 4. Try custom parsers
	for _, parser := range s.moneyParsers {
		result, err := parser(decimalAmount, network)
		if err != nil {
			continue
		}
		if result != nil {
			return *result, nil
		}
	}

	// 5. Default conversion (to EGLD)
	return s.defaultMoneyConversion(decimalAmount, network)
}

func (s *ExactMultiversXScheme) parseMoneyToDecimal(price x402.Price) (float64, error) {
	switch v := price.(type) {
	case string:
		cleanPrice := strings.TrimSpace(v)
		cleanPrice = strings.TrimPrefix(cleanPrice, "$")
		// Remove typical currency suffixes if any, though usually just number
		// For consistency with EVM, we can strip USD etc if needed, but basic float parse is key.
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

func (s *ExactMultiversXScheme) defaultMoneyConversion(amount float64, network x402.Network) (x402.AssetAmount, error) {
	// Default Asset: EGLD (18 decimals)
	decimals := 18

	// Convert decimal to big int string with 18 decimals
	// value = amount * 10^18

	bigFloat := new(big.Float).SetFloat64(amount)
	multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))

	result := new(big.Float).Mul(bigFloat, multiplier)

	finalInt, _ := result.Int(nil)

	return x402.AssetAmount{
		Asset:  "EGLD",
		Amount: finalInt.String(),
	}, nil
}

func (s *ExactMultiversXScheme) EnhancePaymentRequirements(
	ctx context.Context,
	requirements types.PaymentRequirements,
	supportedKind types.SupportedKind,
	extensions []string,
) (types.PaymentRequirements, error) {
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

	if reqCopy.Asset == "" {
		reqCopy.Asset = "EGLD"
	}

	// We could parse amount here similarly if needed, but Enhance usually assumes valid reqs or prepares them.
	// We'll leave basic enhancement.

	if reqCopy.PayTo == "" {
		return reqCopy, fmt.Errorf("PayTo is required for MultiversX payments")
	}

	return reqCopy, nil
}

// ValidatePaymentRequirements valides requirements strictly
func (s *ExactMultiversXScheme) ValidatePaymentRequirements(requirements x402.PaymentRequirements) error {
	// 1. Check PayTo Address
	if !multiversx.IsValidAddress(requirements.PayTo) {
		return x402.NewPaymentError(x402.ErrCodeInvalidPayment, fmt.Sprintf("invalid PayTo address: %s", requirements.PayTo), nil)
	}

	// 2. Check Amount
	if requirements.Amount == "" {
		return x402.NewPaymentError(x402.ErrCodeInvalidPayment, "amount is required", nil)
	}
	// Check if amount is a valid number (big int string)
	if _, err := multiversx.CheckAmount(requirements.Amount); err != nil {
		return x402.NewPaymentError(x402.ErrCodeInvalidPayment, err.Error(), nil)
	}

	// 3. Check Asset (TokenID)
	// If it's EGLD, it's valid (checked by name/convention)
	// If it's something else, must match TokenID format
	if requirements.Asset != "" && requirements.Asset != "EGLD" {
		if !multiversx.IsValidTokenID(requirements.Asset) {
			return x402.NewPaymentError(x402.ErrCodeInvalidPayment, fmt.Sprintf("invalid asset TokenID: %s", requirements.Asset), nil)
		}
	}

	return nil
}
