package multiversx

import (
	"encoding/hex"
	"testing"
)

func TestProcessRelayedPayment_Success(t *testing.T) {
	// Mock Payload
	payload := RelayedPayload{
		Scheme: "v2-multiversx-exact",
	}
	payload.Data.Sender = "erd1sender"
	payload.Data.Receiver = "erd1recipient"
	payload.Data.Value = "1000"
	payload.Data.Data = "pay@" + hex.EncodeToString([]byte("invoice-123"))
	payload.Data.Signature = "mock_sig"

	verifier := NewVerifier("http://mock-api")

	// Check
	txHash, err := verifier.ProcessRelayedPayment(payload, "erd1recipient", "invoice-123", "1000", "EGLD")
	if err != nil {
		t.Errorf("Expected success, got err=%v", err)
	}
	if txHash != "txHashRelayedPending" {
		t.Errorf("Expected pending hash, got %s", txHash)
	}
}

func TestProcessRelayedPayment_InvalidReceiver(t *testing.T) {
	payload := RelayedPayload{
		Scheme: "v2-multiversx-exact",
	}
	payload.Data.Sender = "erd1sender"
	payload.Data.Receiver = "erd1malicious" // Wrong receiver
	payload.Data.Value = "1000"
	payload.Data.Data = "pay@" + hex.EncodeToString([]byte("invoice-123"))

	verifier := NewVerifier("http://mock-api")

	_, err := verifier.ProcessRelayedPayment(payload, "erd1recipient", "invoice-123", "1000", "EGLD")
	if err == nil || err.Error() != "invalid receiver" {
		t.Errorf("Expected 'invalid receiver' error, got %v", err)
	}
}
