package integration_test

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coinbase/x402/go/mechanisms/multiversx"
	"github.com/coinbase/x402/go/mechanisms/multiversx/exact/client"
	"github.com/coinbase/x402/go/mechanisms/multiversx/exact/facilitator"
	"github.com/coinbase/x402/go/types"
)

// Real Test Signer using Alice's Devnet Key
type RealSigner struct {
	privKey ed25519.PrivateKey
	address string
}

func NewRealSigner(privKeyHex string) (*RealSigner, error) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return nil, err
	}
	if len(privKeyBytes) != 32 {
		return nil, fmt.Errorf("invalid keys size: %d (expected 32 bytes)", len(privKeyBytes))
	}
	// Ed25519 private key from seed
	privKey := ed25519.NewKeyFromSeed(privKeyBytes)

	// Derive Address from Public Key
	pubKey := privKey.Public().(ed25519.PublicKey)
	address, err := multiversx.EncodeBech32("erd", pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode address: %v", err)
	}

	return &RealSigner{
		privKey: privKey,
		address: address,
	}, nil
}

func (s *RealSigner) Address() string {
	return s.address
}

func (s *RealSigner) Sign(ctx context.Context, message []byte) ([]byte, error) {
	return ed25519.Sign(s.privKey, message), nil
}

func TestIntegration_AliceFlow(t *testing.T) {
	// 1. Setup Alice Signer (Standard Devnet Alice)
	// Secret Key: 413f42575f7f26fad3317a778771212fdb80245850981e48b58a4f25e344e8f9
	aliceSK := "413f42575f7f26fad3317a778771212fdb80245850981e48b58a4f25e344e8f9"
	signer, err := NewRealSigner(aliceSK)
	if err != nil {
		t.Fatalf("Failed to create Alice signer: %v", err)
	}

	// 2. Setup Client
	ctx := context.Background()
	cScheme := client.NewExactMultiversXScheme(signer)

	// 3. Setup Facilitator (Connected to Devnet)
	devnetURL := "https://devnet-api.multiversx.com"
	fScheme := facilitator.NewExactMultiversXScheme(devnetURL)

	// Fetch Real Nonce for Alice
	provider := client.NewProxyNetworkProvider(devnetURL)
	realNonce, err := provider.GetNonce(ctx, signer.Address())
	if err != nil {
		t.Logf("Failed to fetch real Alice nonce, using fallback 100: %v", err)
		realNonce = 100
	} else {
		t.Logf("Fetched real Alice nonce: %d", realNonce)
	}

	// 4. Requirements (Bob as receiver)
	bobAddr := "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"
	req := types.PaymentRequirements{
		PayTo:             bobAddr,
		Amount:            "1000000000000000000", // 1 EGLD
		Asset:             "EGLD",
		Network:           "multiversx:D",
		MaxTimeoutSeconds: 3600,
		Extra: map[string]interface{}{
			"resourceId": "test-resource-alice",
			"nonce":      realNonce,
		},
	}

	// 5. Create Payload (Client)
	payload, err := cScheme.CreatePaymentPayload(ctx, req)
	if err != nil {
		t.Fatalf("Client failed to create payload: %v", err)
	}

	// 6. Verify Payload (Facilitator)
	// This will hit Devnet API /transaction/simulate
	resp, err := fScheme.Verify(ctx, payload, req)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	} else {
		if !resp.IsValid {
			t.Fatalf("Verification returned invalid (check logs for details)")
		} else {
			t.Log("Verification Successful via Devnet Simulation!")
		}
	}
}

func TestFacilitatorVerify_ESDT_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := multiversx.SimulationResponse{}
		resp.Data.Result.Status = "success"
		resp.Data.Result.Hash = "mock_esdt_hash"
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	scheme := facilitator.NewExactMultiversXScheme(server.URL)

	// Use Real Bech32 Address (Bob) for Strict Verification
	payTo := "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"
	_, pubBytes, err := multiversx.DecodeBech32(payTo)
	if err != nil {
		t.Fatalf("Failed to decode test address: %v", err)
	}
	payToHex := hex.EncodeToString(pubBytes)

	// Token: USDC-123 -> hex ("555344432d313233")
	tokenHex := hex.EncodeToString([]byte("USDC-123"))
	// Amount: 100 -> hex ("64")
	amountHex := "64"

	// Data: "MultiESDTNFTTransfer@<receiver_hex>@01@<token_hex>@00@<amount_hex>"
	// The facilitator expects this exact format.
	dataString := fmt.Sprintf("MultiESDTNFTTransfer@%s@01@%s@00@%s", payToHex, tokenHex, amountHex)

	rp := multiversx.ExactRelayedPayload{}
	rp.Data = dataString
	rp.Value = "0"
	rp.Receiver = payTo // Must match PayTo
	rp.Sender = payTo   // Must be valid Bech32 (using Bob as sender for convenience)
	// Must be valid hex (64 bytes)
	rp.Signature = "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

	payloadBytes, _ := json.Marshal(rp)
	var rpMap map[string]interface{}
	json.Unmarshal(payloadBytes, &rpMap)

	paymentPayload := types.PaymentPayload{
		Payload: rpMap,
	}

	req := types.PaymentRequirements{
		PayTo:  payTo, // Bech32
		Amount: "100",
		Asset:  "USDC-123",
	}

	resp, err := scheme.Verify(context.Background(), paymentPayload, req)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}
	if !resp.IsValid {
		t.Error("IsValid should be true")
	}
}

func TestFacilitatorVerify_EGLD_Alias_MultiESDT(t *testing.T) {
	// Verify that EGLD-000000 via MultiESDT payload is accepted
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := multiversx.SimulationResponse{}
		resp.Data.Result.Status = "success"
		resp.Data.Result.Hash = "mock_egld_alias_hash"
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	scheme := facilitator.NewExactMultiversXScheme(server.URL)

	// PayTo: Bob
	payTo := "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"
	_, pubBytes, _ := multiversx.DecodeBech32(payTo)
	payToHex := hex.EncodeToString(pubBytes)

	// Token: EGLD-000000
	// hex("EGLD-000000") = 45474c442d303030303030
	tokenHex := hex.EncodeToString([]byte("EGLD-000000"))
	amountHex := "64" // 100

	dataString := fmt.Sprintf("MultiESDTNFTTransfer@%s@01@%s@00@%s", payToHex, tokenHex, amountHex)

	rp := multiversx.ExactRelayedPayload{}
	rp.Data = dataString
	rp.Value = "0"
	rp.Receiver = payTo
	rp.Sender = payTo
	rp.Signature = "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

	payloadBytes, _ := json.Marshal(rp)
	var rpMap map[string]interface{}
	json.Unmarshal(payloadBytes, &rpMap)

	paymentPayload := types.PaymentPayload{
		Payload: rpMap,
	}

	req := types.PaymentRequirements{
		PayTo:  payTo, // Bech32
		Amount: "100",
		Asset:  "EGLD-000000",
	}

	resp, err := scheme.Verify(context.Background(), paymentPayload, req)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}
	if !resp.IsValid {
		t.Error("IsValid should be true")
	}
}
