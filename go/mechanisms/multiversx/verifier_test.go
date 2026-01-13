package multiversx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyPayment_EGLD_Success(t *testing.T) {
	// Mock API Response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transactions/txHashEGLD" {
			t.Errorf("Expected path /transactions/txHashEGLD, got %s", r.URL.Path)
		}
		// Return valid JSON
		response := `{
			"hash": "txHashEGLD",
			"receiver": "erd1recipient",
			"value": "1000000000000000000",
			"status": "success",
			"data": "pay@696e766f6963652d313233",
			"logs": { "events": [] }
		}`
		w.Write([]byte(response))
	}))
	defer server.Close()

	verifier := NewVerifier(server.URL)

	// Check
	valid, err := verifier.VerifyPayment("txHashEGLD", "erd1recipient", "invoice-123", "1000000000000000000", "EGLD")
	if !valid || err != nil {
		t.Errorf("Expected valid payment, got valid=%v, err=%v", valid, err)
	}
}

func TestVerifyPayment_ESDT_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return ESDT JSON structure
		response := `{
			"hash": "txHashESDT",
			"receiver": "erd1sender",
			"value": "0",
			"status": "success",
			"data": "MultiESDTNFTTransfer@...", 
			"action": {
				"receiver": "erd1recipient",
				"arguments": {
					"transfers": [
						{ "token": "TOKEN-123456", "tokenIdentifier": "TOKEN-123456", "value": "500" }
					]
				}
			},
			"logs": {
				"events": [
					{ "data": "pay@696e766f6963652d343536" }
				]
			}
		}`
		w.Write([]byte(response))
	}))
	defer server.Close()

	verifier := NewVerifier(server.URL)

	valid, err := verifier.VerifyPayment("txHashESDT", "erd1recipient", "invoice-456", "500", "TOKEN-123456")
	if !valid || err != nil {
		t.Errorf("Expected valid ESDT payment, got valid=%v, err=%v", valid, err)
	}
}

func TestVerifyPayment_ReplayMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"status": "success",
			"receiver": "erd1recipient",
			"value": "1000",
			"data": "pay@wrong_invoice",
			"logs": { "events": [] }
		}`
		w.Write([]byte(response))
	}))
	defer server.Close()

	verifier := NewVerifier(server.URL)

	valid, err := verifier.VerifyPayment("txHashReplay", "erd1recipient", "invoice-123", "1000", "EGLD")
	if valid {
		t.Error("Expected invalid payment due to resource mismatch")
	}
	if err == nil || err.Error() != "resource_id mismatch" {
		t.Errorf("Expected 'resource_id mismatch' error, got %v", err)
	}
}
