package facilitator_test

import (
	"context"
	"testing"

	"github.com/coinbase/x402/go/mechanisms/multiversx/exact/facilitator"
)

type MockSigner struct {
	addr string
}

func (m *MockSigner) Address() string {
	return m.addr
}
func (m *MockSigner) Sign(ctx context.Context, message []byte) ([]byte, error) {
	return []byte("signature"), nil
}

func TestGetSigners(t *testing.T) {
	// 1. Default (No Signer)
	scheme1 := facilitator.NewExactMultiversXScheme("http://localhost")
	signers1 := scheme1.GetSigners("multiversx:D")
	if len(signers1) != 0 {
		t.Errorf("Expected 0 signers, got %d", len(signers1))
	}

	// 2. With Signer
	mockAddr := "erd1test..."
	signer := &MockSigner{addr: mockAddr}
	scheme2 := facilitator.NewExactMultiversXScheme("http://localhost", facilitator.WithSigner(signer))

	signers2 := scheme2.GetSigners("multiversx:D")
	if len(signers2) != 1 {
		t.Errorf("Expected 1 signer, got %d", len(signers2))
	}
	if signers2[0] != mockAddr {
		t.Errorf("Expected signer %s, got %s", mockAddr, signers2[0])
	}
}
