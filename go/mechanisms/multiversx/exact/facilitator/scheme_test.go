package facilitator

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crypto/ed25519"

	"github.com/coinbase/x402/go/mechanisms/multiversx"
	"github.com/coinbase/x402/go/types"
)

// Helper to create valid signature for tests (using verify test helper or simulation)
// Since Verify calls verifyViaSimulation on failure of local signature (which requires valid Ed25519)
// For unit tests, we can mock the server to return success for simulation if Sig is invalid locally,
// OR we can generate a valid signature locally if we want to test that path.
// verify.go implementation checks signature first. If invalid, it returns error/false.
// WAIT: VerifyPayment in verify.go checks signature.
// If payload.Signature is empty -> Error.
// It verifies against Sender public key.
// So for unit test to pass "IsValid", we need a valid signature matching Sender + Payload.

// We'll use a dummy private key.
// We can use the same helper as integration tests, or just replicate simple signing.

// We need ed25519 import, but we are in facilitator package.
// We can use crypto/ed25519.

func TestVerify_EGLD_Direct_Success(t *testing.T) {
	// Setup Mock Server for Simulation (used if local verify fails or as fallback? verify.go usually returns if local passes)
	// Actually if local verify passes, it returns true.
	// So we don't strictly *need* the mock server if we sign correctly, BUT the facilitator constructor requires a URL.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"result":{"status":"success","hash":"sim_hash"}},"error":""}`))
	}))
	defer server.Close()

	scheme := NewExactMultiversXScheme(server.URL)

	// Keys
	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	senderAddr, _ := multiversx.EncodeBech32("erd", pubKey)

	// Payload
	payload := multiversx.ExactRelayedPayload{
		Nonce:       10,
		Value:       "1000",
		Receiver:    senderAddr, // Sending to self for test
		Sender:      senderAddr,
		GasPrice:    1000000000,
		GasLimit:    50000,
		Data:        "",
		ChainID:     "D",
		Version:     1,
		Options:     0,
		ValidAfter:  uint64(time.Now().Unix() - 100),
		ValidBefore: uint64(time.Now().Unix() + 3600),
	}

	// Sign
	tx := payload.ToTransaction()
	txBytes, _ := multiversx.SerializeTransaction(tx)
	sig := ed25519.Sign(privKey, txBytes)
	payload.Signature = hex.EncodeToString(sig)

	// Wrap
	pBytes, _ := json.Marshal(payload)
	var pMap map[string]interface{}
	json.Unmarshal(pBytes, &pMap)

	// Req
	req := types.PaymentRequirements{
		PayTo:  senderAddr,
		Amount: "1000",
		Asset:  multiversx.NativeTokenTicker,
		Extra: map[string]interface{}{
			"assetTransferMethod": multiversx.TransferMethodDirect,
		},
	}

	resp, err := scheme.Verify(context.Background(), types.PaymentPayload{Payload: pMap}, req)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !resp.IsValid {
		t.Error("Expected valid")
	}
}

func TestVerify_AssetMismatch(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	scheme := NewExactMultiversXScheme(server.URL)

	// Just need requirements validation logic here, signature might fail later but we check mismatch first?
	// The Verify function unmarshals, then calls VerifyPayment (sig check), THEN checks requirements.
	// So we need a valid signature to reach logic checks.

	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	senderAddr, _ := multiversx.EncodeBech32("erd", pubKey)

	payload := multiversx.ExactRelayedPayload{
		Nonce:    1,
		Value:    "1000",
		Receiver: senderAddr,
		Sender:   senderAddr,
		GasPrice: 1000000000,
		GasLimit: 50000,
		ChainID:  "D",
		Version:  1,
	}
	tx := payload.ToTransaction()
	txBytes, _ := multiversx.SerializeTransaction(tx)
	sig := ed25519.Sign(privKey, txBytes)
	payload.Signature = hex.EncodeToString(sig)

	pBytes, _ := json.Marshal(payload)
	var pMap map[string]interface{}
	json.Unmarshal(pBytes, &pMap)

	// Req expects wrong amount
	req := types.PaymentRequirements{
		PayTo:  senderAddr,
		Amount: "2000", // Mismatch
		Asset:  multiversx.NativeTokenTicker,
		Extra: map[string]interface{}{
			"assetTransferMethod": multiversx.TransferMethodDirect,
		},
	}

	resp, err := scheme.Verify(context.Background(), types.PaymentPayload{Payload: pMap}, req)
	// VerifyPayment (sig check) passes.
	// Then logic checks mismatch.
	// It returns valid=false? No, returns error for mismatch usually?
	// Look at implementation: return nil, fmt.Errorf("amount mismatch...")
	if err == nil {
		t.Fatal("Expected mismatch error")
	}
	if resp != nil {
		t.Errorf("Expected nil resp on mismatch error")
	}
}
