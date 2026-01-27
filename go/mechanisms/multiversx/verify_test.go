package multiversx

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/multiversx/mx-sdk-go/data"

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/types"
)

func TestVerifyPayment(t *testing.T) {

	// Generate Keypair
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Create Sender Address
	senderAddr := data.NewAddressFromBytes(pubKey)
	senderBech32, err := senderAddr.AddressAsBech32String()
	if err != nil {
		t.Fatalf("Failed to encode address: %v", err)
	}

	// Setup Payload
	payload := ExactRelayedPayload{}
	payload.Nonce = 1
	payload.Value = "0"
	payload.Receiver = "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"
	payload.Sender = senderBech32
	payload.GasPrice = 1000000000
	payload.GasLimit = 50000
	payload.Data = "test"
	payload.ChainID = "D"
	payload.Version = 1
	payload.Options = 0

	// Sign locally
	tx := payload.ToTransaction()
	txBytes, err := SerializeTransaction(&tx)
	if err != nil {
		t.Fatalf("Failed to serialize tx: %v", err)
	}

	sig := ed25519.Sign(privKey, txBytes)
	payload.Signature = hex.EncodeToString(sig)

	req := types.PaymentRequirements{
		PayTo: "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx",
	}

	// Test success case
	successSim := func(p ExactRelayedPayload) (string, error) {
		return "sim_hash", nil
	}

	valid, err := VerifyPayment(context.Background(), payload, req, successSim)
	if err != nil {
		t.Errorf("VerifyPayment failed: %v", err)
	}
	if !valid {
		t.Error("VerifyPayment returned false")
	}

	// Test Bad Sig
	payload.Signature = hex.EncodeToString(make([]byte, 64)) // invalid
	valid, err = VerifyPayment(context.Background(), payload, req, func(p ExactRelayedPayload) (string, error) {
		return "", errors.New("sim fail")
	})

	if valid {
		t.Error("VerifyPayment should fail for bad sig")
	}

	// Assert Generic Error is NOT nil (for now)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Check type
	// Note: VerifyPayment currently returns generic error, so this assertion will fail until implementation update.
	// But VerifyPayment signature is (bool, error).
	// Facilitator logic wraps it.
	// The CONTRIBUTING guide says "Use typed errors from errors.go".
	// So VerifyPayment itself (a library function) should probably return named errors?
	// Or maybe Facilitator wraps it?
	// VerifyPayment is in `verify.go` (mechanisms/multiversx).
	// It is a low level function.
	// We CAN return x402.VerifyError if we import x402.

	var vErr *x402.VerifyError
	if !errors.As(err, &vErr) {
		t.Errorf("Expected *x402.VerifyError, got %T: %v", err, err)
	} else {
		// Optional: check reason code if we define one
	}
}
