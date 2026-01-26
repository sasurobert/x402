package multiversx

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/types"
)

// VerifyPayment verifies the payment payload
func VerifyPayment(ctx context.Context, payload ExactRelayedPayload, requirements types.PaymentRequirements, simulator func(ExactRelayedPayload) (string, error)) (bool, error) {
	// 1. Static Checks
	// Receiver matches PayTo (unless ESDT transfer where internal logic handles it, or Relayer paying gas)
	// Actually, payload.Receiver is who gets the money in Direct transfer.
	// For ESDT, payload.Receiver is Self (Sender).
	// So we can't strictly check payload.Receiver == requirements.PayTo universally without knowing the type.
	// However, the caller (Facilitator) does component-level checks.
	// Here we verify the signature primarily.

	// 2. Signature Presence
	if payload.Signature == "" {
		return false, x402.NewVerifyError(x402.ErrCodeSignatureInvalid, payload.Sender, "multiversx", fmt.Errorf("missing signature"))
	}

	// 3. Local Ed25519 Verification
	tx := payload.ToTransaction()
	// Serialize as canonical JSON for verification
	msgBytes, err := SerializeTransaction(tx)
	if err != nil {
		return false, x402.NewVerifyError("serialization_failed", payload.Sender, "multiversx", err)
	}

	// B. Verify Signature
	// Decode Sender Bech32 -> PubKey
	_, pubKeyBytes, err := DecodeBech32(payload.Sender)
	if err != nil {
		return false, x402.NewVerifyError("invalid_sender_address", payload.Sender, "multiversx", err)
	}

	sigBytes, err := hex.DecodeString(payload.Signature)
	if err != nil {
		return false, x402.NewVerifyError("invalid_signature_hex", payload.Sender, "multiversx", err)
	}

	if len(sigBytes) != 64 {
		return false, x402.NewVerifyError("invalid_signature_length", payload.Sender, "multiversx", fmt.Errorf("expected 64 bytes, got %d", len(sigBytes)))
	}

	if len(pubKeyBytes) != 32 {
		return false, x402.NewVerifyError("invalid_public_key_length", payload.Sender, "multiversx", fmt.Errorf("expected 32 bytes, got %d", len(pubKeyBytes)))
	}

	if ed25519.Verify(pubKeyBytes, msgBytes, sigBytes) {
		// Valid Signature!
		return true, nil
	}

	// 4. Fallback to Simulation
	// If local verify fails, it MIGHT be because our serialization doesn't match the node's
	// or it's a Smart Contract Wallet.
	hash, err := simulator(payload)
	if err != nil {
		// If simulation fails, it's definitely invalid
		// Check error string for signature?
		if strings.Contains(err.Error(), "invalid signature") || strings.Contains(err.Error(), "verification failed") {
			return false, x402.NewVerifyError(x402.ErrCodeSignatureInvalid, payload.Sender, "multiversx", err)
		}
		return false, x402.NewVerifyError("simulation_failed", payload.Sender, "multiversx", err)
	}

	if hash == "" {
		return false, x402.NewVerifyError("simulation_returned_empty_hash", payload.Sender, "multiversx", nil)
	}

	return true, nil
}
