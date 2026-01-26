package server

import (
	"errors"
	"testing"

	x402 "github.com/coinbase/x402/go"
)

func TestValidatePaymentRequirements_ReturnsTypedError(t *testing.T) {
	scheme := NewExactMultiversXScheme()
	req := x402.PaymentRequirements{
		PayTo:  "invalid-address",
		Amount: "100",
		Asset:  "EGLD",
	}

	err := scheme.ValidatePaymentRequirements(req)
	if err == nil {
		t.Fatal("Expected error")
	}

	var payErr *x402.PaymentError
	if !errors.As(err, &payErr) {
		t.Fatalf("Expected error to be of type *x402.PaymentError, got %T: %v", err, err)
	}

	if payErr.Code != x402.ErrCodeInvalidPayment {
		t.Errorf("Expected error code %s, got %s", x402.ErrCodeInvalidPayment, payErr.Code)
	}
}
