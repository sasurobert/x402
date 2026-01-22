package multiversx

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/coinbase/x402/go/types"
)

func TestVerifyPayment(t *testing.T) {
	// 1. Generate Keypair
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// 2. Create Sender Address
	senderBech32, err := EncodeBech32("erd", pubKey)
	if err != nil {
		t.Fatalf("Failed to encode address: %v", err)
	}

	// 3. Setup Payload
	payload := ExactRelayedPayload{}
	payload.Data.Nonce = 1
	payload.Data.Value = "0"
	payload.Data.Receiver = "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"
	payload.Data.Sender = senderBech32
	payload.Data.GasPrice = 1000000000
	payload.Data.GasLimit = 50000
	payload.Data.Data = "test"
	payload.Data.ChainID = "D"
	payload.Data.Version = 1
	payload.Data.Options = 0

	// 4. Sign locally
	// Serialize exact format we expect verification logic to use
	txMap := map[string]interface{}{
		"nonce":    payload.Data.Nonce,
		"value":    payload.Data.Value,
		"receiver": payload.Data.Receiver,
		"sender":   payload.Data.Sender,
		"gasPrice": payload.Data.GasPrice,
		"gasLimit": payload.Data.GasLimit,
		"data":     payload.Data.Data,
		"chainID":  payload.Data.ChainID,
		"version":  payload.Data.Version,
		"options":  payload.Data.Options,
	}

	txBytes, _ := json.Marshal(txMap)

	sig := ed25519.Sign(privKey, txBytes)
	payload.Data.Signature = hex.EncodeToString(sig)

	req := types.PaymentRequirements{
		PayTo: "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx",
	}

	// 5. Test
	// Pass a simulator that fails, so we enforce Local Verification success
	failSim := func(p ExactRelayedPayload) (string, error) {
		return "", errors.New("fallback to sim should not happen if local verifies")
	}

	valid, err := VerifyPayment(context.Background(), payload, req, failSim)
	if err != nil {
		t.Errorf("VerifyPayment failed: %v", err)
	}
	if !valid {
		t.Error("VerifyPayment returned false")
	}

	// Test Bad Sig
	payload.Data.Signature = hex.EncodeToString(make([]byte, 64)) // invalid
	valid, err = VerifyPayment(context.Background(), payload, req, func(p ExactRelayedPayload) (string, error) {
		return "", errors.New("sim fail")
	})
	if valid || err == nil {
		t.Error("Expected failure for bad sig")
	}
}
