package multiversx

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProcessRelayedPayment_Success(t *testing.T) {
	// Mock HTTP Server for Simulation
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validates path
		if r.URL.Path != "/transaction/simulate" {
			t.Errorf("Expected path /transaction/simulate, got %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Return success response
		resp := SimulationResponse{}
		resp.Data.Result.Status = "success"
		resp.Data.Result.Hash = "simulated_hash_123"

		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Mock Payload
	payload := RelayedPayload{
		Scheme: "v2-multiversx-exact",
	}
	payload.Data.Sender = "erd1sender"
	payload.Data.Receiver = "erd1recipient"
	payload.Data.Value = "1000"
	payload.Data.Data = "pay@" + hex.EncodeToString([]byte("invoice-123"))
	payload.Data.Signature = "mock_sig"

	verifier := NewVerifier(mockServer.URL)

	// Check
	txHash, err := verifier.ProcessRelayedPayment(payload, "erd1recipient", "invoice-123", "1000", "EGLD")
	if err != nil {
		t.Errorf("Expected success, got err=%v", err)
	}
	if txHash != "simulated_hash_123" {
		t.Errorf("Expected simulated hash, got %s", txHash)
	}
}

func TestProcessRelayedPayment_SimulationFailure(t *testing.T) {
	// Mock Server returning failure
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SimulationResponse{}
		// Simulate a logic error (e.g. wrong nonce)
		resp.Data.Result.Status = "fail"
		resp.Error = "wrong nonce"

		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	payload := RelayedPayload{
		Scheme: "v2-multiversx-exact",
	}
	payload.Data.Sender = "erd1sender"
	payload.Data.Receiver = "erd1recipient"

	verifier := NewVerifier(mockServer.URL)
	_, err := verifier.ProcessRelayedPayment(payload, "erd1recipient", "invoice-123", "1000", "EGLD")

	if err == nil {
		t.Error("Expected simulation error, got nil")
	}
}

func TestProcessRelayedPayment_InvalidReceiver(t *testing.T) {
	// Even if simulation succeeds, logic check should fail validity
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SimulationResponse{}
		resp.Data.Result.Status = "success"
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	payload := RelayedPayload{
		Scheme: "v2-multiversx-exact",
	}
	payload.Data.Sender = "erd1sender"
	payload.Data.Receiver = "erd1malicious" // Wrong receiver
	payload.Data.Value = "1000"
	payload.Data.Data = "pay@" + hex.EncodeToString([]byte("invoice-123"))

	verifier := NewVerifier(mockServer.URL)

	_, err := verifier.ProcessRelayedPayment(payload, "erd1recipient", "invoice-123", "1000", "EGLD")
	if err == nil || err.Error() != "invalid receiver" {
		t.Errorf("Expected 'invalid receiver' error, got %v", err)
	}
}
